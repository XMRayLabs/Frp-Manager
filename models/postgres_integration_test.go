package models

import (
	"bytes"
	"context"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"os"
	"strings"
	"testing"
)

func postgresTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("FRP_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("FRP_TEST_POSTGRES_DSN not configured")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	namespace := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err = db.Exec("CREATE SCHEMA " + namespace).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Exec("SET search_path TO " + namespace).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Exec("DROP SCHEMA " + namespace + " CASCADE"); sqlDB.Close() })
	return db
}
func TestPostgresMigration(t *testing.T) {
	target := postgresTestDB(t)
	source, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := source.DB()
	raw.SetMaxOpenConns(1)
	defer raw.Close()
	if err = MigrateSchema(source); err != nil {
		t.Fatal(err)
	}
	for _, record := range []interface{}{
		&User{UserEntity: &UserEntity{UserID: 40, UserName: "admin", Email: "admin@example.com", Password: "hash-preserved", Token: "token-preserved", TenantID: 1, Status: 1, Role: "admin"}},
		&Client{ClientEntity: &ClientEntity{ClientID: "admin.c.test", TenantID: 1, UserID: 40, ConnectSecret: "device-secret", Private: true, ConfigContent: []byte("{\"proxies\":[]}")}},
		&Server{ServerEntity: &ServerEntity{ServerID: "s", TenantID: 1, UserID: 40, ConnectSecret: "server-secret", FrpsUrls: GormArray[string]{"example.com"}}},
		&Endpoint{EndpointEntity: &EndpointEntity{Host: "localhost", Port: 6000, ClientID: "admin.c.test"}},
		&ProxyConfig{Model: &gorm.Model{ID: 70}, ProxyConfigEntity: &ProxyConfigEntity{ServerID: "s", ClientID: "admin.c.test", Content: []byte("{}")}},
		&Cert{Name: "master", CertFile: []byte{0, 1, 255}, KeyFile: []byte("private-key")},
		&Network{NetworkEntity: &NetworkEntity{Name: "network", UserId: 40, TenantId: 1}},
		&gormadapter.CasbinRule{ID: 15, Ptype: "p", V0: "user:40", V1: "client:test", V2: "read"},
	} {
		if err = source.Create(record).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err = MigrateNodeIdentity(source); err != nil {
		t.Fatal(err)
	}
	if err = MigrateLanguageGroups(source); err != nil {
		t.Fatal(err)
	}
	counts, err := TransferToPostgres(context.Background(), source, target)
	if err != nil {
		t.Fatal(err)
	}
	if counts["users"] != 1 {
		t.Fatal(counts)
	}
	var client Client
	if err = target.First(&client).Error; err != nil {
		t.Fatal(err)
	}
	if client.ConnectSecret != "device-secret" || !client.Private || string(client.ConfigContent) != "{\"proxies\":[]}" {
		t.Fatal("device changed")
	}
	var user User
	if err = target.First(&user).Error; err != nil {
		t.Fatal(err)
	}
	if user.Password != "hash-preserved" || user.Token != "token-preserved" {
		t.Fatal("credentials changed")
	}
	next := &User{UserEntity: &UserEntity{UserName: "next", Email: "next@example.com"}}
	if err = target.Create(next).Error; err != nil {
		t.Fatal(err)
	}
	if next.UserID <= 40 {
		t.Fatal("sequence not repaired")
	}
	if _, err = TransferToPostgres(context.Background(), source, target); err == nil {
		t.Fatal("nonempty destination accepted")
	}
	if err = MigrateSchema(target); err != nil {
		t.Fatal("restart schema", err)
	}
	var cert Cert
	if err = target.First(&cert).Error; err != nil || !bytes.Equal(cert.CertFile, []byte{0, 1, 255}) {
		t.Fatal("certificate bytes changed", err)
	}
	var server Server
	if err = target.First(&server).Error; err != nil || len(server.FrpsUrls) != 1 || server.FrpsUrls[0] != "example.com" {
		t.Fatal("server JSON changed", err)
	}
	var rule gormadapter.CasbinRule
	if err = target.First(&rule).Error; err != nil || rule.V0 != "user:40" {
		t.Fatal("permissions changed", err)
	}
	var network Network
	if err = target.First(&network).Error; err != nil {
		t.Fatal("JSON roundtrip", err)
	}
}
func TestPostgresMigrationRollback(t *testing.T) {
	target := postgresTestDB(t)
	source, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := source.DB()
	raw.SetMaxOpenConns(1)
	defer raw.Close()
	if err = MigrateSchema(source); err != nil {
		t.Fatal(err)
	}
	if err = source.Exec("CREATE TABLE unknown_plugin_data (id INTEGER)").Error; err != nil {
		t.Fatal(err)
	}
	if _, err = TransferToPostgres(context.Background(), source, target); err == nil {
		t.Fatal("unknown source data ignored")
	}
	tables, err := target.Migrator().GetTables()
	if err != nil || len(tables) != 0 {
		t.Fatalf("failed migration left schema behind: %v %v", tables, err)
	}
}
