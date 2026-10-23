package dao

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/services/frpauth"
	"github.com/Sakurame1/frp-manager/utils"
	frpclient "github.com/fatedier/frp/client"
	"github.com/fatedier/frp/pkg/config/source"
	v1 "github.com/fatedier/frp/pkg/config/v1"
	"github.com/fatedier/frp/pkg/policy/security"
	frpserver "github.com/fatedier/frp/server"
	"github.com/samber/lo"
)

// Exercise the native plugin payload and actual wire prefix, rather than only
// hand-constructed messages, so a valid website is never rejected by the gate.
func TestActualWebsiteRouteRegistration(t *testing.T) {
	db, _, owner := websiteTestDB(t, false)
	site := &models.ProxyConfig{ProxyConfigEntity: &models.ProxyConfigEntity{ServerID: "s", UserID: 1, TenantID: 1, Name: "site", Type: "http", Content: []byte(`{"name":"site","type":"http","subdomain":"qwen-admin","localIP":"127.0.0.1","localPort":3000}`)}}
	if err := db.Create(site).Error; err != nil {
		t.Fatal(err)
	}
	entity := &models.ServerEntity{ServerID: "s", WebsiteBaseURL: lo.ToPtr("https://frp.example.com")}
	var registrations atomic.Int32
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, err := frpauth.Decode(r.Body)
		if err == nil && req.User != owner.UserName {
			err = fmt.Errorf("unexpected owner")
		}
		if err == nil && req.Operation == "NewProxy" {
			err = AuthorizeWebsiteRegistration(db, owner, entity, req.ProxyType, req.NewProxy)
			if err == nil {
				registrations.Add(1)
			}
		}
		reply := map[string]any{"reject": err != nil, "unchange": true}
		if err != nil {
			reply["reject_reason"] = err.Error()
		}
		json.NewEncoder(w).Encode(reply)
	}))
	defer auth.Close()
	unusedPort := func() int {
		l, e := net.Listen("tcp", "127.0.0.1:0")
		if e != nil {
			t.Fatal(e)
		}
		port := l.Addr().(*net.TCPAddr).Port
		l.Close()
		return port
	}
	control, public := unusedPort(), unusedPort()
	cfg := &v1.ServerConfig{BindAddr: "127.0.0.1", BindPort: control, ProxyBindAddr: "127.0.0.1", SubDomainHost: "frp.example.com", VhostHTTPPort: public, HTTPPlugins: []v1.HTTPPluginOptions{{Name: "route-auth", Addr: strings.TrimPrefix(auth.URL, "http://"), Path: "/", Ops: []string{"Login", "NewProxy"}}}}
	if err := cfg.Complete(); err != nil {
		t.Fatal(err)
	}
	srv, err := frpserver.NewService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	go srv.Run(context.Background())
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "route-ok") }))
	defer upstream.Close()
	raw := []byte(fmt.Sprintf(`{"user":"admin","metadatas":{"token":"test"},"serverAddr":"127.0.0.1","serverPort":%d,"proxies":[{"name":"site","type":"http","subdomain":"qwen-admin","localIP":"127.0.0.1","localPort":%s}]}`, control, strings.Split(strings.TrimPrefix(upstream.URL, "http://"), ":")[1]))
	common, proxies, visitors, err := utils.LoadClientConfig(raw, true)
	if err != nil {
		t.Fatal(err)
	}
	src := source.NewConfigSource()
	if err = src.ReplaceAll(proxies, visitors); err != nil {
		t.Fatal(err)
	}
	cli, err := frpclient.NewService(frpclient.ServiceOptions{Common: common, ConfigSourceAggregator: source.NewAggregator(src), UnsafeFeatures: security.NewUnsafeFeatures(nil)})
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	go cli.Run(context.Background())
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
	defer client.CloseIdleConnections()
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/", public), nil)
		req.Host = "qwen-admin.frp.example.com"
		resp, e := client.Do(req)
		if e == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == 200 && string(body) == "route-ok" && registrations.Load() > 0 {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("native website failed route authorization, successful registrations: %d", registrations.Load())
}
