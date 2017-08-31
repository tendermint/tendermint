package main

import (
	"os"

	"github.com/tendermint/tmlibs/cli"
	"github.com/tendermint/tmlibs/log"

	tc "github.com/tendermint/tendermint/cmd/tendermint/commands"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/types"
)

var (
	config = cfg.DefaultConfig()
	logger = log.NewTMLogger(log.NewSyncWriter(os.Stdout)).With("module", "main")
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

	privValidator := types.LoadOrGenPrivValidator(config.PrivValidatorFile(), logger)
	privValidator.SetSigner(types.NewDefaultSigner(privValidator.PrivKey))
	rootCmd.AddCommand(tc.NewRunNodeCmd(privValidator))

	cmd := cli.PrepareBaseCmd(rootCmd, "TM", os.ExpandEnv("$HOME/.tendermint"))
	cmd.Execute()
}
