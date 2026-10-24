package server

import (
	"fmt"
	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/samber/lo"
	"strings"
)

func GetServerHandler(c *app.Context, req *pb.GetServerRequest) (*pb.GetServerResponse, error) {
	var (
		userServerID = req.GetServerId()
		userInfo     = common.GetUserInfo(c)
	)

	if !userInfo.Valid() {
		return &pb.GetServerResponse{
			Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: "invalid user"},
		}, nil
	}

	if len(userServerID) == 0 {
		return &pb.GetServerResponse{
			Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: "invalid client id"},
		}, nil
	}

	if userServerID != defs.DefaultServerID && !strings.Contains(userServerID, ".") {
		userServerID = app.GlobalClientID(userInfo.GetUserName(), "s", userServerID)
	}
	var serverEntity *models.ServerEntity
	var err error
	if token, ok := c.Value(defs.TokenKey).(string); ok && strings.HasPrefix(token, models.EnrollmentTokenPrefix) {
		if err := models.ValidateEnrollmentDeviceSecret(req.GetDeviceSecret()); err != nil {
			return nil, err
		}
		serverEntity, err = dao.NewQuery(c).ValidateServerSecret(userServerID, req.GetDeviceSecret())
		if err != nil || serverEntity.UserID != userInfo.GetUserID() || serverEntity.TenantID != userInfo.GetTenantID() {
			return nil, fmt.Errorf("invalid enrollment device proof")
		}
	} else {
		serverEntity, err = dao.NewQuery(c).GetServerByServerID(userInfo, userServerID)
		if err != nil {
			return nil, err
		}
	}
	return &pb.GetServerResponse{
		Status: &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "ok"},
		Server: &pb.Server{
			Id:             lo.ToPtr(serverEntity.ServerID),
			Config:         lo.ToPtr(serverConfigForUser(userInfo, serverEntity)),
			Secret:         lo.ToPtr(serverSecretForUser(userInfo, serverEntity)),
			Comment:        lo.ToPtr(serverEntity.Comment),
			Ip:             lo.ToPtr(serverEntity.ServerIP),
			FrpsUrls:       serverEntity.FrpsUrls,
			WebsiteBaseUrl: serverEntity.WebsiteBaseURL,
		},
	}, nil
}
