package server

import (
	"context"
	"fmt"

	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/conf"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/pkg/websiteauth"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/services/rpc"
	"github.com/Sakurame1/frp-manager/utils"
	"github.com/Sakurame1/frp-manager/utils/logger"
	v1 "github.com/fatedier/frp/pkg/config/v1"
	"github.com/samber/lo"
)

func UpdateFrpsHander(c *app.Context, req *pb.UpdateFRPSRequest) (*pb.UpdateFRPSResponse, error) {
	logger.Logger(c).Infof("update frps, server: [%s]", req.GetServerId())
	var (
		serverID  = req.GetServerId()
		configStr = req.GetConfig()
		userInfo  = common.GetUserInfo(c)
	)

	if len(configStr) == 0 || len(serverID) == 0 {
		return nil, fmt.Errorf("request invalid")
	}

	q := dao.NewQuery(c)
	srv, err := q.GetServerByServerID(userInfo, serverID)
	if srv == nil || err != nil {
		logger.Logger(context.Background()).WithError(err).Errorf("cannot get server, id: [%s]", serverID)
		return nil, err
	}

	// An old daemon cannot report website route details to the panel. Require
	// an upgrade before opting into this shared gateway, leaving old servers off.
	if req.GetWebsiteBaseUrl() != "" && srv.WebsiteBaseURL == nil && serverID != defs.DefaultServerID {
		snapshot, ok := c.GetApp().GetClientsManager().GetRuntimeSnapshot(serverID)
		if !ok || !snapshot.Healthy || !websiteauth.SupportsHTTPVersion(snapshot.Version.GetGitVersion()) {
			return nil, fmt.Errorf("启用网站入口前，请让服务端在线并升级至 1.1.7 或更新版本")
		}
	}

	srvCfg, err := utils.LoadServerConfig(configStr, true)
	if srvCfg == nil || err != nil {
		logger.Logger(context.Background()).WithError(err).Errorf("cannot load server config")
		return nil, err
	}

	srvCfg.HTTPPlugins = lo.Filter(srvCfg.HTTPPlugins, func(item v1.HTTPPluginOptions, _ int) bool {
		return item.Name != defs.FRP_Plugin_Multiuser
	})
	srvCfg.HTTPPlugins = append(srvCfg.HTTPPlugins, conf.FRPsAuthOption(c.GetApp().GetConfig()))

	if err := srv.SetConfigContent(srvCfg); err != nil {
		logger.Logger(context.Background()).WithError(err).Errorf("cannot set server config")
		return nil, err
	}

	if req.WebsiteBaseUrl != nil {
		if req.GetWebsiteBaseUrl() == "" {
			srv.WebsiteBaseURL = nil
		} else {
			srv.WebsiteBaseURL = lo.ToPtr(req.GetWebsiteBaseUrl())
		}
	}
	srv.Comment = req.GetComment()
	if len(req.GetServerIp()) > 0 {
		srv.ServerIP = req.GetServerIp()
	}

	if len(req.GetFrpsUrls()) > 0 {
		srv.FrpsUrls = req.GetFrpsUrls()
	}

	if err := dao.NewMutation(c).UpdateServer(userInfo, srv); err != nil {
		logger.Logger(context.Background()).WithError(err).Errorf("cannot update server, id: [%s]", serverID)
		return nil, err
	}

	go func() {
		resp, err := rpc.CallClient(app.NewContext(context.Background(), c.GetApp()), req.GetServerId(), pb.Event_EVENT_UPDATE_FRPS, req)
		if err != nil {
			logger.Logger(context.Background()).WithError(err).Errorf("update event send to server error, server id: [%s]", req.GetServerId())
		}
		if resp == nil {
			logger.Logger(c).Errorf("cannot get response, server id: [%s]", req.GetServerId())
		}
	}()

	logger.Logger(c).Infof("update frps success, id: [%s]", serverID)
	return &pb.UpdateFRPSResponse{
		Status: &pb.Status{Code: pb.RespCode_RESP_CODE_SUCCESS, Message: "ok"},
	}, nil
}
