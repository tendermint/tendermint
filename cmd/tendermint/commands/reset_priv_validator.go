package commands

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tmlibs/log"
)

// ResetAllCmd removes the database of this Tendermint core
// instance.
var ResetAllCmd = &cobra.Command{
	Use:   "unsafe_reset_all",
	Short: "(unsafe) Remove all the data and WAL, reset this node's validator to genesis state",
	Run:   resetAll,
}

// ResetPrivValidatorCmd resets the private validator files.
var ResetPrivValidatorCmd = &cobra.Command{
	Use:   "unsafe_reset_priv_validator",
	Short: "(unsafe) Reset this node's validator to genesis state",
	Run:   resetPrivValidator,
}

// XXX: this is totally unsafe.
// it's only suitable for testnets.
func resetAll(cmd *cobra.Command, args []string) {
	ResetAll(config.DBDir(), config.PrivValidatorFile(), logger)
}

// XXX: this is totally unsafe.
// it's only suitable for testnets.
func resetPrivValidator(cmd *cobra.Command, args []string) {
	resetFilePV(config.PrivValidatorFile(), logger)
}

// ResetAll removes the privValidator files.
// Exported so other CLI tools can use it.
func ResetAll(dbDir, addrBookFile, privValFile string, logger log.Logger) {
	resetFilePV(privValFile, logger)
	resetAddrBook(addrBookFile, logger)
	if err := os.RemoveAll(dbDir); err != nil {
		logger.Error("Error removing directory", "err", err)
		return
	}
	logger.Info("Removed all blockchain history", "dir", dbDir)
}

func resetFilePV(privValFile string, logger log.Logger) {
	// Get PrivValidator
	if _, err := os.Stat(privValFile); err == nil {
		pv := privval.LoadFilePV(privValFile)
		pv.Reset()
		logger.Info("Reset PrivValidator to genesis state", "file", privValFile)
	} else {
		pv := privval.GenFilePV(privValFile)
		pv.Save()
		logger.Info("Generated PrivValidator", "file", privValFile)
	}
}

// Deletes the addrbook.json file. It will be generated in node.NewNode, which is executed with
// `tendermint start`.
func resetAddrBook(addrBookFile string, logger log.Logger) {
	if err := os.Remove(addrBookFile); err == nil {
		logger.Info("Cleared AddrBook to an empty state", "file", addrBookFile)
	} else {
		logger.Info("Failed to clear AddrBook to an empty state", "file", addrBookFile)
	}
}
