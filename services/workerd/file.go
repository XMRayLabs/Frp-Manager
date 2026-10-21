package workerd

import (
	"context"
	"path/filepath"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/pb"
)

func WriteWorkerCodeToFile(ctx context.Context, worker *pb.Worker, workerdCWD string) error {
	return writeWorkerFile(worker, workerdCWD, filepath.Join(defs.WorkerCodePath, worker.GetCodeEntry()), worker.GetCode())
}

func CodeFilePath(ctx context.Context, worker *pb.Worker, workerdCWD string) string {
	return filepath.Join(
		WorkerCWDPath(ctx, worker, workerdCWD),
		defs.WorkerCodePath,
		worker.GetCodeEntry())
}

func WorkerCodeRootPath(ctx context.Context, worker *pb.Worker, workerdCWD string) string {
	return filepath.Join(
		WorkerCWDPath(ctx, worker, workerdCWD),
		defs.WorkerCodePath)
}

func WorkerCWDPath(ctx context.Context, worker *pb.Worker, workerdCWD string) string {
	return filepath.Join(
		workerdCWD,
		defs.WorkerInfoPath,
		worker.GetWorkerId(),
	)
}

func ConfigFilePath(ctx context.Context, worker *pb.Worker, workerdCWD string) string {
	return filepath.Join(
		WorkerCWDPath(ctx, worker, workerdCWD),
		defs.CapFileName,
	)
}
