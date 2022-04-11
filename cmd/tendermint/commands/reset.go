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

// MakeResetCommand constructs a command that removes the database of
// the specified Tendermint core instance.
func MakeResetCommand(conf *config.Config, logger log.Logger) *cobra.Command {
	var (
		resetAllFlag       bool
		resetPrivValFlag   bool
		resetPeerStoreFlag bool
		keyType            string
	)

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Remove all tendermint data, resetting to genesis state.",
		Long: `Removes block, state and evidence store. Does not alter priv-validator state or your peer store.
If resetting, don't forget to reset application state as well.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if resetAllFlag {
				return ResetAll(conf.DBDir(), conf.PrivValidator.KeyFile(),
					conf.PrivValidator.StateFile(), logger, keyType)
			}
			if resetPrivValFlag {
				return ResetFilePV(conf.PrivValidator.KeyFile(), conf.PrivValidator.StateFile(), logger, keyType)
			}
			if resetPeerStoreFlag {
				return ResetPeerStore(conf.DBDir())
			}
			return ResetState(conf.DBDir(), logger)
		},
	}
	cmd.Flags().StringVar(&keyType, "key", types.ABCIPubKeyTypeEd25519,
		"Key type to generate privval file with. Options: ed25519, secp256k1")
	cmd.Flags().BoolVar(&resetAllFlag, "all", false, "Resets all block data and priv validator state. Only use in testing")
	cmd.Flags().BoolVar(&resetPrivValFlag, "privval", false, "Resets all priv validator state. Only use in testing")
	cmd.Flags().BoolVar(&resetPeerStoreFlag, "peers", false, "Flushes entire peer address book")

	return cmd
}

// XXX: this is totally unsafe.
// it's only suitable for testnets.

// resetAll removes address book files plus all data, and resets the privValdiator data.
// Exported for extenal CLI usage
func ResetAll(dbDir, privValKeyFile, privValStateFile string, logger log.Logger, keyType string) error {
	if err := os.RemoveAll(dbDir); err == nil {
		logger.Info("Removed all blockchain history", "dir", dbDir)
	} else {
		logger.Error("error removing all blockchain history", "dir", dbDir, "err", err)
	}

	if err := tmos.EnsureDir(dbDir, 0700); err != nil {
		logger.Error("unable to recreate dbDir", "err", err)
	}

	// recreate the dbDir since the privVal state needs to live there
	return ResetFilePV(privValKeyFile, privValStateFile, logger, keyType)
}

// resetState removes address book files plus all databases.
func ResetState(dbDir string, logger log.Logger) error {
	blockdb := filepath.Join(dbDir, "blockstore.db")
	state := filepath.Join(dbDir, "state.db")
	wal := filepath.Join(dbDir, "cs.wal")
	evidence := filepath.Join(dbDir, "evidence.db")
	txIndex := filepath.Join(dbDir, "tx_index.db")

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

	return tmos.EnsureDir(dbDir, 0700)
}

func ResetFilePV(privValKeyFile, privValStateFile string, logger log.Logger, keyType string) error {
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

func ResetPeerStore(dbDir string) error {
	peerstore := filepath.Join(dbDir, "peerstore.db")
	if tmos.FileExists(peerstore) {
		return os.RemoveAll(peerstore)
	}
	return nil
}
