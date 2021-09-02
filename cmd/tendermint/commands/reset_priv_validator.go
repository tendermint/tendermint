package commands

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/types"
)

// ResetAllCmd removes the database of this Tendermint core
// instance.
var ResetAllCmd = &cobra.Command{
	Use:   "unsafe-reset-all",
	Short: "(unsafe) Remove all the data and WAL, reset this node's validator to genesis state",
	RunE:  resetAll,
}

var keepAddrBook bool

func init() {
	ResetAllCmd.Flags().BoolVar(&keepAddrBook, "keep-addr-book", false, "keep the address book intact")
	ResetPrivValidatorCmd.Flags().StringVar(&keyType, "key", types.ABCIPubKeyTypeEd25519,
		"Key type to generate privval file with. Options: ed25519, secp256k1")
}

// ResetPrivValidatorCmd resets the private validator files.
var ResetPrivValidatorCmd = &cobra.Command{
	Use:   "unsafe-reset-priv-validator",
	Short: "(unsafe) Reset this node's validator to genesis state",
	RunE:  resetPrivValidator,
}

// XXX: this is totally unsafe.
// it's only suitable for testnets.
func resetAll(cmd *cobra.Command, args []string) error {
	return ResetAll(config.DBDir(), config.P2P.AddrBookFile(), config.PrivValidator.KeyFile(),
		config.PrivValidator.StateFile(), logger)
}

// XXX: this is totally unsafe.
// it's only suitable for testnets.
func resetPrivValidator(cmd *cobra.Command, args []string) error {
	return resetFilePV(config.PrivValidator.KeyFile(), config.PrivValidator.StateFile(), logger)
}

// ResetAll removes address book files plus all data, and resets the privValdiator data.
// Exported so other CLI tools can use it.
func ResetAll(dbDir, addrBookFile, privValKeyFile, privValStateFile string, logger log.Logger) error {
	if keepAddrBook {
		logger.Info("The address book remains intact")
	} else {
		removeAddrBook(addrBookFile, logger)
	}
	if err := os.RemoveAll(dbDir); err == nil {
		logger.Info("Removed all blockchain history", "dir", dbDir)
	} else {
		logger.Error("Error removing all blockchain history", "dir", dbDir, "err", err)
	}
	// recreate the dbDir since the privVal state needs to live there
	if err := tmos.EnsureDir(dbDir, 0700); err != nil {
		logger.Error("unable to recreate dbDir", "err", err)
	}
	return resetFilePV(privValKeyFile, privValStateFile, logger)
}

func resetFilePV(privValKeyFile, privValStateFile string, logger log.Logger) error {
	if _, err := os.Stat(privValKeyFile); err == nil {
		pv, err := privval.LoadFilePVEmptyState(privValKeyFile, privValStateFile)
		if err != nil {
			return err
		}
		pv.Reset()
		logger.Info("Reset private validator file to genesis state", "keyFile", privValKeyFile,
			"stateFile", privValStateFile)
	} else {
		pv, err := privval.GenFilePV(privValKeyFile, privValStateFile, keyType)
		if err != nil {
			return err
		}
		pv.Save()
		logger.Info("Generated private validator file", "keyFile", privValKeyFile,
			"stateFile", privValStateFile)
	}
	return nil
}

func removeAddrBook(addrBookFile string, logger log.Logger) {
	if err := os.Remove(addrBookFile); err == nil {
		logger.Info("Removed existing address book", "file", addrBookFile)
	} else if !os.IsNotExist(err) {
		logger.Info("Error removing address book", "file", addrBookFile, "err", err)
	}
}
