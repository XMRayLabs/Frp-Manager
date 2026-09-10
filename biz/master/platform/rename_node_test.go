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
	if err = db.AutoMigrate(&models.NodeAlias{}, &models.User{}, &models.Client{}, &models.Server{}, &models.ProxyConfig{}, &models.ProxyStats{}, &models.HistoryProxyStats{}, &models.Endpoint{}); err != nil {
		t.Fatal(err)
	}
	instance := app.NewApp()
	mgr := models.NewDBManager(defs.DBTypeSQLite3)
	mgr.SetDB(defs.DBTypeSQLite3, defs.DBRoleDefault, db)
	instance.SetDBManager(mgr)
	instance.SetClientsManager(rpc.NewClientsManager())
	user := &models.UserEntity{UserID: 1, TenantID: 1, UserName: "admin", Role: defs.UserRole_Admin}
	if err := db.Create(&models.User{UserEntity: user}).Error; err != nil {
		t.Fatal(err)
	}
	ctx := app.NewContext(context.WithValue(context.Background(), defs.UserInfoKey, user), instance)
	for _, row := range []*models.ClientEntity{{ClientID: "admin.c.old", UserID: 1, TenantID: 1, ConnectSecret: "secret", IsShadow: true}, {ClientID: "child", OriginClientID: "admin.c.old", ServerID: "admin.s.old", UserID: 1, TenantID: 1, ConnectSecret: "secret"}} {
		if err = db.Create(&models.Client{ClientEntity: row}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err = db.Create(&models.Server{ServerEntity: &models.ServerEntity{ServerID: "admin.s.old", UserID: 1, TenantID: 1, ConnectSecret: "server-secret", RuntimeID: "admin.s.old", ConfigContent: []byte(`{"bindPort":7000}`)}}).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Create(&models.ProxyConfig{Model: &gorm.Model{}, ProxyConfigEntity: &models.ProxyConfigEntity{ClientID: "child", OriginClientID: "admin.c.old", ServerID: "admin.s.old", Name: "existing-name", TenantID: 1, UserID: 1}}).Error; err != nil {
		t.Fatal(err)
	}
	return ctx, db
}

func TestRenameNodePreservesIdentityAndReferences(t *testing.T) {
	ctx, db := renameFixture(t)
	for _, ref := range []struct{ table, column string }{{"worker_clients", "client_client_id"}, {"endpoints", "client_id"}, {"proxy_stats", "client_id"}, {"history_proxy_stats", "client_id"}} {
		row := map[string]interface{}{ref.column: "admin.c.old"}
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
	if err := db.Exec("INSERT INTO casbin_rule VALUES ('p','client:admin.c.old','tenant:1'),('p','client:admin.c.old','tenant:2')").Error; err != nil {
		t.Fatal(err)
	}
	connector := ctx.GetApp().GetClientsManager().Set("admin.c.old", defs.CliTypeClient, &statusTestStream{ctx: context.Background()}, nil)
	for _, pair := range [][2]string{{"admin.c.old", "admin.c.new"}, {"admin.c.new", "admin.c.final"}} {
		if err := renameNode(ctx, "client", pair[0], pair[1]); err != nil {
			t.Fatal(err)
		}
	}
	if ctx.GetApp().GetClientsManager().Get("admin.c.final") != connector || ctx.GetApp().GetClientsManager().Get("admin.c.old") != nil {
		t.Fatal("live stream was replaced")
	}
	for _, id := range []string{"admin.c.old", "admin.c.new", "admin.c.final"} {
		node, err := dao.NewQuery(ctx).ValidateClientSecret(id, "secret")
		if err != nil || node.ClientID != "admin.c.final" {
			t.Fatalf("identity %s: %+v %v", id, node, err)
		}
		if _, err = dao.NewQuery(ctx).ValidateClientSecret(id, "wrong"); err == nil {
			t.Fatal("wrong secret accepted")
		}
	}
	for _, ref := range []struct{ table, column string }{{"worker_clients", "client_client_id"}, {"endpoints", "client_id"}, {"proxy_stats", "client_id"}, {"history_proxy_stats", "client_id"}} {
		var count int64
		if err := db.Table(ref.table).Where(ref.column+" = ?", "admin.c.final").Count(&count).Error; err != nil || count != 1 {
			t.Fatal("association lost", ref.table, err, count)
		}
	}
	var policies int64
	db.Table("casbin_rule").Where("v1 = ? AND v3 = ?", "client:admin.c.final", "tenant:1").Count(&policies)
	if policies != 1 {
		t.Fatal("sharing policy not migrated")
	}
	db.Table("casbin_rule").Where("v1 = ? AND v3 = ?", "client:admin.c.old", "tenant:2").Count(&policies)
	if policies != 1 {
		t.Fatal("other tenant policy modified")
	}
	// A panel restart resolves the device by its secret, without name aliases.
	ctx.GetApp().SetClientsManager(rpc.NewClientsManager())
	if node, err := dao.NewQuery(ctx).ValidateClientSecret("admin.c.old", "secret"); err != nil || node.ClientID != "admin.c.final" {
		t.Fatal("restart lost identity", err)
	}
	if err := renameNode(ctx, "server", "admin.s.old", "admin.s.new"); err != nil {
		t.Fatal(err)
	}
	var child models.Client
	if err := db.Where("client_id = ?", "child").First(&child).Error; err != nil {
		t.Fatal(err)
	}
	if child.OriginClientID != "admin.c.final" || child.ServerID != "admin.s.new" {
		t.Fatalf("child references: %+v", child.ClientEntity)
	}
	var proxy models.ProxyConfig
	db.First(&proxy)
	if proxy.OriginClientID != "admin.c.final" || proxy.ServerID != "admin.s.new" || proxy.Name != "existing-name" {
		t.Fatalf("proxy references: %+v", proxy.ProxyConfigEntity)
	}
	if models.RuntimeNodeID(db, "server", "admin.s.new") != "admin.s.old" {
		t.Fatal("live FRP server key changed")
	}
	if _, err := dao.NewQuery(ctx).ValidateServerSecret("admin.s.old", "server-secret"); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Client{ClientEntity: &models.ClientEntity{ClientID: "admin.c.old", UserID: 2, TenantID: 1, ConnectSecret: "other-secret"}}).Error; err != nil {
		t.Fatal("released name could not be reused")
	}
	var aliases int64
	db.Model(&models.NodeAlias{}).Count(&aliases)
	if aliases != 0 {
		t.Fatal("rename retained aliases")
	}
	if node, err := dao.NewQuery(ctx).ValidateClientSecret("admin.c.old", "other-secret"); err != nil || node.UserID != 2 {
		t.Fatal("name reuse crossed identities", err)
	}
	if err := db.Where("client_id = ?", "admin.c.final").Delete(&models.Client{}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := dao.NewQuery(ctx).ValidateClientSecret("admin.c.old", "secret"); err == nil {
		t.Fatal("deleted identity accepted")
	}
}

func TestRenameRejectsUnauthorizedAndCollision(t *testing.T) {
	ctx, db := renameFixture(t)
	other := &models.UserEntity{UserID: 2, TenantID: 1, Role: "user"}
	otherCtx := app.NewContext(context.WithValue(context.Background(), defs.UserInfoKey, other), ctx.GetApp())
	if err := renameNode(otherCtx, "client", "admin.c.old", "stolen"); err == nil {
		t.Fatal("non-owner renamed node")
	}
	for _, id := range []string{"admin.s.old", "child", "bad/id", defs.DefaultServerID, "other.c.name"} {
		if err := renameNode(ctx, "client", "admin.c.old", id); err == nil {
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
	if err := renameNode(ctx, "client", "admin.c.old", "admin.c.rollback"); err == nil {
		t.Fatal("expected failure")
	}
	db.Table("clients").Where("client_id = ?", "admin.c.rollback").Count(&count)
	if count != 0 {
		t.Fatal("transaction left replacement row")
	}
	if _, err := dao.NewQuery(ctx).ValidateClientSecret("admin.c.old", "secret"); err != nil {
		t.Fatal("rollback lost original", err)
	}
}

func TestDeviceIdentityIsolationAndDeletion(t *testing.T) {
	ctx, db := renameFixture(t)
	q := dao.NewQuery(ctx)
	if node, err := q.ValidateClientSecret("child", "secret"); err != nil || node.ClientID != "child" {
		t.Fatal("own child configuration unavailable", err)
	}
	foreign := &models.Client{ClientEntity: &models.ClientEntity{ClientID: "foreign-child", OriginClientID: "other-parent", UserID: 2, TenantID: 1, ConnectSecret: "other-secret"}}
	if err := db.Create(foreign).Error; err != nil {
		t.Fatal(err)
	}
	if node, err := q.ValidateClientSecret("foreign-child", "secret"); err != nil || node.ClientID != "admin.c.old" {
		t.Fatal("supplied ID selected another device", err)
	}
	if _, err := q.ValidateClientSecret("admin.c.old", ""); err == nil {
		t.Fatal("empty secret accepted")
	}
	before, err := q.ValidateClientSecret("admin.c.old", "secret")
	if err != nil {
		t.Fatal(err)
	}
	deviceID := before.DeviceID
	if err := renameNode(ctx, "client", "admin.c.old", "admin.c.new"); err != nil {
		t.Fatal(err)
	}
	after, err := q.ValidateClientSecret("admin.c.old", "secret")
	if err != nil || after.DeviceID != deviceID {
		t.Fatal("rename replaced device identity", err)
	}
	if err := dao.NewMutation(ctx).DeleteClient(&models.UserEntity{UserID: 1, TenantID: 1, Role: defs.UserRole_Admin}, "admin.c.new"); err != nil {
		t.Fatal(err)
	}
	if _, err := q.ValidateClientSecret("child", "secret"); err == nil {
		t.Fatal("deleted child secret accepted")
	}
	var count int64
	db.Model(&models.ProxyConfig{}).Count(&count)
	if count != 0 {
		t.Fatal("orphan proxies survived deletion")
	}
	if err := db.Create(&models.Client{ClientEntity: &models.ClientEntity{ClientID: "admin.c.new", ConnectSecret: "fresh", TenantID: 1, UserID: 1}}).Error; err != nil {
		t.Fatal("deleted name was reserved", err)
	}
	fresh, err := q.ValidateClientSecret("admin.c.new", "fresh")
	if err != nil || fresh.DeviceID == deviceID {
		t.Fatal("name reuse inherited old identity", err)
	}
	server, err := q.ValidateServerSecret("ignored", "server-secret")
	if err != nil {
		t.Fatal(err)
	}
	if err := renameNode(ctx, "server", server.ServerID, "admin.s.new"); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Server{ServerEntity: &models.ServerEntity{ServerID: "admin.s.old", ConnectSecret: "fresh-server", TenantID: 1, UserID: 1}}).Error; err != nil {
		t.Fatal(err)
	}
	if models.RuntimeNodeID(db, "server", "admin.s.old") == models.RuntimeNodeID(db, "server", "admin.s.new") {
		t.Fatal("name reuse collided with legacy runtime")
	}
}

func TestIdentityMigrationClearsAliasesAndIsIdempotent(t *testing.T) {
	_, db := renameFixture(t)
	if err := db.Model(&models.Server{}).Where("server_id = ?", "admin.s.old").Updates(map[string]interface{}{"device_id": "", "runtime_id": ""}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.NodeAlias{OldID: "legacy", NewID: "admin.s.old", Kind: "server", RuntimeID: "legacy-runtime"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := models.MigrateNodeIdentity(db); err != nil {
		t.Fatal(err)
	}
	var before, after models.Server
	db.First(&before)
	if before.DeviceID == "" || before.RuntimeID != "legacy-runtime" {
		t.Fatal("legacy migration lost runtime")
	}
	if err := models.MigrateNodeIdentity(db); err != nil {
		t.Fatal(err)
	}
	db.First(&after)
	if after.DeviceID != before.DeviceID || after.RuntimeID != before.RuntimeID {
		t.Fatal("migration changed stable identity")
	}
	var count int64
	db.Model(&models.NodeAlias{}).Count(&count)
	if count != 0 {
		t.Fatal("legacy aliases were retained")
	}
}

func TestDeletedServerNameDoesNotInheritTunnelReferences(t *testing.T) {
	ctx, db := renameFixture(t)
	user := &models.UserEntity{UserID: 1, TenantID: 1, Role: defs.UserRole_Admin}
	if err := dao.NewMutation(ctx).DeleteServer(user, "admin.s.old"); err != nil {
		t.Fatal(err)
	}
	if _, err := dao.NewQuery(ctx).ValidateServerSecret("admin.s.old", "server-secret"); err == nil {
		t.Fatal("deleted server secret accepted")
	}
	var count int64
	db.Model(&models.ProxyConfig{}).Where("server_id = ?", "admin.s.old").Count(&count)
	if count != 0 {
		t.Fatal("deleted server left active proxies")
	}
	db.Model(&models.Client{}).Where("server_id = ?", "admin.s.old").Count(&count)
	if count != 0 {
		t.Fatal("deleted server left client config references")
	}
	if err := db.Create(&models.Server{ServerEntity: &models.ServerEntity{ServerID: "admin.s.old", ConnectSecret: "new-server", UserID: 1, TenantID: 1}}).Error; err != nil {
		t.Fatal(err)
	}
}
