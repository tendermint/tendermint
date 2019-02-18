package main

import (
	"os"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(killCmd)
	rootCmd.AddCommand(dumpCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "tendermint-debug",
	Short: "A debugging utility to kill a Tendermint process while aggregating useful data",
	Long: `A debugging utility that may be used to kill a Tendermint process while also
aggregating Tendermint process data such as the latest node state, including
consensus and networking state, go-routine state, and the node's WAL and config
information. This aggregated data is packaged into a compressed archive.

The tendermint-debug utility may also be used to continuously dump Tendermint
process data into a single location.`,
}
