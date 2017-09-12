package main

import (
	"os"

	// crypto "github.com/tendermint/go-crypto"

	"github.com/tendermint/tmlibs/cli"

	. "github.com/tendermint/tendermint/cmd/tendermint/commands"
	// "github.com/tendermint/tendermint/types"
)

func main() {
	rootCmd := RootCmd
	rootCmd.AddCommand(GenValidatorCmd, InitFilesCmd, ProbeUpnpCmd,
		ReplayCmd, ReplayConsoleCmd, ResetAllCmd, ResetPrivValidatorCmd,
		ShowValidatorCmd, TestnetFilesCmd, VersionCmd)

	// NOTE: Implement your own type that implements the Signer interface
	// and then instantiate it here.
	// signer := types.NewDefaultSigner(pk)
	// privValidator := types.LoadPrivValidatorWithSigner(signer)
	// rootCmd.AddCommand(NewRunNodeCmd(privValidator))

	// Create & start node
	rootCmd.AddCommand(RunNodeCmd)

	cmd := cli.PrepareBaseCmd(rootCmd, "TM", os.ExpandEnv("$HOME/.tendermint"))
	cmd.Execute()
}
