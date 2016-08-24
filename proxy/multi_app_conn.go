package proxy

import (
	"bytes"
	"fmt"
	"sync"

	. "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
	"github.com/tendermint/tendermint/types" // ...
	tmspcli "github.com/tendermint/tmsp/client"
	"github.com/tendermint/tmsp/example/dummy"
	nilapp "github.com/tendermint/tmsp/example/nil"
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

// a multiAppConn is made of a few appConns (mempool, consensus, query)
// and manages their underlying tmsp clients, including the handshake
// which ensures the app and tendermint are synced.
// TODO: on app restart, clients must reboot together
type multiAppConn struct {
	BaseService

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
	multiAppConn.BaseService = *NewBaseService(log, "multiAppConn", multiAppConn)
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
	app.BaseService.OnStart()

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
func (app *multiAppConn) Handshake() error {
	// handshake is done on the query conn
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

	// of the last block (nil if we starting from 0)
	var header *types.Header
	var partsHeader types.PartSetHeader

	// check block
	// if the blockHeight == 0, we will replay everything
	if blockHeight != 0 {
		blockMeta := app.blockStore.LoadBlockMeta(blockHeight)
		if blockMeta == nil {
			return fmt.Errorf("Handshake error. Could not find block #%d", blockHeight)
		}

		// check block hash
		if !bytes.Equal(blockMeta.Hash, blockHash) {
			return fmt.Errorf("Handshake error. Block hash at height %d does not match. Got %X, expected %X", blockHeight, blockHash, blockMeta.Hash)
		}

		// check app hash
		if !bytes.Equal(blockMeta.Header.AppHash, appHash) {
			return fmt.Errorf("Handshake error. App hash at height %d does not match. Got %X, expected %X", blockHeight, appHash, blockMeta.Header.AppHash)
		}

		header = blockMeta.Header
		partsHeader = blockMeta.PartsHeader
	}

	if configInfo != nil {
		// TODO: set config info
		_ = configInfo
	}

	// replay blocks up to the latest in the blockstore
	err := app.state.ReplayBlocks(header, partsHeader, app.consensusConn, app.blockStore)
	if err != nil {
		return fmt.Errorf("Error on replay: %v", err)
	}

	// TODO: (on restart) replay mempool

	return nil
}

//--------------------------------

// Get a connected tmsp client
func NewTMSPClient(addr, transport string) (tmspcli.Client, error) {
	var client tmspcli.Client

	// use local app (for testing)
	// TODO: local proxy app conn
	switch addr {
	case "nilapp":
		app := nilapp.NewNilApplication()
		mtx := new(sync.Mutex) // TODO
		client = tmspcli.NewLocalClient(mtx, app)
	case "dummy":
		app := dummy.NewDummyApplication()
		mtx := new(sync.Mutex) // TODO
		client = tmspcli.NewLocalClient(mtx, app)
	default:
		// Run forever in a loop
		mustConnect := false
		remoteApp, err := tmspcli.NewClient(addr, transport, mustConnect)
		if err != nil {
			return nil, fmt.Errorf("Failed to connect to proxy for mempool: %v", err)
		}
		client = remoteApp
	}
	return client, nil
}
