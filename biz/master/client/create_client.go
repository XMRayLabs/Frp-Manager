package client

import (
	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/utils"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/google/uuid"
	"strings"
)

func InitClientHandler(c *app.Context, req *pb.InitClientRequest) (*pb.InitClientResponse, error) {
	userClientID := req.GetClientId()
	userInfo := common.GetUserInfo(c)

	if !userInfo.Valid() {
		return &pb.InitClientResponse{
			Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: "invalid user"},
		}, nil
	}

	if len(userClientID) == 0 || !utils.IsClientIDPermited(userClientID) {
		return &pb.InitClientResponse{
			Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: "invalid client id"},
		}, nil
	}

	secret := uuid.NewString()
	if token, ok := c.Value(defs.TokenKey).(string); ok && strings.HasPrefix(token, models.EnrollmentTokenPrefix) {
		secret = req.GetDeviceSecret()
		if err := models.ValidateEnrollmentDeviceSecret(secret); err != nil {
			return nil, err
		}
	}
	globalClientID := app.GlobalClientID(userInfo.GetUserName(), "c", userClientID)

	logger.Logger(c).Infof("start to init client, id: [%s]", globalClientID)

	if err := dao.NewMutation(c).CreateClient(userInfo,
		&models.ClientEntity{
			ClientID:      globalClientID,
			TenantID:      userInfo.GetTenantID(),
			UserID:        userInfo.GetUserID(),
			ConnectSecret: secret,
			IsShadow:      true,
			Ephemeral:     req.GetEphemeral(),
		}); err != nil {
		return &pb.InitClientResponse{Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: err.Error()}}, err
	}

	return &pb.InitClientResponse{
		Status:   &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "ok"},
		ClientId: &globalClientID,
	}, nil
}
