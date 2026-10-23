package dao

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/pkg/websiteauth"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/rpc"
	"github.com/Sakurame1/frp-manager/utils"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestWebsiteAuthenticationSaveRequiresNewClientAndHashesPasswords(t *testing.T) {
	testWebsiteAuthenticationSave(t, false)
}
func TestPostgresWebsiteAuthenticationSave(t *testing.T) { testWebsiteAuthenticationSave(t, true) }
func testWebsiteAuthenticationSave(t *testing.T, usePostgres bool) {
	db, e := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if e != nil {
		t.Fatal(e)
	}
	sql, _ := db.DB()
	sql.SetMaxOpenConns(1)
	defer sql.Close()
	if e = db.AutoMigrate(&models.User{}, &models.Client{}, &models.Server{}, &models.ProxyConfig{}, &models.LanguageGroup{}, &models.LanguageGroupServer{}, &models.OrganizationAudit{}); e != nil {
		t.Fatal(e)
	}
	if usePostgres {
		db = daoPostgresTestDB(t)
	}
	admin := &models.UserEntity{UserID: 1, UserName: "admin", Email: "admin", TenantID: 1, Role: defs.UserRole_Admin, Status: 1}
	if e = db.Create(&models.User{UserEntity: admin}).Error; e != nil {
		t.Fatal(e)
	}
	if e = db.Create(&models.Server{ServerEntity: &models.ServerEntity{ServerID: "s", TenantID: 1, UserID: 1}}).Error; e != nil {
		t.Fatal(e)
	}
	node := &models.ClientEntity{ClientID: "child", OriginClientID: "physical", TenantID: 1, UserID: 1, ServerID: "s", ConfigContent: []byte(`{}`)}
	if e = db.Create(&models.Client{ClientEntity: node}).Error; e != nil {
		t.Fatal(e)
	}
	a := app.NewApp()
	mgr := models.NewDBManager(defs.DBTypeSQLite3)
	mgr.SetDB(defs.DBTypeSQLite3, defs.DBRoleDefault, db)
	a.SetDBManager(mgr)
	connections := rpc.NewClientsManager()
	a.SetClientsManager(connections)
	ctx := app.NewContext(context.Background(), a)
	body := []byte(`{"proxies":[{"name":"website","type":"tcp","remotePort":62000,"plugin":{"type":"fm_http_auth","localAddr":"127.0.0.1:8080","accounts":[{"username":"alice","password":"alice-secret"},{"username":"bob","password":"bob-secret"}]}}]}`)
	for _, version := range []string{"", "1.1.5", "main", "1.1.6"} {
		if version != "" {
			connections.Set("physical", defs.CliTypeClient, nil, &pb.ClientVersion{GitVersion: version})
		}
		node.ConfigContent = append([]byte(nil), body...)
		e = SaveTunnelConfig(ctx, admin, node)
		if version != "1.1.6" {
			if e == nil {
				t.Fatalf("version %q accepted", version)
			}
			var old models.Client
			db.Where("client_id = ?", "child").First(&old)
			if string(old.ConfigContent) != "{}" {
				t.Fatal("rejected save changed config")
			}
		} else if e != nil {
			t.Fatal(e)
		}
	}
	var saved models.Client
	if e = db.Where("client_id = ?", "child").First(&saved).Error; e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(saved.ConfigContent), "alice-secret") || strings.Contains(string(saved.ConfigContent), "bob-secret") {
		t.Fatal("plaintext in database")
	}
	var row models.ProxyConfig
	if e = db.First(&row).Error; e != nil {
		t.Fatal(e)
	}
	config, e := row.GetTypedProxyConfig()
	if e != nil {
		t.Fatal(e)
	}
	options := config.GetBaseConfig().Plugin.ClientPluginOptions.(*websiteauth.Options)
	if len(options.Accounts) != 2 {
		t.Fatal("accounts lost")
	}
	options.Accounts = options.Accounts[1:]
	raw, e := config.MarshalJSON()
	if e != nil {
		t.Fatal(e)
	}
	node.ConfigContent = []byte(fmt.Sprintf(`{"proxies":[%s]}`, raw))
	if e = SaveTunnelConfig(ctx, admin, node); e != nil {
		t.Fatal(e)
	}
	if e = db.First(&row).Error; e != nil {
		t.Fatal(e)
	}
	config, e = row.GetTypedProxyConfig()
	if e != nil {
		t.Fatal(e)
	}
	options = config.GetBaseConfig().Plugin.ClientPluginOptions.(*websiteauth.Options)
	if len(options.Accounts) != 1 || options.Accounts[0].Username != "bob" {
		t.Fatal("account deletion not saved")
	}
	// Turning authentication off preserves the original TCP destination and removes hashes.
	node.ConfigContent = []byte(`{"proxies":[{"name":"website","type":"tcp","remotePort":62000,"localIP":"127.0.0.1","localPort":8080}]}`)
	if e = SaveTunnelConfig(ctx, admin, node); e != nil {
		t.Fatal(e)
	}
	if e = db.First(&row).Error; e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(row.Content), "passwordHash") {
		t.Fatal("disabled auth retained hashes")
	}
	var data map[string]interface{}
	_ = json.Unmarshal(row.Content, &data)
	if data["localPort"] != float64(8080) {
		t.Fatal("target changed")
	}
	if _, e = utils.LoadProxyConfig(row.Content); e != nil {
		t.Fatal(e)
	}
}
