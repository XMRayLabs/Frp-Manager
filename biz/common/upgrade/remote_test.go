package upgrade

import (
	"context"
	"testing"
	"time"

	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/samber/lo"
)

func TestRemoteUpgradeAcceptsQuicklyAndPreventsDuplicateTasks(t *testing.T) {
	a := app.NewApp()
	ctx := app.NewContext(context.Background(), a)
	started := make(chan Options, 1)
	release := make(chan struct{})
	start := func(ctx context.Context, opts Options) (StartResult, error) {
		started <- opts
		<-release
		return StartResult{}, nil
	}
	req := &pb.UpgradeFrppRequest{Version: lo.ToPtr("1.1.4")}
	ack, err := remoteUpgrade(ctx, req, start)
	if err != nil || ack.GetStatus().GetCode() != pb.RespCode_RESP_CODE_SUCCESS {
		t.Fatal(ack, err)
	}
	defer func() { close(release); remoteUpgradeMu.Lock(); remoteUpgradeMu.Unlock() }()
	select {
	case opts := <-started:
		if opts.Version != "v1.1.4" || opts.ServiceName != "frpp" || !opts.Backup || !opts.RestartService {
			t.Fatal(opts)
		}
	case <-time.After(time.Second):
		t.Fatal("upgrade not scheduled")
	}
	if _, err = remoteUpgrade(ctx, req, start); err == nil {
		t.Fatal("duplicate upgrade accepted")
	}
}
func TestRemoteUpgradeRejectsUnsafeParametersBeforeExecution(t *testing.T) {
	a := app.NewApp()
	ctx := app.NewContext(context.Background(), a)
	never := func(context.Context, Options) (StartResult, error) {
		t.Error("rejected upgrade executed")
		return StartResult{}, nil
	}
	for _, req := range []*pb.UpgradeFrppRequest{
		{Version: lo.ToPtr("../../elsewhere")},
		{DownloadUrl: lo.ToPtr("https://example.invalid/other")},
		{TargetPath: lo.ToPtr("/tmp/other")},
		{ServiceArgs: []string{"--arbitrary-argument"}},
	} {
		if _, err := remoteUpgrade(ctx, req, never); err == nil {
			t.Fatal("unsafe request accepted", req)
		}
	}
	a.SetCurrentRole("master")
	if _, err := remoteUpgrade(ctx, &pb.UpgradeFrppRequest{}, never); err == nil {
		t.Fatal("embedded master accepted")
	}
}
