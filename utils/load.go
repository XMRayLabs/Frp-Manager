package utils

import (
	"encoding/json"
	"fmt"

	"github.com/Sakurame1/frp-manager/pkg/websiteauth"
	"github.com/fatedier/frp/pkg/config"
	v1 "github.com/fatedier/frp/pkg/config/v1"
)

func LoadContentWithTemplate(content []byte, values *config.Values) ([]byte, error) {
	return config.RenderWithTemplate(content, values)
}

func LoadConfigureFromContent(content []byte, c any, strict bool) error {
	ans, err := LoadContentWithTemplate(content, config.GetValues())
	if err != nil {
		return err
	}
	clientConfig, ok := c.(*v1.ClientConfig)
	if !ok {
		return config.LoadConfigure(ans, c, strict)
	}
	// Decode the document with the upstream TOML/YAML/JSON parser, then extract
	// our extension before upstream's closed plugin-type decoder runs.
	var document map[string]json.RawMessage
	if err := config.LoadConfigure(ans, &document, strict); err != nil {
		return err
	}
	var proxies []map[string]json.RawMessage
	if raw := document["proxies"]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &proxies); err != nil {
			return err
		}
	}
	extensions := map[int]*websiteauth.Options{}
	for i, proxy := range proxies {
		var plugin struct {
			Type string `json:"type"`
		}
		if len(proxy["plugin"]) == 0 {
			continue
		}
		if err := json.Unmarshal(proxy["plugin"], &plugin); err != nil {
			return err
		}
		if plugin.Type != websiteauth.PluginType {
			continue
		}
		var proxyType string
		_ = json.Unmarshal(proxy["type"], &proxyType)
		if proxyType != "tcp" {
			return fmt.Errorf("网站访问认证仅支持 TCP 网站映射")
		}
		options, err := websiteauth.Decode(proxy["plugin"])
		if err != nil {
			return err
		}
		extensions[i] = options
		delete(proxy, "plugin")
	}
	if len(extensions) == 0 {
		return config.LoadConfigure(ans, c, strict)
	}
	raw, err := json.Marshal(proxies)
	if err != nil {
		return err
	}
	document["proxies"] = raw
	raw, err = json.Marshal(document)
	if err != nil {
		return err
	}
	if err = config.LoadConfigure(raw, clientConfig, strict); err != nil {
		return err
	}
	for i, options := range extensions {
		clientConfig.Proxies[i].GetBaseConfig().Plugin = v1.TypedClientPluginOptions{Type: websiteauth.PluginType, ClientPluginOptions: options}
	}
	return nil
}

func LoadProxiesFromContent(content []byte) ([]v1.TypedProxyConfig, error) {
	allCfg := &v1.ClientConfig{}

	if err := LoadConfigureFromContent(content, allCfg, true); err != nil {
		return nil, err
	}

	return allCfg.Proxies, nil
}

func LoadVisitorsFromContent(content []byte) ([]v1.TypedVisitorConfig, error) {
	allCfg := &v1.ClientConfig{}

	if err := LoadConfigureFromContent(content, allCfg, true); err != nil {
		return nil, err
	}

	return allCfg.Visitors, nil
}

func LoadClientConfigNormal(content []byte, strict bool) (*v1.ClientConfig, error) {
	var (
		cliCfg *v1.ClientCommonConfig
	)

	allCfg := v1.ClientConfig{}
	if err := LoadConfigureFromContent(content, &allCfg, strict); err != nil {
		return nil, err
	}
	cliCfg = &allCfg.ClientCommonConfig
	if err := cliCfg.Complete(); err != nil {
		return nil, err
	}
	allCfg.ClientCommonConfig = *cliCfg
	return &allCfg, nil
}

func LoadClientConfig(content []byte, strict bool) (
	*v1.ClientCommonConfig,
	[]v1.ProxyConfigurer,
	[]v1.VisitorConfigurer,
	error,
) {
	var (
		cliCfg      *v1.ClientCommonConfig
		proxyCfgs   = make([]v1.ProxyConfigurer, 0)
		visitorCfgs = make([]v1.VisitorConfigurer, 0)
	)

	allCfg := v1.ClientConfig{}
	if err := LoadConfigureFromContent(content, &allCfg, strict); err != nil {
		return nil, nil, nil, err
	}
	cliCfg = &allCfg.ClientCommonConfig
	for _, c := range allCfg.Proxies {
		proxyCfgs = append(proxyCfgs, c.ProxyConfigurer)
	}
	for _, c := range allCfg.Visitors {
		visitorCfgs = append(visitorCfgs, c.VisitorConfigurer)
	}

	if err := cliCfg.Complete(); err != nil {
		return nil, nil, nil, err
	}
	proxyCfgs, visitorCfgs = config.FilterClientConfigurers(cliCfg, proxyCfgs, visitorCfgs)
	proxyCfgs = config.CompleteProxyConfigurers(proxyCfgs)
	visitorCfgs = config.CompleteVisitorConfigurers(visitorCfgs)
	return cliCfg, proxyCfgs, visitorCfgs, nil
}

func LoadServerConfig(content []byte, strict bool) (*v1.ServerConfig, error) {
	var (
		svrCfg = &v1.ServerConfig{}
	)
	if err := LoadConfigureFromContent(content, svrCfg, strict); err != nil {
		return nil, err
	}

	if err := svrCfg.Complete(); err != nil {
		return nil, err
	}

	return svrCfg, nil
}

// LoadProxyConfig uses the same extension-aware decoder for persisted tunnel rows.
func LoadProxyConfig(content []byte) (v1.TypedProxyConfig, error) {
	var empty v1.TypedProxyConfig
	envelope := append([]byte(`{"proxies":[`), content...)
	envelope = append(envelope, ']', '}')
	configs, err := LoadProxiesFromContent(envelope)
	if err != nil {
		return empty, err
	}
	if len(configs) != 1 {
		return empty, fmt.Errorf("invalid proxy config")
	}
	return configs[0], nil
}
