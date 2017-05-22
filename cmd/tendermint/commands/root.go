package commands

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	cfg "github.com/tendermint/tendermint/config"
	tmflags "github.com/tendermint/tmlibs/cli/flags"
	"github.com/tendermint/tmlibs/log"
)

var (
	config = cfg.DefaultConfig()
	logger = log.NewTMLogger(log.NewSyncWriter(os.Stdout)).With("module", "main")
)

func init() {
	RootCmd.PersistentFlags().String("log_level", config.LogLevel, "Log level")
}

var RootCmd = &cobra.Command{
	Use:   "tendermint",
	Short: "Tendermint Core (BFT Consensus) in Go",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		err := viper.Unmarshal(config)
		if err != nil {
			return err
		}
		config.SetRoot(config.RootDir)
		cfg.EnsureRoot(config.RootDir)
		logger, err = tmflags.ParseLogLevel(config.LogLevel, logger, cfg.DefaultBaseConfig().LogLevel)
		if err != nil {
			return err
		}
		return nil
	},
}
