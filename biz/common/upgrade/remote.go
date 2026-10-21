package upgrade

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Sakurame1/frp-manager/conf"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/utils/logger"
)

var remoteUpgradeMu sync.Mutex

// RemoteUpgrade acknowledges acceptance only; completion is reported by the
// device reconnecting with the new version. Download errors remain in its log.
func RemoteUpgrade(ctx *app.Context, req *pb.UpgradeFrppRequest) (*pb.UpgradeFrppResponse, error) {
	return remoteUpgrade(ctx, req, StartWithResult)
}

func remoteUpgrade(ctx *app.Context, req *pb.UpgradeFrppRequest, start func(context.Context, Options) (StartResult, error)) (*pb.UpgradeFrppResponse, error) {
	if runtime.GOOS == "android" {
		return nil, fmt.Errorf("Android 内核由 APK 管理，请下载安装新版 APK")
	}
	if ctx.GetApp().GetCurrentRole() == "master" || ctx.GetApp().GetDBManager() != nil {
		return nil, fmt.Errorf("内置服务端随管理面板更新，请更新 Docker 镜像")
	}
	version := strings.TrimSpace(req.GetVersion())
	if version == "" {
		version = "latest"
	}
	if version != "latest" {
		parsed, err := parseSemanticVersion(version)
		if err != nil {
			return nil, err
		}
		version = "v" + parsed.normalized()
	}
	if req.GetDownloadUrl() != "" || req.GetTargetPath() != "" || req.GetWorkdir() != "" || len(req.GetServiceArgs()) != 0 || req.GetGithubProxy() != "" || req.GetUseGithubProxy() || req.GetHttpProxy() != "" || (req.GetServiceName() != "" && req.GetServiceName() != "frpp") {
		return nil, fmt.Errorf("远程升级仅支持官方内核和当前设备服务")
	}
	if !remoteUpgradeMu.TryLock() {
		return nil, fmt.Errorf("此设备已有升级任务正在进行，请稍后查看版本")
	}
	opts := Options{Version: version, Backup: true, RestartService: true, ServiceName: "frpp", ClientOnly: conf.IsClientOnlyBinary(), HTTPProxy: ctx.GetApp().GetConfig().HTTP_PROXY}
	go func() {
		defer remoteUpgradeMu.Unlock()
		bg, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		logger.Logger(bg).Infof("remote core upgrade accepted, version: %s", version)
		if _, err := start(bg, opts); err != nil {
			logger.Logger(bg).WithError(err).Error("remote core upgrade failed")
		}
	}()
	return &pb.UpgradeFrppResponse{Status: &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "accepted"}}, nil
}
