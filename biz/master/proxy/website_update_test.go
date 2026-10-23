package proxy

import (
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/pkg/websiteauth"
	"github.com/Sakurame1/frp-manager/utils"
	v1 "github.com/fatedier/frp/pkg/config/v1"
	"testing"
)

func TestHTTPUpdatePreservesLocalDestinationAndWebsiteAuthentication(t *testing.T) {
	cfg, err := utils.LoadProxyConfig([]byte(`{"name":"site","type":"http","subdomain":"qwen-admin","localIP":"127.0.0.1","localPort":3000,"annotations":{"frp-manager.website":"true"},"plugin":{"type":"fm_http_auth","localAddr":"127.0.0.1:3000","accounts":[{"username":"alice","password":"first-secret"}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	for _, worker := range []bool{false, true} {
		copy := v1.TypedProxyConfig{ProxyConfigurer: cfg.ProxyConfigurer.Clone()}
		if worker {
			copy.GetBaseConfig().Annotations[defs.FrpProxyAnnotationsKey_Ingress] = "true"
			copy.GetBaseConfig().Annotations[defs.FrpProxyAnnotationsKey_WorkerId] = "worker"
		}
		result := UpdateWorkerLoadBalancerGroup(copy)
		base := result.GetBaseConfig()
		if base.LocalIP != "127.0.0.1" || base.LocalPort != 3000 || base.Plugin.Type != websiteauth.PluginType || base.Annotations["frp-manager.website"] != "true" {
			t.Fatal("HTTP update discarded local backend or authentication")
		}
		if base.Plugin.ClientPluginOptions.(*websiteauth.Options).Accounts[0].PasswordHash == "" {
			t.Fatal("password hash lost")
		}
	}
}
