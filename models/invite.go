package models

import (
	"time"

	"gorm.io/gorm"
)

type InviteCode struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	Code        string         `json:"code" gorm:"type:varchar(255);uniqueIndex;not null"`
	TenantID    int            `json:"tenant_id" gorm:"not null;index"`
	CreatedBy   int            `json:"created_by" gorm:"not null;index"`
	MaxUses     int            `json:"max_uses" gorm:"not null;default:1"`
	UsedCount   int            `json:"used_count" gorm:"not null;default:0"`
	ExpiresAt   *time.Time     `json:"expires_at" gorm:"index"`
	Disabled    bool           `json:"disabled" gorm:"not null;default:false"`
	Comment     string         `json:"comment"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

func (*InviteCode) TableName() string {
	return "invite_codes"
}

type SystemSetting struct {
	Key       string    `json:"key" gorm:"type:varchar(255);primaryKey"`
	TenantID  int       `json:"tenant_id" gorm:"primaryKey"`
	Value     string    `json:"value"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (*SystemSetting) TableName() string {
	return "system_settings"
}
