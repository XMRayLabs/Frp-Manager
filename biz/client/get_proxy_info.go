package client

import (
	"fmt"

	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/samber/lo"
)

func GetProxyConfig(c *app.Context, req *pb.GetProxyConfigRequest) (*pb.GetProxyConfigResponse, error) {
	workingStatus, err := getProxyWorkingStatus(c, req.GetClientId(), req.GetServerId(), req.GetName())
	if err != nil {
		return nil, err
	}

	return &pb.GetProxyConfigResponse{
		Status:        &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "success"},
		WorkingStatus: workingStatus,
	}, nil
}

func getProxyWorkingStatus(c *app.Context, clientID, serverID, proxyName string) (*pb.ProxyWorkingStatus, error) {
	ctrl := c.GetApp().GetClientController()
	cli := ctrl.Get(clientID, serverID)
	if cli == nil {
		logger.Logger(c).Errorf("cannot get client, clientID: [%s], serverID: [%s]", clientID, serverID)
		return nil, fmt.Errorf("cannot get client")
	}
	workingStatus, ok := cli.GetProxyStatus(proxyName)
	if !ok {
		logger.Logger(c).Errorf("cannot get proxy status, client: [%s], server: [%s], proxy name: [%s]", clientID, serverID, proxyName)
		return nil, fmt.Errorf("cannot get proxy status")
	}

	return &pb.ProxyWorkingStatus{
		Name:       lo.ToPtr(workingStatus.Name),
		Type:       lo.ToPtr(workingStatus.Type),
		Status:     lo.ToPtr(workingStatus.Phase),
		Err:        lo.ToPtr(workingStatus.Err),
		RemoteAddr: lo.ToPtr(workingStatus.RemoteAddr),
	}, nil
}
