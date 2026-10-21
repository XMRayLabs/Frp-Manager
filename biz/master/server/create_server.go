package server

import (
	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"strings"
)

func InitServerHandler(c *app.Context, req *pb.InitServerRequest) (*pb.InitServerResponse, error) {
	var (
		userServerID = req.GetServerId()
		serverIP     = req.GetServerIp()
		userInfo     = common.GetUserInfo(c)
	)

	if !userInfo.Valid() {
		return &pb.InitServerResponse{
			Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: "invalid user"},
		}, nil
	}

	if serverIP == "" {
		if ginCtx, ok := c.Context.(*gin.Context); ok {
			serverIP = ginCtx.ClientIP()
		}
	}
	if len(userServerID) == 0 || len(serverIP) == 0 || !utils.IsClientIDPermited(userServerID) {
		return &pb.InitServerResponse{
			Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: "request invalid"},
		}, nil
	}

	secret := uuid.NewString()
	if token, ok := c.Value(defs.TokenKey).(string); ok && strings.HasPrefix(token, models.EnrollmentTokenPrefix) {
		secret = req.GetDeviceSecret()
		if err := models.ValidateEnrollmentDeviceSecret(secret); err != nil {
			return nil, err
		}
	}
	globalServerID := app.GlobalClientID(userInfo.GetUserName(), "s", userServerID)

	if err := dao.NewMutation(c).CreateServer(userInfo,
		&models.ServerEntity{
			ServerID:      globalServerID,
			TenantID:      userInfo.GetTenantID(),
			UserID:        userInfo.GetUserID(),
			ConnectSecret: secret,
			ServerIP:      serverIP,
		}); err != nil {
		return nil, err
	}

	return &pb.InitServerResponse{
		Status:   &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "ok"},
		ServerId: &globalServerID,
	}, nil
}
