package main

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "tendermintdb",
	Short: "A debugging utility to kill or watch a Tendermint process while aggregating useful data",
}

func init() {
	rootCmd.AddCommand(killCmd)
	rootCmd.AddCommand(dumpCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
