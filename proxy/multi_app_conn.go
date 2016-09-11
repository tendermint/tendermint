package proxy

import (
	"bytes"
	"fmt"

	. "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
	"github.com/tendermint/tendermint/types"
)

//-----------------------------

// Tendermint's interface to the application consists of multiple connections
type AppConns interface {
	Service

	Mempool() AppConnMempool
	Consensus() AppConnConsensus
	Query() AppConnQuery
}

func NewAppConns(config cfg.Config, clientCreator ClientCreator, state State, blockStore BlockStore) AppConns {
	return NewMultiAppConn(config, clientCreator, state, blockStore)
}

//-----------------------------
// multiAppConn implements AppConns

// a multiAppConn is made of a few appConns (mempool, consensus, query)
// and manages their underlying tmsp clients, including the handshake
// which ensures the app and tendermint are synced.
// TODO: on app restart, clients must reboot together
type multiAppConn struct {
	QuitService

	config cfg.Config

	state      State
	blockStore BlockStore

	mempoolConn   *appConnMempool
	consensusConn *appConnConsensus
	queryConn     *appConnQuery

	clientCreator ClientCreator
}

// Make all necessary tmsp connections to the application
func NewMultiAppConn(config cfg.Config, clientCreator ClientCreator, state State, blockStore BlockStore) *multiAppConn {
	multiAppConn := &multiAppConn{
		config:        config,
		state:         state,
		blockStore:    blockStore,
		clientCreator: clientCreator,
	}
	multiAppConn.QuitService = *NewQuitService(log, "multiAppConn", multiAppConn)
	return multiAppConn
}

// Returns the mempool connection
func (app *multiAppConn) Mempool() AppConnMempool {
	return app.mempoolConn
}

// Returns the consensus Connection
func (app *multiAppConn) Consensus() AppConnConsensus {
	return app.consensusConn
}

// Returns the query Connection
func (app *multiAppConn) Query() AppConnQuery {
	return app.queryConn
}

func (app *multiAppConn) OnStart() error {
	app.QuitService.OnStart()

	// query connection
	querycli, err := app.clientCreator.NewTMSPClient()
	if err != nil {
		return err
	}
	app.queryConn = NewAppConnQuery(querycli)

	// mempool connection
	memcli, err := app.clientCreator.NewTMSPClient()
	if err != nil {
		return err
	}
	app.mempoolConn = NewAppConnMempool(memcli)

	// consensus connection
	concli, err := app.clientCreator.NewTMSPClient()
	if err != nil {
		return err
	}
	app.consensusConn = NewAppConnConsensus(concli)

	// ensure app is synced to the latest state
	return app.Handshake()
}

// TODO: retry the handshake once if it fails the first time
// ... let Info take an argument determining its behaviour
func (app *multiAppConn) Handshake() error {
	// handshake is done via info request on the query conn
	res, tmspInfo, blockInfo, configInfo := app.queryConn.InfoSync()
	if res.IsErr() {
		return fmt.Errorf("Error calling Info. Code: %v; Data: %X; Log: %s", res.Code, res.Data, res.Log)
	}

	if blockInfo == nil {
		log.Warn("blockInfo is nil, aborting handshake")
		return nil
	}

	log.Notice("TMSP Handshake", "height", blockInfo.BlockHeight, "block_hash", blockInfo.BlockHash, "app_hash", blockInfo.AppHash)

	// TODO: check overflow or change pb to int32
	blockHeight := int(blockInfo.BlockHeight)
	blockHash := blockInfo.BlockHash
	appHash := blockInfo.AppHash

	if tmspInfo != nil {
		// TODO: check tmsp version (or do this in the tmspcli?)
		_ = tmspInfo
	}

	// last block (nil if we starting from 0)
	var header *types.Header
	var partsHeader types.PartSetHeader

	// replay all blocks after blockHeight
	// if blockHeight == 0, we will replay everything
	if blockHeight != 0 {
		blockMeta := app.blockStore.LoadBlockMeta(blockHeight)
		if blockMeta == nil {
			return fmt.Errorf("Handshake error. Could not find block #%d", blockHeight)
		}

		// check block hash
		if !bytes.Equal(blockMeta.Hash, blockHash) {
			return fmt.Errorf("Handshake error. Block hash at height %d does not match. Got %X, expected %X", blockHeight, blockHash, blockMeta.Hash)
		}

		// NOTE: app hash should be in the next block ...
		// check app hash
		/*if !bytes.Equal(blockMeta.Header.AppHash, appHash) {
			return fmt.Errorf("Handshake error. App hash at height %d does not match. Got %X, expected %X", blockHeight, appHash, blockMeta.Header.AppHash)
		}*/

		header = blockMeta.Header
		partsHeader = blockMeta.PartsHeader
	}

	if configInfo != nil {
		// TODO: set config info
		_ = configInfo
	}

	// replay blocks up to the latest in the blockstore
	err := app.state.ReplayBlocks(appHash, header, partsHeader, app.consensusConn, app.blockStore)
	if err != nil {
		return fmt.Errorf("Error on replay: %v", err)
	}

	// TODO: (on restart) replay mempool

	return nil
}
