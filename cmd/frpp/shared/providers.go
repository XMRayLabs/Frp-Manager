package shared

import (
	"context"
	"crypto/tls"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	bizcommon "github.com/Sakurame1/frp-manager/biz/common"
	"github.com/Sakurame1/frp-manager/conf"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/api"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/dao"
	"github.com/Sakurame1/frp-manager/services/master"
	"github.com/Sakurame1/frp-manager/services/mux"
	"github.com/Sakurame1/frp-manager/services/rbac"
	"github.com/Sakurame1/frp-manager/services/rpc"
	"github.com/Sakurame1/frp-manager/services/watcher"
	"github.com/Sakurame1/frp-manager/services/wg"
	"github.com/Sakurame1/frp-manager/services/workerd"
	"github.com/Sakurame1/frp-manager/utils"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/Sakurame1/frp-manager/utils/wsgrpc"
	"github.com/casbin/casbin/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/websocket"
	"github.com/kardianos/service"
	"go.uber.org/fx"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Finish struct {
	fx.Out

	Context context.Context
}

func NewLogHookManager() app.StreamLogHookMgr {
	return &bizcommon.HookMgr{}
}

func NewBaseApp(param struct {
	fx.In

	Cfg     conf.Config `name:"originConfig"`
	CliMgr  app.ClientsManager
	HookMgr app.StreamLogHookMgr
	PtyMgr  app.ShellPTYMgr
}) app.Application {
	appInstance := app.NewApp()
	appInstance.SetConfig(param.Cfg)
	appInstance.SetClientsManager(param.CliMgr)
	appInstance.SetStreamLogHookMgr(param.HookMgr)
	appInstance.SetShellPTYMgr(param.PtyMgr)
	appInstance.SetClientRecvMap(&sync.Map{})
	appInstance.SetNetworkTopologyCache(wg.NewNetworkTopologyCache())
	return appInstance
}

func NewPatchedConfig(param struct {
	fx.In

	AppInstance app.Application
	CommonArgs  CommonArgs
}) conf.Config {
	patchedCfg := patchConfig(param.AppInstance, param.CommonArgs)
	param.AppInstance.SetConfig(patchedCfg)

	return patchedCfg
}

func NewContext(appInstance app.Application) *app.Context {
	return app.NewContext(context.Background(), appInstance)
}

func NewAndFinishNormalContext(param struct {
	fx.In

	Ctx *app.Context
	Cfg conf.Config
}) Finish {

	return Finish{
		Context: param.Ctx,
	}
}

func NewDBManager(ctx *app.Context, appInstance app.Application) app.DBManager {
	logger.Logger(ctx).Infof("start to init database, type: %s", appInstance.GetConfig().DB.Type)
	mgr := models.NewDBManager(appInstance.GetConfig().DB.Type)
	appInstance.SetDBManager(mgr)

	if appInstance.GetConfig().IsDebug {
		appInstance.GetDBManager().SetDebug(true)
	}

	switch appInstance.GetConfig().DB.Type {
	case defs.DBTypeSQLite3:
		if err := utils.EnsureDirectoryExists(appInstance.GetConfig().DB.DSN); err != nil {
			logger.Logger(ctx).WithError(err).Warnf("ensure directory failed, data location: [%s], keep data in current directory",
				appInstance.GetConfig().DB.DSN)
			tmpCfg := appInstance.GetConfig()
			tmpCfg.DB.DSN = filepath.Base(appInstance.GetConfig().DB.DSN)
			appInstance.SetConfig(tmpCfg)
			logger.Logger(ctx).Infof("new data location: [%s]", appInstance.GetConfig().DB.DSN)
		}

		if sqlitedb, err := gorm.Open(sqlite.Open(appInstance.GetConfig().DB.DSN), &gorm.Config{}); err != nil {
			logger.Logger(ctx).Panic(err)
		} else {
			appInstance.GetDBManager().SetDB(defs.DBTypeSQLite3, defs.DBRoleDefault, sqlitedb)
			logger.Logger(ctx).Infof("init database success, data location: [%s]", appInstance.GetConfig().DB.DSN)
		}
	case defs.DBTypeMysql:
		if mysqlDB, err := gorm.Open(mysql.Open(appInstance.GetConfig().DB.DSN), &gorm.Config{}); err != nil {
			logger.Logger(ctx).Panic(err)
		} else {
			appInstance.GetDBManager().SetDB(defs.DBTypeMysql, defs.DBRoleDefault, mysqlDB)
			logger.Logger(ctx).Infof("init database success, data type: [%s]", "mysql")
		}
	case defs.DBTypePostgres:
		if postgresDB, err := gorm.Open(postgres.Open(appInstance.GetConfig().DB.DSN), &gorm.Config{}); err != nil {
			logger.Logger(ctx).Panic(err)
		} else {
			appInstance.GetDBManager().SetDB(defs.DBTypePostgres, defs.DBRoleDefault, postgresDB)
			logger.Logger(ctx).Infof("init database success, data type: [%s]", "postgres")
		}
	default:
		logger.Logger(ctx).Panicf("currently unsupported database type: %s", appInstance.GetConfig().DB.Type)
	}

	memoryDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		logger.Logger(ctx).Panic(err)
	}
	appInstance.GetDBManager().SetDB(defs.DBTypeSQLite3, defs.DBRoleRam, memoryDB)
	logger.Logger(ctx).Infof("init memory database success")

	appInstance.GetDBManager().Init()
	return mgr
}

func NewMasterTLSConfig(ctx *app.Context) *tls.Config {
	return dao.NewMutation(ctx).InitCert(conf.GetCertTemplate(ctx.GetApp().GetConfig()))
}

func NewTLSMasterService(appInstance app.Application, masterTLSConfig *tls.Config) master.MasterService {
	return master.NewMasterService(appInstance, credentials.NewTLS(masterTLSConfig))
}

func NewHTTPMasterService(appInstance app.Application) master.MasterService {
	return master.NewMasterService(appInstance, insecure.NewCredentials())
}

func NewMux(param struct {
	fx.In

	MasterService master.MasterService `name:"tlsMasterService"`
	Router        *gin.Engine          `name:"masterRouter"`
	LisOpt        conf.LisOpt
	TLSCfg        *tls.Config
}) mux.MuxServer {
	return mux.NewMux(param.MasterService.GetServer(), param.Router, param.LisOpt.MuxLis, param.TLSCfg)
}

func NewHTTPMux(param struct {
	fx.In

	MasterService master.MasterService `name:"httpMasterService"`
	Router        *gin.Engine          `name:"masterRouter"`
	LisOpt        conf.LisOpt
}) mux.MuxServer {
	return mux.NewMux(param.MasterService.GetServer(), param.Router, param.LisOpt.ApiLis, nil)
}

func NewWatcher() watcher.Client {
	return watcher.NewClient()
}

func NewWSListener(ctx *app.Context, cfg conf.Config) *wsgrpc.WSListener {
	return wsgrpc.NewWSListener("ws-listener", "wsgrpc", 100)
}

func NewWSGrpcHandler(ctx *app.Context, ws *wsgrpc.WSListener, upgrader *websocket.Upgrader) gin.HandlerFunc {
	return wsgrpc.GinWSHandler(ws, upgrader)
}

func NewWSUpgrader(ctx *app.Context, cfg conf.Config) *websocket.Upgrader {
	return &websocket.Upgrader{
		CheckOrigin: utils.SameOriginWebSocketCheck,
	}
}

func NewServerAPI(param struct {
	fx.In
	Ctx          *app.Context
	ServerRouter *gin.Engine `name:"serverRouter"`
}) app.Service {
	l, err := net.Listen("tcp", conf.ServerAPIListenAddr(param.Ctx.GetApp().GetConfig()))
	if err != nil {
		logger.Logger(param.Ctx).WithError(err).Fatalf("failed to listen addr: %v", conf.ServerAPIListenAddr(param.Ctx.GetApp().GetConfig()))
		return nil
	}

	return api.NewApiService(l, param.ServerRouter, true)
}

func NewServerCred(appInstance app.Application) credentials.TransportCredentials {
	cfg := appInstance.GetConfig()
	clientID := cfg.Client.ID
	clientSecret := cfg.Client.Secret
	ctx := context.Background()

	cred := waitForClientCredentials(ctx, appInstance, clientID, clientSecret, pb.ClientType_CLIENT_TYPE_FRPS, cfg.Client.TLSInsecureSkipVerify)
	logger.Logger(ctx).Infof("new tls server cert success")

	return cred
}

func NewClientCred(appInstance app.Application) credentials.TransportCredentials {
	cfg := appInstance.GetConfig()
	clientID := cfg.Client.ID
	clientSecret := cfg.Client.Secret
	ctx := context.Background()

	cred := waitForClientCredentials(ctx, appInstance, clientID, clientSecret, pb.ClientType_CLIENT_TYPE_FRPC, cfg.Client.TLSInsecureSkipVerify)
	logger.Logger(ctx).Infof("new tls client cert success")

	return cred
}

func waitForClientCredentials(
	ctx context.Context,
	appInstance app.Application,
	clientID string,
	clientSecret string,
	clientType pb.ClientType,
	insecureSkipVerify bool,
) credentials.TransportCredentials {
	for {
		cert, err := rpc.GetClientCert(appInstance, clientID, clientSecret, clientType)
		var cred credentials.TransportCredentials
		if err == nil {
			if insecureSkipVerify {
				cred, err = utils.TLSClientCertNoValidate(cert)
			} else {
				cred, err = utils.TLSClientCert(cert)
			}
		}
		if err == nil {
			return cred
		}
		if service.Interactive() {
			logger.Logger(ctx).WithError(err).Fatal("new TLS client certificate failed")
		}
		logger.Logger(ctx).WithError(err).Warn("new TLS client certificate failed; retrying in 3 seconds")
		time.Sleep(3 * time.Second)
	}
}

func NewDefaultServerConfig(ctx *app.Context) conf.Config {
	appInstance := ctx.GetApp()

	logger.Logger(ctx).Infof("init default internal server")

	dao.NewMutation(ctx).InitDefaultServer(appInstance.GetConfig().Master.APIHost)
	defaultServer, err := dao.NewQuery(ctx).GetDefaultServer()

	if err != nil {
		logger.Logger(ctx).WithError(err).Fatal("get default server failed")
	}

	tmpCfg := appInstance.GetConfig()
	tmpCfg.Client.ID = defaultServer.ServerID
	tmpCfg.Client.Secret = defaultServer.ConnectSecret
	appInstance.SetConfig(tmpCfg)

	return tmpCfg
}

const splitter = "\n--------------------------------------------\n"

func NewConfigPrinter(param struct {
	fx.In

	Ctx    *app.Context
	Config conf.Config
}) {
	var (
		ctx    = param.Ctx
		config = param.Config
	)
	logger.Logger(ctx).Infof("%srunning config is: %s%s", splitter, config.PrintStr(), splitter)
	logger.Logger(ctx).Infof("%scurrent version: \n%s%s", splitter, conf.GetVersion().String(), splitter)
}

func NewAutoJoin(param struct {
	fx.In
	Role       defs.AppRole
	Ctx        *app.Context
	Cfg        conf.Config `name:"argsPatchedConfig"`
	CommonArgs CommonArgs
}) (conf.Config, error) {
	cfg, err := autoJoinNode(param.Cfg, param.CommonArgs, param.Role)
	if err != nil {
		return cfg, err
	}
	param.Ctx.GetApp().SetConfig(cfg)
	return cfg, nil
}

func NewPermissionManager(param struct {
	fx.In

	Enforcer    *casbin.Enforcer
	AppInstance app.Application
}) app.PermissionManager {
	permMgr := rbac.NewPermManager(param.Enforcer)
	param.AppInstance.SetPermManager(permMgr)
	return permMgr
}

func NewEnforcer(param struct {
	fx.In

	Ctx         *app.Context
	DBmanager   app.DBManager
	AppInstance app.Application
}) *casbin.Enforcer {
	e, err := rbac.InitializeCasbin(param.Ctx, param.DBmanager.GetDefaultDB())
	if err != nil {
		logger.Logger(param.Ctx).WithError(err).Fatal("initialize casbin failed")
	}
	param.AppInstance.SetEnforcer(e)
	return e
}

func NewWorkersManager(lx fx.Lifecycle, mgr app.WorkerExecManager, appInstance app.Application) app.WorkersManager {
	if !appInstance.GetConfig().Client.Features.EnableFunctions {
		return nil
	}

	workerMgr := workerd.NewWorkersManager()
	appInstance.SetWorkersManager(workerMgr)

	lx.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			workerMgr.StopAllWorkers(app.NewContext(ctx, appInstance))
			logger.Logger(ctx).Info("stop all workers")
			return nil
		},
	})

	return workerMgr
}

func NewWorkerExecManager(cfg conf.Config, appInstance app.Application) app.WorkerExecManager {
	if !appInstance.GetConfig().Client.Features.EnableFunctions {
		return nil
	}

	workerdBinPath := cfg.Client.Worker.WorkerdBinaryPath

	if err := os.MkdirAll(cfg.Client.Worker.WorkerdWorkDir, 0755); err != nil {
		logger.Logger(context.Background()).WithError(err).Errorf(
			"functions disabled because the workerd directory could not be created, path: [%s]",
			cfg.Client.Worker.WorkerdWorkDir,
		)
		runtimeCfg := appInstance.GetConfig()
		runtimeCfg.Client.Features.EnableFunctions = false
		appInstance.SetConfig(runtimeCfg)
		return nil
	}

	mgr := workerd.NewExecManager(workerdBinPath,
		[]string{"serve", "--watch", "--verbose"})
	appInstance.SetWorkerExecManager(mgr)
	return mgr
}

func NewWireGuardManager(appInstance app.Application) app.WireGuardManager {
	return wg.NewWireGuardManager(appInstance)
}
