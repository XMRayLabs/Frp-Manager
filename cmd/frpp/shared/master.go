package shared

import (
	"context"
	"errors"

	"github.com/Sakurame1/frp-manager/biz/master/auth"
	"github.com/Sakurame1/frp-manager/biz/master/proxy"
	"github.com/Sakurame1/frp-manager/conf"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/cache"
	"github.com/Sakurame1/frp-manager/services/master"
	"github.com/Sakurame1/frp-manager/services/mux"
	"github.com/Sakurame1/frp-manager/services/watcher"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/Sakurame1/frp-manager/utils/wsgrpc"
	"github.com/gin-gonic/gin"
	"github.com/sourcegraph/conc"
	"go.uber.org/fx"
	"google.golang.org/grpc"
)

type runMasterParam struct {
	fx.In

	Lc fx.Lifecycle

	Ctx                 *app.Context
	AppInstance         app.Application
	DBManagerMgr        app.DBManager
	HTTPMuxServer       mux.MuxServer `name:"httpMux"`
	TLSMuxServer        mux.MuxServer `name:"tlsMux"`
	MasterRouter        *gin.Engine   `name:"masterRouter"`
	ClientLogManager    app.ClientLogManager
	WsGrpcHandler       gin.HandlerFunc      `name:"wsGrpcHandler"`
	MasterService       master.MasterService `name:"wsMasterService"`
	TaskManager         watcher.Client       `name:"masterTaskManager"`
	WsListener          *wsgrpc.WSListener
	DefaultServerConfig conf.Config `name:"defaultServerConfig"`
	PermManager         app.PermissionManager
}

func runMaster(param runMasterParam) {

	param.AppInstance.SetClientLogManager(param.ClientLogManager)
	param.MasterRouter.GET("/wsgrpc", param.WsGrpcHandler)

	cache.InitCache(param.AppInstance.GetConfig())
	auth.InitAuth(param.AppInstance)

	param.TaskManager.AddCronTask("0 0 3 * * *", proxy.CollectDailyStats, param.AppInstance)
	defer param.TaskManager.Stop()

	logger.Logger(param.Ctx).Infof("start to run master")
	var wg conc.WaitGroup

	param.Lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			wg.Go(func() {
				if err := param.MasterService.GetServer().Serve(param.WsListener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
					logger.Logger(param.Ctx).WithError(err).Error("gRPC server stopped unexpectedly")
				}
			})
			wg.Go(param.TLSMuxServer.Run)
			wg.Go(param.HTTPMuxServer.Run)
			wg.Go(param.TaskManager.Run)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			param.MasterService.Stop()
			param.TLSMuxServer.Stop()
			param.HTTPMuxServer.Stop()
			param.TaskManager.Stop()
			wg.Wait()
			return nil
		},
	})
}
