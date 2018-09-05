package commands

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/privval"
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
	ResetAll(config.DBDir(), config.P2P.AddrBookFile(), config.PrivValidatorFile(), logger)
}

// XXX: this is totally unsafe.
// it's only suitable for testnets.
func resetPrivValidator(cmd *cobra.Command, args []string) {
	resetFilePV(config.PrivValidatorFile(), logger)
}

// ResetAll removes the privValidator and address book files plus all data.
// Exported so other CLI tools can use it.
func ResetAll(dbDir, addrBookFile, privValFile string, logger log.Logger) {
	resetFilePV(privValFile, logger)
	removeAddrBook(addrBookFile, logger)
	if err := os.RemoveAll(dbDir); err == nil {
		logger.Info("Removed all blockchain history", "dir", dbDir)
	} else {
		logger.Error("Error removing all blockchain history", "dir", dbDir, "err", err)
	}
}

func resetFilePV(privValFile string, logger log.Logger) {
	if _, err := os.Stat(privValFile); err == nil {
		pv := privval.LoadFilePV(privValFile)
		pv.Reset()
		logger.Info("Reset private validator file to genesis state", "file", privValFile)
	} else {
		pv := privval.GenFilePV(privValFile)
		pv.Save()
		logger.Info("Generated private validator file", "file", privValFile)
	}
}

func removeAddrBook(addrBookFile string, logger log.Logger) {
	if err := os.Remove(addrBookFile); err == nil {
		logger.Info("Removed existing address book", "file", addrBookFile)
	} else if !os.IsNotExist(err) {
		logger.Info("Error removing address book", "file", addrBookFile, "err", err)
	}
}
