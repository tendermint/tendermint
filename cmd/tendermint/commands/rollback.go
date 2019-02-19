package commands

import (
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	bc "github.com/tendermint/tendermint/blockchain"
	cfg "github.com/tendermint/tendermint/config"
	cmn "github.com/tendermint/tendermint/libs/common"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/node"
	"github.com/tendermint/tendermint/privval"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

var (
	genesisDocKey = []byte("genesisDoc")
)

func init() {
	RollbackCmd.Flags().StringVar(&config.RootDir, "home", "/root/.tendermint/",
		"root path")
	RollbackCmd.Flags().StringVar(&config.PrivValidatorKey, "prival_key", "config/priv_validator_key.json",
		"privalidator key filepath")
	RollbackCmd.Flags().StringVar(&config.PrivValidatorState, "prival_state", "data/priv_validator_state.json",
		"privalidator state filepath")
	RollbackCmd.Flags().StringVar(&config.DBBackend, "db_backend", "leveldb",
		"DBBackend Type")
	RollbackCmd.Flags().StringVar(&config.DBPath, "db_dir", "data",
		"DBBackend Path")

	RollbackCmd.Flags().BoolVar(&config.RollbackFlag, "rollback_data", true, "rollback data flag")
	RollbackCmd.Flags().Int64Var(&config.RollbackHeight, "rollback_height", 1000000, "rollback data height")
	RollbackCmd.Flags().BoolVar(&config.RollbackHeightFlag, "rollback_height_flag", true, "rollback height flag")
}

// InitFilesCmd initialises a fresh Tendermint Core instance.
var RollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "rollback Tendermint data",
	RunE:  rollbackData,
}

func rollbackData(cmd *cobra.Command, args []string) error {
	fmt.Println("you are rollbacking data")
	// Get BlockStore

	newPrivValKey := config.PrivValidatorKeyFile()
	newPrivValState := config.PrivValidatorStateFile()
	privValidator := privval.LoadOrGenFilePV(newPrivValKey, newPrivValState)
	fmt.Println(privValidator.String())
	fmt.Println(config.GenesisFile())

	blockStoreDB, err := node.DefaultDBProvider(&node.DBContext{"blockstore", config})
	if err != nil {
		return err
	}
	blockStore := bc.NewBlockStore(blockStoreDB)

	// Get State
	stateDB, err := node.DefaultDBProvider(&node.DBContext{"state", config})
	if err != nil {
		return err
	}

	genDoc, err := loadGenesisDoc(stateDB)
	if err != nil {
		genDoc, err = node.DefaultGenesisDocProviderFunc(config)()
		if err != nil {
			return err
		}
		// save genesis doc to prevent a certain class of user errors (e.g. when it
		// was changed, accidentally or not). Also good for audit trail.
		saveGenesisDoc(stateDB, genDoc)
	}

	state, err := sm.LoadStateFromDBOrGenesisDoc(stateDB, genDoc)
	if err != nil {
		return err
	}

	if config.RollbackFlag {
		unsafeRollbackData(config, state, stateDB, blockStore, blockStoreDB, privValidator)
	}

	return nil
}

func loadGenesisDoc(db dbm.DB) (*types.GenesisDoc, error) {
	bytes := db.Get(genesisDocKey)
	if len(bytes) == 0 {
		return nil, errors.New("Genesis doc not found")
	}
	var genDoc *types.GenesisDoc
	err := cdc.UnmarshalJSON(bytes, &genDoc)
	if err != nil {
		cmn.PanicCrisis(fmt.Sprintf("Failed to load genesis doc due to unmarshaling error: %v (bytes: %X)", err, bytes))
	}
	return genDoc, nil
}

// panics if failed to marshal the given genesis document
func saveGenesisDoc(db dbm.DB, genDoc *types.GenesisDoc) {
	bytes, err := cdc.MarshalJSON(genDoc)
	if err != nil {
		cmn.PanicCrisis(fmt.Sprintf("Failed to save genesis doc due to marshaling error: %v", err))
	}
	db.SetSync(genesisDocKey, bytes)
}

func unsafeRollbackData(config *cfg.Config, state sm.State, stateDb dbm.DB, blockStore *bc.BlockStore,
	blockStoreDB dbm.DB, privValidator types.PrivValidator) error {

	if state.LastBlockHeight > config.BaseConfig.RollbackHeight {
		stateRestore := restoreStateFromBlock(stateDb, blockStore, config.BaseConfig.RollbackHeight)
		sm.SaveState(stateDb, stateRestore)
		bc.BlockStoreStateJSON{Height: config.BaseConfig.RollbackHeight}.Save(blockStoreDB)
		modifyPrivValidatorsFile(config, config.BaseConfig.RollbackHeight)
		blockStore = bc.NewBlockStore(blockStoreDB)
		return nil
	} else {
	}
	return nil
}

func restoreStateFromBlock(stateDb dbm.DB, blockStore *bc.BlockStore, rollbackHeight int64) sm.State {
	validator, _ := sm.LoadValidators(stateDb, rollbackHeight+1)
	validatorChanged := sm.LoadValidatorsChanged(stateDb, rollbackHeight)
	lastvalidator, _ := sm.LoadValidators(stateDb, rollbackHeight)
	nextvalidator, _ := sm.LoadValidators(stateDb, rollbackHeight+2)

	consensusParams, _ := sm.LoadConsensusParams(stateDb, rollbackHeight)
	consensusParamsChanged := sm.LoadConsensusParamsChanged(stateDb, rollbackHeight)
	software := sm.LoadSoftware(stateDb, rollbackHeight)

	block := blockStore.LoadBlock(rollbackHeight)
	nextBlock := blockStore.LoadBlock(rollbackHeight + 1)

	return sm.State{
		ChainID: block.ChainID,
		Version: sm.Version{Consensus: block.Version, Software: software},

		LastBlockID:      nextBlock.LastBlockID, //? true
		LastBlockHeight:  block.Height,
		LastBlockTime:    block.Time,
		LastBlockTotalTx: block.TotalTxs,

		NextValidators:              nextvalidator.Copy(),
		Validators:                  validator.Copy(),
		LastValidators:              lastvalidator.Copy(),
		LastHeightValidatorsChanged: validatorChanged,

		ConsensusParams:                  consensusParams,
		LastHeightConsensusParamsChanged: consensusParamsChanged,

		AppHash: nextBlock.AppHash.Bytes(),

		LastResultsHash: block.LastResultsHash,
	}
}

func modifyPrivValidatorsFile(config *cfg.Config, rollbackHeight int64) error{
	var sig []byte
	filePv := privval.LoadOrGenFilePV(config.PrivValidatorKeyFile(), config.PrivValidatorStateFile())
	if config.RollbackHeightFlag {
		filePv.LastSignState.Height = rollbackHeight
		filePv.LastSignState.Round = 0
		filePv.LastSignState.Step = 0
		filePv.LastSignState.Signature = sig
		filePv.LastSignState.SignBytes = nil
		filePv.Save()
	}

	return nil
}
