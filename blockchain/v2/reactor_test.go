package v2

import (
	"net"
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	dbm "github.com/tendermint/tm-db"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/behaviour"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/mock"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/conn"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

type mockPeer struct {
	service.Service
	id p2p.ID
}

func (mp mockPeer) FlushStop()           {}
func (mp mockPeer) ID() p2p.ID           { return mp.id }
func (mp mockPeer) RemoteIP() net.IP     { return net.IP{} }
func (mp mockPeer) RemoteAddr() net.Addr { return &net.TCPAddr{IP: mp.RemoteIP(), Port: 8800} }

func (mp mockPeer) IsOutbound() bool   { return true }
func (mp mockPeer) IsPersistent() bool { return true }
func (mp mockPeer) CloseConn() error   { return nil }

func (mp mockPeer) NodeInfo() p2p.NodeInfo {
	return p2p.DefaultNodeInfo{
		DefaultNodeID: "",
		ListenAddr:    "",
	}
}
func (mp mockPeer) Status() conn.ConnectionStatus { return conn.ConnectionStatus{} }
func (mp mockPeer) SocketAddr() *p2p.NetAddress   { return &p2p.NetAddress{} }

func (mp mockPeer) Send(byte, []byte) bool    { return true }
func (mp mockPeer) TrySend(byte, []byte) bool { return true }

func (mp mockPeer) Set(string, interface{}) {}
func (mp mockPeer) Get(string) interface{}  { return struct{}{} }

//nolint:unused
type mockBlockStore struct {
	blocks map[int64]*types.Block
}

func (ml *mockBlockStore) Height() int64 {
	return int64(len(ml.blocks))
}

func (ml *mockBlockStore) LoadBlock(height int64) *types.Block {
	return ml.blocks[height]
}

func (ml *mockBlockStore) SaveBlock(block *types.Block, part *types.PartSet, commit *types.Commit) {
	ml.blocks[block.Height] = block
}

type mockBlockApplier struct {
}

// XXX: Add whitelist/blacklist?
func (mba *mockBlockApplier) ApplyBlock(
	state sm.State, blockID types.BlockID, block *types.Block,
) (sm.State, int64, error) {
	state.LastBlockHeight++
	return state, 0, nil
}

type mockSwitchIo struct {
	mtx                 sync.Mutex
	switchedToConsensus bool
	numStatusResponse   int
	numBlockResponse    int
	numNoBlockResponse  int
}

func (sio *mockSwitchIo) sendBlockRequest(peerID p2p.ID, height int64) error {
	return nil
}

func (sio *mockSwitchIo) sendStatusResponse(height int64, peerID p2p.ID) error {
	sio.mtx.Lock()
	defer sio.mtx.Unlock()
	sio.numStatusResponse++
	return nil
}

func (sio *mockSwitchIo) sendBlockToPeer(block *types.Block, peerID p2p.ID) error {
	sio.mtx.Lock()
	defer sio.mtx.Unlock()
	sio.numBlockResponse++
	return nil
}

func (sio *mockSwitchIo) sendBlockNotFound(height int64, peerID p2p.ID) error {
	sio.mtx.Lock()
	defer sio.mtx.Unlock()
	sio.numNoBlockResponse++
	return nil
}

func (sio *mockSwitchIo) trySwitchToConsensus(state sm.State, blocksSynced int) {
	sio.mtx.Lock()
	defer sio.mtx.Unlock()
	sio.switchedToConsensus = true
}

func (sio *mockSwitchIo) broadcastStatusRequest(base int64, height int64) {
}

type testReactorParams struct {
	logger      log.Logger
	genDoc      *types.GenesisDoc
	privVals    []types.PrivValidator
	startHeight int64
	bufferSize  int
	mockA       bool
}

func newTestReactor(p testReactorParams) *BlockchainReactor {
	store, state, _ := newReactorStore(p.genDoc, p.privVals, p.startHeight)
	reporter := behaviour.NewMockReporter()
	var appl blockApplier

	if p.mockA {
		appl = &mockBlockApplier{}
	} else {
		app := &testApp{}
		cc := proxy.NewLocalClientCreator(app)
		proxyApp := proxy.NewAppConns(cc)
		err := proxyApp.Start()
		if err != nil {
			panic(errors.Wrap(err, "error start app"))
		}
		db := dbm.NewMemDB()
		appl = sm.NewBlockExecutor(db, p.logger, proxyApp.Consensus(), mock.Mempool{}, sm.MockEvidencePool{})
		sm.SaveState(db, state)
	}

	r := newReactor(state, store, reporter, appl, p.bufferSize, true)
	logger := log.TestingLogger()
	r.SetLogger(logger.With("module", "blockchain"))

	return r
}

// This test is left here and not deleted to retain the termination cases for
// future improvement in [#4482](https://github.com/tendermint/tendermint/issues/4482).
// func TestReactorTerminationScenarios(t *testing.T) {

// 	config := cfg.ResetTestRoot("blockchain_reactor_v2_test")
// 	defer os.RemoveAll(config.RootDir)
// 	genDoc, privVals := randGenesisDoc(config.ChainID(), 1, false, 30)
// 	refStore, _, _ := newReactorStore(genDoc, privVals, 20)

// 	params := testReactorParams{
// 		logger:      log.TestingLogger(),
// 		genDoc:      genDoc,
// 		privVals:    privVals,
// 		startHeight: 10,
// 		bufferSize:  100,
// 		mockA:       true,
// 	}

// 	type testEvent struct {
// 		evType string
// 		peer   string
// 		height int64
// 	}

// 	tests := []struct {
// 		name   string
// 		params testReactorParams
// 		msgs   []testEvent
// 	}{
// 		{
// 			name:   "simple termination on max peer height - one peer",
// 			params: params,
// 			msgs: []testEvent{
// 				{evType: "AddPeer", peer: "P1"},
// 				{evType: "ReceiveS", peer: "P1", height: 13},
// 				{evType: "BlockReq"},
// 				{evType: "ReceiveB", peer: "P1", height: 11},
// 				{evType: "BlockReq"},
// 				{evType: "BlockReq"},
// 				{evType: "ReceiveB", peer: "P1", height: 12},
// 				{evType: "Process"},
// 				{evType: "ReceiveB", peer: "P1", height: 13},
// 				{evType: "Process"},
// 			},
// 		},
// 		{
// 			name:   "simple termination on max peer height - two peers",
// 			params: params,
// 			msgs: []testEvent{
// 				{evType: "AddPeer", peer: "P1"},
// 				{evType: "AddPeer", peer: "P2"},
// 				{evType: "ReceiveS", peer: "P1", height: 13},
// 				{evType: "ReceiveS", peer: "P2", height: 15},
// 				{evType: "BlockReq"},
// 				{evType: "BlockReq"},
// 				{evType: "ReceiveB", peer: "P1", height: 11},
// 				{evType: "ReceiveB", peer: "P2", height: 12},
// 				{evType: "Process"},
// 				{evType: "BlockReq"},
// 				{evType: "BlockReq"},
// 				{evType: "ReceiveB", peer: "P1", height: 13},
// 				{evType: "Process"},
// 				{evType: "ReceiveB", peer: "P2", height: 14},
// 				{evType: "Process"},
// 				{evType: "BlockReq"},
// 				{evType: "ReceiveB", peer: "P2", height: 15},
// 				{evType: "Process"},
// 			},
// 		},
// 		{
// 			name:   "termination on max peer height - two peers, noBlock error",
// 			params: params,
// 			msgs: []testEvent{
// 				{evType: "AddPeer", peer: "P1"},
// 				{evType: "AddPeer", peer: "P2"},
// 				{evType: "ReceiveS", peer: "P1", height: 13},
// 				{evType: "ReceiveS", peer: "P2", height: 15},
// 				{evType: "BlockReq"},
// 				{evType: "BlockReq"},
// 				{evType: "ReceiveNB", peer: "P1", height: 11},
// 				{evType: "BlockReq"},
// 				{evType: "ReceiveB", peer: "P2", height: 12},
// 				{evType: "ReceiveB", peer: "P2", height: 11},
// 				{evType: "Process"},
// 				{evType: "BlockReq"},
// 				{evType: "BlockReq"},
// 				{evType: "ReceiveB", peer: "P2", height: 13},
// 				{evType: "Process"},
// 				{evType: "ReceiveB", peer: "P2", height: 14},
// 				{evType: "Process"},
// 				{evType: "BlockReq"},
// 				{evType: "ReceiveB", peer: "P2", height: 15},
// 				{evType: "Process"},
// 			},
// 		},
// 		{
// 			name:   "termination on max peer height - two peers, remove one peer",
// 			params: params,
// 			msgs: []testEvent{
// 				{evType: "AddPeer", peer: "P1"},
// 				{evType: "AddPeer", peer: "P2"},
// 				{evType: "ReceiveS", peer: "P1", height: 13},
// 				{evType: "ReceiveS", peer: "P2", height: 15},
// 				{evType: "BlockReq"},
// 				{evType: "BlockReq"},
// 				{evType: "RemovePeer", peer: "P1"},
// 				{evType: "BlockReq"},
// 				{evType: "ReceiveB", peer: "P2", height: 12},
// 				{evType: "ReceiveB", peer: "P2", height: 11},
// 				{evType: "Process"},
// 				{evType: "BlockReq"},
// 				{evType: "BlockReq"},
// 				{evType: "ReceiveB", peer: "P2", height: 13},
// 				{evType: "Process"},
// 				{evType: "ReceiveB", peer: "P2", height: 14},
// 				{evType: "Process"},
// 				{evType: "BlockReq"},
// 				{evType: "ReceiveB", peer: "P2", height: 15},
// 				{evType: "Process"},
// 			},
// 		},
// 	}

// 	for _, tt := range tests {
// 		tt := tt
// 		t.Run(tt.name, func(t *testing.T) {
// 			reactor := newTestReactor(params)
// 			reactor.Start()
// 			reactor.reporter = behaviour.NewMockReporter()
// 			mockSwitch := &mockSwitchIo{switchedToConsensus: false}
// 			reactor.io = mockSwitch
// 			// time for go routines to start
// 			time.Sleep(time.Millisecond)

// 			for _, step := range tt.msgs {
// 				switch step.evType {
// 				case "AddPeer":
// 					reactor.scheduler.send(bcAddNewPeer{peerID: p2p.ID(step.peer)})
// 				case "RemovePeer":
// 					reactor.scheduler.send(bcRemovePeer{peerID: p2p.ID(step.peer)})
// 				case "ReceiveS":
// 					reactor.scheduler.send(bcStatusResponse{
// 						peerID: p2p.ID(step.peer),
// 						height: step.height,
// 						time:   time.Now(),
// 					})
// 				case "ReceiveB":
// 					reactor.scheduler.send(bcBlockResponse{
// 						peerID: p2p.ID(step.peer),
// 						block:  refStore.LoadBlock(step.height),
// 						size:   10,
// 						time:   time.Now(),
// 					})
// 				case "ReceiveNB":
// 					reactor.scheduler.send(bcNoBlockResponse{
// 						peerID: p2p.ID(step.peer),
// 						height: step.height,
// 						time:   time.Now(),
// 					})
// 				case "BlockReq":
// 					reactor.scheduler.send(rTrySchedule{time: time.Now()})
// 				case "Process":
// 					reactor.processor.send(rProcessBlock{})
// 				}
// 				// give time for messages to propagate between routines
// 				time.Sleep(time.Millisecond)
// 			}

// 			// time for processor to finish and reactor to switch to consensus
// 			time.Sleep(20 * time.Millisecond)
// 			assert.True(t, mockSwitch.hasSwitchedToConsensus())
// 			reactor.Stop()
// 		})
// 	}
// }

func TestReactorHelperMode(t *testing.T) {
	var (
		channelID = byte(0x40)
	)

	config := cfg.ResetTestRoot("blockchain_reactor_v2_test")
	defer os.RemoveAll(config.RootDir)
	genDoc, privVals := randGenesisDoc(config.ChainID(), 1, false, 30)

	params := testReactorParams{
		logger:      log.TestingLogger(),
		genDoc:      genDoc,
		privVals:    privVals,
		startHeight: 20,
		bufferSize:  100,
		mockA:       true,
	}

	type testEvent struct {
		peer  string
		event interface{}
	}

	tests := []struct {
		name   string
		params testReactorParams
		msgs   []testEvent
	}{
		{
			name:   "status request",
			params: params,
			msgs: []testEvent{
				{"P1", bcStatusRequestMessage{}},
				{"P1", bcBlockRequestMessage{Height: 13}},
				{"P1", bcBlockRequestMessage{Height: 20}},
				{"P1", bcBlockRequestMessage{Height: 22}},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			reactor := newTestReactor(params)
			reactor.Start()
			mockSwitch := &mockSwitchIo{switchedToConsensus: false}
			reactor.io = mockSwitch

			for i := 0; i < len(tt.msgs); i++ {
				step := tt.msgs[i]
				switch ev := step.event.(type) {
				case bcStatusRequestMessage:
					old := mockSwitch.numStatusResponse
					reactor.Receive(channelID, mockPeer{id: p2p.ID(step.peer)}, cdc.MustMarshalBinaryBare(ev))
					assert.Equal(t, old+1, mockSwitch.numStatusResponse)
				case bcBlockRequestMessage:
					if ev.Height > params.startHeight {
						old := mockSwitch.numNoBlockResponse
						reactor.Receive(channelID, mockPeer{id: p2p.ID(step.peer)}, cdc.MustMarshalBinaryBare(ev))
						assert.Equal(t, old+1, mockSwitch.numNoBlockResponse)
					} else {
						old := mockSwitch.numBlockResponse
						reactor.Receive(channelID, mockPeer{id: p2p.ID(step.peer)}, cdc.MustMarshalBinaryBare(ev))
						assert.Equal(t, old+1, mockSwitch.numBlockResponse)
					}
				}
			}
			reactor.Stop()
		})
	}
}

func TestReactorSetSwitchNil(t *testing.T) {
	config := cfg.ResetTestRoot("blockchain_reactor_v2_test")
	defer os.RemoveAll(config.RootDir)
	genDoc, privVals := randGenesisDoc(config.ChainID(), 1, false, 30)

	reactor := newTestReactor(testReactorParams{
		logger:   log.TestingLogger(),
		genDoc:   genDoc,
		privVals: privVals,
	})
	reactor.SetSwitch(nil)

	assert.Nil(t, reactor.Switch)
	assert.Nil(t, reactor.io)
}

//----------------------------------------------
// utility funcs

func makeTxs(height int64) (txs []types.Tx) {
	for i := 0; i < 10; i++ {
		txs = append(txs, types.Tx([]byte{byte(height), byte(i)}))
	}
	return txs
}

func makeBlock(height int64, state sm.State, lastCommit *types.Commit) *types.Block {
	block, _ := state.MakeBlock(height, makeTxs(height), lastCommit, nil, state.Validators.GetProposer().Address)
	return block
}

type testApp struct {
	abci.BaseApplication
}

func randGenesisDoc(chainID string, numValidators int, randPower bool, minPower int64) (
	*types.GenesisDoc, []types.PrivValidator) {
	validators := make([]types.GenesisValidator, numValidators)
	privValidators := make([]types.PrivValidator, numValidators)
	for i := 0; i < numValidators; i++ {
		val, privVal := types.RandValidator(randPower, minPower)
		validators[i] = types.GenesisValidator{
			PubKey: val.PubKey,
			Power:  val.VotingPower,
		}
		privValidators[i] = privVal
	}
	sort.Sort(types.PrivValidatorsByAddress(privValidators))

	return &types.GenesisDoc{
		GenesisTime: tmtime.Now(),
		ChainID:     chainID,
		Validators:  validators,
	}, privValidators
}

// Why are we importing the entire blockExecutor dependency graph here
// when we have the facilities to
func newReactorStore(
	genDoc *types.GenesisDoc,
	privVals []types.PrivValidator,
	maxBlockHeight int64) (*store.BlockStore, sm.State, *sm.BlockExecutor) {
	if len(privVals) != 1 {
		panic("only support one validator")
	}
	app := &testApp{}
	cc := proxy.NewLocalClientCreator(app)
	proxyApp := proxy.NewAppConns(cc)
	err := proxyApp.Start()
	if err != nil {
		panic(errors.Wrap(err, "error start app"))
	}

	stateDB := dbm.NewMemDB()
	blockStore := store.NewBlockStore(dbm.NewMemDB())

	state, err := sm.LoadStateFromDBOrGenesisDoc(stateDB, genDoc)
	if err != nil {
		panic(errors.Wrap(err, "error constructing state from genesis file"))
	}

	db := dbm.NewMemDB()
	blockExec := sm.NewBlockExecutor(db, log.TestingLogger(), proxyApp.Consensus(),
		mock.Mempool{}, sm.MockEvidencePool{})
	sm.SaveState(db, state)

	// add blocks in
	for blockHeight := int64(1); blockHeight <= maxBlockHeight; blockHeight++ {
		lastCommit := types.NewCommit(blockHeight-1, 0, types.BlockID{}, nil)
		if blockHeight > 1 {
			lastBlockMeta := blockStore.LoadBlockMeta(blockHeight - 1)
			lastBlock := blockStore.LoadBlock(blockHeight - 1)
			vote, err := types.MakeVote(
				lastBlock.Header.Height,
				lastBlockMeta.BlockID,
				state.Validators,
				privVals[0],
				lastBlock.Header.ChainID,
				time.Now(),
			)
			if err != nil {
				panic(err)
			}
			lastCommit = types.NewCommit(vote.Height, vote.Round,
				lastBlockMeta.BlockID, []types.CommitSig{vote.CommitSig()})
		}

		thisBlock := makeBlock(blockHeight, state, lastCommit)

		thisParts := thisBlock.MakePartSet(types.BlockPartSizeBytes)
		blockID := types.BlockID{Hash: thisBlock.Hash(), PartsHeader: thisParts.Header()}

		state, _, err = blockExec.ApplyBlock(state, blockID, thisBlock)
		if err != nil {
			panic(errors.Wrap(err, "error apply block"))
		}

		blockStore.SaveBlock(thisBlock, thisParts, lastCommit)
	}
	return blockStore, state, blockExec
}
