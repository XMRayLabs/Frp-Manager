package models

import (
	"fmt"
	"gorm.io/gorm"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// BackupBeforeLanguageMigration runs before any schema changes. VACUUM INTO
// includes committed WAL pages and fails startup rather than migrating without
// a complete snapshot. External databases use the operator's normal snapshot.
func BackupBeforeLanguageMigration(db *gorm.DB) error {
	if db.Dialector.Name() != "sqlite" || !db.Migrator().HasTable(&User{}) || db.Migrator().HasTable(&LanguageGroup{}) {
		return nil
	}
	var databases []struct {
		Name string
		File string
	}
	if err := db.Raw("PRAGMA database_list").Scan(&databases).Error; err != nil {
		return err
	}
	for _, database := range databases {
		if database.Name != "main" || database.File == "" {
			continue
		}
		destination := database.File + ".before-1.1.0-" + time.Now().UTC().Format("20060102T150405.000000000") + ".bak"
		if _, err := os.Stat(destination); err == nil {
			return fmt.Errorf("backup already exists: %s", destination)
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := db.Exec("VACUUM INTO '" + strings.ReplaceAll(filepath.ToSlash(destination), "'", "''") + "'").Error; err != nil {
			return fmt.Errorf("pre-migration backup failed: %w", err)
		}
		if err := os.Chmod(destination, 0600); err != nil {
			return err
		}
	}
	return nil
}
