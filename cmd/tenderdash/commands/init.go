package commands

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dashevo/dashd-go/btcjson"
	"github.com/spf13/cobra"
	cfg "github.com/tendermint/tendermint/config"
	tmos "github.com/tendermint/tendermint/libs/os"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/types"
)

// InitFilesCmd initialises a fresh Tendermint Core instance.
var InitFilesCmd = &cobra.Command{
	Use:       "init [full|validator|seed|single]",
	Short:     "Initializes a Tenderdash node",
	ValidArgs: []string{"full", "validator", "seed", "single"},
	// We allow for zero args so we can throw a more informative error
	Args: cobra.MaximumNArgs(1),
	RunE: initFiles,
}

var (
	quorumType             int
	coreChainLockedHeight  uint32
	initChainInitialHeight int64
	appHash                []byte
	proTxHash              []byte
)

func AddInitFlags(cmd *cobra.Command) {
	cmd.Flags().IntVar(&quorumType, "quorumType", 0, "Quorum Type")
	cmd.Flags().Uint32Var(&coreChainLockedHeight, "coreChainLockedHeight", 1, "Initial Core Chain Locked Height")
	cmd.Flags().Int64Var(&initChainInitialHeight, "initialHeight", 0, "Initial Height")
	cmd.Flags().BytesHexVar(&proTxHash, "proTxHash", []byte(nil), "Node pro tx hash")
	cmd.Flags().BytesHexVar(&appHash, "appHash", []byte(nil), "App hash")
}

func initFiles(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return errors.New("must specify a node type: tendermint init [validator|full|seed|single]")
	}
	config.Mode = args[0]
	return initFilesWithConfig(config)
}

func initFilesWithConfig(config *cfg.Config) error {
	var (
		pv  *privval.FilePV
		err error
	)

	if config.Mode == cfg.ModeValidator {
		// private validator
		privValKeyFile := config.PrivValidator.KeyFile()
		privValStateFile := config.PrivValidator.StateFile()
		if tmos.FileExists(privValKeyFile) {
			pv, err = privval.LoadFilePV(privValKeyFile, privValStateFile)
			if err != nil {
				return err
			}

			logger.Info("Found private validator", "keyFile", privValKeyFile,
				"stateFile", privValStateFile)
		} else {
			pv = privval.GenFilePV(privValKeyFile, privValStateFile)
			if err != nil {
				return err
			}
			pv.Save()
			logger.Info("Generated private validator", "keyFile", privValKeyFile,
				"stateFile", privValStateFile)
		}
	}

	nodeKeyFile := config.NodeKeyFile()
	if tmos.FileExists(nodeKeyFile) {
		logger.Info("Found node key", "path", nodeKeyFile)
	} else {
		if _, err := types.LoadOrGenNodeKey(nodeKeyFile); err != nil {
			return err
		}
		logger.Info("Generated node key", "path", nodeKeyFile)
	}

	// genesis file
	genFile := config.GenesisFile()
	if tmos.FileExists(genFile) {
		logger.Info("Found genesis file", "path", genFile)
	} else {

		genDoc := types.GenesisDoc{
			ChainID:                      fmt.Sprintf("test-chain-%v", tmrand.Str(6)),
			GenesisTime:                  time.Now(),
			ConsensusParams:              types.DefaultConsensusParams(),
			QuorumType:                   btcjson.LLMQType(quorumType),
			InitialCoreChainLockedHeight: coreChainLockedHeight,
			InitialHeight:                initChainInitialHeight,
			AppHash:                      appHash,
		}

		ctx, cancel := context.WithTimeout(context.TODO(), ctxTimeout)
		defer cancel()

		// if this is a validator we add it to genesis
		if pv != nil {
			proTxHash, err := pv.GetProTxHash(ctx)
			if err != nil {
				return fmt.Errorf("can't get proTxHash: %w", err)
			}
			genesisValidator := types.GenesisValidator{
				ProTxHash: proTxHash,
				Power:     types.DefaultDashVotingPower,
			}
			quorumHash, _ := pv.GetFirstQuorumHash(ctx)
			if quorumHash != nil {
				pubKey, err := pv.GetPubKey(ctx, quorumHash)
				if err != nil {
					return fmt.Errorf("can't get pubkey: %w", err)
				}
				genesisValidator.PubKey = pubKey

				genDoc.QuorumHash = quorumHash
				genDoc.ThresholdPublicKey = pubKey
			}

			genDoc.Validators = []types.GenesisValidator{genesisValidator}
		}

		if err := genDoc.SaveAs(genFile); err != nil {
			return err
		}
		logger.Info("Generated genesis file", "path", genFile)
	}

	// write config file
	if err := cfg.WriteConfigFile(config.RootDir, config); err != nil {
		return err
	}
	logger.Info("Generated config", "mode", config.Mode)

	return nil
}
