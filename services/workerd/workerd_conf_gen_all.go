package workerd

import (
	"context"
	"errors"

	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
)

func GenCapnpConfig(ctx context.Context, workerdDir string, workerList *pb.WorkerList) error {
	var hasError bool
	for _, worker := range workerList.Workers {
		if err := GenWorkerConfig(worker, workerdDir); err != nil {
			logrus.WithError(err).Error("failed to generate worker configuration")
			hasError = true
		}
	}

	logger.Logger(ctx).Infof("GenCapnpConfig has error: %v, workerList: %+v", hasError,
		lo.SliceToMap(workerList.GetWorkers(), func(w *pb.Worker) (string, bool) { return w.GetWorkerId(), true }))

	if hasError {
		return errors.New("GenCapnpConfig has error")
	}
	return nil
}
