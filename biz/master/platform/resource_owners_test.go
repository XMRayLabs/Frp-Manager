package platform

import (
	"context"
	"testing"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestResourceOwnersUseDatabaseOwnershipAndVisibility(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:resource-owner-test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.User{}, &models.Client{}, &models.ProxyConfig{}); err != nil {
		t.Fatal(err)
	}
	for _, u := range []*models.UserEntity{{UserID: 1, TenantID: 1, UserName: "admin", Email: "admin@test.de"}, {UserID: 2, TenantID: 1, UserName: "ig01", Email: "ig01@test.de"}, {UserID: 3, TenantID: 2, UserName: "hidden", Email: "hidden@test.de"}} {
		if err = db.Create(&models.User{UserEntity: u}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []*models.ClientEntity{{ClientID: "legacy-name", UserID: 1, TenantID: 1}, {ClientID: "admin.c.misleading", UserID: 2, TenantID: 1}, {ClientID: "hidden", UserID: 3, TenantID: 2}} {
		if err = db.Create(&models.Client{ClientEntity: c}).Error; err != nil {
			t.Fatal(err)
		}
		if err = db.Create(&models.ProxyConfig{ProxyConfigEntity: &models.ProxyConfigEntity{ClientID: c.ClientID, UserID: c.UserID, TenantID: c.TenantID, Name: "legacy-proxy"}}).Error; err != nil {
			t.Fatal(err)
		}
	}
	instance := app.NewApp()
	mgr := models.NewDBManager(defs.DBTypeSQLite3)
	mgr.SetDB(defs.DBTypeSQLite3, defs.DBRoleDefault, db)
	instance.SetDBManager(mgr)
	for _, role := range []string{defs.UserRole_Admin, defs.UserRole_Normal} {
		user := &models.UserEntity{UserID: 1, TenantID: 1, Role: role}
		ctx := app.NewContext(context.WithValue(context.Background(), defs.UserInfoKey, user), instance)
		for _, kind := range []string{"client", "proxy"} {
			result, err := getResourceOwners(ctx, kind)
			if err != nil {
				t.Fatal(err)
			}
			want := 1
			if role == defs.UserRole_Admin {
				want = 2
			}
			if len(result.Owners) != want || len(result.Resources) != want {
				t.Fatalf("%s/%s scope: %+v", role, kind, result)
			}
			for _, owner := range result.Owners {
				if owner.ID == 3 {
					t.Fatal("foreign tenant owner exposed")
				}
			}
			if kind == "client" && role == defs.UserRole_Admin && result.Resources["admin.c.misleading"] != 2 {
				t.Fatal("owner guessed from node name")
			}
		}
	}
}
