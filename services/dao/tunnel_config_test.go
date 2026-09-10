package dao

import (
	"context"
	"fmt"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestTunnelPortConflictRollsBackAcrossGroups(t *testing.T) { testTunnelPortConflict(t, false) }
func TestPostgresTunnelPortConflict(t *testing.T)              { testTunnelPortConflict(t, true) }
func testTunnelPortConflict(t *testing.T, usePostgres bool) {
	db, err := gorm.Open(sqlite.Open("file:org-ports?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.User{}, &models.Client{}, &models.Server{}, &models.ProxyConfig{}, &models.LanguageGroup{}, &models.LanguageGroupServer{}, &models.OrganizationAudit{}); err != nil {
		t.Fatal(err)
	}
	if usePostgres {
		db = daoPostgresTestDB(t)
	}
	a := app.NewApp()
	mgr := models.NewDBManager(defs.DBTypeSQLite3)
	mgr.SetDB(defs.DBTypeSQLite3, defs.DBRoleDefault, db)
	a.SetDBManager(mgr)
	ctx := app.NewContext(context.Background(), a)
	admin := &models.UserEntity{UserID: 1, UserName: "admin", Email: "a", TenantID: 1, Role: defs.UserRole_Admin, Status: 1}
	member := &models.UserEntity{UserID: 2, UserName: "member", Email: "b", TenantID: 1, Role: defs.UserRole_Normal, LanguageGroupID: "b", Status: 1}
	for _, u := range []*models.UserEntity{admin, member} {
		if err = db.Create(&models.User{UserEntity: u}).Error; err != nil {
			t.Fatal(err)
		}
	}
	db.Create(&models.LanguageGroup{ID: "b", Name: "B", TenantID: 1})
	db.Create(&models.LanguageGroupServer{LanguageGroupID: "b", ServerID: "s"})
	db.Create(&models.Server{ServerEntity: &models.ServerEntity{ServerID: "s", TenantID: 1, UserID: 1}})
	first := &models.ClientEntity{ClientID: "c1", TenantID: 1, UserID: 1, ServerID: "s", ConfigContent: []byte(`{"proxies":[{"name":"one","type":"tcp","remotePort":6000,"localPort":80}]}`)}
	second := &models.ClientEntity{ClientID: "c2", TenantID: 1, UserID: 2, ServerID: "s", ConfigContent: []byte(`{}`)}
	for _, c := range []*models.ClientEntity{first, second} {
		if err = db.Create(&models.Client{ClientEntity: c}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err = SaveTunnelConfig(ctx, admin, first); err != nil {
		t.Fatal(err)
	}
	second.ConfigContent = []byte(`{"proxies":[{"name":"two","type":"tcp","remotePort":6000,"localPort":81}]}`)
	if err = SaveTunnelConfig(ctx, member, second); err == nil {
		t.Fatal("duplicate server port accepted")
	}
	var saved models.Client
	db.Where("client_id = ?", "c2").First(&saved)
	if string(saved.ConfigContent) != "{}" {
		t.Fatal("failed save mutated client")
	}
	second.ConfigContent = []byte(`{"proxies":[{"name":"two","type":"udp","remotePort":6000,"localPort":81}]}`)
	if err = SaveTunnelConfig(ctx, member, second); err != nil {
		t.Fatal(err)
	}
	db.Where("language_group_id = ?", "b").Delete(&models.LanguageGroupServer{})
	if err = SaveTunnelConfig(ctx, member, second); err == nil {
		t.Fatal("revoked server accepted")
	}
}

func daoPostgresTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("FRP_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("FRP_TEST_POSTGRES_DSN not configured")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	name := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err = admin.Exec("CREATE SCHEMA " + name).Error; err != nil {
		t.Fatal(err)
	}
	// RuntimeParams ensure every pooled connection uses the isolated schema.
	db, err := gorm.Open(postgres.Open(dsn+" search_path="+name), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
		admin.Exec("DROP SCHEMA " + name + " CASCADE")
		sqlAdmin, _ := admin.DB()
		sqlAdmin.Close()
	})
	if err = models.MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	return db
}
func TestPostgresConcurrentPortAllocation(t *testing.T) {
	db := daoPostgresTestDB(t)
	a := app.NewApp()
	mgr := models.NewDBManager(defs.DBTypePostgres)
	mgr.SetDB(defs.DBTypePostgres, defs.DBRoleDefault, db)
	a.SetDBManager(mgr)
	ctx := app.NewContext(context.Background(), a)
	admin := &models.UserEntity{UserID: 1, UserName: "admin", Email: "admin@example.com", Role: defs.UserRole_Admin, Status: 1, TenantID: 1}
	if err := db.Create(&models.User{UserEntity: admin}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Server{ServerEntity: &models.ServerEntity{ServerID: "s", TenantID: 1, UserID: 1}}).Error; err != nil {
		t.Fatal(err)
	}
	nodes := []*models.ClientEntity{}
	for i := 0; i < 8; i++ {
		id := fmt.Sprintf("client-%d", i)
		node := &models.ClientEntity{ClientID: id, ServerID: "s", TenantID: 1, UserID: 1, ConfigContent: []byte("{}")}
		if err := db.Create(&models.Client{ClientEntity: node}).Error; err != nil {
			t.Fatal(err)
		}
		node.ConfigContent = []byte(fmt.Sprintf("{\"proxies\":[{\"name\":\"p%d\",\"type\":\"tcp\",\"remotePort\":6000,\"localPort\":80}]}", i))
		nodes = append(nodes, node)
	}
	start := make(chan struct{})
	results := make(chan error, len(nodes))
	var wg sync.WaitGroup
	for _, node := range nodes {
		wg.Add(1)
		go func(n *models.ClientEntity) { defer wg.Done(); <-start; results <- SaveTunnelConfig(ctx, admin, n) }(node)
	}
	close(start)
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !strings.Contains(err.Error(), "端口已被占用") {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatalf("expected one winner, got %d", success)
	}
	var count int64
	if err := db.Model(&models.ProxyConfig{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}
