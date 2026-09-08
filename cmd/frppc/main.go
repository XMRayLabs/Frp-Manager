package main

import (
	"os"

	"github.com/Sakurame1/frp-manager/cmd/frpp/shared"
	"github.com/Sakurame1/frp-manager/conf"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/fatedier/golib/crypto"
	"github.com/spf13/cobra"
)

func main() {
	crypto.DefaultSalt = "frp"
	logger.InitLogger()
	cobra.MousetrapHelpText = ""
	cfg := conf.NewConfig()
	shared.ConfigureLogger(cfg)

	rootCmd := shared.NewRootCmd(
		shared.NewClientCmd(cfg),
		shared.NewJoinCmd(),
		shared.NewInstallServiceCmd(),
		shared.NewUninstallServiceCmd(),
		shared.NewStartServiceCmd(),
		shared.NewStopServiceCmd(),
		shared.NewRestartServiceCmd(),
		shared.NewUpgradeCmd(cfg),
		shared.NewUpgradeWorkerCmd(),
		shared.NewVersionCmd(),
	)

	shared.SetClientCommandIfNonePresent(rootCmd)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
