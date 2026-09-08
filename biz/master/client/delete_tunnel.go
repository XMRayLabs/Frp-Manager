package client

import (
	"context"
	"fmt"

	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/services/rpc"
	"github.com/Sakurame1/frp-manager/utils/logger"
)

func RemoveFrpcHandler(c *app.Context, req *pb.RemoveFRPCRequest) (*pb.RemoveFRPCResponse, error) {
	logger.Logger(c).Infof("remove frpc, req: [%+v]", req)

	var (
		clientID = req.GetClientId()
		userInfo = common.GetUserInfo(c)
	)

	if len(clientID) == 0 {
		logger.Logger(c).Errorf("invalid client id")
		return nil, fmt.Errorf("invalid client id")
	}

	_, err := dao.NewQuery(c).GetClientByClientID(userInfo, clientID)
	if err != nil {
		logger.Logger(context.Background()).WithError(err).Errorf("cannot get client, id: [%s]", clientID)
		return nil, err
	}

	err = dao.NewMutation(c).DeleteClient(userInfo, clientID)
	if err != nil {
		logger.Logger(context.Background()).WithError(err).Errorf("cannot delete client, id: [%s]", clientID)
		return nil, err
	}

	go func() {
		resp, err := rpc.CallClient(c, req.GetClientId(), pb.Event_EVENT_REMOVE_FRPC, req)
		if err != nil {
			logger.Logger(context.Background()).WithError(err).Errorf("remove event send to client error, client id: [%s]", req.GetClientId())
		}

		if resp == nil {
			logger.Logger(c).Errorf("cannot get response, client id: [%s]", req.GetClientId())
		}
	}()

	logger.Logger(c).Infof("remove frpc success, client id: [%s]", req.GetClientId())
	return &pb.RemoveFRPCResponse{
		Status: &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "ok"},
	}, nil
}
