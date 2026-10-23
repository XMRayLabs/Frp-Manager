package frpauth

import (
	"encoding/json"
	"github.com/fatedier/frp/pkg/msg"
	"strings"
	"testing"
)

func TestDecodePluginOperations(t *testing.T) {
	cases := []struct {
		body, op string
		port     int32
	}{
		{`{"op":"Login","content":{"user":"alice","metas":{"token":"secret"}}}`, "Login", 0},
		{`{"op":"NewProxy","content":{"user":{"user":"alice","metas":{"token":"secret"}},"proxy_type":"tcp","remote_port":62000}}`, "NewProxy", 62000},
		{`{"op":"Ping","content":{"user":{"user":"alice","metas":{"token":"secret"}}}}`, "Ping", 0},
		{`{"op":"NewWorkConn","content":{"user":{"user":"alice","metas":{"token":"secret"}}}}`, "NewWorkConn", 0},
	}
	for _, c := range cases {
		req, err := Decode(strings.NewReader(c.body))
		if err != nil {
			t.Fatal(err)
		}
		if req.User != "alice" || req.Token != "secret" || req.Operation != c.op || req.RemotePort != c.port || req.Base != nil {
			t.Fatalf("wrong operation decoded: %s", c.op)
		}
	}
	for _, body := range []string{`{}`, `{"op":"Login","content":{"user":"alice"}}`, `{"op":"NewProxy","content":{"user":{"user":"alice","metas":{"token":"secret"}},"remote_port":65536}}`, `{"op":"Other"}`} {
		if _, err := Decode(strings.NewReader(body)); err == nil {
			t.Fatal("invalid request accepted")
		}
	}
}

func TestDecodeWebsiteRoute(t *testing.T) {
	req, err := Decode(strings.NewReader(`{"op":"NewProxy","content":{"user":{"user":"alice","metas":{"token":"secret"}},"proxy_name":"alice.site","proxy_type":"http","subdomain":"qwen-alice","custom_domains":["site.example.com"],"locations":["/api"],"route_by_http_user":"bob","group":"web","group_key":"key","http_user":"viewer","http_pwd":"password"}}`))
	if err != nil {
		t.Fatal(err)
	}
	var route msg.NewProxy
	if err = json.Unmarshal(req.GetNewProxy(), &route); err != nil {
		t.Fatal(err)
	}
	if route.ProxyName != "alice.site" || route.SubDomain != "qwen-alice" || len(route.CustomDomains) != 1 || route.Locations[0] != "/api" || route.RouteByHTTPUser != "bob" || route.Group != "web" || route.GroupKey != "key" || route.HTTPUser != "viewer" || route.HTTPPwd != "password" {
		t.Fatal("lost website routing fields")
	}
}
