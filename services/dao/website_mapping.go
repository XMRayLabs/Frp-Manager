package dao

import (
	"encoding/json"
	"fmt"
	"github.com/Sakurame1/frp-manager/utils"
	"github.com/fatedier/frp/pkg/msg"
	"github.com/fatedier/frp/pkg/naming"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/Sakurame1/frp-manager/models"
	v1 "github.com/fatedier/frp/pkg/config/v1"
)

const WebsiteMappingAnnotation = "frp-manager.website"

var dnsLabel = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

func validWebsiteDomain(host string) bool {
	if len(host) > 253 || !strings.Contains(host, ".") || net.ParseIP(host) != nil {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if !dnsLabel.MatchString(label) {
			return false
		}
	}
	return true
}

// The URL is panel metadata, never injected into the strict upstream FRPS config.
// nil is the default: existing HTTP, TCP and proxy configurations are unaffected.
func NormalizeServerWebsite(server *models.ServerEntity) error {
	if server.WebsiteBaseURL == nil {
		return nil
	}
	value := strings.TrimSpace(*server.WebsiteBaseURL)
	if value == "" {
		server.WebsiteBaseURL = nil
		return nil
	}
	u, err := url.Parse(value)
	if err != nil || u.User != nil || u.RawQuery != "" || u.ForceQuery || strings.Contains(value, "#") || u.Fragment != "" || u.Opaque != "" || (u.Path != "" && u.Path != "/") || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("网站入口应为 http(s)://域名后缀，可选端口，不包含路径或凭据")
	}
	host := strings.ToLower(u.Hostname())
	if strings.HasSuffix(u.Host, ":") || !validWebsiteDomain(host) {
		return fmt.Errorf("网站入口需要有效域名，例如 https://frp.xmray.de")
	}
	if port := u.Port(); port != "" {
		var n int
		if _, err = fmt.Sscan(port, &n); err != nil || n < 1 || n > 65535 || fmt.Sprint(n) != port {
			return fmt.Errorf("网站入口端口应为 1-65535")
		}
	}
	cfg, err := server.GetConfigContent()
	if err != nil || cfg.VhostHTTPPort < 1 || cfg.VhostHTTPPort > 65535 {
		return fmt.Errorf("请先设置有效的网站 HTTP 入口端口")
	}
	if cfg.SubDomainHost != host {
		return fmt.Errorf("网站入口域名必须与服务端子域名后缀一致（使用小写，不带末尾点）")
	}
	port := u.Port()
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = host
	if port != "" && !((u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80")) {
		u.Host = net.JoinHostPort(host, port)
	}
	u.Path = ""
	u.RawPath = ""
	if cfg.VhostHTTPPort == cfg.BindPort || cfg.VhostHTTPPort == cfg.VhostHTTPSPort || cfg.VhostHTTPPort == cfg.TCPMuxHTTPConnectPort || (cfg.WebServer.Port > 0 && cfg.VhostHTTPPort == cfg.WebServer.Port) {
		return fmt.Errorf("网站 HTTP 入口端口不能与服务端已有监听端口相同")
	}
	value = u.String()
	server.WebsiteBaseURL = &value
	return nil
}
func validateWebsiteMapping(server *models.Server, cfg v1.TypedProxyConfig) error {
	if cfg.GetBaseConfig().Annotations[WebsiteMappingAnnotation] != "true" {
		return nil
	}
	if server.WebsiteBaseURL == nil || *server.WebsiteBaseURL == "" {
		return fmt.Errorf("此服务端尚未启用网站映射，请联系网站管理员配置网站入口")
	}
	if err := NormalizeServerWebsite(server.ServerEntity); err != nil {
		return err
	}
	httpCfg, ok := cfg.ProxyConfigurer.(*v1.HTTPProxyConfig)
	if !ok || !dnsLabel.MatchString(httpCfg.SubDomain) || len(httpCfg.CustomDomains) != 0 || len(httpCfg.Locations) != 0 || httpCfg.RouteByHTTPUser != "" || httpCfg.LoadBalancer.Group != "" {
		return fmt.Errorf("快捷网站映射必须使用单个子域名与完整网站路径")
	}
	return nil
}

type websiteRoute struct {
	host, protocol, path, user, group, groupKey string
	managed                                     bool
}

func websiteRoutesFromRow(content, serverContent []byte) ([]websiteRoute, error) {
	var p struct {
		Type            string            `json:"type"`
		Subdomain       string            `json:"subdomain"`
		CustomDomains   []string          `json:"customDomains"`
		Locations       []string          `json:"locations"`
		RouteByHTTPUser string            `json:"routeByHTTPUser"`
		Annotations     map[string]string `json:"annotations"`
		LoadBalancer    struct {
			Group    string `json:"group"`
			GroupKey string `json:"groupKey"`
		} `json:"loadBalancer"`
	}
	if err := json.Unmarshal(content, &p); err != nil {
		return nil, err
	}
	if p.Type != "http" && p.Type != "https" {
		return nil, nil
	}
	var s struct {
		SubDomainHost string `json:"subDomainHost"`
	}
	if len(serverContent) > 0 {
		if err := json.Unmarshal(serverContent, &s); err != nil {
			return nil, err
		}
	}
	hosts := append([]string(nil), p.CustomDomains...)
	if p.Subdomain != "" && s.SubDomainHost != "" {
		hosts = append(hosts, p.Subdomain+"."+s.SubDomainHost)
	}
	paths := p.Locations
	if len(paths) == 0 || p.Type == "https" {
		paths = []string{"/"}
	}
	var routes []websiteRoute
	for _, host := range hosts {
		for _, path := range paths {
			if path == "" {
				path = "/"
			}
			routes = append(routes, websiteRoute{host: strings.ToLower(strings.TrimSuffix(host, ".")), protocol: p.Type, path: path, user: p.RouteByHTTPUser, group: p.LoadBalancer.Group, groupKey: p.LoadBalancer.GroupKey, managed: p.Annotations[WebsiteMappingAnnotation] == "true"})
		}
	}
	return routes, nil
}
func websiteHostsOverlap(a, b string) bool {
	if a == b {
		return true
	}
	for _, pair := range [][2]string{{a, b}, {b, a}} {
		if strings.HasPrefix(pair[0], "*.") && strings.HasSuffix(pair[1], pair[0][1:]) {
			return true
		}
	}
	return false
}
func reserveWebsiteRoutes(occupied *[]websiteRoute, content, serverContent []byte) error {
	routes, err := websiteRoutesFromRow(content, serverContent)
	if err != nil {
		return err
	}
	for _, current := range routes {
		for _, old := range *occupied {
			if current.protocol != old.protocol || !websiteHostsOverlap(current.host, old.host) {
				continue
			}
			// Quick websites reserve the entire host; advanced path and user-based routing stay available elsewhere.
			conflict := current.managed || old.managed || (current.path == old.path && current.user == old.user)
			sameGroup := !current.managed && !old.managed && current.host == old.host && current.group != "" && current.group == old.group && current.groupKey == old.groupKey
			if conflict && !sameGroup {
				return fmt.Errorf("网站域名或路由已被占用：%s", current.host)
			}
		}
	}
	*occupied = append(*occupied, routes...)
	return nil
}

func isWebsiteEntryConflict(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "website_base_url") && (strings.Contains(message, "unique") || strings.Contains(message, "duplicate"))
}

// Serialize server entry changes with tunnel creation, using the same server lock.
func saveServerWebsiteConfig(db *gorm.DB, server *models.Server) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Server{}).Where("server_id = ?", server.ServerID).UpdateColumn("server_id", server.ServerID).Error; err != nil {
			return err
		}
		var previous models.Server
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("server_id = ?", server.ServerID).First(&previous).Error; err != nil {
			return err
		}
		var before, after struct {
			SubDomainHost string `json:"subDomainHost"`
		}
		_ = json.Unmarshal(previous.ConfigContent, &before)
		_ = json.Unmarshal(server.ConfigContent, &after)
		if previous.WebsiteBaseURL != nil && (server.WebsiteBaseURL == nil || !strings.EqualFold(before.SubDomainHost, after.SubDomainHost)) {
			var rows []models.ProxyConfig
			if err := tx.Where("server_id = ? AND type = ?", server.ServerID, "http").Find(&rows).Error; err != nil {
				return err
			}
			for _, row := range rows {
				var cfg struct {
					Annotations map[string]string `json:"annotations"`
				}
				if err := json.Unmarshal(row.Content, &cfg); err != nil {
					return err
				}
				if cfg.Annotations[WebsiteMappingAnnotation] == "true" {
					return fmt.Errorf("此服务端仍有网站映射，请先删除或转换这些隧道，再关闭入口或修改域名后缀")
				}
			}
		}
		if err := tx.Save(server).Error; err != nil {
			if isWebsiteEntryConflict(err) {
				return fmt.Errorf("该网站入口已分配给其他服务端，请使用不同的域名后缀或入口端口")
			}
			return err
		}
		return nil
	})
}

// Only enabled website gateways require persisted routes. This also prevents a
// manually configured FRPC with a valid personal token from taking another
// account's domain while its legitimate client is offline.
func AuthorizeWebsiteRegistration(db *gorm.DB, user models.UserInfo, server *models.ServerEntity, proxyType string, payload []byte) error {
	if server.WebsiteBaseURL == nil || (proxyType != "http" && proxyType != "https") {
		return nil
	}
	if len(payload) == 0 {
		return fmt.Errorf("网站入口路由校验需要 1.1.7 或更新服务端内核，请先升级服务端")
	}
	var incoming msg.NewProxy
	if err := json.Unmarshal(payload, &incoming); err != nil || incoming.ProxyType != proxyType || incoming.ProxyName == "" {
		return fmt.Errorf("invalid website registration")
	}
	var rows []models.ProxyConfig
	if err := db.Where("server_id = ? AND user_id = ? AND tenant_id = ? AND type = ? AND stopped = ?", server.ServerID, user.GetUserID(), user.GetTenantID(), proxyType, false).Where("name = ?", naming.StripUserPrefix(user.GetUserName(), incoming.ProxyName)).Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		if incoming.ProxyName != naming.AddUserPrefix(user.GetUserName(), row.Name) {
			continue
		}
		cfg, err := utils.LoadProxyConfig(row.Content)
		if err != nil {
			return fmt.Errorf("invalid persisted website route")
		}
		var expected msg.NewProxy
		cfg.MarshalToMsg(&expected)
		// These fields determine host/path/user routing, load balancing and the
		// native single-account HTTP gate; backend settings stay on the client.
		if incoming.SubDomain == expected.SubDomain && slices.Equal(incoming.CustomDomains, expected.CustomDomains) && slices.Equal(incoming.Locations, expected.Locations) && incoming.RouteByHTTPUser == expected.RouteByHTTPUser && incoming.Group == expected.Group && utils.SecureStringEqual(incoming.GroupKey, expected.GroupKey) && incoming.HTTPUser == expected.HTTPUser && utils.SecureStringEqual(incoming.HTTPPwd, expected.HTTPPwd) {
			return nil
		}
	}
	return fmt.Errorf("网站路由未授权、已停止或与面板配置不一致，请同步隧道配置")
}
