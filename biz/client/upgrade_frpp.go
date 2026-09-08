package client

import (
	"context"

	bizupgrade "github.com/Sakurame1/frp-manager/biz/common/upgrade"
	"github.com/Sakurame1/frp-manager/conf"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/utils/logger"
)

// UpgradeFrpp 鏀跺埌 master 涓嬪彂鐨勫崌绾ф寚浠ゅ悗锛屽紓姝ユ墽琛屽崌绾у苟蹇€?ACK銆?
// 璇存槑锛氬繀椤诲揩閫熻繑鍥?response锛岄伩鍏?master 绔?HTTP/RPC 闀挎椂闂撮樆濉烇紱
//
//	鐪熸鐨?stop/restart 灏嗙敱 upgrader service/worker 鎵ц锛堜細瀵艰嚧杩炴帴鐭殏鏂紑锛屽睘浜庨鏈燂級銆?
func UpgradeFrpp(ctx *app.Context, req *pb.UpgradeFrppRequest) (*pb.UpgradeFrppResponse, error) {
	log := logger.Logger(ctx)
	log.Infof("upgrade frpp request received, clientIds=%v version=%s downloadUrl=%s", req.GetClientIds(), req.GetVersion(), req.GetDownloadUrl())

	// 榛樿鍊煎鐞嗭紙proto3 optional 鏈缃椂 GetXXX 杩斿洖闆跺€硷級
	backup := true
	if req.Backup != nil {
		backup = req.GetBackup()
	}
	restartService := true
	if req.RestartService != nil {
		restartService = req.GetRestartService()
	}
	useGithubProxy := false
	if req.UseGithubProxy != nil {
		useGithubProxy = req.GetUseGithubProxy()
	}

	opts := bizupgrade.Options{
		Version:        req.GetVersion(),
		DownloadURL:    req.GetDownloadUrl(),
		GithubProxy:    req.GetGithubProxy(),
		UseGithubProxy: useGithubProxy,
		HTTPProxy:      req.GetHttpProxy(),
		TargetPath:     req.GetTargetPath(),
		Backup:         backup,
		ServiceName:    req.GetServiceName(),
		RestartService: restartService,
		WorkDir:        req.GetWorkdir(),
		ServiceArgs:    req.GetServiceArgs(),
		ClientOnly:     conf.IsClientOnlyBinary(),
	}

	// 寮傛鎵ц锛氱‘淇濊兘蹇€熷洖 ACK锛岄伩鍏嶈繙绋嬭Е鍙戦摼璺洜閲嶅惎/鏂繛鍗℃
	go func() {
		bg := context.Background()
		if _, err := bizupgrade.StartWithResult(bg, opts); err != nil {
			logger.Logger(bg).WithError(err).Error("upgrade frpp failed")
		}
	}()

	return &pb.UpgradeFrppResponse{
		Status: &pb.Status{
			Code:    pb.RespCode_RESP_CODE_SUCCESS,
			Message: "accepted",
		},
	}, nil
}
