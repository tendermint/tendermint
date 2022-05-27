package main

import (
	"fmt"
	"os"

	"github.com/tendermint/tendermint/scripts/abcidump/cmd"
)

func main() {
	rootCmd := cmd.MakeRootCmd()
	parseCmd := cmd.ParseCmd{}
	captureCmd := cmd.CaptureCmd{}
	cborCmd := cmd.CborCmd{}

	rootCmd.AddCommand(
		parseCmd.Command(),
		captureCmd.Command(),
		cborCmd.Command(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
}
