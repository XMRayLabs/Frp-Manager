package platform

import (
	"context"
	"fmt"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/services/rpc"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"testing"
)

func renameFixture(t *testing.T) (*app.Context, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.NodeAlias{}, &models.Client{}, &models.Server{}, &models.ProxyConfig{}, &models.ProxyStats{}, &models.HistoryProxyStats{}); err != nil {
		t.Fatal(err)
	}
	instance := app.NewApp()
	mgr := models.NewDBManager(defs.DBTypeSQLite3)
	mgr.SetDB(defs.DBTypeSQLite3, defs.DBRoleDefault, db)
	instance.SetDBManager(mgr)
	instance.SetClientsManager(rpc.NewClientsManager())
	user := &models.UserEntity{UserID: 1, TenantID: 1, Role: defs.UserRole_Admin}
	ctx := app.NewContext(context.WithValue(context.Background(), defs.UserInfoKey, user), instance)
	for _, row := range []*models.ClientEntity{{ClientID: "old-client", UserID: 1, TenantID: 1, ConnectSecret: "secret", IsShadow: true}, {ClientID: "child", OriginClientID: "old-client", ServerID: "old-server", UserID: 1, TenantID: 1, ConnectSecret: "secret"}} {
		if err = db.Create(&models.Client{ClientEntity: row}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err = db.Create(&models.Server{ServerEntity: &models.ServerEntity{ServerID: "old-server", UserID: 1, TenantID: 1, ConnectSecret: "server-secret", ConfigContent: []byte(`{"bindPort":7000}`)}}).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.ProxyConfig{Model: &gorm.Model{}, ProxyConfigEntity: &models.ProxyConfigEntity{ClientID: "child", OriginClientID: "old-client", ServerID: "old-server", Name: "existing-name", TenantID: 1, UserID: 1}}).Error; err != nil {
		t.Fatal(err)
	}
	return ctx, db
}

func TestRenameNodePreservesIdentityAndReferences(t *testing.T) {
	ctx, db := renameFixture(t)
	for _, ref := range []struct{ table, column string }{{"worker_clients", "client_client_id"}, {"endpoints", "client_id"}, {"proxy_stats", "client_id"}, {"history_proxy_stats", "client_id"}} {
		row := map[string]interface{}{ref.column: "old-client"}
		if ref.table == "worker_clients" {
			row["worker_id"] = "worker"
		}
		if err := db.Table(ref.table).Create(row).Error; err != nil {
			t.Fatal(ref.table, err)
		}
	}
	if err := db.Exec("CREATE TABLE casbin_rule (ptype TEXT, v1 TEXT, v3 TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO casbin_rule VALUES ('p','client:old-client','tenant:1'),('p','client:old-client','tenant:2')").Error; err != nil {
		t.Fatal(err)
	}
	connector := ctx.GetApp().GetClientsManager().Set("old-client", defs.CliTypeClient, &statusTestStream{ctx: context.Background()}, nil)
	for _, pair := range [][2]string{{"old-client", "new-client"}, {"new-client", "final-client"}} {
		if err := renameNode(ctx, "client", pair[0], pair[1]); err != nil {
			t.Fatal(err)
		}
	}
	if ctx.GetApp().GetClientsManager().Get("final-client") != connector || ctx.GetApp().GetClientsManager().Get("old-client") != connector {
		t.Fatal("live stream was replaced")
	}
	for _, id := range []string{"old-client", "new-client", "final-client"} {
		node, err := dao.NewQuery(ctx).ValidateClientSecret(id, "secret")
		if err != nil || node.ClientID != "final-client" {
			t.Fatalf("identity %s: %+v %v", id, node, err)
		}
		if _, err = dao.NewQuery(ctx).ValidateClientSecret(id, "wrong"); err == nil {
			t.Fatal("wrong secret accepted")
		}
	}
	for _, ref := range []struct{ table, column string }{{"worker_clients", "client_client_id"}, {"endpoints", "client_id"}, {"proxy_stats", "client_id"}, {"history_proxy_stats", "client_id"}} {
		var count int64
		if err := db.Table(ref.table).Where(ref.column+" = ?", "final-client").Count(&count).Error; err != nil || count != 1 {
			t.Fatal("association lost", ref.table, err, count)
		}
	}
	var policies int64
	db.Table("casbin_rule").Where("v1 = ? AND v3 = ?", "client:final-client", "tenant:1").Count(&policies)
	if policies != 1 {
		t.Fatal("sharing policy not migrated")
	}
	db.Table("casbin_rule").Where("v1 = ? AND v3 = ?", "client:old-client", "tenant:2").Count(&policies)
	if policies != 1 {
		t.Fatal("other tenant policy modified")
	}
	// New process uses persistent DB aliases rather than manager memory.
	ctx.GetApp().SetClientsManager(rpc.NewClientsManager())
	if node, err := dao.NewQuery(ctx).AdminGetClientByClientID("old-client"); err != nil || node.ClientID != "final-client" {
		t.Fatal("restart lost identity", err)
	}
	if err := renameNode(ctx, "server", "old-server", "new-server"); err != nil {
		t.Fatal(err)
	}
	var child models.Client
	if err := db.Where("client_id = ?", "child").First(&child).Error; err != nil {
		t.Fatal(err)
	}
	if child.OriginClientID != "final-client" || child.ServerID != "new-server" {
		t.Fatalf("child references: %+v", child.ClientEntity)
	}
	var proxy models.ProxyConfig
	db.First(&proxy)
	if proxy.OriginClientID != "final-client" || proxy.ServerID != "new-server" || proxy.Name != "existing-name" {
		t.Fatalf("proxy references: %+v", proxy.ProxyConfigEntity)
	}
	if models.RuntimeNodeID(db, "server", "new-server") != "old-server" {
		t.Fatal("live FRP server key changed")
	}
	if _, err := dao.NewQuery(ctx).ValidateServerSecret("old-server", "server-secret"); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Client{ClientEntity: &models.ClientEntity{ClientID: "old-client", UserID: 2, TenantID: 1}}).Error; err == nil {
		t.Fatal("historical ID was reused")
	}
	if err := db.Where("client_id = ?", "final-client").Delete(&models.Client{}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := dao.NewQuery(ctx).ValidateClientSecret("old-client", "secret"); err == nil {
		t.Fatal("deleted identity accepted")
	}
}

func TestRenameRejectsUnauthorizedAndCollision(t *testing.T) {
	ctx, db := renameFixture(t)
	other := &models.UserEntity{UserID: 2, TenantID: 1, Role: "user"}
	otherCtx := app.NewContext(context.WithValue(context.Background(), defs.UserInfoKey, other), ctx.GetApp())
	if err := renameNode(otherCtx, "client", "old-client", "stolen"); err == nil {
		t.Fatal("non-owner renamed node")
	}
	for _, id := range []string{"old-server", "child", "bad/id", defs.DefaultServerID} {
		if err := renameNode(ctx, "client", "old-client", id); err == nil {
			t.Fatalf("accepted %s", id)
		}
	}
	var count int64
	db.Model(&models.NodeAlias{}).Count(&count)
	if count != 0 {
		t.Fatal("failed rename left aliases")
	}
	// Force an update failure after the replacement row has been created.
	if err := db.Exec("CREATE TRIGGER reject_rename BEFORE UPDATE OF origin_client_id ON clients BEGIN SELECT RAISE(ABORT, 'rollback test'); END").Error; err != nil {
		t.Fatal(err)
	}
	if err := renameNode(ctx, "client", "old-client", "rollback"); err == nil {
		t.Fatal("expected failure")
	}
	db.Table("clients").Where("client_id = ?", "rollback").Count(&count)
	if count != 0 {
		t.Fatal("transaction left replacement row")
	}
	if _, err := dao.NewQuery(ctx).ValidateClientSecret("old-client", "secret"); err != nil {
		t.Fatal("rollback lost original", err)
	}
}
