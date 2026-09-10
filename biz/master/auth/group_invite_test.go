package auth

import (
	"github.com/Sakurame1/frp-manager/models"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"testing"
)

func TestGroupInviteConsumptionRollsBackWithRegistration(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:group-invite?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.InviteCode{}, &models.LanguageGroup{}, &models.User{}); err != nil {
		t.Fatal(err)
	}
	db.Create(&models.LanguageGroup{ID: "a", Name: "A", TenantID: 7})
	db.Create(&models.InviteCode{Code: "one-use", TenantID: 7, LanguageGroupID: "a", MaxUses: 1})
	db.Create(&models.User{UserEntity: &models.UserEntity{UserName: "existing", Email: "existing"}})
	err = db.Transaction(func(tx *gorm.DB) error {
		invite, err := consumeGroupInvite(tx, "one-use")
		if err != nil {
			return err
		}
		if invite.LanguageGroupID != "a" || invite.TenantID != 7 {
			t.Fatal("wrong membership")
		}
		return tx.Create(&models.User{UserEntity: &models.UserEntity{UserName: "existing", Email: "different"}}).Error
	})
	if err == nil {
		t.Fatal("duplicate account accepted")
	}
	var stored models.InviteCode
	db.Where("code = ?", "one-use").First(&stored)
	if stored.UsedCount != 0 {
		t.Fatal("failed registration consumed invite")
	}
	if err = db.Transaction(func(tx *gorm.DB) error { _, err := consumeGroupInvite(tx, "one-use"); return err }); err != nil {
		t.Fatal(err)
	}
	if err = db.Transaction(func(tx *gorm.DB) error { _, err := consumeGroupInvite(tx, "one-use"); return err }); err == nil {
		t.Fatal("exhausted invite reused")
	}
}
