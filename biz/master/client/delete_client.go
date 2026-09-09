package client

import (
	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/utils/logger"
)

func DeleteClientHandler(ctx *app.Context, req *pb.DeleteClientRequest) (*pb.DeleteClientResponse, error) {
	logger.Logger(ctx).Infof("delete client, req: [%+v]", req)

	userInfo := common.GetUserInfo(ctx)
	clientID := req.GetClientId()

	if !userInfo.Valid() {
		return &pb.DeleteClientResponse{
			Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: "invalid user"},
		}, nil
	}

	if len(clientID) == 0 {
		return &pb.DeleteClientResponse{
			Status: &pb.Status{Code: pb.RespCode_RESP_CODE_INVALID, Message: "invalid client id"},
		}, nil
	}

	if err := dao.NewMutation(ctx).DeleteClient(userInfo, clientID); err != nil {
		return nil, err
	}

	return &pb.DeleteClientResponse{
		Status: &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "ok"},
	}, nil
}
