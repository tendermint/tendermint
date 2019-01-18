package commands

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/tendermint/tendermint/libs/log"
)

var (
	flagVerbose bool
)

var logger = log.NewTMLogger(log.NewSyncWriter(os.Stdout))

func NewRootCmd() *cobra.Command {
	runCmd := NewRunCmd()
	versionCmd := NewVersionCmd()

	rootCmd := &cobra.Command{
		Use:   "remote_val_harness",
		Short: "Test harness for Tendermint remote validator",
		Long: `
This application provides test harness capability for integration testing with
a Tendermint remote validator (e.g. KMS: https://github.com/tendermint/kms),
where it runs a suite of tests against the remote validator to ensure
compatiblity.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Name() == versionCmd.Name() {
				return nil
			}
			if flagVerbose {
				logger = log.NewFilter(logger, log.AllowDebug())
			} else {
				logger = log.NewFilter(logger, log.AllowInfo())
			}
			return nil
		},
	}

	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "increase output logging verbosity")
	rootCmd.AddCommand(
		runCmd,
		versionCmd,
	)
	return rootCmd
}
