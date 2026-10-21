package dao

import (
	"fmt"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/utils"
	"gorm.io/gorm"
)

// Always consult current account state: revoked tokens and banned accounts must not
// remain valid in the process-wide token cache.
func AuthenticateFRPUser(db *gorm.DB, name, token string) (*models.UserEntity, error) {
	if name == "" || token == "" {
		return nil, fmt.Errorf("invalid FRP credentials")
	}
	var account models.User
	if err := db.Where("user_name = ?", name).First(&account).Error; err != nil || !account.Valid() || account.MustChangePassword || !utils.SecureStringEqual(account.Token, token) {
		return nil, fmt.Errorf("invalid FRP credentials")
	}
	return account.UserEntity, nil
}
