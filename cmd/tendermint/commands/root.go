package commands

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/tendermint/tendermint/node"
	"github.com/tendermint/tmlibs/logger"
)

var (
	viperConfig *viper.Viper
	config      *node.Config
	log         = logger.New("module", "main")
)

func init() {
	// Set config to be used as defaults by flags.
	// This will be overwritten by whatever is unmarshalled from viper
	config = node.NewDefaultConfig("")

}

// unmarshal viper into the Tendermint config
func getConfig() *node.Config {
	return node.ConfigFromViper(viperConfig)
}

//global flag
var logLevel string

var RootCmd = &cobra.Command{
	Use:   "tendermint",
	Short: "Tendermint Core (BFT Consensus) in Go",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// set the log level
		config := getConfig()
		logger.SetLogLevel(config.LogLevel)
	},
}

func init() {
	//parse flag and set config
	RootCmd.PersistentFlags().StringVar(&logLevel, "log_level", config.LogLevel, "Log level")
	viperConfig.BindPFlag("log_level", RootCmd.Flags().Lookup("log_level"))
}
