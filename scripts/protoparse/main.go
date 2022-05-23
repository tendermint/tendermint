package main

import (
	"os"

	"github.com/tendermint/tendermint/libs/log"
)

var logger log.Logger

func main() {
	logger = log.NewTMLogger(os.Stderr)
	// logger = log.NewDefaultLogger(log.LogFormatText, log.LogLevelInfo, false)

	rootCmd := MakeRootCmd()

	parseCmd := ParseCmd{}
	rootCmd.AddCommand(
		parseCmd.Command(),
	)

	if err := rootCmd.Execute(); err != nil {
		logger.Error(err.Error())
	}
}
