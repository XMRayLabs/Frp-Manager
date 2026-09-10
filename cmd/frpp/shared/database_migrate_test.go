package shared

import (
	"bytes"
	"crypto/sha256"
	"github.com/Sakurame1/frp-manager/conf"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPostgresMigrationCommand(t *testing.T) {
	dsn := os.Getenv("FRP_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("FRP_TEST_POSTGRES_DSN not configured")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	rawAdmin, _ := admin.DB()
	defer rawAdmin.Close()
	name := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err = admin.Exec("CREATE SCHEMA " + name).Error; err != nil {
		t.Fatal(err)
	}
	defer admin.Exec("DROP SCHEMA " + name + " CASCADE")
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "old.db")
	source, err := gorm.Open(sqlite.Open(sourcePath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = source.AutoMigrate(&models.User{}); err != nil {
		t.Fatal(err)
	}
	if err = source.Create(&models.User{UserEntity: &models.UserEntity{UserID: 50, UserName: "legacy", Email: "legacy@example.com", Password: "old-password-hash", Token: "old-token", TenantID: 1, Status: 1, Role: "user"}}).Error; err != nil {
		t.Fatal(err)
	}
	sqlSource, _ := source.DB()
	sqlSource.Close()
	before, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	cfg := conf.Config{}
	cfg.DB.Type = "postgres"
	cfg.DB.DSN = dsn + " search_path=" + name
	command := NewMigrateDatabaseCmd(cfg)
	command.SetArgs([]string{"--source", sourcePath, "--temp-dir", dir, "--offline"})
	var output bytes.Buffer
	command.SetOut(&output)
	if err = command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Migration committed") {
		t.Fatal(output.String())
	}
	after, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("source file changed")
	}
	target, err := gorm.Open(postgres.Open(cfg.DB.DSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	rawTarget, _ := target.DB()
	defer rawTarget.Close()
	var user models.User
	if err = target.First(&user).Error; err != nil {
		t.Fatal(err)
	}
	if user.Password != "old-password-hash" || user.Token != "old-token" || user.LanguageGroupID == "" {
		t.Fatal("legacy account was not preserved and upgraded")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			t.Fatal("temporary snapshot not cleaned")
		}
	}
}
func TestMigrationRequiresOffline(t *testing.T) {
	command := NewMigrateDatabaseCmd(conf.Config{})
	command.SetArgs(nil)
	if err := command.Execute(); err == nil {
		t.Fatal("missing offline confirmation accepted")
	}
}
