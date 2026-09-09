package models

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
)

// Historical IDs remain reserved, including after node deletion. They are accepted
// only with the current node secret, so existing installations can reconnect.
type NodeAlias struct {
	RuntimeID string
	OldID     string `gorm:"primaryKey;size:255"`
	NewID     string `gorm:"index;size:255"`
	Kind      string `gorm:"size:16"`
}

func ResolveNodeID(db *gorm.DB, kind, id string) (string, error) {
	if !db.Migrator().HasTable(&NodeAlias{}) {
		return id, nil
	}
	var alias NodeAlias
	err := db.Where("old_id = ? AND kind = ?", id, kind).Limit(1).Find(&alias).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && alias.NewID == "") {
		return id, nil
	}
	if err != nil {
		return "", err
	}
	return alias.NewID, nil
}

func CheckReservedNodeID(db *gorm.DB, id string) error {
	if !db.Migrator().HasTable(&NodeAlias{}) {
		return nil
	}
	var count int64
	if err := db.Model(&NodeAlias{}).Where("old_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("该 ID 已被历史节点保留，请使用其他 ID")
	}
	return nil
}

func (c *Client) BeforeCreate(tx *gorm.DB) error { return CheckReservedNodeID(tx, c.ClientID) }
func (s *Server) BeforeCreate(tx *gorm.DB) error { return CheckReservedNodeID(tx, s.ServerID) }

// Preserve the server key used by already-running FRP instances on old kernels.
func RuntimeNodeID(db *gorm.DB, kind, id string) string {
	if !db.Migrator().HasTable(&NodeAlias{}) {
		return id
	}
	var alias NodeAlias
	if db.Where("new_id = ? AND kind = ?", id, kind).Limit(1).Find(&alias).Error == nil && alias.RuntimeID != "" {
		return alias.RuntimeID
	}
	return id
}
