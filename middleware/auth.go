package middleware

import (
	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

func AuthCtx(appInstance app.Application) func(*gin.Context) {
	return func(c *gin.Context) {
		var err error
		var u *models.UserEntity
		appCtx := app.NewContext(c, appInstance)

		defer func() {
			logger.Logger(c).Debugf("finish auth user middleware")
		}()

		userID, err := cast.ToIntE(c.Value(defs.UserIDKey))
		if err != nil || userID <= 0 {
			logger.Logger(c).WithError(err).Errorf("invalid user id: %v", c.Value(defs.UserIDKey))
			common.ErrUnAuthorized(c, "token invalid")
			c.Abort()
			return
		}

		u, err = dao.NewQuery(appCtx).GetUserByUserID(userID)
		if err != nil {
			logger.Logger(c).WithError(err).Errorf("get user by user id failed, userID: [%d]", userID)
			common.ErrUnAuthorized(c, "token invalid")
			c.Abort()
			return
		}

		logger.Logger(c).Debugf("auth middleware authed user is: [%+v]", u.GetSafeUserInfo())

		if u.Valid() {
			logger.Logger(c).Debugf("set auth user to context, login success")
			c.Set(defs.UserInfoKey, u)
			c.Next()
			return
		}
		logger.Logger(c).Errorf("invalid authorization, auth ctx middleware login failed")
		common.ErrUnAuthorized(c, "token invalid")
		c.Abort()
		return
	}
}

func AuthAdmin(c *gin.Context) {
	u := common.GetUserInfo(c)
	if u == nil || u.GetRole() != defs.UserRole_Admin {
		common.ErrUnAuthorized(c, "permission denied")
		c.Abort()
		return
	}
	c.Next()
}
