package models

import (
	"fmt"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"testing"
)

func TestLanguageMigrationDoesNotRestoreRevokedGrants(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:org-migration?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&User{}, &Server{}, &Client{}, &InviteCode{}, &UserGroup{}, &SystemSetting{}); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 2; i++ {
		u := &User{UserEntity: &UserEntity{UserID: i, UserName: fmt.Sprint(i), Email: fmt.Sprint(i), TenantID: 1, Role: defs.UserRole_Normal}}
		if i == 1 {
			u.Role = defs.UserRole_Admin
		}
		if err = db.Create(u).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err = db.Create(&Server{ServerEntity: &ServerEntity{ServerID: "s", TenantID: 1, UserID: 1, ConnectSecret: "unchanged"}}).Error; err != nil {
		t.Fatal(err)
	}
	if err = MigrateLanguageGroups(db); err != nil {
		t.Fatal(err)
	}
	var u User
	db.First(&u, 2)
	if u.LanguageGroupID != "default-1" {
		t.Fatalf("membership=%s", u.LanguageGroupID)
	}
	var n int64
	db.Model(&LanguageGroupServer{}).Count(&n)
	if n != 1 {
		t.Fatalf("grant count=%d", n)
	}
	db.Where("server_id = ?", "s").Delete(&LanguageGroupServer{})
	if err = MigrateLanguageGroups(db); err != nil {
		t.Fatal(err)
	}
	db.Model(&LanguageGroupServer{}).Count(&n)
	if n != 0 {
		t.Fatal("restart restored revoked grant")
	}
	var server Server
	db.Where("server_id = ?", "s").First(&server)
	if server.ConnectSecret != "unchanged" {
		t.Fatal("migration changed device credential")
	}
}
