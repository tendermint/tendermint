package commands

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
)

var (
	config     = cfg.DefaultConfig()
	logger     = log.MustNewDefaultLogger(log.LogFormatPlain, log.LogLevelInfo, false)
	ctxTimeout = 4 * time.Second
)

func init() {
	registerFlagsRootCmd(RootCmd)
}

func registerFlagsRootCmd(cmd *cobra.Command) {
	cmd.PersistentFlags().String("log-level", config.LogLevel, "log level")
}

// ParseConfig retrieves the default environment configuration,
// sets up the Tendermint root and ensures that the root exists
func ParseConfig() (*cfg.Config, error) {
	conf := cfg.DefaultConfig()
	err := viper.Unmarshal(conf)
	if err != nil {
		return nil, err
	}
	conf.SetRoot(conf.RootDir)
	cfg.EnsureRoot(conf.RootDir)
	if err := conf.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("error in config file: %v", err)
	}
	return conf, nil
}

// RootCmd is the root command for Tendermint core.
var RootCmd = &cobra.Command{
	Use:   "tendermint",
	Short: "BFT state machine replication for applications in any programming languages",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		if cmd.Name() == VersionCmd.Name() {
			return nil
		}

		config, err = ParseConfig()
		if err != nil {
			return err
		}

		logger, err = log.NewDefaultLogger(config.LogFormat, config.LogLevel, false)
		if err != nil {
			return err
		}

		logger = logger.With("module", "main")
		return nil
	},
}
