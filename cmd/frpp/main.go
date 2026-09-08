package main

import (
	"embed"
	"os"

	"github.com/Sakurame1/frp-manager/cmd/frpp/shared"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/fatedier/golib/crypto"
	"github.com/spf13/cobra"
)

//go:embed all:out
var fs embed.FS

func main() {
	crypto.DefaultSalt = "frp"
	logger.InitLogger()
	cobra.MousetrapHelpText = ""

	rootCmd := shared.BuildCommand(fs)
	shared.SetMasterCommandIfNonePresent(rootCmd)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
