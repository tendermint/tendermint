package commands

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
)

const ctxTimeout = 4 * time.Second

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
		return nil, fmt.Errorf("error in config file: %w", err)
	}
	return conf, nil
}

func RootCommand(conf *cfg.Config, logger log.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tendermint",
		Short: "BFT state machine replication for applications in any programming languages",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Name() == VersionCmd.Name() {
				return nil
			}
			var err error

			pconf, err := ParseConfig()
			if err != nil {
				return err
			}
			*conf = *pconf

			return nil
		},
	}
	cmd.PersistentFlags().String("log-level", conf.LogLevel, "log level")
	return cmd
}
