package user

import (
	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/gin-gonic/gin"
	"net/http"
)

func EnrollmentTokenHandler(instance app.Application, rotate bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := common.GetUserInfo(c)
		if user == nil || !user.Valid() {
			common.ErrUnAuthorized(c, "invalid user")
			return
		}
		var req struct {
			Role         string `json:"role"`
			Confirm      bool   `json:"confirm"`
			CurrentToken string `json:"currentToken"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || (req.Role != "client" && req.Role != "server") {
			c.JSON(http.StatusBadRequest, common.Err("invalid request"))
			return
		}
		if req.Role == "server" && !user.IsAdmin() {
			common.ErrUnAuthorized(c, "admin required")
			return
		}
		if rotate && !req.Confirm {
			c.JSON(http.StatusBadRequest, common.Err("请先确认更换接入令牌"))
			return
		}
		db := instance.GetDBManager().GetDefaultDB()
		var token string
		var err error
		if rotate {
			token, err = models.RotateEnrollmentToken(db, user.GetUserID(), req.Role, req.CurrentToken)
		} else {
			token, err = models.GetEnrollmentToken(db, user.GetUserID(), req.Role)
		}
		if err != nil {
			c.JSON(http.StatusBadRequest, common.Err(err.Error()))
			return
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, common.OK(defs.ReqSuccess).WithBody(gin.H{"token": token}))
	}
}
