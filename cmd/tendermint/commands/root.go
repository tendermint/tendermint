package commands

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tmlibs/logger"
)

var (
	config *cfg.Config
	log    = logger.New("module", "main")
)

func init() {
	config = cfg.DefaultConfig()
	RootCmd.PersistentFlags().String("log_level", config.LogLevel, "Log level")
}

var RootCmd = &cobra.Command{
	Use:   "tendermint",
	Short: "Tendermint Core (BFT Consensus) in Go",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		err := viper.Unmarshal(config)
		logger.SetLogLevel(config.LogLevel)
		return err
	},
}
