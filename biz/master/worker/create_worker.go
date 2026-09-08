package worker

import (
	"fmt"

	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/services/rpc"
	"github.com/Sakurame1/frp-manager/services/workerd"
	"github.com/Sakurame1/frp-manager/utils/logger"
)

func CreateWorker(ctx *app.Context, req *pb.CreateWorkerRequest) (*pb.CreateWorkerResponse, error) {
	var (
		userInfo  = common.GetUserInfo(ctx)
		clientId  = req.GetClientId()
		reqWorker = req.GetWorker()
	)
	q := dao.NewQuery(ctx)
	m := dao.NewMutation(ctx)

	if err := validateCreateWorker(req); err != nil {
		logger.Logger(ctx).WithError(err).Errorf("invalid create worker request, origin is: [%s]", req.String())
		return nil, err
	}

	cli, err := q.GetClientByClientID(userInfo, clientId)
	if err != nil {
		logger.Logger(ctx).WithError(err).Errorf("cannot get client, id: [%s], workerName: [%s]", clientId, reqWorker.GetName())
		return nil, err
	}

	workerd.FillWorkerValue(reqWorker, uint(userInfo.GetUserID()))

	workerToCreate := (&models.Worker{}).FromPB(reqWorker)
	workerToCreate.WorkerModel = nil

	workerToCreate.Clients = append(workerToCreate.Clients, *cli)

	if err := m.CreateWorker(userInfo, workerToCreate); err != nil {
		logger.Logger(ctx).WithError(err).Errorf("cannot create worker, workerName: [%s]", workerToCreate.Name)
		return nil, err
	}

	go func() {
		bgCtx := ctx.Background()
		resp := &pb.CreateWorkerResponse{}
		err := rpc.CallClientWrapper(bgCtx, clientId, pb.Event_EVENT_CREATE_WORKER, req, resp)
		if err != nil {
			logger.Logger(bgCtx).WithError(err).Errorf("create worker event send to client error, client id: [%s], worker name: [%s]", clientId, workerToCreate.Name)
		}
	}()

	logger.Logger(ctx).Infof("create worker success, workerName: [%s], start to create worker's proxy", workerToCreate.Name)
	return &pb.CreateWorkerResponse{
		Status:   &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "ok"},
		WorkerId: &workerToCreate.ID,
	}, nil
}

func validateCreateWorker(req *pb.CreateWorkerRequest) error {
	if len(req.GetClientId()) == 0 {
		return fmt.Errorf("invalid client id")
	}

	if req.GetWorker() == nil {
		return fmt.Errorf("invalid worker")
	}

	return nil
}
