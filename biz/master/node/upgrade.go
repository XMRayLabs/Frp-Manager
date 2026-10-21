package node

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/services/rpc"
	"github.com/samber/lo"
	"google.golang.org/protobuf/proto"
)

var releaseVersion = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$`)

// Upgrade dispatches only official core updates to authenticated physical nodes.
func Upgrade(ctx *app.Context, req *pb.UpgradeFrppRequest, kind string) (*pb.UpgradeFrppResponse, error) {
	user := common.GetUserInfo(ctx)
	if user == nil || !user.Valid() {
		return nil, fmt.Errorf("请先登录")
	}
	if kind != defs.CliTypeClient && kind != defs.CliTypeServer {
		return nil, fmt.Errorf("无效设备类型")
	}
	if kind == defs.CliTypeServer && !user.IsAdmin() {
		return nil, fmt.Errorf("仅网站管理员可升级服务端")
	}
	if len(req.GetClientIds()) == 0 || len(req.GetClientIds()) > 100 {
		return nil, fmt.Errorf("请选择 1 至 100 台设备")
	}
	version := strings.TrimSpace(req.GetVersion())
	if version == "" {
		version = "latest"
	}
	if version != "latest" {
		if !releaseVersion.MatchString(version) {
			return nil, fmt.Errorf("版本应为 latest 或正式版本号，例如 1.1.4")
		}
		if !strings.HasPrefix(version, "v") {
			version = "v" + version
		}
	}
	// Remote requests cannot select arbitrary programs, paths or service arguments.
	if req.GetDownloadUrl() != "" || req.GetTargetPath() != "" || req.GetWorkdir() != "" || len(req.GetServiceArgs()) != 0 || req.GetGithubProxy() != "" || req.GetUseGithubProxy() || req.GetHttpProxy() != "" || (req.GetServiceName() != "" && req.GetServiceName() != defs.DefaultServiceName) {
		return nil, fmt.Errorf("面板升级仅支持官方发布的内核，不支持自定义下载地址、路径或服务参数")
	}
	targets := make(map[string]*defs.Connector)
	order := make([]string, 0, len(req.GetClientIds()))
	// Check every target before sending any request, including mixed batches.
	for _, id := range req.GetClientIds() {
		if _, exists := targets[id]; exists {
			continue
		}
		if kind == defs.CliTypeClient {
			if err := dao.CanManageClient(ctx, user, id); err != nil {
				return nil, err
			}
			row, err := dao.NewQuery(ctx).AdminGetClientByClientID(id)
			if err != nil {
				return nil, err
			}
			if row.OriginClientID != "" {
				return nil, fmt.Errorf("请升级实际客户端，而不是其隧道配置节点")
			}
		} else {
			if id == defs.DefaultServerID {
				return nil, fmt.Errorf("内置服务端随管理面板更新，请更新 Docker 镜像")
			}
			row, err := dao.NewQuery(ctx).AdminGetServerByServerID(id)
			if err != nil {
				return nil, err
			}
			if row.TenantID != user.GetTenantID() {
				return nil, fmt.Errorf("无权升级此服务端")
			}
		}
		connector := ctx.GetApp().GetClientsManager().Get(id)
		if connector == nil || connector.CliType != kind {
			return nil, fmt.Errorf("设备 %s 不在线，无法发送升级请求", id)
		}
		targets[id] = connector
		order = append(order, id)
	}
	for _, id := range order {
		timeout, cancel := context.WithTimeout(ctx, 8*time.Second)
		response, err := rpc.CallConnector(app.NewContext(timeout, ctx.GetApp()), id, pb.Event_EVENT_UPGRADE_FRPP, &pb.UpgradeFrppRequest{
			ClientIds: []string{id}, Version: lo.ToPtr(version), Backup: lo.ToPtr(true), RestartService: lo.ToPtr(true), ServiceName: lo.ToPtr(defs.DefaultServiceName),
		}, targets[id])
		cancel()
		if err != nil {
			return nil, fmt.Errorf("设备 %s 升级请求未被接受（旧服务端可能需先手动升级一次）：%w", id, err)
		}
		ack := &pb.UpgradeFrppResponse{}
		if err = proto.Unmarshal(response.GetData(), ack); err != nil {
			return nil, fmt.Errorf("设备 %s 返回了无效的升级应答", id)
		}
		if ack.Status == nil || ack.GetStatus().GetCode() != pb.RespCode_RESP_CODE_SUCCESS {
			return nil, fmt.Errorf("设备 %s 拒绝升级：%s", id, ack.GetStatus().GetMessage())
		}
	}
	return &pb.UpgradeFrppResponse{Status: &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "设备已接受升级请求，完成后会重新连接并上报新版本"}}, nil
}
