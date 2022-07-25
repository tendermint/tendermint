package v2

import (
	"fmt"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abciclient "github.com/tendermint/tendermint/abci/client"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/blocksync/v2/internal/behavior"
	"github.com/tendermint/tendermint/internal/consensus"
	"github.com/tendermint/tendermint/internal/mempool/mock"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/internal/p2p/conn"
	"github.com/tendermint/tendermint/internal/proxy"
	sm "github.com/tendermint/tendermint/internal/state"
	sf "github.com/tendermint/tendermint/internal/state/test/factory"
	tmstore "github.com/tendermint/tendermint/internal/store"
	"github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	bcproto "github.com/tendermint/tendermint/proto/tendermint/blocksync"
	"github.com/tendermint/tendermint/types"
)

type mockPeer struct {
	service.Service
	id types.NodeID
}

func (mp mockPeer) FlushStop()           {}
func (mp mockPeer) ID() types.NodeID     { return mp.id }
func (mp mockPeer) RemoteIP() net.IP     { return net.IP{} }
func (mp mockPeer) RemoteAddr() net.Addr { return &net.TCPAddr{IP: mp.RemoteIP(), Port: 8800} }

func (mp mockPeer) IsOutbound() bool   { return true }
func (mp mockPeer) IsPersistent() bool { return true }
func (mp mockPeer) CloseConn() error   { return nil }

func (mp mockPeer) NodeInfo() types.NodeInfo {
	return types.NodeInfo{
		NodeID:     "",
		ListenAddr: "",
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

//nolint:unused
func (ml *mockBlockStore) Height() int64 {
	return int64(len(ml.blocks))
}

//nolint:unused
func (ml *mockBlockStore) LoadBlock(height int64) *types.Block {
	return ml.blocks[height]
}

//nolint:unused
func (ml *mockBlockStore) SaveBlock(block *types.Block, part *types.PartSet, commit *types.Commit) {
	ml.blocks[block.Height] = block
}

type mockBlockApplier struct {
}

// XXX: Add whitelist/blacklist?
func (mba *mockBlockApplier) ApplyBlock(
	state sm.State, blockID types.BlockID, block *types.Block,
) (sm.State, error) {
	state.LastBlockHeight++
	return state, nil
}

type mockSwitchIo struct {
	mtx                 sync.Mutex
	switchedToConsensus bool
	numStatusResponse   int
	numBlockResponse    int
	numNoBlockResponse  int
	numStatusRequest    int
}

var _ iIO = (*mockSwitchIo)(nil)

func (sio *mockSwitchIo) sendBlockRequest(_ p2p.Peer, _ int64) error {
	return nil
}

func (sio *mockSwitchIo) sendStatusResponse(_, _ int64, _ p2p.Peer) error {
	sio.mtx.Lock()
	defer sio.mtx.Unlock()
	sio.numStatusResponse++
	return nil
}

func (sio *mockSwitchIo) sendBlockToPeer(_ *types.Block, _ p2p.Peer) error {
	sio.mtx.Lock()
	defer sio.mtx.Unlock()
	sio.numBlockResponse++
	return nil
}

func (sio *mockSwitchIo) sendBlockNotFound(_ int64, _ p2p.Peer) error {
	sio.mtx.Lock()
	defer sio.mtx.Unlock()
	sio.numNoBlockResponse++
	return nil
}

func (sio *mockSwitchIo) trySwitchToConsensus(_ sm.State, _ bool) bool {
	sio.mtx.Lock()
	defer sio.mtx.Unlock()
	sio.switchedToConsensus = true
	return true
}

func (sio *mockSwitchIo) broadcastStatusRequest() error {
	return nil
}

func (sio *mockSwitchIo) sendStatusRequest(_ p2p.Peer) error {
	sio.mtx.Lock()
	defer sio.mtx.Unlock()
	sio.numStatusRequest++
	return nil
}

type testReactorParams struct {
	logger      log.Logger
	genDoc      *types.GenesisDoc
	privVals    []types.PrivValidator
	startHeight int64
	mockA       bool
}

func newTestReactor(t *testing.T, p testReactorParams) *BlockchainReactor {
	store, state, _ := newReactorStore(t, p.genDoc, p.privVals, p.startHeight)
	reporter := behavior.NewMockReporter()

	var appl blockApplier

	if p.mockA {
		appl = &mockBlockApplier{}
	} else {
		app := &testApp{}
		cc := abciclient.NewLocalCreator(app)
		proxyApp := proxy.NewAppConns(cc, proxy.NopMetrics())
		err := proxyApp.Start()
		require.NoError(t, err)
		db := dbm.NewMemDB()
		stateStore := sm.NewStore(db, false)
		blockStore := tmstore.NewBlockStore(dbm.NewMemDB())
		appl = sm.NewBlockExecutor(
			stateStore, p.logger, proxyApp.Consensus(), mock.Mempool{}, sm.EmptyEvidencePool{}, blockStore)
		err = stateStore.Save(state)
		require.NoError(t, err)
	}

	r := newReactor(state, store, reporter, appl, true, consensus.NopMetrics())
	logger := log.TestingLogger()
	r.SetLogger(logger.With("module", "blockchain"))

	return r
}

// This test is left here and not deleted to retain the termination cases for
// future improvement in [#4482](https://github.com/tendermint/tendermint/issues/4482).
// func TestReactorTerminationScenarios(t *testing.T) {

//	config := cfg.ResetTestRoot("blockchain_reactor_v2_test")
//	defer os.RemoveAll(config.RootDir)
//	genDoc, privVals := randGenesisDoc(config.ChainID(), 1, false, 30)
//	refStore, _, _ := newReactorStore(genDoc, privVals, 20)

//	params := testReactorParams{
//		logger:      log.TestingLogger(),
//		genDoc:      genDoc,
//		privVals:    privVals,
//		startHeight: 10,
//		bufferSize:  100,
//		mockA:       true,
//	}

//	type testEvent struct {
//		evType string
//		peer   string
//		height int64
//	}

//	tests := []struct {
//		name   string
//		params testReactorParams
//		msgs   []testEvent
//	}{
//		{
//			name:   "simple termination on max peer height - one peer",
//			params: params,
//			msgs: []testEvent{
//				{evType: "AddPeer", peer: "P1"},
//				{evType: "ReceiveS", peer: "P1", height: 13},
//				{evType: "BlockReq"},
//				{evType: "ReceiveB", peer: "P1", height: 11},
//				{evType: "BlockReq"},
//				{evType: "BlockReq"},
//				{evType: "ReceiveB", peer: "P1", height: 12},
//				{evType: "Process"},
//				{evType: "ReceiveB", peer: "P1", height: 13},
//				{evType: "Process"},
//			},
//		},
//		{
//			name:   "simple termination on max peer height - two peers",
//			params: params,
//			msgs: []testEvent{
//				{evType: "AddPeer", peer: "P1"},
//				{evType: "AddPeer", peer: "P2"},
//				{evType: "ReceiveS", peer: "P1", height: 13},
//				{evType: "ReceiveS", peer: "P2", height: 15},
//				{evType: "BlockReq"},
//				{evType: "BlockReq"},
//				{evType: "ReceiveB", peer: "P1", height: 11},
//				{evType: "ReceiveB", peer: "P2", height: 12},
//				{evType: "Process"},
//				{evType: "BlockReq"},
//				{evType: "BlockReq"},
//				{evType: "ReceiveB", peer: "P1", height: 13},
//				{evType: "Process"},
//				{evType: "ReceiveB", peer: "P2", height: 14},
//				{evType: "Process"},
//				{evType: "BlockReq"},
//				{evType: "ReceiveB", peer: "P2", height: 15},
//				{evType: "Process"},
//			},
//		},
//		{
//			name:   "termination on max peer height - two peers, noBlock error",
//			params: params,
//			msgs: []testEvent{
//				{evType: "AddPeer", peer: "P1"},
//				{evType: "AddPeer", peer: "P2"},
//				{evType: "ReceiveS", peer: "P1", height: 13},
//				{evType: "ReceiveS", peer: "P2", height: 15},
//				{evType: "BlockReq"},
//				{evType: "BlockReq"},
//				{evType: "ReceiveNB", peer: "P1", height: 11},
//				{evType: "BlockReq"},
//				{evType: "ReceiveB", peer: "P2", height: 12},
//				{evType: "ReceiveB", peer: "P2", height: 11},
//				{evType: "Process"},
//				{evType: "BlockReq"},
//				{evType: "BlockReq"},
//				{evType: "ReceiveB", peer: "P2", height: 13},
//				{evType: "Process"},
//				{evType: "ReceiveB", peer: "P2", height: 14},
//				{evType: "Process"},
//				{evType: "BlockReq"},
//				{evType: "ReceiveB", peer: "P2", height: 15},
//				{evType: "Process"},
//			},
//		},
//		{
//			name:   "termination on max peer height - two peers, remove one peer",
//			params: params,
//			msgs: []testEvent{
//				{evType: "AddPeer", peer: "P1"},
//				{evType: "AddPeer", peer: "P2"},
//				{evType: "ReceiveS", peer: "P1", height: 13},
//				{evType: "ReceiveS", peer: "P2", height: 15},
//				{evType: "BlockReq"},
//				{evType: "BlockReq"},
//				{evType: "RemovePeer", peer: "P1"},
//				{evType: "BlockReq"},
//				{evType: "ReceiveB", peer: "P2", height: 12},
//				{evType: "ReceiveB", peer: "P2", height: 11},
//				{evType: "Process"},
//				{evType: "BlockReq"},
//				{evType: "BlockReq"},
//				{evType: "ReceiveB", peer: "P2", height: 13},
//				{evType: "Process"},
//				{evType: "ReceiveB", peer: "P2", height: 14},
//				{evType: "Process"},
//				{evType: "BlockReq"},
//				{evType: "ReceiveB", peer: "P2", height: 15},
//				{evType: "Process"},
//			},
//		},
//	}

//	for _, tt := range tests {
//		tt := tt
//		t.Run(tt.name, func(t *testing.T) {
//			reactor := newTestReactor(params)
//			reactor.Start()
//			reactor.reporter = behavior.NewMockReporter()
//			mockSwitch := &mockSwitchIo{switchedToConsensus: false}
//			reactor.io = mockSwitch
//			// time for go routines to start
//			time.Sleep(time.Millisecond)

//			for _, step := range tt.msgs {
//				switch step.evType {
//				case "AddPeer":
//					reactor.scheduler.send(bcAddNewPeer{peerID: p2p.ID(step.peer)})
//				case "RemovePeer":
//					reactor.scheduler.send(bcRemovePeer{peerID: p2p.ID(step.peer)})
//				case "ReceiveS":
//					reactor.scheduler.send(bcStatusResponse{
//						peerID: p2p.ID(step.peer),
//						height: step.height,
//						time:   time.Now(),
//					})
//				case "ReceiveB":
//					reactor.scheduler.send(bcBlockResponse{
//						peerID: p2p.ID(step.peer),
//						block:  refStore.LoadBlock(step.height),
//						size:   10,
//						time:   time.Now(),
//					})
//				case "ReceiveNB":
//					reactor.scheduler.send(bcNoBlockResponse{
//						peerID: p2p.ID(step.peer),
//						height: step.height,
//						time:   time.Now(),
//					})
//				case "BlockReq":
//					reactor.scheduler.send(rTrySchedule{time: time.Now()})
//				case "Process":
//					reactor.processor.send(rProcessBlock{})
//				}
//				// give time for messages to propagate between routines
//				time.Sleep(time.Millisecond)
//			}

//			// time for processor to finish and reactor to switch to consensus
//			time.Sleep(20 * time.Millisecond)
//			assert.True(t, mockSwitch.hasSwitchedToConsensus())
//			reactor.Stop()
//		})
//	}
// }

func TestReactorHelperMode(t *testing.T) {
	var (
		channelID = byte(0x40)
	)

	cfg, err := config.ResetTestRoot("blockchain_reactor_v2_test")
	require.NoError(t, err)
	defer os.RemoveAll(cfg.RootDir)
	genDoc, privVals := factory.RandGenesisDoc(cfg, 1, false, 30)

	params := testReactorParams{
		logger:      log.TestingLogger(),
		genDoc:      genDoc,
		privVals:    privVals,
		startHeight: 20,
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
				{"P1", bcproto.StatusRequest{}},
				{"P1", bcproto.BlockRequest{Height: 13}},
				{"P1", bcproto.BlockRequest{Height: 20}},
				{"P1", bcproto.BlockRequest{Height: 22}},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			reactor := newTestReactor(t, params)
			mockSwitch := &mockSwitchIo{switchedToConsensus: false}
			reactor.io = mockSwitch
			err := reactor.Start()
			require.NoError(t, err)

			for i := 0; i < len(tt.msgs); i++ {
				step := tt.msgs[i]
				switch ev := step.event.(type) {
				case bcproto.StatusRequest:
					old := mockSwitch.numStatusResponse

					msgProto := new(bcproto.Message)
					require.NoError(t, msgProto.Wrap(&ev))

					msgBz, err := proto.Marshal(msgProto)
					require.NoError(t, err)

					reactor.Receive(channelID, mockPeer{id: types.NodeID(step.peer)}, msgBz)
					assert.Equal(t, old+1, mockSwitch.numStatusResponse)
				case bcproto.BlockRequest:
					if ev.Height > params.startHeight {
						old := mockSwitch.numNoBlockResponse

						msgProto := new(bcproto.Message)
						require.NoError(t, msgProto.Wrap(&ev))

						msgBz, err := proto.Marshal(msgProto)
						require.NoError(t, err)

						reactor.Receive(channelID, mockPeer{id: types.NodeID(step.peer)}, msgBz)
						assert.Equal(t, old+1, mockSwitch.numNoBlockResponse)
					} else {
						old := mockSwitch.numBlockResponse

						msgProto := new(bcproto.Message)
						require.NoError(t, msgProto.Wrap(&ev))

						msgBz, err := proto.Marshal(msgProto)
						require.NoError(t, err)

						reactor.Receive(channelID, mockPeer{id: types.NodeID(step.peer)}, msgBz)
						assert.Equal(t, old+1, mockSwitch.numBlockResponse)
					}
				}
			}
			err = reactor.Stop()
			require.NoError(t, err)
		})
	}
}

func TestReactorSetSwitchNil(t *testing.T) {
	cfg, err := config.ResetTestRoot("blockchain_reactor_v2_test")
	require.NoError(t, err)
	defer os.RemoveAll(cfg.RootDir)
	genDoc, privVals := factory.RandGenesisDoc(cfg, 1, false, 30)

	reactor := newTestReactor(t, testReactorParams{
		logger:   log.TestingLogger(),
		genDoc:   genDoc,
		privVals: privVals,
	})
	reactor.SetSwitch(nil)

	assert.Nil(t, reactor.Switch)
	assert.Nil(t, reactor.io)
}

type testApp struct {
	abci.BaseApplication
}

func newReactorStore(
	t *testing.T,
	genDoc *types.GenesisDoc,
	privVals []types.PrivValidator,
	maxBlockHeight int64) (*tmstore.BlockStore, sm.State, *sm.BlockExecutor) {
	t.Helper()

	require.Len(t, privVals, 1)
	app := &testApp{}
	cc := abciclient.NewLocalCreator(app)
	proxyApp := proxy.NewAppConns(cc, proxy.NopMetrics())
	err := proxyApp.Start()
	if err != nil {
		panic(fmt.Errorf("error start app: %w", err))
	}

	stateDB := dbm.NewMemDB()
	blockStore := tmstore.NewBlockStore(dbm.NewMemDB())
	stateStore := sm.NewStore(stateDB, false)
	state, err := sm.MakeGenesisState(genDoc)
	require.NoError(t, err)

	blockExec := sm.NewBlockExecutor(stateStore, log.TestingLogger(), proxyApp.Consensus(),
		mock.Mempool{}, sm.EmptyEvidencePool{}, blockStore)
	err = stateStore.Save(state)
	require.NoError(t, err)

	// add blocks in
	for blockHeight := int64(1); blockHeight <= maxBlockHeight; blockHeight++ {
		lastCommit := types.NewCommit(blockHeight-1, 0, types.BlockID{}, nil)
		if blockHeight > 1 {
			lastBlockMeta := blockStore.LoadBlockMeta(blockHeight - 1)
			lastBlock := blockStore.LoadBlock(blockHeight - 1)
			vote, err := factory.MakeVote(
				privVals[0],
				lastBlock.Header.ChainID, 0,
				lastBlock.Header.Height, 0, 2,
				lastBlockMeta.BlockID,
				time.Now(),
			)
			require.NoError(t, err)
			lastCommit = types.NewCommit(vote.Height, vote.Round,
				lastBlockMeta.BlockID, []types.CommitSig{vote.CommitSig()})
		}

		thisBlock := sf.MakeBlock(state, blockHeight, lastCommit)

		thisParts := thisBlock.MakePartSet(types.BlockPartSizeBytes)
		blockID := types.BlockID{Hash: thisBlock.Hash(), PartSetHeader: thisParts.Header()}

		state, err = blockExec.ApplyBlock(state, blockID, thisBlock)
		require.NoError(t, err)

		blockStore.SaveBlock(thisBlock, thisParts, lastCommit)
	}
	return blockStore, state, blockExec
}
