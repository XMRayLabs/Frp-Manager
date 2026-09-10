package client

import (
	"context"

	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/services/rpc"
	"github.com/Sakurame1/frp-manager/utils/logger"
)

func StartFRPCHandler(ctx *app.Context, req *pb.StartFRPCRequest) (*pb.StartFRPCResponse, error) {
	logger.Logger(ctx).Infof("master get a start client request, origin is: [%+v]", req)

	userInfo := common.GetUserInfo(ctx)
	clientID := req.GetClientId()

	if !userInfo.Valid() {
		return &pb.StartFRPCResponse{
			Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: "invalid user"},
		}, nil
	}

	if len(clientID) == 0 {
		return &pb.StartFRPCResponse{
			Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: "invalid client id"},
		}, nil
	}

	cli, err := dao.NewQuery(ctx).GetClientByClientID(userInfo, clientID)
	if err != nil {
		return nil, err
	}

	client := cli.ClientEntity

	if client.ServerID != "" {
		owner, err := dao.NewQuery(ctx).GetUserByUserID(client.UserID)
		if err != nil {
			return nil, err
		}
		owner.Status = 1
		if err := dao.CanUseServer(ctx.GetApp().GetDBManager().GetDefaultDB(), owner, client.ServerID); err != nil {
			return nil, err
		}
	}
	client.Stopped = false

	if err = dao.NewMutation(ctx).UpdateClient(userInfo, client); err != nil {
		return nil, err
	}

	go func() {
		resp, err := rpc.CallClient(app.NewContext(context.Background(), ctx.GetApp()), req.GetClientId(), pb.Event_EVENT_START_FRPC, req)
		if err != nil {
			logger.Logger(context.Background()).WithError(err).Errorf("start client event send to client error, client id: [%s]", req.GetClientId())
		}

		if resp == nil {
			logger.Logger(ctx).Errorf("cannot get response, client id: [%s]", req.GetClientId())
		}
	}()

	return &pb.StartFRPCResponse{
		Status: &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "ok"},
	}, nil
}
