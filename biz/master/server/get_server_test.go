package server

import (
	"context"
	"strings"
	"testing"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/glebarez/sqlite"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

func TestGetDefaultServerPreservesIDAndAuthorization(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.AutoMigrate(&models.Server{}, &models.LanguageGroupServer{}); err != nil {
		t.Fatal(err)
	}
	instance := app.NewApp()
	manager := models.NewDBManager(defs.DBTypeSQLite3)
	manager.SetDB(defs.DBTypeSQLite3, defs.DBRoleDefault, db)
	instance.SetDBManager(manager)
	entry := "https://frp.example.com"
	if err := db.Create(&models.Server{ServerEntity: &models.ServerEntity{
		ServerID: defs.DefaultServerID, ConnectSecret: "private-server-secret",
		WebsiteBaseURL: &entry,
		ConfigContent:  []byte(`{"bindPort":7000,"vhostHTTPPort":9080,"subDomainHost":"frp.example.com","auth":{"token":"private-auth"}}`),
	}}).Error; err != nil {
		t.Fatal(err)
	}
	get := func(user *models.UserEntity) (*pb.GetServerResponse, error) {
		ctx := app.NewContext(context.WithValue(context.Background(), defs.UserInfoKey, user), instance)
		return GetServerHandler(ctx, &pb.GetServerRequest{ServerId: lo.ToPtr(defs.DefaultServerID)})
	}
	admin := &models.UserEntity{UserID: 1, TenantID: 1, UserName: "admin", Role: defs.UserRole_Admin}
	response, err := get(admin)
	if err != nil {
		t.Fatal(err)
	}
	if response.GetServer().GetId() != defs.DefaultServerID || response.GetServer().GetWebsiteBaseUrl() != entry || response.GetServer().GetSecret() != "private-server-secret" {
		t.Fatalf("default server configuration not returned: %v", response)
	}
	member := &models.UserEntity{UserID: 2, TenantID: 1, UserName: "member", Role: "user", LanguageGroupID: "group"}
	if _, err := get(member); err == nil {
		t.Fatal("unassigned member can read the default server")
	}
	if err := db.Create(&models.LanguageGroupServer{LanguageGroupID: "group", ServerID: defs.DefaultServerID}).Error; err != nil {
		t.Fatal(err)
	}
	response, err = get(member)
	if err != nil {
		t.Fatal(err)
	}
	if response.GetServer().GetId() != defs.DefaultServerID || response.GetServer().GetWebsiteBaseUrl() != entry || response.GetServer().GetSecret() != "" {
		t.Fatalf("assigned member received incorrect server data: %v", response)
	}
	config := response.GetServer().GetConfig()
	if !strings.Contains(config, "9080") || strings.Contains(config, "private-auth") || strings.Contains(config, `"auth"`) {
		t.Fatal("assigned member received incorrect public configuration")
	}
}
