package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	tmflags "github.com/tendermint/tendermint/cmd/tendermint/commands/flags"
	cfg "github.com/tendermint/tendermint/config"
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
		config.SetRoot(config.RootDir)
		cfg.EnsureRoot(config.RootDir)
		if tmflags.IsLogLevelSimple(config.LogLevel) {
			var option log.Option
			switch config.LogLevel {
			case "info":
				option = log.AllowInfo()
			case "debug":
				option = log.AllowDebug()
			case "error":
				option = log.AllowError()
			case "none":
				option = log.AllowNone()
			default:
				return fmt.Errorf("Expected log level to be either \"info\", \"debug\", \"error\" or \"none\", given %s", config.LogLevel)
			}
			logger = log.NewFilter(logger, option)
		}
		return err
	},
}
