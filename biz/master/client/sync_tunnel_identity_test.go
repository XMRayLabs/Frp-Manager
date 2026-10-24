package client

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestSyncTunnelOnlyChangesAccountOwnerAndPreservesMetadata(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := db.DB()
	sql.SetMaxOpenConns(1)
	defer sql.Close()
	if err := db.AutoMigrate(&models.Client{}, &models.User{}); err != nil {
		t.Fatal(err)
	}
	admin := &models.UserEntity{UserID: 1, TenantID: 1, UserName: "admin", Role: defs.UserRole_Admin, Token: "updated-token", Status: models.STATUS_NORMAL}
	original := []byte(`{"user":"alice","metadatas":{"custom":"keep","frp-panel-client-id":"old-id"},"proxies":[{"name":"alice.home.default.60001.tcp","type":"tcp","remotePort":60001}]}`)
	for _, r := range []struct {
		id    string
		owner int
		cfg   []byte
	}{{"admin.c.home@1", 1, original}, {"alice.c.home@1", 2, original}, {"empty", 1, nil}} {
		if err := db.Create(&models.Client{ClientEntity: &models.ClientEntity{ClientID: r.id, UserID: r.owner, TenantID: 1, ConfigContent: r.cfg}}).Error; err != nil {
			t.Fatal(err)
		}
	}
	instance := app.NewApp()
	mgr := models.NewDBManager(defs.DBTypeSQLite3)
	mgr.SetDB(defs.DBTypeSQLite3, defs.DBRoleDefault, db)
	instance.SetDBManager(mgr)
	ctx := app.NewContext(context.Background(), instance)
	if err := SyncTunnel(ctx, admin); err != nil {
		t.Fatal(err)
	}
	var own, other models.Client
	db.Where("client_id = ?", "admin.c.home@1").First(&own)
	db.Where("client_id = ?", "alice.c.home@1").First(&other)
	if string(other.ConfigContent) != string(original) {
		t.Fatal("updating admin overwrote another owner's tunnel")
	}
	var cfg struct {
		User      string
		Metadatas map[string]string
	}
	if err := json.Unmarshal(own.ConfigContent, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.User != "admin" || cfg.Metadatas[defs.FRPAuthTokenKey] != "updated-token" || cfg.Metadatas[defs.FRPClientIDKey] != own.ClientID || cfg.Metadatas["custom"] != "keep" {
		t.Fatalf("credential or metadata mismatch: %+v", cfg)
	}
}
