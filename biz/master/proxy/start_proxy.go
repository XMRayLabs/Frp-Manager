package proxy

import (
	"github.com/Sakurame1/frp-manager/biz/master/client"
	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/utils/logger"
	v1 "github.com/fatedier/frp/pkg/config/v1"
	"github.com/samber/lo"
)

func StartProxy(ctx *app.Context, req *pb.StartProxyRequest) (*pb.StartProxyResponse, error) {

	var (
		userInfo  = common.GetUserInfo(ctx)
		clientID  = req.GetClientId()
		serverID  = req.GetServerId()
		proxyName = req.GetName()
	)

	clientEntity, err := GetClientWithMakeShadow(ctx, clientID, serverID)
	if err != nil {
		logger.Logger(ctx).WithError(err).Errorf("cannot get client, id: [%s]", clientID)
		return nil, err
	}

	_, err = dao.NewQuery(ctx).GetServerByServerID(userInfo, serverID)
	if err != nil {
		logger.Logger(ctx).WithError(err).Errorf("cannot get server, id: [%s]", serverID)
		return nil, err
	}

	proxyConfig, err := dao.NewQuery(ctx).GetProxyConfigByFilter(userInfo, &models.ProxyConfigEntity{
		ClientID: clientID,
		ServerID: serverID,
		Name:     proxyName,
	})
	if err != nil {
		logger.Logger(ctx).WithError(err).Errorf("cannot get proxy config, client: [%s], server: [%s], proxy name: [%s]", clientID, serverID, proxyName)
		return nil, err
	}

	// 1. 鏇存柊proxy鐘舵€?
	proxyConfig.Stopped = false
	err = dao.NewMutation(ctx).UpdateProxyConfig(userInfo, proxyConfig)
	if err != nil {
		logger.Logger(ctx).WithError(err).Errorf("cannot update proxy config, client: [%s], server: [%s], proxy name: [%s]", clientID, serverID, proxyName)
		return nil, err
	}

	typedProxyConfig, err := proxyConfig.GetTypedProxyConfig()
	if err != nil {
		logger.Logger(ctx).WithError(err).Errorf("cannot get typed proxy config, client: [%s], server: [%s], proxy name: [%s]", clientID, serverID, proxyName)
		return nil, err
	}

	// 2. 娣诲姞 proxy鍒癱lient
	if oldCfg, err := clientEntity.GetConfigContent(); err != nil {
		logger.Logger(ctx).WithError(err).Errorf("cannot get client config, id: [%s]", clientID)
		return nil, err
	} else {
		oldCfg.Proxies = lo.Filter(oldCfg.Proxies, func(proxy v1.TypedProxyConfig, _ int) bool {
			return proxy.GetBaseConfig().Name != proxyName
		})
		oldCfg.Proxies = append(oldCfg.Proxies, typedProxyConfig)

		if err := clientEntity.SetConfigContent(*oldCfg); err != nil {
			logger.Logger(ctx).WithError(err).Errorf("cannot set client config, id: [%s]", clientID)
			return nil, err
		}
	}

	// 3. 鏇存柊client鐨勯厤缃?
	rawCfg, err := clientEntity.MarshalJSONConfig()
	if err != nil {
		logger.Logger(ctx).WithError(err).Errorf("cannot marshal client config, id: [%s]", clientID)
		return nil, err
	}

	_, err = client.UpdateFrpcHander(ctx, &pb.UpdateFRPCRequest{
		ClientId: &clientEntity.ClientID,
		ServerId: &serverID,
		Config:   rawCfg,
		Comment:  &clientEntity.Comment,
		FrpsUrl:  &clientEntity.FrpsUrl,
	})
	if err != nil {
		logger.Logger(ctx).WithError(err).Warnf("cannot update frpc, id: [%s]", clientID)
	}

	return &pb.StartProxyResponse{
		Status: &pb.Status{
			Code:    pb.RespCode_RESP_CODE_SUCCESS,
			Message: "start proxy success",
		},
	}, nil
}
