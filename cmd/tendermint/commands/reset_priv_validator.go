package commands

import (
	"os"
	"path/filepath"

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

// MakeResetStateCommand constructs a command that removes the database of
// the specified Tendermint core instance.
func MakeResetStateCommand(conf *config.Config, logger log.Logger) *cobra.Command {
	var keyType string

	return &cobra.Command{
		Use:   "reset-state",
		Short: "Remove all the data and WAL",
		RunE: func(cmd *cobra.Command, args []string) error {
			return resetState(conf.DBDir(), logger, keyType)
		},
	}
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
func resetAll(dbDir, privValKeyFile, privValStateFile string, logger log.Logger, keyType string) error {
	if err := os.RemoveAll(dbDir); err == nil {
		logger.Info("Removed all blockchain history", "dir", dbDir)
	} else {
		logger.Error("error removing all blockchain history", "dir", dbDir, "err", err)
	}

	return resetFilePV(privValKeyFile, privValStateFile, logger, keyType)
}

// resetState removes address book files plus all databases.
func resetState(dbDir string, logger log.Logger, keyType string) error {
	blockdb := filepath.Join(dbDir, "blockstore.db")
	state := filepath.Join(dbDir, "state.db")
	wal := filepath.Join(dbDir, "cs.wal")
	evidence := filepath.Join(dbDir, "evidence.db")
	txIndex := filepath.Join(dbDir, "tx_index.db")
	peerstore := filepath.Join(dbDir, "peerstore.db")

	if tmos.FileExists(blockdb) {
		if err := os.RemoveAll(blockdb); err == nil {
			logger.Info("Removed all blockstore.db", "dir", blockdb)
		} else {
			logger.Error("error removing all blockstore.db", "dir", blockdb, "err", err)
		}
	}

	if tmos.FileExists(state) {
		if err := os.RemoveAll(state); err == nil {
			logger.Info("Removed all state.db", "dir", state)
		} else {
			logger.Error("error removing all state.db", "dir", state, "err", err)
		}
	}

	if tmos.FileExists(wal) {
		if err := os.RemoveAll(wal); err == nil {
			logger.Info("Removed all cs.wal", "dir", wal)
		} else {
			logger.Error("error removing all cs.wal", "dir", wal, "err", err)
		}
	}

	if tmos.FileExists(evidence) {
		if err := os.RemoveAll(evidence); err == nil {
			logger.Info("Removed all evidence.db", "dir", evidence)
		} else {
			logger.Error("error removing all evidence.db", "dir", evidence, "err", err)
		}
	}

	if tmos.FileExists(txIndex) {
		if err := os.RemoveAll(txIndex); err == nil {
			logger.Info("Removed tx_index.db", "dir", txIndex)
		} else {
			logger.Error("error removing tx_index.db", "dir", txIndex, "err", err)
		}
	}

	if tmos.FileExists(peerstore) {
		if err := os.RemoveAll(peerstore); err == nil {
			logger.Info("Removed peerstore.db", "dir", peerstore)
		} else {
			logger.Error("error removing peerstore.db", "dir", peerstore, "err", err)
		}
	}
	if err := tmos.EnsureDir(dbDir, 0700); err != nil {
		logger.Error("unable to recreate dbDir", "err", err)
	}
	return nil
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
