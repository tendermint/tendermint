package commands

import (
	"github.com/spf13/cobra"

	cfg "github.com/tendermint/go-config"
	logger "github.com/tendermint/go-logger"
	tmcfg "github.com/tendermint/tendermint/config/tendermint"
)

var config cfg.Config
var log = logger.New("module", "main")

// global flags
var logLevel, dataDir string

var RootCmd = &cobra.Command{
	Use:   "tendermint",
	Short: "Tendermint Core (BFT Consensus) in Go",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		config = tmcfg.GetConfig(dataDir)
		config.Set("log_level", logLevel)
		logger.SetLogLevel(logLevel)
	},
}

func init() {
	RootCmd.PersistentFlags().StringVar(&logLevel, "log_level", "notice", "Log level")
	RootCmd.PersistentFlags().StringVar(&dataDir, "data", tmcfg.GetTMRoot(""), "Directory to store data")
}
