package dao

import (
	"context"
	"fmt"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"testing"
)

func TestOrganizationClientIsolation(t *testing.T)   { testOrganizationIsolation(t, false) }
func TestPostgresOrganizationIsolation(t *testing.T) { testOrganizationIsolation(t, true) }
func testOrganizationIsolation(t *testing.T, pg bool) {
	db, err := gorm.Open(sqlite.Open("file:org-isolation?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.User{}, &models.Client{}, &models.LanguageGroup{}, &models.LanguageGroupServer{}, &models.Server{}); err != nil {
		t.Fatal(err)
	}
	if pg {
		db = daoPostgresTestDB(t)
	}
	application := app.NewApp()
	manager := models.NewDBManager(defs.DBTypeSQLite3)
	manager.SetDB(defs.DBTypeSQLite3, defs.DBRoleDefault, db)
	application.SetDBManager(manager)
	ctx := app.NewContext(context.Background(), application)
	users := []*models.UserEntity{}
	for i, g := range []string{"a", "a", "a", "b", ""} {
		role := defs.UserRole_Normal
		if i == 2 {
			role = defs.UserRole_GroupAdmin
		}
		if i == 4 {
			role = defs.UserRole_Admin
		}
		u := &models.UserEntity{UserID: i + 1, UserName: fmt.Sprint(i), Email: fmt.Sprint(i), TenantID: 1, LanguageGroupID: g, Role: role, Status: models.STATUS_NORMAL}
		users = append(users, u)
		if err = db.Create(&models.User{UserEntity: u}).Error; err != nil {
			t.Fatal(err)
		}
	}
	db.Create(&models.LanguageGroup{ID: "a", Name: "A", TenantID: 1})
	db.Create(&models.LanguageGroup{ID: "b", Name: "B", TenantID: 1, Shared: true})
	for i := 0; i < 4; i++ {
		if err = db.Create(&models.Client{ClientEntity: &models.ClientEntity{ClientID: fmt.Sprint(i + 1), TenantID: 1, UserID: i + 1}}).Error; err != nil {
			t.Fatal(err)
		}
	}
	check := func(u models.UserInfo, id string, want bool) {
		t.Helper()
		err := CanAccessClient(ctx, u, id, defs.RBACActionEdit)
		if (err == nil) != want {
			t.Fatalf("user %d client %s allowed=%v want=%v: %v", u.GetUserID(), id, err == nil, want, err)
		}
	}
	check(users[0], "1", true)
	check(users[0], "2", false)
	check(users[2], "1", true)
	check(users[2], "4", false)
	check(users[4], "4", true)
	db.Model(&models.LanguageGroup{}).Where("id = ?", "a").Update("shared", true)
	check(models.User{UserEntity: users[0]}, "2", true)
	check(users[0], "4", false)
	if CanManageClient(ctx, users[0], "2") == nil {
		t.Fatal("shared member can manage device credentials")
	}
	var visible []models.Client
	if err = organizationScope(db, ctx, users[0], defs.RBACObjClient, "client_id", defs.RBACActionView).Find(&visible).Error; err != nil {
		t.Fatal(err)
	}
	if len(visible) != 3 {
		t.Fatalf("shared list got %d nodes", len(visible))
	}
	db.Model(&models.Client{}).Where("client_id = ?", "2").Update("private", true)
	check(users[0], "2", false)
	check(users[2], "2", true)
	users[0].Status = models.STATUS_BANED
	check(users[0], "1", false)
}
