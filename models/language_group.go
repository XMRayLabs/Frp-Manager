package models

import (
	"fmt"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type LanguageGroup struct {
	ID        string    `json:"id" gorm:"primaryKey;size:64"`
	TenantID  int       `json:"tenant_id" gorm:"uniqueIndex:idx_language_group_name"`
	Name      string    `json:"name" gorm:"uniqueIndex:idx_language_group_name;size:128;not null"`
	Shared    bool      `json:"shared" gorm:"not null;default:false"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type LanguageGroupServer struct {
	LanguageGroupID string `json:"language_group_id" gorm:"primaryKey;size:64"`
	ServerID        string `json:"server_id" gorm:"primaryKey;size:255"`
}
type OrganizationAudit struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	TenantID        int       `json:"tenant_id" gorm:"index"`
	LanguageGroupID string    `json:"language_group_id" gorm:"index"`
	ActorID         int       `json:"actor_id"`
	Action          string    `json:"action"`
	Target          string    `json:"target"`
	Detail          string    `json:"detail"`
	CreatedAt       time.Time `json:"created_at"`
}

func GroupID(u UserInfo) string {
	if u != nil {
		return u.GetSafeUserInfo().LanguageGroupID
	}
	return ""
}
func IsGroupAdmin(u UserInfo) bool {
	return u != nil && u.GetRole() == defs.UserRole_GroupAdmin && GroupID(u) != ""
}
func IsAccountManager(u UserInfo) bool { return u != nil && (u.IsAdmin() || IsGroupAdmin(u)) }
func DefaultLanguageGroup(db *gorm.DB, tenant int) (string, error) {
	id := fmt.Sprintf("default-%d", tenant)
	row := LanguageGroup{ID: id, TenantID: tenant, Name: "默认语系"}
	err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error
	return id, err
}

// Migrate once per tenant. Never re-grant server assignments on subsequent restarts.
func MigrateLanguageGroups(db *gorm.DB) error {
	if err := db.AutoMigrate(&LanguageGroup{}, &LanguageGroupServer{}, &OrganizationAudit{}); err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var tenants []int
		if err := tx.Model(&User{}).Distinct("tenant_id").Pluck("tenant_id", &tenants).Error; err != nil {
			return err
		}
		for _, tenant := range tenants {
			marker := SystemSetting{Key: "language_groups_v1", TenantID: tenant}
			var n int64
			if err := tx.Model(&SystemSetting{}).Where("key = ? AND tenant_id = ?", marker.Key, tenant).Count(&n).Error; err != nil {
				return err
			}
			if n > 0 {
				continue
			}
			id, err := DefaultLanguageGroup(tx, tenant)
			if err != nil {
				return err
			}
			if err = tx.Model(&User{}).Where("tenant_id = ? AND role <> ? AND (language_group_id = '' OR language_group_id IS NULL)", tenant, defs.UserRole_Admin).Update("language_group_id", id).Error; err != nil {
				return err
			}
			var servers []Server
			if err = tx.Where("tenant_id = ? OR server_id = ?", tenant, defs.DefaultServerID).Find(&servers).Error; err != nil {
				return err
			}
			for _, server := range servers {
				if err = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&LanguageGroupServer{LanguageGroupID: id, ServerID: server.ServerID}).Error; err != nil {
					return err
				}
			}
			for _, model := range []interface{}{&InviteCode{}, &UserGroup{}} {
				if err = tx.Model(model).Where("tenant_id = ? AND (language_group_id = '' OR language_group_id IS NULL)", tenant).Update("language_group_id", id).Error; err != nil {
					return err
				}
			}
			marker.Value = uuid.NewString()
			if err = tx.Create(&marker).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
