package dao

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fatedier/frp/pkg/msg"
	"github.com/fatedier/frp/pkg/naming"
	"strings"
	"testing"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/glebarez/sqlite"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

func TestWebsiteEntryIsOptionalAndValidated(t *testing.T) {
	for _, entry := range []string{"", "https://frp.example.com", "https://FRP.EXAMPLE.COM:443/", "http://frp.example.com:8080"} {
		server := &models.ServerEntity{WebsiteBaseURL: lo.ToPtr(entry), ConfigContent: []byte(`{"subDomainHost":"frp.example.com","vhostHTTPPort":9080}`)}
		if err := NormalizeServerWebsite(server); err != nil {
			t.Fatalf("%s: %v", entry, err)
		}
		if entry == "" && server.WebsiteBaseURL != nil {
			t.Fatal("empty is not disabled")
		}
		if strings.Contains(entry, ":443") && *server.WebsiteBaseURL != "https://frp.example.com" {
			t.Fatal("not normalized")
		}
	}
	for _, entry := range []string{"https://frp.example.com/web", "https://u:p@frp.example.com", "https://frp.example.com?", "https://frp.example.com#", "https://other.example.com", "https://127.0.0.1", "file:///frp.example.com", "https://frp.example.com:65536", "https://frp.example.com:"} {
		server := &models.ServerEntity{WebsiteBaseURL: lo.ToPtr(entry), ConfigContent: []byte(`{"subDomainHost":"frp.example.com","vhostHTTPPort":9080}`)}
		if err := NormalizeServerWebsite(server); err == nil {
			t.Fatalf("accepted %q", entry)
		}
	}
}
func websiteTestDB(t *testing.T, pg bool) (*gorm.DB, *app.Context, *models.UserEntity) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := db.DB()
	sql.SetMaxOpenConns(1)
	t.Cleanup(func() { sql.Close() })
	if err = db.AutoMigrate(&models.User{}, &models.Server{}, &models.Client{}, &models.ProxyConfig{}, &models.LanguageGroup{}, &models.LanguageGroupServer{}, &models.OrganizationAudit{}); err != nil {
		t.Fatal(err)
	}
	if pg {
		db = daoPostgresTestDB(t)
	}
	a := app.NewApp()
	mgr := models.NewDBManager(defs.DBTypeSQLite3)
	mgr.SetDB(defs.DBTypeSQLite3, defs.DBRoleDefault, db)
	a.SetDBManager(mgr)
	admin := &models.UserEntity{UserID: 1, TenantID: 1, UserName: "admin", Email: "admin", Role: defs.UserRole_Admin, Status: 1}
	if err = db.Create(&models.User{UserEntity: admin}).Error; err != nil {
		t.Fatal(err)
	}
	return db, app.NewContext(context.Background(), a), admin
}
func TestWebsiteMappingReservation(t *testing.T)         { testWebsiteMappingReservation(t, false) }
func TestPostgresWebsiteMappingReservation(t *testing.T) { testWebsiteMappingReservation(t, true) }
func testWebsiteMappingReservation(t *testing.T, pg bool) {
	db, ctx, admin := websiteTestDB(t, pg)
	server := &models.Server{ServerEntity: &models.ServerEntity{ServerID: "s", TenantID: 1, UserID: 1, ConfigContent: []byte(`{"bindPort":7000,"subDomainHost":"frp.example.com","vhostHTTPPort":9080}`)}}
	if err := db.Create(server).Error; err != nil {
		t.Fatal(err)
	}
	makeNode := func(id string) *models.ClientEntity {
		n := &models.ClientEntity{ClientID: id, TenantID: 1, UserID: 1, ServerID: "s", ConfigContent: []byte(`{}`)}
		if err := db.Create(&models.Client{ClientEntity: n}).Error; err != nil {
			t.Fatal(err)
		}
		return n
	}
	one, two := makeNode("one"), makeNode("two")
	website := `{"proxies":[{"name":"site","type":"http","subdomain":"qwen-admin","localIP":"127.0.0.1","localPort":3000,"annotations":{"frp-manager.website":"true"}}]}`
	one.ConfigContent = []byte(website)
	if err := SaveTunnelConfig(ctx, admin, one); err == nil {
		t.Fatal("disabled server accepted quick website")
	}
	server.WebsiteBaseURL = lo.ToPtr("https://frp.example.com")
	if err := NewMutation(ctx).UpdateServer(admin, server.ServerEntity); err != nil {
		t.Fatal(err)
	}
	if err := SaveTunnelConfig(ctx, admin, one); err != nil {
		t.Fatal(err)
	}
	// Pausing is not a release of the domain, including advanced path routes.
	if err := db.Model(&models.ProxyConfig{}).Where("client_id = ?", "one").Update("stopped", true).Error; err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{website, `{"proxies":[{"name":"other","type":"http","subdomain":"QWEN-ADMIN","locations":["/api"],"localPort":80}]}`, `{"proxies":[{"name":"other","type":"http","customDomains":["*.frp.example.com"],"localPort":80}]}`} {
		two.ConfigContent = []byte(raw)
		if err := SaveTunnelConfig(ctx, admin, two); err == nil || !strings.Contains(err.Error(), "占用") {
			t.Fatalf("domain collision not blocked: %v", err)
		}
	}
	var saved models.Client
	if err := db.Where("client_id = ?", "two").First(&saved).Error; err != nil || string(saved.ConfigContent) != "{}" {
		t.Fatal("rejected write changed client")
	}
	two.ConfigContent = []byte(strings.ReplaceAll(website, "qwen-admin", "other-admin"))
	if err := SaveTunnelConfig(ctx, admin, two); err != nil {
		t.Fatal(err)
	}
	server.WebsiteBaseURL = nil
	if err := NewMutation(ctx).UpdateServer(admin, server.ServerEntity); err == nil {
		t.Fatal("disabled live domain entry")
	}
	var kept models.Server
	if err := db.Where("server_id = ?", "s").First(&kept).Error; err != nil || kept.WebsiteBaseURL == nil {
		t.Fatal("failed disable changed entry")
	}
	// Website entry is optional per server; two disabled servers can coexist.
	for i := 0; i < 2; i++ {
		if err := db.Create(&models.Server{ServerEntity: &models.ServerEntity{ServerID: fmt.Sprintf("off%d", i), TenantID: 1, UserID: 1}}).Error; err != nil {
			t.Fatal(err)
		}
	}
	// Same address cannot silently target two independently routed FRPS servers.
	duplicate := &models.ServerEntity{ServerID: "off0", TenantID: 1, UserID: 1, WebsiteBaseURL: lo.ToPtr("https://frp.example.com:443/"), ConfigContent: kept.ConfigContent}
	if err := NewMutation(ctx).UpdateServer(admin, duplicate); err == nil || !strings.Contains(err.Error(), "其他服务端") {
		t.Fatalf("duplicate entry: %v", err)
	}
	// Existing port grants still apply to the shared HTTP entry.
	if err := models.ValidateProxyAllowedPorts("62000-63999", []byte(`{"type":"http"}`), kept.ConfigContent); err == nil {
		t.Fatal("website bypassed port authorization")
	}
	// Removing both sites releases the entry, so disabling is safe.
	if err := db.Unscoped().Where("server_id = ?", "s").Delete(&models.ProxyConfig{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := NewMutation(ctx).UpdateServer(admin, server.ServerEntity); err != nil {
		t.Fatal(err)
	}
}
func TestAdvancedWebsiteRoutesRemainAvailable(t *testing.T) {
	occupied := []websiteRoute{}
	server := []byte(`{"subDomainHost":"frp.example.com"}`)
	for _, path := range []string{"/news", "/about"} {
		raw := []byte(fmt.Sprintf(`{"type":"http","subdomain":"legacy","locations":[%q]}`, path))
		if err := reserveWebsiteRoutes(&occupied, raw, server); err != nil {
			t.Fatal(err)
		}
	}
	if err := reserveWebsiteRoutes(&occupied, []byte(`{"type":"http","subdomain":"legacy","locations":["/news"]}`), server); err == nil {
		t.Fatal("identical route accepted")
	}
}

func TestWebsiteRegistrationAuthorization(t *testing.T) {
	testWebsiteRegistrationAuthorization(t, false)
}
func TestPostgresWebsiteRegistrationAuthorization(t *testing.T) {
	testWebsiteRegistrationAuthorization(t, true)
}
func testWebsiteRegistrationAuthorization(t *testing.T, pg bool) {
	db, _, user := websiteTestDB(t, pg)
	server := &models.ServerEntity{ServerID: "s", WebsiteBaseURL: lo.ToPtr("https://frp.example.com")}
	row := &models.ProxyConfig{ProxyConfigEntity: &models.ProxyConfigEntity{ServerID: "s", ClientID: "one", UserID: 1, TenantID: 1, Name: "site", Type: "http", Content: []byte(`{"name":"site","type":"http","subdomain":"qwen-admin","locations":["/"],"localPort":3000}`)}}
	if err := db.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	incoming := msg.NewProxy{ProxyName: naming.AddUserPrefix(user.GetUserName(), "site"), ProxyType: "http", SubDomain: "qwen-admin", Locations: []string{"/"}}
	authorize := func(p msg.NewProxy) error {
		raw, _ := json.Marshal(p)
		return AuthorizeWebsiteRegistration(db, user, server, "http", raw)
	}
	if err := authorize(incoming); err != nil {
		t.Fatalf("legitimate FRP wire name: %v", err)
	}
	changed := incoming
	changed.SubDomain = "victim"
	if authorize(changed) == nil {
		t.Fatal("arbitrary domain accepted")
	}
	changed = incoming
	changed.CustomDomains = []string{"victim.frp.example.com"}
	if authorize(changed) == nil {
		t.Fatal("extra domain accepted")
	}
	changed = incoming
	changed.Locations = nil
	if authorize(changed) == nil {
		t.Fatal("expanded path accepted")
	}
	changed = incoming
	changed.Group = "hijack"
	if authorize(changed) == nil {
		t.Fatal("changed load balancer accepted")
	}
	changed = incoming
	changed.ProxyName = "site"
	if authorize(changed) == nil {
		t.Fatal("missing user prefix accepted")
	}
	other := *user
	other.UserID = 2
	other.UserName = "other"
	raw, _ := json.Marshal(incoming)
	if AuthorizeWebsiteRegistration(db, &other, server, "http", raw) == nil {
		t.Fatal("another account accepted")
	}
	if err := db.Model(row).Update("stopped", true).Error; err != nil {
		t.Fatal(err)
	}
	if authorize(incoming) == nil {
		t.Fatal("stopped route accepted")
	}
	if AuthorizeWebsiteRegistration(db, user, server, "http", nil) == nil {
		t.Fatal("old server silently bypassed checks")
	}
	if err := AuthorizeWebsiteRegistration(db, user, server, "tcp", nil); err != nil {
		t.Fatal(err)
	}
	server.WebsiteBaseURL = nil
	if err := AuthorizeWebsiteRegistration(db, user, server, "http", nil); err != nil {
		t.Fatal("optional disabled entry changed existing authentication")
	}
}
