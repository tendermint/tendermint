package main

import (
	"os"

	tc "github.com/tendermint/tendermint/cmd/tendermint/commands"
	"github.com/tendermint/tmlibs/cli"

	"github.com/tendermint/tendermint/cmd/hsm/commands"
)

func main() {
	// TODO: Make it easier to build a tendermint instance from scratch.
	// All commands should be exported and it should be easy to override
	// certain aspects of a single command.
	// Probably every command should have a constructor that allows a user
	// to vary the configuration. This is at least true for run_node.go

	rootCmd := tc.RootCmd
	rootCmd.AddCommand(tc.GenValidatorCmd)
	rootCmd.AddCommand(tc.InitFilesCmd)
	rootCmd.AddCommand(tc.ProbeUpnpCmd)
	rootCmd.AddCommand(tc.ReplayCmd)
	rootCmd.AddCommand(tc.ReplayConsoleCmd)
	rootCmd.AddCommand(tc.ResetAllCmd)
	rootCmd.AddCommand(tc.ResetPrivValidatorCmd)
	rootCmd.AddCommand(tc.ShowValidatorCmd)
	rootCmd.AddCommand(tc.TestnetFilesCmd)
	rootCmd.AddCommand(tc.VersionCmd)

	rootCmd.AddCommand(commands.RunNodeCmd)

	cmd := cli.PrepareBaseCmd(rootCmd, "TM", os.ExpandEnv("$HOME/.tendermint"))
	cmd.Execute()
}
