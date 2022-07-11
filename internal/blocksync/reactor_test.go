package blocksync

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abciclient "github.com/tendermint/tendermint/abci/client"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/internal/consensus"
	"github.com/tendermint/tendermint/internal/eventbus"
	mpmocks "github.com/tendermint/tendermint/internal/mempool/mocks"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/internal/p2p/p2ptest"
	"github.com/tendermint/tendermint/internal/proxy"
	sm "github.com/tendermint/tendermint/internal/state"
	sf "github.com/tendermint/tendermint/internal/state/test/factory"
	"github.com/tendermint/tendermint/internal/store"
	"github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/libs/log"
	bcproto "github.com/tendermint/tendermint/proto/tendermint/blocksync"
	"github.com/tendermint/tendermint/types"
)

type reactorTestSuite struct {
	network *p2ptest.Network
	logger  log.Logger
	nodes   []types.NodeID

	reactors map[types.NodeID]*Reactor
	app      map[types.NodeID]abciclient.Client

	blockSyncChannels map[types.NodeID]p2p.Channel
	peerChans         map[types.NodeID]chan p2p.PeerUpdate
	peerUpdates       map[types.NodeID]*p2p.PeerUpdates
}

func setup(
	ctx context.Context,
	t *testing.T,
	genDoc *types.GenesisDoc,
	privVal types.PrivValidator,
	maxBlockHeights []int64,
) *reactorTestSuite {
	t.Helper()

	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)

	numNodes := len(maxBlockHeights)
	require.True(t, numNodes >= 1,
		"must specify at least one block height (nodes)")

	rts := &reactorTestSuite{
		logger:            log.NewNopLogger().With("module", "block_sync", "testCase", t.Name()),
		network:           p2ptest.MakeNetwork(ctx, t, p2ptest.NetworkOptions{NumNodes: numNodes}),
		nodes:             make([]types.NodeID, 0, numNodes),
		reactors:          make(map[types.NodeID]*Reactor, numNodes),
		app:               make(map[types.NodeID]abciclient.Client, numNodes),
		blockSyncChannels: make(map[types.NodeID]p2p.Channel, numNodes),
		peerChans:         make(map[types.NodeID]chan p2p.PeerUpdate, numNodes),
		peerUpdates:       make(map[types.NodeID]*p2p.PeerUpdates, numNodes),
	}

	chDesc := &p2p.ChannelDescriptor{ID: BlockSyncChannel, MessageType: new(bcproto.Message)}
	rts.blockSyncChannels = rts.network.MakeChannelsNoCleanup(ctx, t, chDesc)

	i := 0
	for nodeID := range rts.network.Nodes {
		rts.addNode(ctx, t, nodeID, genDoc, privVal, maxBlockHeights[i])
		i++
	}

	t.Cleanup(func() {
		cancel()
		for _, nodeID := range rts.nodes {
			if rts.reactors[nodeID].IsRunning() {
				rts.reactors[nodeID].Wait()
				rts.app[nodeID].Wait()

				require.False(t, rts.reactors[nodeID].IsRunning())
			}
		}
	})
	t.Cleanup(leaktest.Check(t))

	return rts
}

func makeReactor(
	ctx context.Context,
	t *testing.T,
	nodeID types.NodeID,
	genDoc *types.GenesisDoc,
	privVal types.PrivValidator,
	channelCreator p2p.ChannelCreator,
	peerEvents p2p.PeerEventSubscriber) *Reactor {

	logger := log.NewNopLogger()

	app := proxy.New(abciclient.NewLocalClient(logger, &abci.BaseApplication{}), logger, proxy.NopMetrics())
	require.NoError(t, app.Start(ctx))

	blockDB := dbm.NewMemDB()
	stateDB := dbm.NewMemDB()
	stateStore := sm.NewStore(stateDB)
	blockStore := store.NewBlockStore(blockDB)

	state, err := sm.MakeGenesisState(genDoc)
	require.NoError(t, err)
	require.NoError(t, stateStore.Save(state))
	mp := &mpmocks.Mempool{}
	mp.On("Lock").Return()
	mp.On("Unlock").Return()
	mp.On("FlushAppConn", mock.Anything).Return(nil)
	mp.On("Update",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(nil)

	eventbus := eventbus.NewDefault(logger)
	require.NoError(t, eventbus.Start(ctx))

	blockExec := sm.NewBlockExecutor(
		stateStore,
		log.NewNopLogger(),
		app,
		mp,
		sm.EmptyEvidencePool{},
		blockStore,
		eventbus,
		sm.NopMetrics(),
	)

	return NewReactor(
		logger,
		stateStore,
		blockExec,
		blockStore,
		nil,
		channelCreator,
		peerEvents,
		true,
		consensus.NopMetrics(),
		nil, // eventbus, can be nil
	)
}

func (rts *reactorTestSuite) addNode(
	ctx context.Context,
	t *testing.T,
	nodeID types.NodeID,
	genDoc *types.GenesisDoc,
	privVal types.PrivValidator,
	maxBlockHeight int64,
) {
	t.Helper()

	logger := log.NewNopLogger()

	rts.nodes = append(rts.nodes, nodeID)
	rts.app[nodeID] = proxy.New(abciclient.NewLocalClient(logger, &abci.BaseApplication{}), logger, proxy.NopMetrics())
	require.NoError(t, rts.app[nodeID].Start(ctx))

	rts.peerChans[nodeID] = make(chan p2p.PeerUpdate)
	rts.peerUpdates[nodeID] = p2p.NewPeerUpdates(rts.peerChans[nodeID], 1)
	rts.network.Nodes[nodeID].PeerManager.Register(ctx, rts.peerUpdates[nodeID])

	chCreator := func(ctx context.Context, chdesc *p2p.ChannelDescriptor) (p2p.Channel, error) {
		return rts.blockSyncChannels[nodeID], nil
	}

	peerEvents := func(ctx context.Context) *p2p.PeerUpdates { return rts.peerUpdates[nodeID] }
	reactor := makeReactor(ctx, t, nodeID, genDoc, privVal, chCreator, peerEvents)

	lastExtCommit := &types.ExtendedCommit{}

	state, err := reactor.stateStore.Load()
	require.NoError(t, err)
	for blockHeight := int64(1); blockHeight <= maxBlockHeight; blockHeight++ {
		block, blockID, partSet, seenExtCommit := makeNextBlock(ctx, t, state, privVal, blockHeight, lastExtCommit)

		state, err = reactor.blockExec.ApplyBlock(ctx, state, blockID, block)
		require.NoError(t, err)

		reactor.store.SaveBlockWithExtendedCommit(block, partSet, seenExtCommit)
		lastExtCommit = seenExtCommit
	}

	rts.reactors[nodeID] = reactor
	require.NoError(t, reactor.Start(ctx))
	require.True(t, reactor.IsRunning())
}

func makeNextBlock(ctx context.Context,
	t *testing.T,
	state sm.State,
	signer types.PrivValidator,
	height int64,
	lc *types.ExtendedCommit) (*types.Block, types.BlockID, *types.PartSet, *types.ExtendedCommit) {

	lastExtCommit := lc.Clone()

	block := sf.MakeBlock(state, height, lastExtCommit.ToCommit())
	partSet, err := block.MakePartSet(types.BlockPartSizeBytes)
	require.NoError(t, err)
	blockID := types.BlockID{Hash: block.Hash(), PartSetHeader: partSet.Header()}

	// Simulate a commit for the current height
	vote, err := factory.MakeVote(
		ctx,
		signer,
		block.Header.ChainID,
		0,
		block.Header.Height,
		0,
		2,
		blockID,
		time.Now(),
	)
	require.NoError(t, err)
	seenExtCommit := &types.ExtendedCommit{
		Height:             vote.Height,
		Round:              vote.Round,
		BlockID:            blockID,
		ExtendedSignatures: []types.ExtendedCommitSig{vote.ExtendedCommitSig()},
	}
	return block, blockID, partSet, seenExtCommit

}

func (rts *reactorTestSuite) start(ctx context.Context, t *testing.T) {
	t.Helper()
	rts.network.Start(ctx, t)
	require.Len(t,
		rts.network.RandomNode().PeerManager.Peers(),
		len(rts.nodes)-1,
		"network does not have expected number of nodes")
}

func TestReactor_AbruptDisconnect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.ResetTestRoot(t.TempDir(), "block_sync_reactor_test")
	require.NoError(t, err)
	defer os.RemoveAll(cfg.RootDir)

	valSet, privVals := factory.ValidatorSet(ctx, t, 1, 30)
	genDoc := factory.GenesisDoc(cfg, time.Now(), valSet.Validators, factory.ConsensusParams())
	maxBlockHeight := int64(64)

	rts := setup(ctx, t, genDoc, privVals[0], []int64{maxBlockHeight, 0})

	require.Equal(t, maxBlockHeight, rts.reactors[rts.nodes[0]].store.Height())

	rts.start(ctx, t)

	secondaryPool := rts.reactors[rts.nodes[1]].pool

	require.Eventually(
		t,
		func() bool {
			height, _, _ := secondaryPool.GetStatus()
			return secondaryPool.MaxPeerHeight() > 0 && height > 0 && height < 10
		},
		10*time.Second,
		10*time.Millisecond,
		"expected node to be partially synced",
	)

	// Remove synced node from the syncing node which should not result in any
	// deadlocks or race conditions within the context of poolRoutine.
	rts.peerChans[rts.nodes[1]] <- p2p.PeerUpdate{
		Status: p2p.PeerStatusDown,
		NodeID: rts.nodes[0],
	}
	rts.network.Nodes[rts.nodes[1]].PeerManager.Disconnected(ctx, rts.nodes[0])
}

func TestReactor_SyncTime(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.ResetTestRoot(t.TempDir(), "block_sync_reactor_test")
	require.NoError(t, err)
	defer os.RemoveAll(cfg.RootDir)

	valSet, privVals := factory.ValidatorSet(ctx, t, 1, 30)
	genDoc := factory.GenesisDoc(cfg, time.Now(), valSet.Validators, factory.ConsensusParams())
	maxBlockHeight := int64(101)

	rts := setup(ctx, t, genDoc, privVals[0], []int64{maxBlockHeight, 0})
	require.Equal(t, maxBlockHeight, rts.reactors[rts.nodes[0]].store.Height())
	rts.start(ctx, t)

	require.Eventually(
		t,
		func() bool {
			return rts.reactors[rts.nodes[1]].GetRemainingSyncTime() > time.Nanosecond &&
				rts.reactors[rts.nodes[1]].pool.getLastSyncRate() > 0.001
		},
		10*time.Second,
		10*time.Millisecond,
		"expected node to be partially synced",
	)
}

func TestReactor_NoBlockResponse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.ResetTestRoot(t.TempDir(), "block_sync_reactor_test")
	require.NoError(t, err)
	defer os.RemoveAll(cfg.RootDir)

	valSet, privVals := factory.ValidatorSet(ctx, t, 1, 30)
	genDoc := factory.GenesisDoc(cfg, time.Now(), valSet.Validators, factory.ConsensusParams())
	maxBlockHeight := int64(65)

	rts := setup(ctx, t, genDoc, privVals[0], []int64{maxBlockHeight, 0})

	require.Equal(t, maxBlockHeight, rts.reactors[rts.nodes[0]].store.Height())

	rts.start(ctx, t)

	testCases := []struct {
		height   int64
		existent bool
	}{
		{maxBlockHeight + 2, false},
		{10, true},
		{1, true},
		{100, false},
	}

	secondaryPool := rts.reactors[rts.nodes[1]].pool
	require.Eventually(
		t,
		func() bool { return secondaryPool.MaxPeerHeight() > 0 && secondaryPool.IsCaughtUp() },
		10*time.Second,
		10*time.Millisecond,
		"expected node to be fully synced",
	)

	for _, tc := range testCases {
		block := rts.reactors[rts.nodes[1]].store.LoadBlock(tc.height)
		if tc.existent {
			require.True(t, block != nil)
		} else {
			require.Nil(t, block)
		}
	}
}

func TestReactor_BadBlockStopsPeer(t *testing.T) {
	// Ultimately, this should be refactored to be less integration test oriented
	// and more unit test oriented by simply testing channel sends and receives.
	// See: https://github.com/tendermint/tendermint/issues/6005
	t.SkipNow()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.ResetTestRoot(t.TempDir(), "block_sync_reactor_test")
	require.NoError(t, err)
	defer os.RemoveAll(cfg.RootDir)

	maxBlockHeight := int64(48)
	valSet, privVals := factory.ValidatorSet(ctx, t, 1, 30)
	genDoc := factory.GenesisDoc(cfg, time.Now(), valSet.Validators, factory.ConsensusParams())

	rts := setup(ctx, t, genDoc, privVals[0], []int64{maxBlockHeight, 0, 0, 0, 0})

	require.Equal(t, maxBlockHeight, rts.reactors[rts.nodes[0]].store.Height())

	rts.start(ctx, t)

	require.Eventually(
		t,
		func() bool {
			caughtUp := true
			for _, id := range rts.nodes[1 : len(rts.nodes)-1] {
				if rts.reactors[id].pool.MaxPeerHeight() == 0 || !rts.reactors[id].pool.IsCaughtUp() {
					caughtUp = false
				}
			}

			return caughtUp
		},
		10*time.Minute,
		10*time.Millisecond,
		"expected all nodes to be fully synced",
	)

	for _, id := range rts.nodes[:len(rts.nodes)-1] {
		require.Len(t, rts.reactors[id].pool.peers, 3)
	}

	// Mark testSuites[3] as an invalid peer which will cause newSuite to disconnect
	// from this peer.
	//
	// XXX: This causes a potential race condition.
	// See: https://github.com/tendermint/tendermint/issues/6005
	valSet, otherPrivVals := factory.ValidatorSet(ctx, t, 1, 30)
	otherGenDoc := factory.GenesisDoc(cfg, time.Now(), valSet.Validators, factory.ConsensusParams())
	newNode := rts.network.MakeNode(ctx, t, p2ptest.NodeOptions{
		MaxPeers:     uint16(len(rts.nodes) + 1),
		MaxConnected: uint16(len(rts.nodes) + 1),
	})
	rts.addNode(ctx, t, newNode.NodeID, otherGenDoc, otherPrivVals[0], maxBlockHeight)

	// add a fake peer just so we do not wait for the consensus ticker to timeout
	rts.reactors[newNode.NodeID].pool.SetPeerRange("00ff", 10, 10)

	// wait for the new peer to catch up and become fully synced
	require.Eventually(
		t,
		func() bool {
			return rts.reactors[newNode.NodeID].pool.MaxPeerHeight() > 0 && rts.reactors[newNode.NodeID].pool.IsCaughtUp()
		},
		10*time.Minute,
		10*time.Millisecond,
		"expected new node to be fully synced",
	)

	require.Eventuallyf(
		t,
		func() bool { return len(rts.reactors[newNode.NodeID].pool.peers) < len(rts.nodes)-1 },
		10*time.Minute,
		10*time.Millisecond,
		"invalid number of peers; expected < %d, got: %d",
		len(rts.nodes)-1,
		len(rts.reactors[newNode.NodeID].pool.peers),
	)
}

/*
func TestReactorReceivesNoExtendedCommit(t *testing.T) {
	blockDB := dbm.NewMemDB()
	stateDB := dbm.NewMemDB()
	stateStore := sm.NewStore(stateDB)
	blockStore := store.NewBlockStore(blockDB)
	blockExec := sm.NewBlockExecutor(
		stateStore,
		log.NewNopLogger(),
		rts.app[nodeID],
		mp,
		sm.EmptyEvidencePool{},
		blockStore,
		eventbus,
		sm.NopMetrics(),
	)
	NewReactor(
		log.NewNopLogger(),
		stateStore,
		blockExec,
		blockStore,
		nil,
		chCreator,
		func(ctx context.Context) *p2p.PeerUpdates { return rts.peerUpdates[nodeID] },
		rts.blockSync,
		consensus.NopMetrics(),
		nil, // eventbus, can be nil
	)

}
*/
