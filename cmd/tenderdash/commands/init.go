package commands

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dashevo/dashd-go/btcjson"
	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/types"
)

type nodeConfig struct {
	*config.Config
	quorumType             int
	coreChainLockedHeight  uint32
	initChainInitialHeight int64
	appHash                []byte
	proTxHash              []byte
}

// MakeInitFilesCommand returns the command to initialize a fresh Tendermint Core instance.
func MakeInitFilesCommand(conf *config.Config, logger log.Logger) *cobra.Command {
	nodeConf := nodeConfig{Config: conf}

	cmd := &cobra.Command{
		Use:       "init [full|validator|seed]",
		Short:     "Initializes a Tenderdash node",
		ValidArgs: []string{"full", "validator", "seed"},
		// We allow for zero args so we can throw a more informative error
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.New("must specify a node type: tendermint init [validator|full|seed]")
			}
			nodeConf.Mode = args[0]
			return initFilesWithConfig(cmd.Context(), nodeConf, logger)
		},
	}

	cmd.Flags().IntVar(&nodeConf.quorumType, "quorumType", 0, "Quorum Type")
	cmd.Flags().Uint32Var(&nodeConf.coreChainLockedHeight, "coreChainLockedHeight", 1, "Initial Core Chain Locked Height")
	cmd.Flags().Int64Var(&nodeConf.initChainInitialHeight, "initialHeight", 0, "Initial Height")
	cmd.Flags().BytesHexVar(&nodeConf.proTxHash, "proTxHash", []byte(nil), "Node pro tx hash")
	cmd.Flags().BytesHexVar(&nodeConf.appHash, "appHash", []byte(nil), "App hash")

	return cmd
}

func initFilesWithConfig(ctx context.Context, conf nodeConfig, logger log.Logger) error {
	var (
		pv  *privval.FilePV
		err error
	)

	if conf.Mode == config.ModeValidator {
		// private validator
		privValKeyFile := conf.PrivValidator.KeyFile()
		privValStateFile := conf.PrivValidator.StateFile()
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
			if err := pv.Save(); err != nil {
				return err
			}
			logger.Info("Generated private validator", "keyFile", privValKeyFile,
				"stateFile", privValStateFile)
		}
	}

	nodeKeyFile := conf.NodeKeyFile()
	if tmos.FileExists(nodeKeyFile) {
		logger.Info("Found node key", "path", nodeKeyFile)
	} else {
		if _, err := types.LoadOrGenNodeKey(nodeKeyFile); err != nil {
			return err
		}
		logger.Info("Generated node key", "path", nodeKeyFile)
	}

	// genesis file
	genFile := conf.GenesisFile()
	if tmos.FileExists(genFile) {
		logger.Info("Found genesis file", "path", genFile)
	} else {

		genDoc := types.GenesisDoc{
			ChainID:                      fmt.Sprintf("test-chain-%v", tmrand.Str(6)),
			GenesisTime:                  time.Now(),
			ConsensusParams:              types.DefaultConsensusParams(),
			QuorumType:                   btcjson.LLMQType(conf.quorumType),
			InitialCoreChainLockedHeight: conf.coreChainLockedHeight,
			InitialHeight:                conf.initChainInitialHeight,
			AppHash:                      conf.appHash,
		}

		ctx, cancel := context.WithTimeout(ctx, ctxTimeout)
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
	if err := config.WriteConfigFile(conf.RootDir, conf.Config); err != nil {
		return err
	}
	logger.Info("Generated config", "mode", conf.Mode)

	return nil
}
