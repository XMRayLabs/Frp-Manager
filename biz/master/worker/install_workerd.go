package worker

import (
	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/services/rpc"
	"github.com/Sakurame1/frp-manager/utils/logger"
)

func InstallWorkerd(ctx *app.Context, req *pb.InstallWorkerdRequest) (*pb.InstallWorkerdResponse, error) {
	var (
		userInfo = common.GetUserInfo(ctx)
		clientId = req.GetClientId()
	)
	logger.Logger(ctx).Infof("installw orkerd called with userInfo: %v, clientId: %s", userInfo, clientId)

	_, err := dao.NewQuery(ctx).GetClientByClientID(userInfo, clientId)
	if err != nil {
		logger.Logger(ctx).WithError(err).Errorf("failed to get client by clientID: %s", clientId)
		return nil, err
	}

	resp := &pb.InstallWorkerdResponse{}
	if err := rpc.CallClientWrapper(ctx, clientId, pb.Event_EVENT_INSTALL_WORKERD, req, resp); err != nil {
		logger.Logger(ctx).WithError(err).Errorf("failed to call install workerd with clientId: %s", clientId)
		return nil, err
	}
	logger.Logger(ctx).Infof("install workerd success with clientId: %s", clientId)

	return &pb.InstallWorkerdResponse{
		Status: &pb.Status{
			Code:    pb.RespCode_RESP_CODE_SUCCESS,
			Message: "ok",
		},
	}, nil
}
