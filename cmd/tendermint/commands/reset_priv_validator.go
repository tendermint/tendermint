package commands

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/types"
)

// MakeResetAllCommand constructs a command that removes the database of
// the specified Tendermint core instance.
func MakeResetAllCommand(conf *config.Config, logger log.Logger) *cobra.Command {
	var keyType string

	cmd := &cobra.Command{
		Use:   "unsafe-reset-all",
		Short: "(unsafe) Remove all the data and WAL, reset this node's validator to genesis state",
		RunE: func(cmd *cobra.Command, args []string) error {
			return resetAll(conf.DBDir(), conf.PrivValidator.KeyFile(),
				conf.PrivValidator.StateFile(), logger, keyType)
		},
	}
	cmd.Flags().StringVar(&keyType, "key", types.ABCIPubKeyTypeEd25519,
		"Key type to generate privval file with. Options: ed25519, secp256k1")

	return cmd
}

func MakeResetPrivateValidatorCommand(conf *config.Config, logger log.Logger) *cobra.Command {
	var keyType string

	cmd := &cobra.Command{
		Use:   "unsafe-reset-priv-validator",
		Short: "(unsafe) Reset this node's validator to genesis state",
		RunE: func(cmd *cobra.Command, args []string) error {
			return resetFilePV(conf.PrivValidator.KeyFile(), conf.PrivValidator.StateFile(), logger, keyType)
		},
	}

	cmd.Flags().StringVar(&keyType, "key", types.ABCIPubKeyTypeEd25519,
		"Key type to generate privval file with. Options: ed25519, secp256k1")
	return cmd

}

// XXX: this is totally unsafe.
// it's only suitable for testnets.

// XXX: this is totally unsafe.
// it's only suitable for testnets.

// resetAll removes address book files plus all data, and resets the privValdiator data.
// Exported so other CLI tools can use it.
func resetAll(dbDir, privValKeyFile, privValStateFile string, logger log.Logger, keyType string) error {
	if err := os.RemoveAll(dbDir); err == nil {
		logger.Info("Removed all blockchain history", "dir", dbDir)
	} else {
		logger.Error("error removing all blockchain history", "dir", dbDir, "err", err)
	}
	// recreate the dbDir since the privVal state needs to live there
	if err := tmos.EnsureDir(dbDir, 0700); err != nil {
		logger.Error("unable to recreate dbDir", "err", err)
	}
	return resetFilePV(privValKeyFile, privValStateFile, logger, keyType)
}

func resetFilePV(privValKeyFile, privValStateFile string, logger log.Logger, keyType string) error {
	if _, err := os.Stat(privValKeyFile); err == nil {
		pv, err := privval.LoadFilePVEmptyState(privValKeyFile, privValStateFile)
		if err != nil {
			return err
		}
		if err := pv.Reset(); err != nil {
			return err
		}
		logger.Info("Reset private validator file to genesis state", "keyFile", privValKeyFile,
			"stateFile", privValStateFile)
	} else {
		pv, err := privval.GenFilePV(privValKeyFile, privValStateFile, keyType)
		if err != nil {
			return err
		}
		if err := pv.Save(); err != nil {
			return err
		}
		logger.Info("Generated private validator file", "keyFile", privValKeyFile,
			"stateFile", privValStateFile)
	}
	return nil
}
