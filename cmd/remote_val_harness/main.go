package main

import (
	"github.com/tendermint/tendermint/cmd/remote_val_harness/commands"
)

func main() {
	rootCmd := commands.NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}
