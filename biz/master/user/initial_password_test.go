package user

import (
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/utils"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"testing"
)

func TestInitialPasswordMustChangeBeforeUnlock(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:first-password-test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.User{}); err != nil {
		t.Fatal(err)
	}
	initial := "ig01@xmray.de"
	hash, err := utils.HashPassword(initial)
	if err != nil {
		t.Fatal(err)
	}
	user := &models.UserEntity{UserName: "ig01", Email: initial, Password: hash, MustChangePassword: true}
	if err = db.Create(&models.User{UserEntity: user}).Error; err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{{"wrong", "NewPassword123!"}, {initial, initial}, {initial, "short"}} {
		if err = changeInitialPassword(db, user, pair[0], pair[1]); err == nil {
			t.Fatal("invalid change accepted")
		}
	}
	var stored models.User
	db.First(&stored, user.UserID)
	if !stored.MustChangePassword {
		t.Fatal("failed change unlocked account")
	}
	if err = changeInitialPassword(db, user, initial, "NewPassword123!"); err != nil {
		t.Fatal(err)
	}
	db.First(&stored, user.UserID)
	if stored.MustChangePassword || !utils.CheckPasswordHash("NewPassword123!", stored.Password) || utils.CheckPasswordHash(initial, stored.Password) {
		t.Fatal("password or flag not updated")
	}
}
