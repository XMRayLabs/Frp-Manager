package auth

import (
	"fmt"
	"strings"
	"time"

	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/services/app"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	RegisterInviteRequiredSettingKey = "register_invite_required"
	RegisterEnabledSettingKey        = "register_enabled"
)

func registerEnabled(ctx *app.Context) bool {
	db := ctx.GetApp().GetDBManager().GetDefaultDB()
	setting := &models.SystemSetting{}
	err := db.Where(&models.SystemSetting{
		Key:      RegisterEnabledSettingKey,
		TenantID: 0,
	}).First(setting).Error
	if err != nil {
		return ctx.GetApp().GetConfig().App.EnableRegister
	}
	return setting.Value == "true"
}

func inviteRequired(ctx *app.Context) bool {
	db := ctx.GetApp().GetDBManager().GetDefaultDB()
	setting := &models.SystemSetting{}
	err := db.Where(&models.SystemSetting{
		Key:      RegisterInviteRequiredSettingKey,
		TenantID: 0,
	}).First(setting).Error
	if err != nil {
		return true
	}
	return setting.Value != "false"
}

func consumeInviteCode(ctx *app.Context, code string) (int, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return 0, fmt.Errorf("invite code is required")
	}

	db := ctx.GetApp().GetDBManager().GetDefaultDB()
	var tenantID int
	err := db.Transaction(func(tx *gorm.DB) error {
		invite := &models.InviteCode{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where(&models.InviteCode{Code: code}).
			First(invite).Error; err != nil {
			return fmt.Errorf("invalid invite code")
		}
		if invite.Disabled {
			_ = tx.Unscoped().Delete(invite).Error
			return fmt.Errorf("invite code is disabled")
		}
		if invite.ExpiresAt != nil && time.Now().After(*invite.ExpiresAt) {
			_ = tx.Unscoped().Delete(invite).Error
			return fmt.Errorf("invite code is expired")
		}
		if invite.MaxUses > 0 && invite.UsedCount >= invite.MaxUses {
			_ = tx.Unscoped().Delete(invite).Error
			return fmt.Errorf("invite code has no remaining uses")
		}

		nextUsedCount := invite.UsedCount + 1
		if err := tx.Model(invite).Update("used_count", nextUsedCount).Error; err != nil {
			return err
		}
		if invite.MaxUses > 0 && nextUsedCount >= invite.MaxUses {
			if err := tx.Unscoped().Delete(invite).Error; err != nil {
				return err
			}
		}
		tenantID = invite.TenantID
		return nil
	})
	return tenantID, err
}
