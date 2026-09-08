package client

import (
	"fmt"

	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/workerd"
	"github.com/Sakurame1/frp-manager/utils/logger"
)

func CreateWorker(ctx *app.Context, req *pb.CreateWorkerRequest) (*pb.CreateWorkerResponse, error) {
	if !ctx.GetApp().GetConfig().Client.Features.EnableFunctions {
		logger.Logger(ctx).Errorf("function features are not enabled")
		return nil, fmt.Errorf("function features are not enabled")
	}

	mgr := ctx.GetApp().GetWorkersManager()

	ctrl := workerd.NewWorkerdController(req.GetWorker(), ctx.GetApp().GetConfig().Client.Worker.WorkerdWorkDir)

	if err := mgr.RunWorker(ctx, req.GetWorker().GetWorkerId(), ctrl); err != nil {
		return nil, err
	}

	logger.Logger(ctx).Infof("create worker success, id: [%s], running at: [%s]", req.GetWorker().GetWorkerId(), req.GetWorker().GetSocket().GetAddress())

	return &pb.CreateWorkerResponse{
		Status: &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "ok"},
	}, nil
}
