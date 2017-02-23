package commands

import (
	"github.com/spf13/cobra"

	cfg "github.com/tendermint/go-config"
	"github.com/tendermint/go-logger"
	tmcfg "github.com/tendermint/tendermint/config/tendermint"
)

var config cfg.Config
var log = logger.New("module", "main")

//global flag
var logLevel string

var RootCmd = &cobra.Command{
	Use:   "tendermint",
	Short: "Tendermint Core (BFT Consensus) in Go",
}

func init() {

	// Get configuration
	config = tmcfg.GetConfig("")

	//parse flag and set config
	RootCmd.PersistentFlags().StringVar(&logLevel, "log_level", config.GetString("log_level"), "Log level")
	config.Set("log_level", logLevel)

	// set the log level
	logger.SetLogLevel(config.GetString("log_level"))
}
