package commands

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	cfg "github.com/tendermint/tendermint/config/tendermint"
	"github.com/tendermint/tendermint/node"
	"github.com/tendermint/tmlibs/logger"
)

var (
	viperConfig *viper.Viper
	config      *node.Config
	log         = logger.New("module", "main")
)

func init() {
	viperConfig = cfg.GetConfig("")
}

// unmarshal viper into the Tendermint config
func getConfig() *node.Config {
	return node.ConfigFromViper(viperConfig)
}

var RootCmd = &cobra.Command{
	Use:   "tendermint",
	Short: "Tendermint Core (BFT Consensus) in Go",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// set the log level
		config = getConfig()
		logger.SetLogLevel(config.LogLevel)
	},
}

func init() {
	//parse flag and set config
	RootCmd.PersistentFlags().String("log_level", viperConfig.GetString("log_level"), "Log level")
	viperConfig.BindPFlag("log_level", RootCmd.PersistentFlags().Lookup("log_level"))
}
