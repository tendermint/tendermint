package main

import (
	"fmt"
	"os"

	"github.com/tendermint/tendermint/cmd/tendermint/commands"
)

func main() {
	if err := commands.RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
