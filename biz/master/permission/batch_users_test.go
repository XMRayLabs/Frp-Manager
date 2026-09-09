package permission

import (
	"fmt"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/utils"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"testing"
)

func TestBatchIdentitiesThirtyAndWidthBoundary(t *testing.T) {
	rows, err := batchIdentities(batchUsersRequest{Username: "ig01", Email: "ig01@xmray.de", Count: 30})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 30 || rows[0].UserName != "ig01" || rows[29].UserName != "ig30" || rows[29].Email != "ig30@xmray.de" {
		t.Fatalf("bad range: %+v", rows)
	}
	rows, err = batchIdentities(batchUsersRequest{Username: "ig99", Email: "ig99@xmray.de", Count: 2})
	if err != nil || rows[1].UserName != "ig100" {
		t.Fatal("width rollover failed")
	}
	for _, req := range []batchUsersRequest{{"ig", "ig01@xmray.de", 30}, {"ig01", "bad", 30}, {"ig01", "ig01@xmray.de", 0}, {"ig01", "ig01@xmray.de", 101}} {
		if _, err = batchIdentities(req); err == nil {
			t.Fatalf("accepted invalid request %+v", req)
		}
	}
}

func TestBatchUsersHashPasswordsAndRollback(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:batch-test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&models.User{}); err != nil {
		t.Fatal(err)
	}
	req := batchUsersRequest{Username: "ig01", Email: "ig01@xmray.de", Count: 2}
	rows, err := createBatchUsers(db, 9, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Password != "" || rows[0].Token != "" || !rows[0].MustChangePassword {
		t.Fatal("unsafe or incomplete response")
	}
	var stored models.User
	db.Where("user_name = ?", "ig01").First(&stored)
	if stored.TenantID != 9 || stored.Password == stored.Email || !utils.CheckPasswordHash(stored.Email, stored.Password) || !stored.MustChangePassword {
		t.Fatal("invalid initial credentials")
	}
	if _, err = createBatchUsers(db, 9, req); err == nil {
		t.Fatal("duplicate batch accepted")
	}
	// Fail the second insert to verify the first insert is rolled back as well.
	db.Callback().Create().Before("gorm:create").Register("test:fail-second", func(tx *gorm.DB) {
		if user, ok := tx.Statement.Dest.(*models.User); ok && user.UserName == "tx02" {
			tx.AddError(fmt.Errorf("injected failure"))
		}
	})
	if _, err = createBatchUsers(db, 9, batchUsersRequest{"tx01", "tx01@xmray.de", 2}); err == nil {
		t.Fatal("failure not propagated")
	}
	var count int64
	db.Model(&models.User{}).Where("user_name LIKE ?", "tx%").Count(&count)
	if count != 0 {
		t.Fatal("batch only partially rolled back")
	}
}
