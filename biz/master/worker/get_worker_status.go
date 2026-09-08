package worker

import (
	"context"
	"fmt"
	"maps"
	"time"

	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/services/rpc"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/samber/lo"
	"github.com/sourcegraph/conc/pool"
)

func GetWorkerStatus(ctx *app.Context, req *pb.GetWorkerStatusRequest) (*pb.GetWorkerStatusResponse, error) {
	var (
		workerID = req.GetWorkerId()
		userInfo = common.GetUserInfo(ctx)
	)

	if len(workerID) == 0 {
		logger.Logger(ctx).Errorf("worker id is empty")
		return nil, fmt.Errorf("worker id is empty")
	}

	workerRecord, err := dao.NewQuery(ctx).GetWorkerByWorkerID(userInfo, workerID)
	if err != nil {
		logger.Logger(ctx).WithError(err).Errorf("get worker by id failed")
		return nil, err
	}

	clientIds := lo.Map(workerRecord.Clients, func(cli models.Client, _ int) string {
		return cli.ClientID
	})

	statusPool := pool.NewWithResults[*pb.GetWorkerStatusResponse]().
		WithErrors().
		WithMaxGoroutines(16)
	for _, clientID := range clientIds {
		statusPool.Go(func() (*pb.GetWorkerStatusResponse, error) {
			callCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			bgCtx := app.NewContext(callCtx, ctx.GetApp())
			cliResp := &pb.GetWorkerStatusResponse{}
			err := rpc.CallClientWrapper(bgCtx, clientID, pb.Event_EVENT_GET_WORKER_STATUS, &pb.GetWorkerStatusRequest{
				WorkerId: lo.ToPtr(workerID),
			}, cliResp)
			return cliResp, err
		})
	}

	resps, err := statusPool.Wait()
	if err != nil {
		logger.Logger(ctx).WithError(err).Warnf("get worker status failed")
	}

	statusMap := map[string]string{}

	for _, r := range resps {
		s := r.GetWorkerStatus()
		maps.Copy(statusMap, s)
	}

	return &pb.GetWorkerStatusResponse{
		Status:       &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "ok"},
		WorkerStatus: statusMap,
	}, nil
}
