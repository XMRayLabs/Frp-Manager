package models

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"testing"
)

func TestEnrollmentTokenPersistsAndRotatesIndependently(t *testing.T) {
	path := filepath.Join(t.TempDir(), "enrollment.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.AutoMigrate(&EnrollmentToken{}); err != nil {
		t.Fatal(err)
	}
	client, err := GetEnrollmentToken(db, 1, "client")
	if err != nil {
		t.Fatal(err)
	}
	server, err := GetEnrollmentToken(db, 1, "server")
	if err != nil {
		t.Fatal(err)
	}
	other, err := GetEnrollmentToken(db, 2, "client")
	if err != nil {
		t.Fatal(err)
	}
	if client == server || client == other {
		t.Fatal("account/role tokens must differ")
	}
	sqlDB, _ := db.DB()
	sqlDB.Close()
	db, err = gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	reopened, _ := db.DB()
	defer reopened.Close()
	same, err := GetEnrollmentToken(db, 1, "client")
	if err != nil || same != client {
		t.Fatal("token changed after reopening database")
	}
	fresh, err := RotateEnrollmentToken(db, 1, "client", client)
	if err != nil || fresh == client {
		t.Fatal("rotation failed")
	}
	if _, err = ResolveEnrollmentToken(db, client); err == nil {
		t.Fatal("old token still accepted")
	}
	if _, err = ResolveEnrollmentToken(db, fresh); err != nil {
		t.Fatal(err)
	}
	if _, err = ResolveEnrollmentToken(db, server); err != nil {
		t.Fatal("server token revoked with client token")
	}
	if _, err = RotateEnrollmentToken(db, 1, "client", client); err == nil {
		t.Fatal("stale confirmation accepted")
	}
	if _, err = RotateEnrollmentToken(db, 2, "client", fresh); err == nil {
		t.Fatal("cross-account rotation accepted")
	}
}
