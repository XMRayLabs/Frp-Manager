package websiteauth

import (
	"bufio"
	"context"
	"encoding/base64"
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

	plugin "github.com/fatedier/frp/pkg/plugin/client"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"
)

func testOptions(t *testing.T, addr string) *Options {
	t.Helper()
	o := &Options{Type: PluginType, LocalAddr: addr, Accounts: []Account{{Username: "alice", Password: "alice-secret"}, {Username: "bob", Password: "bob-secret"}}}
	if err := o.Normalize(); err != nil {
		t.Fatal(err)
	}
	return o
}
func pluginHTTP(t *testing.T, p plugin.Plugin) *http.Client {
	t.Helper()
	transport := &http.Transport{DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		client, server := net.Pipe()
		go p.Handle(ctx, &plugin.ConnectionInfo{Conn: server, UnderlyingConn: server})
		return client, nil
	}}
	t.Cleanup(transport.CloseIdleConnections)
	return &http.Client{Transport: transport, Timeout: 5 * time.Second}
}
func TestAuthenticationAndAccountRevocation(t *testing.T) {
	var hits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Header.Get("Authorization") != "" || r.Header.Get("Proxy-Authorization") != "" {
			t.Error("outer credentials leaked upstream")
		}
		fmt.Fprintf(w, "%s %s %s", r.Host, r.URL.RequestURI(), r.Header.Get("Upgrade"))
	}))
	defer upstream.Close()
	opts := testOptions(t, strings.TrimPrefix(upstream.URL, "http://"))
	raw, _ := json.Marshal(opts)
	if strings.Contains(string(raw), "alice-secret") || strings.Contains(string(raw), "bob-secret") {
		t.Fatal("plaintext persisted")
	}
	pp, err := NewPlugin(plugin.PluginContext{}, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer pp.Close()
	client := pluginHTTP(t, pp)
	for _, tc := range []struct {
		user, pass string
		code       int
	}{{"", "", 401}, {"alice", "wrong", 401}, {"unknown", "bob-secret", 401}, {"alice", "alice-secret", 200}, {"bob", "bob-secret", 200}} {
		req, _ := http.NewRequest("GET", "http://public.example:62000/path?q=1", nil)
		if tc.user != "" {
			req.SetBasicAuth(tc.user, tc.pass)
		}
		req.Header.Set("Proxy-Authorization", "must-not-reach-upstream")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != tc.code {
			t.Fatalf("%s expected %d got %d", tc.user, tc.code, resp.StatusCode)
		}
		if tc.code == 401 && resp.Header.Get("WWW-Authenticate") == "" {
			t.Fatal("missing browser challenge")
		}
		if tc.code == 200 && string(body) != "public.example:62000 /path?q=1 " {
			t.Fatalf("routing changed: %q", body)
		}
	}
	if hits.Load() != 2 {
		t.Fatalf("unauthorized requests reached upstream: %d", hits.Load())
	}
	// A replaced configuration uses a fresh handler and cache, so deleted passwords stop working.
	opts.Accounts = opts.Accounts[1:]
	next := httptest.NewServer(newAuthHandler(opts, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })))
	defer next.Close()
	req, _ := http.NewRequest("GET", next.URL, nil)
	req.SetBasicAuth("alice", "alice-secret")
	resp, err := next.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatal("deleted account accepted")
	}
	req.SetBasicAuth("bob", "bob-secret")
	resp, err = next.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatal("remaining account rejected")
	}
}
func TestWebSocketAndCloseRevokesExistingConnections(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("credential leaked")
		}
		c, e := upgrader.Upgrade(w, r, nil)
		if e != nil {
			return
		}
		defer c.Close()
		for {
			kind, data, e := c.ReadMessage()
			if e != nil {
				return
			}
			if c.WriteMessage(kind, data) != nil {
				return
			}
		}
	}))
	defer upstream.Close()
	opts := testOptions(t, strings.TrimPrefix(upstream.URL, "http://"))
	pp, e := NewPlugin(plugin.PluginContext{}, opts)
	if e != nil {
		t.Fatal(e)
	}
	defer pp.Close()
	dialer := websocket.Dialer{NetDialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		a, b := net.Pipe()
		go pp.Handle(ctx, &plugin.ConnectionInfo{Conn: b, UnderlyingConn: b})
		return a, nil
	}, HandshakeTimeout: 5 * time.Second}
	header := http.Header{"Authorization": []string{"Basic " + base64.StdEncoding.EncodeToString([]byte("alice:alice-secret"))}}
	c, _, e := dialer.Dial("ws://public.example/socket", header)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	if e = c.WriteMessage(websocket.TextMessage, []byte("hello")); e != nil {
		t.Fatal(e)
	}
	_, body, e := c.ReadMessage()
	if e != nil || string(body) != "hello" {
		t.Fatalf("websocket failed: %s %v", body, e)
	}
	if e = pp.Close(); e != nil {
		t.Fatal(e)
	}
	c.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, e = c.ReadMessage(); e == nil {
		t.Fatal("revoked websocket stayed open")
	}
}
func TestAuthenticationEveryRequestAndNoConnectTunnel(t *testing.T) {
	opts := testOptions(t, "127.0.0.1:1")
	pp, e := NewPlugin(plugin.PluginContext{}, opts)
	if e != nil {
		t.Fatal(e)
	}
	defer pp.Close()
	a, b := net.Pipe()
	defer a.Close()
	go pp.Handle(context.Background(), &plugin.ConnectionInfo{Conn: b, UnderlyingConn: b})
	// A CONNECT request can never convert the authenticated gateway to a forward proxy.
	fmt.Fprint(a, "CONNECT arbitrary.example:22 HTTP/1.1\r\nHost: arbitrary.example\r\n\r\n")
	resp, e := http.ReadResponse(bufio.NewReader(a), nil)
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	if resp.StatusCode != 405 {
		t.Fatal(resp.StatusCode)
	}
}
func TestConfigValidationCloneAndVersions(t *testing.T) {
	opts := testOptions(t, "[::1]:8080")
	clone := opts.Clone().(*Options)
	clone.Accounts[0].Username = "other"
	if opts.Accounts[0].Username != "alice" {
		t.Fatal("clone aliases accounts")
	}
	for _, addr := range []string{"https://localhost:8080", "localhost:0", "::1:8080", "localhost:65536", "a/b:80"} {
		o := opts.Clone().(*Options)
		o.LocalAddr = addr
		if o.Normalize() == nil {
			t.Errorf("accepted %q", addr)
		}
	}
	for _, accounts := range [][]Account{nil, {{Username: "a:b", Password: "valid-pass"}}, {{Username: "a", Password: "short"}}, {{Username: "a", PasswordHash: "invalid"}}, {{Username: "a", Password: "first-secret"}, {Username: "a", Password: "second-secret"}}} {
		o := opts.Clone().(*Options)
		o.Accounts = accounts
		if o.Normalize() == nil {
			t.Error("accepted invalid accounts")
		}
	}
	if bcrypt.CompareHashAndPassword([]byte(opts.Accounts[0].PasswordHash), []byte("alice-secret")) != nil {
		t.Fatal("hash mismatch")
	}
	for _, v := range []string{"main", "1.1.5", "1.1.6-test", "", "garbage"} {
		if SupportsVersion(v) {
			t.Errorf("accepted old version %q", v)
		}
	}
	for _, v := range []string{"1.1.6", "v1.1.6", "1.1.10", "1.2.0", "2.0.0"} {
		if !SupportsVersion(v) {
			t.Errorf("rejected version %q", v)
		}
	}
}
