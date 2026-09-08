package proxy

import (
	"time"

	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/samber/lo"
)

func GetProxyConfig(c *app.Context, req *pb.GetProxyConfigRequest) (*pb.GetProxyConfigResponse, error) {
	var (
		userInfo  = common.GetUserInfo(c)
		clientID  = req.GetClientId()
		serverID  = req.GetServerId()
		proxyName = req.GetName()
	)

	proxyConfig, err := dao.NewQuery(c).GetProxyConfigByFilter(userInfo, &models.ProxyConfigEntity{
		ClientID: clientID,
		ServerID: serverID,
		Name:     proxyName,
	})
	if err != nil {
		logger.Logger(c).WithError(err).Errorf("cannot get proxy config, client: [%s], server: [%s], proxy name: [%s]", clientID, serverID, proxyName)
		return nil, err
	}

	workingStatus := proxyStatus("stopped", "")
	if !proxyConfig.Stopped {
		cachedStatus, ok := loadProxyStatus(uint32(proxyConfig.ID), time.Now())
		if ok {
			workingStatus = cachedStatus
		} else {
			workingStatus = proxyStatus("unknown", "")
			go collectProxyStatuses(c.Background(), userInfo.GetUserName(), []*models.ProxyConfig{proxyConfig})
		}
	}

	return &pb.GetProxyConfigResponse{
		Status: &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "success"},
		ProxyConfig: &pb.ProxyConfig{
			Id:             lo.ToPtr(uint32(proxyConfig.ID)),
			Name:           lo.ToPtr(proxyConfig.Name),
			Type:           lo.ToPtr(proxyConfig.Type),
			ClientId:       lo.ToPtr(proxyConfig.ClientID),
			ServerId:       lo.ToPtr(proxyConfig.ServerID),
			Config:         lo.ToPtr(string(proxyConfig.Content)),
			OriginClientId: lo.ToPtr(proxyConfig.OriginClientID),
			Stopped:        lo.ToPtr(proxyConfig.Stopped),
		},
		WorkingStatus: workingStatus,
	}, nil
}
