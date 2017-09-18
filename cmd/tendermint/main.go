package main

import (
	"os"

	"github.com/tendermint/tmlibs/cli"

	cmd "github.com/tendermint/tendermint/cmd/tendermint/commands"
)

func main() {
	rootCmd := cmd.RootCmd
	rootCmd.AddCommand(
		cmd.GenValidatorCmd,
		cmd.InitFilesCmd,
		cmd.ProbeUpnpCmd,
		cmd.ReplayCmd,
		cmd.ReplayConsoleCmd,
		cmd.ResetAllCmd,
		cmd.ResetPrivValidatorCmd,
		cmd.ShowValidatorCmd,
		cmd.TestnetFilesCmd,
		cmd.VersionCmd)

	// NOTE:
	// Users wishing to:
	//	* Use an external signer for their validators
	//	* Supply an in-proc abci app
	// can copy this file and use something other than the
	// default SignerAndApp function
	signerAndApp := cmd.DefaultSignerAndApp

	// Create & start node
	rootCmd.AddCommand(cmd.NewRunNodeCmd(signerAndApp))

	cmd := cli.PrepareBaseCmd(rootCmd, "TM", os.ExpandEnv("$HOME/.tendermint"))
	cmd.Execute()
}
