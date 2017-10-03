package commands

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tmlibs/log"
)

// ResetAllCmd removes the database of this Tendermint core
// instance.
var ResetAllCmd = &cobra.Command{
	Use:   "unsafe_reset_all",
	Short: "(unsafe) Remove all the data and WAL, reset this node's validator",
	Run:   resetAll,
}

// ResetPrivValidatorCmd resets the private validator files.
var ResetPrivValidatorCmd = &cobra.Command{
	Use:   "unsafe_reset_priv_validator",
	Short: "(unsafe) Reset this node's validator",
	Run:   resetPrivValidator,
}

// ResetAll removes the privValidator files.
// Exported so other CLI tools can use it.
func ResetAll(dbDir, privValFile string, logger log.Logger) {
	resetPrivValidatorFS(privValFile, logger)
	if err := os.RemoveAll(dbDir); err != nil {
		logger.Error("Error removing directory", "err", err)
		return
	}
	logger.Info("Removed all data", "dir", dbDir)
}

// XXX: this is totally unsafe.
// it's only suitable for testnets.
func resetAll(cmd *cobra.Command, args []string) {
	ResetAll(config.DBDir(), config.PrivValidatorFile(), logger)
}

// XXX: this is totally unsafe.
// it's only suitable for testnets.
func resetPrivValidator(cmd *cobra.Command, args []string) {
	resetPrivValidatorFS(config.PrivValidatorFile(), logger)
}

func resetPrivValidatorFS(privValFile string, logger log.Logger) {
	// Get PrivValidator
	if _, err := os.Stat(privValFile); err == nil {
		privValidator := types.LoadPrivValidatorFS(privValFile)
		privValidator.Reset()
		logger.Info("Reset PrivValidator", "file", privValFile)
	} else {
		privValidator := types.GenPrivValidatorFS(privValFile)
		privValidator.Save()
		logger.Info("Generated PrivValidator", "file", privValFile)
	}
}
