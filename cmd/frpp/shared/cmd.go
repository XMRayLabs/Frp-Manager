package shared

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/Sakurame1/frp-manager/biz/common/upgrade"
	"github.com/Sakurame1/frp-manager/conf"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/pb"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/services/rpc"
	"github.com/Sakurame1/frp-manager/utils"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/joho/godotenv"
	"github.com/kardianos/service"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/fx"
)

type CommonArgs struct {
	ClientSecret *string
	ClientID     *string
	RpcUrl       *string
	ApiUrl       *string

	RpcHost           *string
	ApiHost           *string
	RpcPort           *int
	ApiPort           *int
	ApiScheme         *string
	JoinToken         *string
	EnrollmentAttempt *string

	Ephemeral *bool
}

func BuildCommand(fs embed.FS) *cobra.Command {
	cfg := conf.NewConfig()

	ConfigureLogger(cfg)

	return NewRootCmd(
		NewMasterCmd(cfg, fs),
		NewClientCmd(cfg),
		NewServerCmd(cfg),
		NewJoinCmd(),
		NewInstallServiceCmd(),
		NewUninstallServiceCmd(),
		NewStartServiceCmd(),
		NewStopServiceCmd(),
		NewRestartServiceCmd(),
		NewUpgradeCmd(cfg),
		NewUpgradeWorkerCmd(),
		NewVersionCmd(),
	)
}

func ConfigureLogger(cfg conf.Config) {
	logger.UpdateLoggerOpt(
		cfg.Logger.FRPLoggerLevel,
		cfg.Logger.DefaultLoggerLevel,
		cfg.IsDebug,
		os.Getenv("LOGGER_FILE"),
	)
}

func AddCommonFlags(commonCmd *cobra.Command) {
	commonCmd.Flags().StringP("secret", "s", "", "client secret")
	commonCmd.Flags().StringP("id", "i", "", "client id")
	commonCmd.Flags().String("rpc-url", "", "rpc url, master rpc url, scheme can be grpc/ws/wss://hostname:port")
	commonCmd.Flags().String("api-url", "", "api url, master api url, scheme can be http/https://hostname:port")
	commonCmd.Flags().String("enrollment-attempt", "", "unique attempt ID for explicit re-enrollment; reuse the same value on restart")
	commonCmd.Flags().StringP("join-token", "j", "", "your token from master, auto join with out webui")
	commonCmd.Flags().Bool("ephemeral", false, "auto join with join-token, whether the client is ephemeral, change flag to --ephemeral=false to disable")

	// deprecated start
	commonCmd.Flags().StringP("app", "a", "", "app secret")
	commonCmd.Flags().StringP("rpc-host", "r", "", "deprecated, use --rpc-url instead, rpc host, canbe ip or domain")
	commonCmd.Flags().StringP("api-host", "t", "", "deprecated, use --api-url instead, api host, canbe ip or domain")
	commonCmd.Flags().IntP("rpc-port", "c", 0, "deprecated, use --rpc-url instead, rpc port, master rpc port, scheme is grpc")
	commonCmd.Flags().IntP("api-port", "p", 0, "deprecated, use --api-url instead, api port, master api port, scheme is http/https")
	commonCmd.Flags().StringP("api-scheme", "e", "", "deprecated, use --api-url instead, api scheme, master api scheme, scheme is http/https")
	// deprecated end
}

func GetCommonArgs(cmd *cobra.Command) CommonArgs {
	var commonArgs CommonArgs

	if clientSecret, err := cmd.Flags().GetString("secret"); err == nil {
		commonArgs.ClientSecret = &clientSecret
	}

	if clientID, err := cmd.Flags().GetString("id"); err == nil {
		commonArgs.ClientID = &clientID
	}

	if rpcURL, err := cmd.Flags().GetString("rpc-url"); err == nil {
		commonArgs.RpcUrl = &rpcURL
	}

	if apiURL, err := cmd.Flags().GetString("api-url"); err == nil {
		commonArgs.ApiUrl = &apiURL
	}

	if rpcHost, err := cmd.Flags().GetString("rpc-host"); err == nil {
		commonArgs.RpcHost = &rpcHost
	}

	if apiHost, err := cmd.Flags().GetString("api-host"); err == nil {
		commonArgs.ApiHost = &apiHost
	}

	if rpcPort, err := cmd.Flags().GetInt("rpc-port"); err == nil {
		commonArgs.RpcPort = &rpcPort
	}

	if apiPort, err := cmd.Flags().GetInt("api-port"); err == nil {
		commonArgs.ApiPort = &apiPort
	}

	if apiScheme, err := cmd.Flags().GetString("api-scheme"); err == nil {
		commonArgs.ApiScheme = &apiScheme
	}

	if attempt, err := cmd.Flags().GetString("enrollment-attempt"); err == nil {
		commonArgs.EnrollmentAttempt = &attempt
	}
	if joinToken, err := cmd.Flags().GetString("join-token"); err == nil {
		commonArgs.JoinToken = &joinToken
	}

	if ephemeral, err := cmd.Flags().GetBool("ephemeral"); err == nil {
		commonArgs.Ephemeral = &ephemeral
	}

	return commonArgs
}

func NewJoinCmd() *cobra.Command {
	joinCmd := &cobra.Command{
		Use:   "join [-j join token] [-r rpc host] [-p api port] [-e api scheme]",
		Short: "join to master with token, save param to config",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			commonArgs := GetCommonArgs(cmd)

			warnDepParam(cmd)

			cli, err := JoinMaster(conf.NewConfig(), commonArgs)
			if err != nil {
				logger.Logger(ctx).Fatalf("join master failed: %s", err.Error())
			}
			saveConfig(ctx, cli, commonArgs)
		},
	}

	AddCommonFlags(joinCmd)

	return joinCmd
}

func NewMasterCmd(cfg conf.Config, fs embed.FS) *cobra.Command {
	return &cobra.Command{
		Use:   "master",
		Short: "run frp-manager manager",
		Run: func(cmd *cobra.Command, args []string) {

			warnDepParam(cmd)

			opts := []fx.Option{
				fx.StartTimeout(defs.AppStartTimeout),
				commonMod,
				masterMod,
				serverMod,
				fx.Supply(
					CommonArgs{},
					fx.Annotate(cfg, fx.ResultTags(`name:"originConfig"`)),
					fs,
					defs.AppRole_Master,
				),
				fx.Provide(fx.Annotate(NewDefaultServerConfig, fx.ResultTags(`name:"defaultServerConfig"`))),
				fx.Invoke(NewConfigPrinter),
				fx.Invoke(runMaster),
				fx.Invoke(runServer),
			}

			if !cfg.IsDebug {
				opts = append(opts, nodeErrorLogging())
			}

			run := func() {
				masterApp := fx.New(opts...)
				masterApp.Run()
				if err := masterApp.Err(); err != nil {
					logger.Logger(context.Background()).Fatalf("masterApp FX Application Error: %v", err)
				}
			}

			if srv, err := utils.CreateSystemService(defs.DefaultServiceName, args, run); err != nil {
				run()
			} else {
				srv.Run()
			}
		},
	}
}

func NewClientCmd(cfg conf.Config) *cobra.Command {
	clientCmd := &cobra.Command{
		Use:   "client [-s client secret] [-i client id] [-a app secret] [-t api host] [-r rpc host] [-c rpc port] [-p api port]",
		Short: "run managed frpc",
		Run: func(cmd *cobra.Command, args []string) {
			commonArgs := GetCommonArgs(cmd)

			warnDepParam(cmd)

			opts := []fx.Option{
				fx.StartTimeout(defs.AppStartTimeout),
				clientMod,
				commonMod,
				fx.Supply(
					commonArgs,
					fx.Annotate(cfg, fx.ResultTags(`name:"originConfig"`)),
					defs.AppRole_Client,
				),
				fx.Invoke(NewConfigPrinter),
				fx.Invoke(runClient),
			}

			if !cfg.IsDebug {
				opts = append(opts, nodeErrorLogging())
			}

			run := func() {
				if autoUpdateBeforeNodeStart(cfg, args) {
					return
				}
				clientApp := fx.New(opts...)
				clientApp.Run()
				if err := clientApp.Err(); err != nil {
					logger.Logger(context.Background()).Fatalf("clientApp FX Application Error: %v", err)
				}
			}
			if srv, err := utils.CreateSystemService(defs.DefaultServiceName, args, run); err != nil {
				run()
			} else {
				srv.Run()
			}
		},
	}

	AddCommonFlags(clientCmd)

	return clientCmd
}

func NewServerCmd(cfg conf.Config) *cobra.Command {
	serverCmd := &cobra.Command{
		Use:   "server [-s client secret] [-i client id] [-a app secret] [-r rpc host] [-c rpc port] [-p api port]",
		Short: "run managed frps",
		Run: func(cmd *cobra.Command, args []string) {
			commonArgs := GetCommonArgs(cmd)

			warnDepParam(cmd)

			opts := []fx.Option{
				fx.StartTimeout(defs.AppStartTimeout),
				serverMod,
				commonMod,
				fx.Supply(
					commonArgs,
					fx.Annotate(cfg, fx.ResultTags(`name:"originConfig"`)),
					defs.AppRole_Server,
				),
				fx.Invoke(runServer),
			}

			if !cfg.IsDebug {
				opts = append(opts, nodeErrorLogging())
			}

			run := func() {
				if autoUpdateBeforeNodeStart(cfg, args) {
					return
				}
				serverApp := fx.New(opts...)
				serverApp.Run()
				if err := serverApp.Err(); err != nil {
					logger.Logger(context.Background()).Fatalf("serverApp FX Application Error: %v", err)
				}
			}
			if srv, err := utils.CreateSystemService(defs.DefaultServiceName, args, run); err != nil {
				run()
			} else {
				srv.Run()
			}
		},
	}

	AddCommonFlags(serverCmd)

	return serverCmd
}

func NewInstallServiceCmd() *cobra.Command {
	return &cobra.Command{
		Use:                   "install",
		Short:                 "install frp-manager as service",
		DisableFlagParsing:    true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return utils.ControlSystemService(defs.DefaultServiceName, args, "install", func() {})
		},
	}
}

func NewUninstallServiceCmd() *cobra.Command {
	return &cobra.Command{
		Use:                   "uninstall",
		Short:                 "uninstall frp-manager service",
		DisableFlagParsing:    true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return utils.ControlSystemService(defs.DefaultServiceName, args, "uninstall", func() {})
		},
	}
}

func NewStartServiceCmd() *cobra.Command {
	return &cobra.Command{
		Use:                   "start",
		Short:                 "start frp-manager service",
		DisableFlagParsing:    true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return utils.ControlSystemService(defs.DefaultServiceName, args, "start", func() {})
		},
	}
}

func NewStopServiceCmd() *cobra.Command {
	return &cobra.Command{
		Use:                   "stop",
		Short:                 "stop frp-manager service",
		DisableFlagParsing:    true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return utils.ControlSystemService(defs.DefaultServiceName, args, "stop", func() {})
		},
	}
}

func NewRestartServiceCmd() *cobra.Command {
	return &cobra.Command{
		Use:                   "restart",
		Short:                 "restart frp-manager service",
		DisableFlagParsing:    true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return utils.ControlSystemService(defs.DefaultServiceName, args, "restart", func() {})
		},
	}
}

func NewUpgradeCmd(cfg conf.Config) *cobra.Command {
	upgradeCmd := &cobra.Command{
		Use:   "upgrade",
		Short: "OTA upgrade frp-manager binary (no service interruption unless restart)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			version, _ := cmd.Flags().GetString("version")
			downloadURL, _ := cmd.Flags().GetString("download-url")
			githubProxy, _ := cmd.Flags().GetString("github-proxy")
			httpProxy, _ := cmd.Flags().GetString("http-proxy")
			binPath, _ := cmd.Flags().GetString("bin")
			noBackup, _ := cmd.Flags().GetBool("no-backup")
			serviceName, _ := cmd.Flags().GetString("service-name")
			useGithubProxy, _ := cmd.Flags().GetBool("use-github-proxy")
			workDir, _ := cmd.Flags().GetString("workdir")
			restartService, _ := cmd.Flags().GetBool("restart-service")

			if useGithubProxy && len(githubProxy) == 0 {
				githubProxy = cfg.App.GithubProxyUrl
			}
			if len(httpProxy) == 0 {
				httpProxy = cfg.HTTP_PROXY
			}

			opts := upgrade.Options{
				Version:        version,
				DownloadURL:    downloadURL,
				GithubProxy:    githubProxy,
				UseGithubProxy: useGithubProxy,
				HTTPProxy:      httpProxy,
				TargetPath:     binPath,
				Backup:         !noBackup,
				ServiceName:    serviceName,
				RestartService: restartService,
				WorkDir:        workDir,
				ServiceArgs:    args,
				ClientOnly:     conf.IsClientOnlyBinary(),
			}

			res, err := upgrade.StartWithResult(ctx, opts)
			if err != nil {
				logger.Logger(ctx).Errorf("upgrade failed: %v", err)
				return err
			}

			if res.Dispatched {
				logger.Logger(ctx).Infof("upgrade dispatched to background worker, connection may drop; plan: %s", res.PlanPath)
				return nil
			}

			if restartService {
				logger.Logger(ctx).Info("upgrade completed (service restarted if applicable)")
				return nil
			}
			logger.Logger(ctx).Info("upgrade completed. to take effect, restart service/process when convenient")
			return nil
		},
	}

	upgradeCmd.Flags().StringP("version", "v", "latest", "target version, default latest")
	upgradeCmd.Flags().String("download-url", "", "custom download url (highest priority), if set will ignore github-proxy")
	upgradeCmd.Flags().Bool("use-github-proxy", false, "use an explicitly configured trusted GitHub proxy")
	upgradeCmd.Flags().String("github-proxy", "", "trusted GitHub proxy prefix")
	upgradeCmd.Flags().String("http-proxy", "", "http/https proxy for download, default HTTP_PROXY")
	upgradeCmd.Flags().String("bin", "", "binary path to overwrite, default current running binary")
	upgradeCmd.Flags().Bool("no-backup", false, "do not create .bak backup before overwrite")
	upgradeCmd.Flags().String("service-name", "frpp", "systemd service name to control")
	upgradeCmd.Flags().String("workdir", "", "upgrade worker plan/lock directory (default system temp)")
	upgradeCmd.Flags().Bool("restart-service", true, "restart service after replace (will interrupt service)")

	upgradeCmd.AddCommand(NewUpgradeStatusCmd())

	return upgradeCmd
}

func autoUpdateBeforeNodeStart(cfg conf.Config, serviceArgs []string) bool {
	if !cfg.App.AutoUpdate {
		logger.Logger(context.Background()).Info("auto update: disabled by APP_AUTO_UPDATE")
		return false
	}
	ctx := context.Background()
	stopStartup, err := upgrade.AutoUpdateOnStartup(ctx, upgrade.AutoOptions{
		CurrentVersion: conf.GetVersion().GitVersion,
		ClientOnly:     conf.IsClientOnlyBinary(),
		HTTPProxy:      cfg.HTTP_PROXY,
		ServiceArgs:    serviceArgs,
	})
	if err != nil {
		logger.Logger(ctx).WithError(err).Warn("auto update check failed; continuing with the current binary")
		return false
	}
	if stopStartup && runtime.GOOS == "windows" && service.Interactive() {
		logger.Logger(ctx).Info("auto update: exiting so the Windows replacement worker can update and relaunch")
		os.Exit(0)
	}
	return stopStartup
}

func NewUpgradeStatusCmd() *cobra.Command {
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "show last upgrade status (from workdir/status.json)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			workDir, _ := cmd.Flags().GetString("workdir")
			st, p, err := upgrade.ReadStatus(workDir)
			if err != nil {
				logger.Logger(ctx).Errorf("read upgrade status failed: %v", err)
				return err
			}
			logger.Logger(ctx).Infof("status file: %s", p)
			logger.Logger(ctx).Infof("success=%v updated_at=%s message=%s", st.Success, st.UpdatedAt.Format(time.RFC3339), st.Message)
			return nil
		},
	}
	statusCmd.Flags().String("workdir", "", "upgrade worker plan/lock directory (default system temp)")
	return statusCmd
}

func NewUpgradeWorkerCmd() *cobra.Command {
	workerCmd := &cobra.Command{
		Use:                   "__upgrade-worker",
		Short:                 "internal upgrade worker (do not call manually)",
		Hidden:                true,
		DisableFlagParsing:    false,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			planPath, _ := cmd.Flags().GetString("plan")
			if len(planPath) == 0 {
				return errors.New("missing --plan")
			}
			return upgrade.RunWorker(ctx, planPath)
		},
	}
	workerCmd.Flags().String("plan", "", "upgrade plan file path")
	_ = workerCmd.Flags().MarkHidden("plan")
	return workerCmd
}

func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version info of frp-manager",
		Long:  `All software has versions. This is frp-manager's`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(conf.GetVersion().String())
		},
	}
}

func patchConfig(appInstance app.Application, commonArgs CommonArgs) conf.Config {
	c := context.Background()
	tmpCfg := appInstance.GetConfig()

	if commonArgs.RpcHost != nil && len(*commonArgs.RpcHost) > 0 {
		tmpCfg.Master.RPCHost = *commonArgs.RpcHost
		tmpCfg.Master.APIHost = *commonArgs.RpcHost
	}

	if commonArgs.ApiHost != nil && len(*commonArgs.ApiHost) > 0 {
		tmpCfg.Master.APIHost = *commonArgs.ApiHost
	}

	if commonArgs.RpcPort != nil && *commonArgs.RpcPort > 0 {
		tmpCfg.Master.RPCPort = *commonArgs.RpcPort
	}
	if commonArgs.ApiPort != nil && *commonArgs.ApiPort > 0 {
		tmpCfg.Master.APIPort = *commonArgs.ApiPort
	}
	if commonArgs.ApiScheme != nil && len(*commonArgs.ApiScheme) > 0 {
		tmpCfg.Master.APIScheme = *commonArgs.ApiScheme
	}
	if commonArgs.ClientID != nil && len(*commonArgs.ClientID) > 0 {
		tmpCfg.Client.ID = *commonArgs.ClientID
	}
	if commonArgs.ClientSecret != nil && len(*commonArgs.ClientSecret) > 0 {
		tmpCfg.Client.Secret = *commonArgs.ClientSecret
	}

	if commonArgs.ApiUrl != nil && len(*commonArgs.ApiUrl) > 0 {
		tmpCfg.Client.APIUrl = *commonArgs.ApiUrl
	}
	if commonArgs.RpcUrl != nil && len(*commonArgs.RpcUrl) > 0 {
		tmpCfg.Client.RPCUrl = *commonArgs.RpcUrl
	}

	if lo.FromPtrOr(commonArgs.RpcPort, 0) != 0 || lo.FromPtrOr(commonArgs.ApiPort, 0) != 0 ||
		lo.FromPtrOr(commonArgs.ApiScheme, "") != "" ||
		lo.FromPtrOr(commonArgs.RpcHost, "") != "" || lo.FromPtrOr(commonArgs.ApiHost, "") != "" {
		logger.Logger(c).Warnf("deprecated env config; use API URL and RPC URL instead (rpc host: %s, rpc port: %d, api host: %s, api port: %d, api scheme: %s)",
			tmpCfg.Master.RPCHost, tmpCfg.Master.RPCPort,
			tmpCfg.Master.APIHost, tmpCfg.Master.APIPort,
			tmpCfg.Master.APIScheme)
	} else if len(tmpCfg.Client.APIUrl) > 0 || len(tmpCfg.Client.RPCUrl) > 0 {
		logger.Logger(c).Infof("env config, api url: %s, rpc url: %s", tmpCfg.Client.APIUrl, tmpCfg.Client.RPCUrl)
	}

	return tmpCfg
}

func warnDepParam(cmd *cobra.Command) {
	if appSecret, _ := cmd.Flags().GetString("app"); len(appSecret) != 0 {
		logger.Logger(context.Background()).Errorf(
			"\n鈿狅笍\n\n-a / -app / APP_SECRET 鍙傛暟宸插仠姝娇鐢紝璇峰垹闄よ鍙傛暟閲嶆柊鍚姩\n\n" +
				"The -a / -app / APP_SECRET parameter is deprecated. Please remove it and restart.\n\n")
	}
}

func SetMasterCommandIfNonePresent(rootCmd *cobra.Command) {
	cmd, _, err := rootCmd.Find(os.Args[1:])
	if err == nil && cmd.Use == rootCmd.Use && cmd.Flags().Parse(os.Args[1:]) != pflag.ErrHelp {
		args := append([]string{"master"}, os.Args[1:]...)
		rootCmd.SetArgs(args)
	}
}

func SetClientCommandIfNonePresent(rootCmd *cobra.Command) {
	cmd, _, err := rootCmd.Find(os.Args[1:])
	if err == nil && cmd.Use == rootCmd.Use && cmd.Flags().Parse(os.Args[1:]) != pflag.ErrHelp {
		args := append([]string{"client"}, os.Args[1:]...)
		rootCmd.SetArgs(args)
	}
}

func JoinMaster(cfg conf.Config, joinArgs CommonArgs) (*pb.Client, error) {
	if err := checkPullParams(joinArgs); err != nil {
		return nil, err
	}
	cfg.Client.APIUrl, cfg.Client.RPCUrl = *joinArgs.ApiUrl, *joinArgs.RpcUrl
	id := lo.FromPtr(joinArgs.ClientID)
	if id == "" {
		id = utils.GetHostnameWithIP()
	}
	id = utils.MakeClientIDPermited(id)
	get := func(id string) (*pb.Client, error) {
		response, err := rpc.GetClient(cfg, id, *joinArgs.JoinToken)
		if err != nil {
			return nil, err
		}
		if response == nil || response.GetStatus() == nil || response.GetStatus().GetCode() != pb.RespCode_RESP_CODE_SUCCESS || response.GetClient().GetId() == "" || response.GetClient().GetSecret() == "" {
			return nil, errors.New("panel returned invalid client credentials")
		}
		return response.GetClient(), nil
	}
	if node, err := get(id); err == nil {
		return node, nil
	}
	response, err := rpc.InitClient(cfg, id, *joinArgs.JoinToken, joinArgs.Ephemeral)
	if err != nil {
		return nil, err
	}
	if response == nil || response.GetStatus() == nil || response.GetStatus().GetCode() != pb.RespCode_RESP_CODE_SUCCESS || response.GetClientId() == "" {
		return nil, fmt.Errorf("register client: %s", response.GetStatus().GetMessage())
	}
	return get(response.GetClientId())
}

func saveConfig(ctx context.Context, cli *pb.Client, joinArgs CommonArgs) {
	if err := utils.EnsureDirectoryExists(defs.SysEnvPath); err != nil {
		logger.Logger(ctx).Errorf("ensure directory failed: %s", err.Error())
		return
	}

	envMap, err := godotenv.Read(defs.SysEnvPath)
	if err != nil {
		envMap = make(map[string]string)
		logger.Logger(ctx).Warnf("read env file failed, try to create: %s", err.Error())
	}

	envMap[defs.EnvClientID] = cli.GetId()
	envMap[defs.EnvClientSecret] = cli.GetSecret()
	envMap[defs.EnvClientAPIUrl] = *joinArgs.ApiUrl
	envMap[defs.EnvClientRPCUrl] = *joinArgs.RpcUrl

	if err = godotenv.Write(envMap, defs.SysEnvPath); err != nil {
		logger.Logger(ctx).Errorf("write env file failed: %s", err.Error())
		return
	}
	logger.Logger(ctx).Infof("config saved to env file: %s; you can use `frp-manager client` without arguments", defs.SysEnvPath)
}

func checkPullParams(joinArgs CommonArgs) error {
	if joinToken := joinArgs.JoinToken; joinToken == nil || len(*joinToken) == 0 {
		return errors.New("join token is empty")
	}

	var (
		apiUrlAvaliable = joinArgs.ApiUrl != nil && len(*joinArgs.ApiUrl) > 0
		rpcUrlAvaliable = joinArgs.RpcUrl != nil && len(*joinArgs.RpcUrl) > 0
	)

	if !apiUrlAvaliable {
		return errors.New("api url is empty")
	}

	if !rpcUrlAvaliable {
		return errors.New("rpc url is empty")
	}

	return nil
}

func NewRootCmd(cmds ...*cobra.Command) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "frp-manager",
		Short: "frp-manager is a frp manager QwQ",
	}

	rootCmd.AddCommand(cmds...)

	return rootCmd
}
