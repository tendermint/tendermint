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
	var keyType string

	resetCmd := &cobra.Command{
		Use:   "reset",
		Short: "Set of commands to conveniently reset tendermint related data",
	}

	resetBlocksCmd := &cobra.Command{
		Use:   "blockchain",
		Short: "Removes all blocks, state, transactions and evidence stored by the tendermint node",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ResetState(conf.DBDir(), logger)
		},
	}

	resetPeersCmd := &cobra.Command{
		Use:   "peers",
		Short: "Removes all peer addresses",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ResetPeerStore(conf.DBDir())
		},
	}

	resetSignerCmd := &cobra.Command{
		Use:   "unsafe-signer",
		Short: "esets private validator signer state",
		Long: `Resets private validator signer state. 
Only use in testing. This can cause the node to double sign`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ResetFilePV(conf.PrivValidator.KeyFile(), conf.PrivValidator.StateFile(), logger, keyType)
		},
	}

	resetAllCmd := &cobra.Command{
		Use:   "unsafe-all",
		Short: "Removes all tendermint data including signing state",
		Long: `Removes all tendermint data including signing state. 
Only use in testing. This can cause the node to double sign`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ResetAll(conf.DBDir(), conf.PrivValidator.KeyFile(),
				conf.PrivValidator.StateFile(), logger, keyType)
		},
	}

	resetSignerCmd.Flags().StringVar(&keyType, "key", types.ABCIPubKeyTypeEd25519,
		"Signer key type. Options: ed25519, secp256k1")

	resetAllCmd.Flags().StringVar(&keyType, "key", types.ABCIPubKeyTypeEd25519,
		"Signer key type. Options: ed25519, secp256k1")

	resetCmd.AddCommand(resetBlocksCmd)
	resetCmd.AddCommand(resetPeersCmd)
	resetCmd.AddCommand(resetSignerCmd)
	resetCmd.AddCommand(resetAllCmd)

	return resetCmd
}

// ResetAll removes address book files plus all data, and resets the privValdiator data.
// Exported for extenal CLI usage
// XXX: this is unsafe and should only suitable for testnets.
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

// ResetState removes all blocks, tendermint state, indexed transactions and evidence.
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

// ResetFilePV loads the file private validator and resets the watermark to 0. If used on an existing network,
// this can cause the node to double sign.
// XXX: this is unsafe and should only suitable for testnets.
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

// ResetPeerStore removes the peer store containing all information used by the tendermint networking layer
// In the case of a reset, new peers will need to be set either via the config or through the discovery mechanism
func ResetPeerStore(dbDir string) error {
	peerstore := filepath.Join(dbDir, "peerstore.db")
	if tmos.FileExists(peerstore) {
		return os.RemoveAll(peerstore)
	}
	return nil
}
