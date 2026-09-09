package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"sync"
)

// Serialize identity changes with authenticated stream registration.
var NodeIdentityMu sync.Mutex

// Legacy migration input only. Old names are no longer resolved or reserved.
type NodeAlias struct {
	RuntimeID string
	OldID     string `gorm:"primaryKey;size:255"`
	NewID     string
	Kind      string
}

func (c *Client) BeforeCreate(tx *gorm.DB) error {
	if c.ClientEntity == nil {
		return nil
	}
	if c.DeviceID == "" {
		c.DeviceID = uuid.NewString()
	}
	if c.RuntimeID == "" {
		c.RuntimeID = c.ClientID
	}
	return nil
}
func (s *Server) BeforeCreate(tx *gorm.DB) error {
	if s.ServerEntity == nil {
		return nil
	}
	if s.DeviceID == "" {
		s.DeviceID = uuid.NewString()
	}
	if s.RuntimeID == "" {
		s.RuntimeID = s.DeviceID
	}
	return nil
}

// Legacy kernels use these internal FRP keys; they do not reserve display names.
func RuntimeNodeID(db *gorm.DB, kind, id string) string {
	var value string
	table, column := "clients", "client_id"
	if kind == "server" {
		table, column = "servers", "server_id"
	}
	if db.Table(table).Select("runtime_id").Where(column+" = ? AND deleted_at IS NULL", id).Scan(&value).Error == nil && value != "" {
		return value
	}
	return id
}

func MigrateNodeIdentity(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		for _, target := range []struct{ table, column, kind string }{{"clients", "client_id", "client"}, {"servers", "server_id", "server"}} {
			var rows []struct {
				ID        string
				DeviceID  string
				RuntimeID string
			}
			if err := tx.Table(target.table).Select(target.column + " AS id, device_id, runtime_id").Scan(&rows).Error; err != nil {
				return err
			}
			for _, row := range rows {
				updates := map[string]interface{}{}
				if row.DeviceID == "" {
					updates["device_id"] = uuid.NewString()
				}
				if row.RuntimeID == "" {
					runtimeID := row.ID
					if tx.Migrator().HasTable(&NodeAlias{}) {
						var alias NodeAlias
						if err := tx.Where("new_id = ? AND kind = ?", row.ID, target.kind).Limit(1).Find(&alias).Error; err != nil {
							return err
						}
						if alias.RuntimeID != "" {
							runtimeID = alias.RuntimeID
						}
					}
					updates["runtime_id"] = runtimeID
				}
				if len(updates) > 0 {
					if err := tx.Table(target.table).Where(target.column+" = ?", row.ID).Updates(updates).Error; err != nil {
						return err
					}
				}
			}
		}
		if tx.Migrator().HasTable(&NodeAlias{}) {
			return tx.Where("1 = 1").Delete(&NodeAlias{}).Error
		}
		return nil
	})
}
