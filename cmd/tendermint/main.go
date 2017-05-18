package main

import (
	"os"

	"github.com/tendermint/tendermint/cmd/tendermint/commands"
	"github.com/tendermint/tmlibs/cli"
)

func main() {
	cmd := cli.PrepareBaseCmd(commands.RootCmd, "TM", os.ExpandEnv("$HOME/.tendermint"))
	cmd.Execute()
}
