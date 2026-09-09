package user

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func PasswordStatus(c *gin.Context) {
	user := common.GetUserInfo(c)
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, common.OK("ok").WithBody(gin.H{"mustChangePassword": user.GetSafeUserInfo().MustChangePassword}))
}

func changeInitialPassword(db *gorm.DB, user *models.UserEntity, current, password string) error {
	if !user.MustChangePassword {
		return fmt.Errorf("当前账号无需首次改密")
	}
	if !utils.CheckPasswordHash(current, user.Password) {
		return fmt.Errorf("当前密码不正确")
	}
	if password == current || strings.EqualFold(password, user.Email) {
		return fmt.Errorf("新密码不能与初始密码或邮箱相同")
	}
	if err := utils.ValidatePassword(password, user.UserName); err != nil {
		return err
	}
	hashed, err := utils.HashPassword(password)
	if err != nil {
		return err
	}
	result := db.Model(&models.User{}).Where("user_id = ? AND password = ? AND must_change_password = ?", user.UserID, user.Password, true).Updates(map[string]any{"password": hashed, "must_change_password": false})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("密码已变更，请重新登录")
	}
	return nil
}

func ChangeInitialPassword(instance app.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			CurrentPassword string `json:"currentPassword"`
			NewPassword     string `json:"newPassword"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, common.Err("invalid request"))
			return
		}
		user, ok := common.GetUserInfo(c).(*models.UserEntity)
		if !ok {
			common.ErrUnAuthorized(c, "invalid user")
			return
		}
		if err := changeInitialPassword(instance.GetDBManager().GetDefaultDB(), user, req.CurrentPassword, req.NewPassword); err != nil {
			c.JSON(http.StatusBadRequest, common.Err(err.Error()))
			return
		}
		c.JSON(http.StatusOK, common.OK("密码已修改"))
	}
}
