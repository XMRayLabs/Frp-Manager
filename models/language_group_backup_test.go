package models

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"testing"
)

func TestLanguageMigrationBackupContainsData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err = db.AutoMigrate(&User{}); err != nil {
		t.Fatal(err)
	}
	db.Create(&User{UserEntity: &UserEntity{UserID: 1, UserName: "old", Email: "old"}})
	if err = BackupBeforeLanguageMigration(db); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(path + ".before-1.1.0-*.bak")
	if err != nil || len(files) != 1 {
		t.Fatalf("missing backup: %v %v", files, err)
	}
	copyDB, err := gorm.Open(sqlite.Open(files[0]), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlCopy, _ := copyDB.DB()
	defer sqlCopy.Close()
	var user User
	if err = copyDB.First(&user, 1).Error; err != nil || user.UserName != "old" {
		t.Fatal("backup lost existing data")
	}
}
