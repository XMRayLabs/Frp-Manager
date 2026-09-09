package conf

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/utils"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/gin-gonic/gin"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
	"github.com/tidwall/pretty"
)

type Config struct {
	App struct {
		UseGvisorNet   bool   `env:"USE_GVISOR_NET" env-default:"false" env-description:"use gvisor netstack for TUN device"`
		GlobalSecret   string `env:"GLOBAL_SECRET" env-default:"" env-description:"global secret, used in manager gen secret, keep it safe"`
		CookieAge      int    `env:"COOKIE_AGE" env-default:"86400" env-description:"cookie age in second, default is 1 day"`
		CookieName     string `env:"COOKIE_NAME" env-default:"frp-manager-cookie" env-description:"cookie name"`
		CookiePath     string `env:"COOKIE_PATH" env-default:"/" env-description:"cookie path"`
		CookieDomain   string `env:"COOKIE_DOMAIN" env-default:"" env-description:"cookie domain"`
		CookieSecure   bool   `env:"COOKIE_SECURE" env-default:"false" env-description:"cookie secure"`
		CookieHTTPOnly bool   `env:"COOKIE_HTTP_ONLY" env-default:"true" env-description:"cookie http only"`
		EnableRegister bool   `env:"ENABLE_REGISTER" env-default:"false" env-description:"enable user registration explicitly"`
		AutoUpdate     bool   `env:"AUTO_UPDATE" env-default:"true" env-description:"check and install binary updates when client or server starts"`
		GithubProxyUrl string `env:"GITHUB_PROXY_URL" env-default:"" env-description:"optional trusted GitHub proxy URL"`
		TrustedProxies string `env:"TRUSTED_PROXIES" env-default:"127.0.0.1,::1" env-description:"comma-separated reverse proxy addresses or CIDRs"`
	} `env-prefix:"APP_"`
	Master struct {
		APIPort               int    `env:"API_PORT" env-default:"9000" env-description:"master api port"`
		APIHost               string `env:"API_HOST" env-description:"master host, can behind proxy like cdn"`
		APIScheme             string `env:"API_SCHEME" env-default:"http" env-description:"master api scheme"`
		CacheSize             int    `env:"CACHE_SIZE" env-default:"10" env-description:"cache size in MB"`
		RPCHost               string `env:"RPC_HOST" env-default:"127.0.0.1" env-description:"master host, is a public ip or domain"`
		RPCPort               int    `env:"RPC_PORT" env-default:"9001" env-description:"master rpc port"`
		InternalFRPServerHost string `env:"INTERNAL_FRP_SERVER_HOST" env-description:"internal frp server host, used for client connection"`
	} `env-prefix:"MASTER_"`
	Server struct {
		APIPort int `env:"API_PORT" env-default:"8999" env-description:"server api port"`
	} `env-prefix:"SERVER_"`
	DB struct {
		Type string `env:"TYPE" env-default:"sqlite3" env-description:"db type, mysql or sqlite3 and so on"`
		DSN  string `env:"DSN" env-default:"/data/data.db?_pragma=journal_mode(WAL)" env-description:"db dsn, for sqlite is path, other is dsn, look at https://github.com/go-sql-driver/mysql#dsn-data-source-name"`
	} `env-prefix:"DB_"`
	Client struct {
		EnrollmentAttempt     string `env:"ENROLLMENT_ATTEMPT" env-description:"unique ID for an explicit enrollment attempt; keep unchanged on restart"`
		JoinToken             string `env:"JOIN_TOKEN" env-description:"panel enrollment token used only until node identity is saved"`
		ID                    string `env:"ID" env-description:"client id"`
		Secret                string `env:"SECRET" env-description:"client secret"`
		TLSRpc                bool   `env:"TLS_RPC" env-default:"true" env-description:"use tls for rpc connection"`
		RPCUrl                string `env:"RPC_URL" env-description:"rpc url, support ws or wss or grpc scheme, eg: ws://127.0.0.1:9000"`
		APIUrl                string `env:"API_URL" env-description:"api url, support http or https scheme, eg: http://127.0.0.1:9000"`
		TLSInsecureSkipVerify bool   `env:"TLS_INSECURE_SKIP_VERIFY" env-default:"false" env-description:"skip tls verify"`
		Worker                struct {
			WorkerdBinaryPath  string `env:"WORKERD_BINARY_PATH" env-description:"workerd binary path"`
			WorkerdWorkDir     string `env:"WORKERD_WORK_DIR" env-default:"/tmp/frpp/workerd" env-description:"workerd work dir"`
			WorkerdDownloadURL struct {
				UseProxy   bool   `env:"USE_PROXY" env-default:"false" env-description:"use explicitly configured trusted proxy"`
				LinuxArm64 string `env:"LINUX_ARM64" env-default:"https://github.com/cloudflare/workerd/releases/download/v1.20250505.0/workerd-linux-arm64.gz"`
				LinuxX8664 string `env:"LINUX_X86_64" env-default:"https://github.com/cloudflare/workerd/releases/download/v1.20250505.0/workerd-linux-64.gz"`
			} `env-prefix:"WORKERD_DOWNLOAD_URL_" env-description:"workerd download url"`
		} `env-prefix:"WORKER_" env-description:"worker's config"`
		Features struct {
			EnableFunctions   bool `env:"ENABLE_FUNCTIONS" env-default:"true" env-description:"enable functions"`
			EnableRemoteShell bool `env:"ENABLE_REMOTE_SHELL" env-default:"true" env-description:"enable remote shell"`
		} `env-prefix:"FEATURES_" env-description:"features config"`
	} `env-prefix:"CLIENT_"`
	IsDebug bool `env:"IS_DEBUG" env-default:"false" env-description:"is debug mode"`
	Debug   struct {
		ProfilerEnabled bool `env:"PROFILER_ENABLED" env-default:"false" env-description:"enable profiler"`
		ProfilerPort    int  `env:"PROFILER_PORT" env-default:"6961" env-description:"profiler port"`
	} `env-prefix:"DEBUG_"`
	Logger struct {
		DefaultLoggerLevel string `env:"DEFAULT_LOGGER_LEVEL" env-default:"info" env-description:"frp-manager internal default logger level"`
		FRPLoggerLevel     string `env:"FRP_LOGGER_LEVEL" env-default:"info" env-description:"frp logger level"`
		File               string `env:"FILE" env-default:"" env-description:"optional log file path"`
	} `env-prefix:"LOGGER_"`
	HTTP_PROXY string `env:"HTTP_PROXY" env-description:"http proxy"`
}

func NewConfig() Config {
	var (
		err        error
		useEnvFile bool
		ctx        = context.Background()
	)

	envFiles := []string{
		// 瓒婂墠闈紭鍏堢骇瓒婇珮锛屽悗闈㈢殑涓嶄細瑕嗙洊鍓嶉潰鐨?	envFiles := []string{
		defs.CurEnvPath,
		defs.SysEnvPath,
	}
	envFiles = prependExplicitEnvFile(envFiles, os.Getenv("FRP_MANAGER_ENV_FILE"))

	for _, envFile := range envFiles {
		if err = godotenv.Load(envFile); err == nil {
			logger.Logger(ctx).Infof("load env file success: %s", envFile)
			useEnvFile = true
		}
	}

	if count := utils.ConfigureRuntimeDNSFromEnv(); count > 0 {
		logger.Logger(ctx).Infof("using %d DNS server(s) supplied by the host platform", count)
	}

	if !useEnvFile {
		logger.Logger(ctx).Info("use runtime env variables")
	}

	cfg := Config{}
	if err = cleanenv.ReadEnv(&cfg); err != nil {
		logger.Logger(ctx).Panic(err)
	}
	cfg.Complete()

	if !cfg.IsDebug {
		gin.SetMode(gin.ReleaseMode)
	}

	return cfg
}

func prependExplicitEnvFile(envFiles []string, explicit string) []string {
	if explicit = strings.TrimSpace(explicit); explicit != "" {
		return append([]string{explicit}, envFiles...)
	}
	return envFiles
}

func (cfg *Config) Complete() {
	if strings.TrimSpace(cfg.App.GlobalSecret) == "" || cfg.App.GlobalSecret == "frp-manager" {
		secret, err := utils.SecureRandomString(48)
		if err != nil {
			logger.Logger(context.Background()).WithError(err).Fatal("failed to generate APP_GLOBAL_SECRET")
		}
		cfg.App.GlobalSecret = secret
		logger.Logger(context.Background()).Warn("APP_GLOBAL_SECRET is empty or uses the legacy default; generated a runtime secret. Set APP_GLOBAL_SECRET explicitly in production to keep sessions stable across restarts.")
	}

	if len(cfg.Master.InternalFRPServerHost) == 0 {
		cfg.Master.InternalFRPServerHost = cfg.Master.RPCHost
	}

	if len(cfg.Master.APIHost) == 0 {
		cfg.Master.APIHost = cfg.Master.RPCHost
	}

	cfg.Client.Worker.WorkerdWorkDir = platformWorkerdDir(
		runtime.GOOS,
		os.TempDir(),
		cfg.Client.Worker.WorkerdWorkDir,
	)

	hostname, err := os.Hostname()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if len(cfg.Client.ID) == 0 {
		cfg.Client.ID = hostname
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println("failed to get current working directory:", err)
		os.Exit(1)
	}

	if len(cfg.Client.Worker.WorkerdBinaryPath) == 0 {
		w, _ := utils.FindExecutableNames(func(name string) bool {
			return strings.HasPrefix(name, "workerd")
		}, cwd, "/")
		if len(w) > 0 {
			cfg.Client.Worker.WorkerdBinaryPath = w[0]
		}
	}
}

func platformWorkerdDir(goos, tempDir, configured string) string {
	if goos == "android" && configured == "/tmp/frpp/workerd" {
		return filepath.Join(tempDir, "frpp", "workerd")
	}
	return configured
}

func (cfg Config) PrintStr() string {
	cfg.App.GlobalSecret = "[redacted]"
	cfg.Client.Secret = "[redacted]"
	cfg.Client.JoinToken = "[redacted]"
	cfg.DB.DSN = "[redacted]"
	cfg.HTTP_PROXY = "[redacted]"
	raw, _ := json.Marshal(cfg)
	return string(pretty.Pretty(raw))
}
