package models

import (
	"fmt"
	"gorm.io/gorm"
	"time"
)

// A single SQLite connection also keeps :memory: databases on the same connection.
func ConfigureDBPool(db *gorm.DB, open, idle int, lifetime time.Duration) error {
	if open < 1 || idle < 0 || idle > open || lifetime < 0 {
		return fmt.Errorf("invalid database pool configuration")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	if db.Dialector.Name() == "sqlite" {
		open, idle, lifetime = 1, 1, 0
	}
	sqlDB.SetMaxOpenConns(open)
	sqlDB.SetMaxIdleConns(idle)
	sqlDB.SetConnMaxLifetime(lifetime)
	return nil
}
