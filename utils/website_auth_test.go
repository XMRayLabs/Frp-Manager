package utils

import (
	"encoding/json"
	"github.com/Sakurame1/frp-manager/pkg/websiteauth"
	"github.com/fatedier/frp/pkg/config"
	v1 "github.com/fatedier/frp/pkg/config/v1"
	"strings"
	"testing"
)

func TestWebsiteAuthConfigRoundTripAndOldClientRejection(t *testing.T) {
	for _, raw := range []string{
		`{"proxies":[{"name":"website","type":"tcp","remotePort":62000,"plugin":{"type":"fm_http_auth","localAddr":"127.0.0.1:8080","accounts":[{"username":"user","password":"secret-pass"}]}}]}`,
		`[[proxies]]
name="website"
type="tcp"
remotePort=62000
[proxies.plugin]
type="fm_http_auth"
localAddr="127.0.0.1:8080"
[[proxies.plugin.accounts]]
username="user"
password="secret-pass"`,
		`proxies:
  - name: website
    type: tcp
    remotePort: 62000
    plugin:
      type: fm_http_auth
      localAddr: 127.0.0.1:8080
      accounts:
        - username: user
          password: secret-pass`,
	} {
		cfg, err := LoadClientConfigNormal([]byte(raw), true)
		if err != nil {
			t.Fatal(err)
		}
		plugin := cfg.Proxies[0].GetBaseConfig().Plugin
		if plugin.Type != websiteauth.PluginType {
			t.Fatal("plugin lost")
		}
		encoded, err := json.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), "secret-pass") {
			t.Fatal("plaintext password persisted")
		}
		reloaded, err := LoadClientConfigNormal(encoded, true)
		if err != nil {
			t.Fatal(err)
		}
		p, err := reloaded.Proxies[0].MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		one, err := LoadProxyConfig(p)
		if err != nil {
			t.Fatal(err)
		}
		if one.GetBaseConfig().Plugin.Type != websiteauth.PluginType {
			t.Fatal("row lost authentication")
		}
		if config.LoadConfigure(encoded, &v1.ClientConfig{}, false) == nil {
			t.Fatal("unmodified/old FRP silently accepted custom plugin")
		}
		_, runtime, _, err := LoadClientConfig(encoded, true)
		if err != nil {
			t.Fatal(err)
		}
		clone := runtime[0].Clone()
		if clone.GetBaseConfig().Plugin.Type != websiteauth.PluginType {
			t.Fatal("runtime clone lost authentication")
		}
	}
}
func TestWebsiteAuthRejectsInvalidProtocolAndFields(t *testing.T) {
	for _, raw := range []string{
		`{"proxies":[{"name":"x","type":"udp","plugin":{"type":"fm_http_auth","localAddr":"127.0.0.1:8080","accounts":[]}}]}`,
		`{"proxies":[{"name":"x","type":"tcp","plugin":{"type":"fm_http_auth","localAddr":"127.0.0.1:8080","accounts":[{"username":"a","password":"secret-pass","extra":"bad"}]}}]}`,
	} {
		if _, err := LoadClientConfigNormal([]byte(raw), true); err == nil {
			t.Fatal("invalid authentication config accepted")
		}
	}
}
