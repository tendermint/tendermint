package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/cmd/tenderdash/commands"
	"github.com/tendermint/tendermint/libs/log"
)

// MakeRootCmd constructs the root command-line entry point for Tendermint core.
func MakeRootCmd() *cobra.Command {
	var (
		logLevel string
		trace    bool
	)
	_ = logLevel // TODO remove in 0.8

	cmd := &cobra.Command{
		Use:   "protoparse",
		Short: "Parse dump of protobuf communication between two nodes",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
			if cmd.Name() == commands.VersionCmd.Name() {
				return nil
			}
			logger = log.NewTMLogger(os.Stderr)

			// logger, err = log.NewDefaultLogger(log.LogFormatText, logLevel, trace) // TODO uncomment in 0.8
			if err != nil {
				return err
			}

			logger = logger.With("module", "main")
			return nil
		},
	}

	// cmd.PersistentFlags().StringVar(&logLevel, "log-level", log.LogLevelInfo, "log level") // TODO uncomment in 0.8
	cmd.PersistentFlags().BoolVar(&trace, "trace", false, "")

	return cmd
}
