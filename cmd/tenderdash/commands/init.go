package commands

import (
	"fmt"
	"github.com/dashevo/dashd-go/btcjson"
	"github.com/spf13/cobra"
	cfg "github.com/tendermint/tendermint/config"
	tmos "github.com/tendermint/tendermint/libs/os"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

// InitFilesCmd initialises a fresh Tendermint Core instance.
var InitFilesCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Tenderdash for network use",
	RunE:  initFiles,
}

// LocalInitFilesCmd initialises a fresh Tendermint Core instance.
var LocalInitFilesCmd = &cobra.Command{
	Use:   "init-single",
	Short: "Initialize Tenderdash for single node use",
	RunE:  initFilesSingleNode,
}

func initFilesSingleNode(cmd *cobra.Command, args []string) error {
	return initFilesSingleNodeWithConfig(config)
}

var (
	quorumType int
	coreChainLockedHeight uint32
	initChainInitialHeight int64
	appHash []byte
	proTxHash []byte
)

func AddInitFlags(cmd *cobra.Command) {
	cmd.Flags().IntVar(&quorumType, "quorumType", 0, "Quorum Type")
	cmd.Flags().Uint32Var(&coreChainLockedHeight, "coreChainLockedHeight", 1, "Initial Core Chain Locked Height")
	cmd.Flags().Int64Var(&initChainInitialHeight, "initialHeight", 0, "Initial Height")
	cmd.Flags().BytesHexVar(&proTxHash, "proTxHash", []byte(nil), "Node pro tx hash")
	cmd.Flags().BytesHexVar(&appHash, "appHash", []byte(nil), "App hash")
}

func initFiles(cmd *cobra.Command, args []string) error {
	return initFilesWithConfig(config)
}

func initializeNodeKey(config *cfg.Config) error {
	nodeKeyFile := config.NodeKeyFile()
	if tmos.FileExists(nodeKeyFile) {
		logger.Info("Found node key", "path", nodeKeyFile)
	} else {
		if _, err := p2p.LoadOrGenNodeKey(nodeKeyFile); err != nil {
			return err
		}
		logger.Info("Generated node key", "path", nodeKeyFile)
	}
	return nil
}

func initFilesWithConfig(config *cfg.Config) error {
	// node key
	if err := initializeNodeKey(config); err != nil {
		return err
	}

	// genesis file
	genFile := config.GenesisFile()
	if tmos.FileExists(genFile) {
		logger.Info("Found genesis file", "path", genFile)
	} else {
		genDoc := types.GenesisDoc{
			ChainID:         fmt.Sprintf("test-chain-%v", tmrand.Str(6)),
			GenesisTime:     tmtime.Now(),
			ConsensusParams: types.DefaultConsensusParams(),
			QuorumType:      btcjson.LLMQType(quorumType),
			InitialCoreChainLockedHeight: coreChainLockedHeight,
			InitialHeight:   initChainInitialHeight,
			AppHash:         appHash,
		}

		if err := genDoc.SaveAs(genFile); err != nil {
			return err
		}
		logger.Info("Generated genesis file", "path", genFile)
	}

	return nil
}

func initFilesSingleNodeWithConfig(config *cfg.Config) error {
	// private validator
	privValKeyFile := config.PrivValidatorKeyFile()
	privValStateFile := config.PrivValidatorStateFile()
	var pv *privval.FilePV
	if tmos.FileExists(privValKeyFile) {
		pv = privval.LoadFilePV(privValKeyFile, privValStateFile)
		logger.Info("Found private validator", "keyFile", privValKeyFile,
			"stateFile", privValStateFile)
	} else {
		pv = privval.GenFilePV(privValKeyFile, privValStateFile)
		pv.Save()
		logger.Info("Generated private validator", "keyFile", privValKeyFile,
			"stateFile", privValStateFile)
	}

	// node key
	if err := initializeNodeKey(config); err != nil {
		return err
	}

	// genesis file
	genFile := config.GenesisFile()
	if tmos.FileExists(genFile) {
		logger.Info("Found genesis file", "path", genFile)
	} else {
		quorumHash, err := pv.GetFirstQuorumHash()
		if err != nil {
			return fmt.Errorf("there is no quorum hash: %w", err)
		}
		pubKey, err := pv.GetPubKey(quorumHash)
		if err != nil {
			return fmt.Errorf("can't get pubkey in init files with config: %w", err)
		}

		proTxHash, err := pv.GetProTxHash()
		if err != nil {
			return fmt.Errorf("can't get proTxHash: %w", err)
		}

		logger.Info("Found proTxHash", "proTxHash", proTxHash)

		genDoc := types.GenesisDoc{
			ChainID:         fmt.Sprintf("test-chain-%v", tmrand.Str(6)),
			GenesisTime:     tmtime.Now(),
			ConsensusParams: types.DefaultConsensusParams(),
			ThresholdPublicKey: pubKey,
			QuorumHash: quorumHash,
			InitialCoreChainLockedHeight: 1,
		}

		genDoc.Validators = []types.GenesisValidator{{
			PubKey:    pubKey,
			ProTxHash: proTxHash,
			Power:     types.DefaultDashVotingPower,
		}}

		if err := genDoc.SaveAs(genFile); err != nil {
			return err
		}
		logger.Info("Generated genesis file", "path", genFile)
	}

	return nil
}
