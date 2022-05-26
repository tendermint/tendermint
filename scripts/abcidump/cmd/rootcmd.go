package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/cmd/tenderdash/commands"
	"github.com/tendermint/tendermint/libs/log"
)

var logger log.Logger

// MakeRootCmd constructs the root command-line entry point for Tendermint core.
func MakeRootCmd() *cobra.Command {
	var (
		logLevel string
		trace    bool
	)

	cmd := &cobra.Command{
		Use:   "abcidump",
		Short: "Parse dump of protobuf communication between two nodes",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
			if cmd.Name() == commands.VersionCmd.Name() {
				return nil
			}

			// logger, err = log.NewDefaultLogger(log.LogFormatText, logLevel, trace) // TODO uncomment in 0.8
			// if err != nil {
			// 	return err
			// }
			allowLevel, err := log.AllowLevel(logLevel)
			if err != nil {
				return fmt.Errorf("invalid log level %s: %w", logLevel, err)
			}

			logger = log.NewFilter(log.NewTMLogger(log.NewSyncWriter(cmd.ErrOrStderr())), allowLevel)
			logger = logger.With("module", "main")
			return nil
		},
	}
	cmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level") // TODO uncomment in 0.8
	cmd.PersistentFlags().BoolVar(&trace, "trace", false, "")

	return cmd
}
