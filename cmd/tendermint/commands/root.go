package commands

import (
	"github.com/spf13/cobra"

	"github.com/tendermint/tmlibs/logger"
	tmcfg "github.com/tendermint/tendermint/config/tendermint"
)

var (
	config = tmcfg.GetConfig("")
	log    = logger.New("module", "main")
)

//global flag
var logLevel string

var RootCmd = &cobra.Command{
	Use:   "tendermint",
	Short: "Tendermint Core (BFT Consensus) in Go",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// set the log level in the config and logger
		config.Set("log_level", logLevel)
		logger.SetLogLevel(logLevel)
	},
}

func init() {
	//parse flag and set config
	RootCmd.PersistentFlags().StringVar(&logLevel, "log_level", config.GetString("log_level"), "Log level")
}
