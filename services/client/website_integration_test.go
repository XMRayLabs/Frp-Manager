package client

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sakurame1/frp-manager/pkg/websiteauth"
	"github.com/Sakurame1/frp-manager/utils"
	frpclient "github.com/fatedier/frp/client"
	"github.com/fatedier/frp/pkg/config/source"
	v1 "github.com/fatedier/frp/pkg/config/v1"
	"github.com/fatedier/frp/pkg/config/v1/validation"
	"github.com/fatedier/frp/pkg/policy/security"
	"github.com/fatedier/frp/server"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}
func TestWebsiteOverActualFRPTunnelAndCredentialUpdate(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "website-ok") }))
	defer upstream.Close()
	controlPort, publicPort := freePort(t), freePort(t)
	cfg := &v1.ServerConfig{BindAddr: "127.0.0.1", BindPort: controlPort, ProxyBindAddr: "127.0.0.1"}
	if e := cfg.Complete(); e != nil {
		t.Fatal(e)
	}
	svr, e := server.NewService(cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer svr.Close()
	go svr.Run(context.Background())
	raw := []byte(fmt.Sprintf(`{"serverAddr":"127.0.0.1","serverPort":%d,"proxies":[{"name":"website","type":"tcp","remotePort":%d,"plugin":{"type":"fm_http_auth","localAddr":%q,"accounts":[{"username":"alice","password":"alice-secret"},{"username":"bob","password":"bob-secret"}]}}]}`, controlPort, publicPort, strings.TrimPrefix(upstream.URL, "http://")))
	common, proxies, visitors, e := utils.LoadClientConfig(raw, true)
	if e != nil {
		t.Fatal(e)
	}
	unsafe := security.NewUnsafeFeatures(nil)
	if _, e = validation.ValidateAllClientConfig(common, proxies, visitors, unsafe); e != nil {
		t.Fatal(e)
	}
	src := source.NewConfigSource()
	if e = src.ReplaceAll(proxies, visitors); e != nil {
		t.Fatal(e)
	}
	cli, e := frpclient.NewService(frpclient.ServiceOptions{Common: common, ConfigSourceAggregator: source.NewAggregator(src), UnsafeFeatures: unsafe})
	if e != nil {
		t.Fatal(e)
	}
	defer cli.Close()
	go cli.Run(context.Background())
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
	defer client.CloseIdleConnections()
	url := fmt.Sprintf("http://127.0.0.1:%d/", publicPort)
	check := func(user, pass string) (int, string, error) {
		req, _ := http.NewRequest("GET", url, nil)
		if user != "" {
			req.SetBasicAuth(user, pass)
		}
		resp, e := client.Do(req)
		if e != nil {
			return 0, "", e
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(body), nil
	}
	wait := func(user, pass string, want int) {
		t.Helper()
		deadline := time.Now().Add(12 * time.Second)
		var code int
		var body string
		var e error
		for time.Now().Before(deadline) {
			code, body, e = check(user, pass)
			if e == nil && code == want {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatalf("%s wanted %d got %d %q %v", user, want, code, body, e)
	}
	wait("", "", 401)
	wait("alice", "alice-secret", 200)
	wait("bob", "bob-secret", 200)
	options := proxies[0].GetBaseConfig().Plugin.ClientPluginOptions.(*websiteauth.Options)
	replacement := proxies[0].Clone()
	copy := options.Clone().(*websiteauth.Options)
	copy.Accounts = copy.Accounts[1:]
	replacement.GetBaseConfig().Plugin.ClientPluginOptions = copy
	cli.UpdateAllConfigurer([]v1.ProxyConfigurer{replacement}, visitors)
	wait("alice", "alice-secret", 401)
	wait("bob", "bob-secret", 200)
}
