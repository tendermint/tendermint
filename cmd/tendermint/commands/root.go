package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/cli"
	"github.com/tendermint/tendermint/libs/log"
)

const ctxTimeout = 4 * time.Second

// ParseConfig retrieves the default environment configuration,
// sets up the Tendermint root and ensures that the root exists
func ParseConfig(conf *config.Config) (*config.Config, error) {
	if err := viper.Unmarshal(conf); err != nil {
		return nil, err
	}

	conf.SetRoot(conf.RootDir)

	if err := conf.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("error in config file: %w", err)
	}
	return conf, nil
}

// RootCommand constructs the root command-line entry point for Tendermint core.
func RootCommand(conf *config.Config, logger log.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tendermint",
		Short: "BFT state machine replication for applications in any programming languages",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Name() == VersionCmd.Name() {
				return nil
			}

			if err := cli.BindFlagsLoadViper(cmd, args); err != nil {
				return err
			}

			pconf, err := ParseConfig(conf)
			if err != nil {
				return err
			}
			*conf = *pconf
			config.EnsureRoot(conf.RootDir)
			if err := log.OverrideWithNewLogger(logger, conf.LogFormat, conf.LogLevel); err != nil {
				return err
			}
			if warning := pconf.DeprecatedFieldWarning(); warning != nil {
				logger.Info("WARNING", "deprecated field warning", warning)
			}

			return nil
		},
	}
	cmd.PersistentFlags().StringP(cli.HomeFlag, "", os.ExpandEnv(filepath.Join("$HOME", config.DefaultTendermintDir)), "directory for config and data")
	cmd.PersistentFlags().Bool(cli.TraceFlag, false, "print out full stack trace on errors")
	cmd.PersistentFlags().String("log-level", conf.LogLevel, "log level")
	cobra.OnInitialize(func() { cli.InitEnv("TM") })
	return cmd
}
