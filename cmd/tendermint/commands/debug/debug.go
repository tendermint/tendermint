package debug

import (
	"github.com/spf13/cobra"
)

// DebugCmd defines the root command containing subcommands that assist in
// debugging running Tendermint processes.
var DebugCmd = &cobra.Command{
	Use:   "debug",
	Short: "A utility to kill or watch a Tendermint process while aggregating debugging data",
}

func init() {
	DebugCmd.AddCommand(killCmd)
	DebugCmd.AddCommand(dumpCmd)
}
