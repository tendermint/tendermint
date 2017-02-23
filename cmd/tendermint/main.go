package main

import (
	"fmt"
	"os"

	cmd "github.com/tendermint/tendermint/cmd/tendermint/commands"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
