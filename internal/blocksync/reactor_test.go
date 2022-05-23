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

	blockSyncChannels map[types.NodeID]*p2p.Channel
	peerChans         map[types.NodeID]chan p2p.PeerUpdate
	peerUpdates       map[types.NodeID]*p2p.PeerUpdates

	blockSync bool
}

func setup(
	ctx context.Context,
	t *testing.T,
	genDoc *types.GenesisDoc,
	privValArray []types.PrivValidator,
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
		blockSyncChannels: make(map[types.NodeID]*p2p.Channel, numNodes),
		peerChans:         make(map[types.NodeID]chan p2p.PeerUpdate, numNodes),
		peerUpdates:       make(map[types.NodeID]*p2p.PeerUpdates, numNodes),
		blockSync:         true,
	}

	chDesc := &p2p.ChannelDescriptor{ID: BlockSyncChannel, MessageType: new(bcproto.Message)}
	rts.blockSyncChannels = rts.network.MakeChannelsNoCleanup(ctx, t, chDesc)

	if maxBlockHeights[1] != 0 {
		rts.addMultipleNodes(ctx, t, rts.network.NodeIDs(), genDoc, privValArray, maxBlockHeights, 0)
	} else {
		i := 0
		for nodeID := range rts.network.Nodes {
			rts.addNode(ctx, t, nodeID, genDoc, privValArray[0], maxBlockHeights[i])
			i++
		}
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

// We add multiple nodes with varying initial heights
// Allows us to test whether block sync works when a node
// has previous state
// maxBlockHeightPerNode - the heights for which the node already has state
// maxBlockHeightIdx - the index of the node with maximum height
func (rts *reactorTestSuite) addMultipleNodes(
	ctx context.Context,
	t *testing.T,
	nodeIDs []types.NodeID,
	genDoc *types.GenesisDoc,
	privValArray []types.PrivValidator,
	maxBlockHeightPerNode []int64,
	maxBlockHeightIdx int64,

) {
	t.Helper()

	logger := log.NewNopLogger()
	blockDB := make([]*dbm.MemDB, len(nodeIDs))
	stateDB := make([]*dbm.MemDB, len(nodeIDs))
	blockExecutors := make([]*sm.BlockExecutor, len(nodeIDs))
	blockStores := make([]*store.BlockStore, len(nodeIDs))
	stateStores := make([]sm.Store, len(nodeIDs))

	state, err := sm.MakeGenesisState(genDoc)
	require.NoError(t, err)

	for idx, nodeID := range nodeIDs {
		rts.nodes = append(rts.nodes, nodeID)
		rts.app[nodeID] = proxy.New(abciclient.NewLocalClient(logger, &abci.BaseApplication{}), logger, proxy.NopMetrics())
		require.NoError(t, rts.app[nodeID].Start(ctx))
		stateDB[idx] = dbm.NewMemDB()
		stateStores[idx] = sm.NewStore(stateDB[idx])

		blockDB[idx] = dbm.NewMemDB()
		blockStores[idx] = store.NewBlockStore(blockDB[idx])

		require.NoError(t, stateStores[idx].Save(state))
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
			mock.Anything).Return(nil)

		eventbus := eventbus.NewDefault(logger)
		require.NoError(t, eventbus.Start(ctx))

		blockExecutors[idx] = sm.NewBlockExecutor(stateStores[idx],
			log.NewNopLogger(),
			rts.app[nodeID],
			mp,
			sm.EmptyEvidencePool{},
			blockStores[idx],
			eventbus,
			sm.NopMetrics(),
		)
	}
	var lastExtCommit *types.ExtendedCommit

	// The commit we are building for the current height.
	seenExtCommit := &types.ExtendedCommit{}

	for blockHeight := int64(1); blockHeight <= maxBlockHeightPerNode[maxBlockHeightIdx]; blockHeight++ {
		lastExtCommit = seenExtCommit.Clone()

		// if blockHeight > 1 {
		// lastBlockMeta := blockStores[maxBlockHeightIdx].LoadBlockMeta(blockHeight - 1)
		// lastBlock := blockStores[maxBlockHeightIdx].LoadBlock(blockHeight - 1)
		thisBlock := sf.MakeBlock(state, blockHeight, lastExtCommit.StripExtensions())
		thisParts, err := thisBlock.MakePartSet(types.BlockPartSizeBytes)
		require.NoError(t, err)
		blockID := types.BlockID{Hash: thisBlock.Hash(), PartSetHeader: thisParts.Header()}

		commitSigs := make([]types.ExtendedCommitSig, len(privValArray))
		votes := make([]types.Vote, len(privValArray))
		for i, val := range privValArray {

			vote, err := factory.MakeVote(
				ctx,
				val,
				thisBlock.Header.ChainID,
				0,
				thisBlock.Header.Height,
				0,
				2,
				blockID,
				time.Now(),
			)
			require.NoError(t, err)
			votes[i] = *vote
			commitSigs[i] = vote.ExtendedCommitSig()

		}
		seenExtCommit = &types.ExtendedCommit{
			Height:             votes[0].Height,
			Round:              votes[0].Round,
			BlockID:            votes[0].BlockID,
			ExtendedSignatures: commitSigs,
		}

		// }

		for idx := range nodeIDs {

			if blockHeight <= maxBlockHeightPerNode[idx] {
				lastState, err := stateStores[idx].Load()
				require.NoError(t, err)
				state, err = blockExecutors[idx].ApplyBlock(ctx, lastState, blockID, thisBlock)
				require.NoError(t, err)
				blockStores[idx].SaveBlock(thisBlock, thisParts, seenExtCommit)
			}
		}
	}

	for idx, nodeID := range nodeIDs {
		rts.peerChans[nodeID] = make(chan p2p.PeerUpdate)
		rts.peerUpdates[nodeID] = p2p.NewPeerUpdates(rts.peerChans[nodeID], 1)
		rts.network.Nodes[nodeID].PeerManager.Register(ctx, rts.peerUpdates[nodeID])

		chCreator := func(ctx context.Context, chdesc *p2p.ChannelDescriptor) (*p2p.Channel, error) {
			return rts.blockSyncChannels[nodeID], nil
		}
		rts.reactors[nodeID] = NewReactor(
			rts.logger.With("nodeID", nodeID),
			stateStores[idx],
			blockExecutors[idx],
			blockStores[idx],
			nil,
			chCreator,
			func(ctx context.Context) *p2p.PeerUpdates { return rts.peerUpdates[nodeID] },
			rts.blockSync,
			consensus.NopMetrics(),
			nil, // eventbus, can be nil
		)

		require.NoError(t, rts.reactors[nodeID].Start(ctx))
		require.True(t, rts.reactors[nodeID].IsRunning())
	}
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
		mock.Anything).Return(nil)

	eventbus := eventbus.NewDefault(logger)
	require.NoError(t, eventbus.Start(ctx))

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

	var lastExtCommit *types.ExtendedCommit

	// The commit we are building for the current height.
	seenExtCommit := &types.ExtendedCommit{}

	for blockHeight := int64(1); blockHeight <= maxBlockHeight; blockHeight++ {
		lastExtCommit = seenExtCommit.Clone()

		thisBlock := sf.MakeBlock(state, blockHeight, lastExtCommit.StripExtensions())
		thisParts, err := thisBlock.MakePartSet(types.BlockPartSizeBytes)
		require.NoError(t, err)
		blockID := types.BlockID{Hash: thisBlock.Hash(), PartSetHeader: thisParts.Header()}

		// Simulate a commit for the current height
		vote, err := factory.MakeVote(
			ctx,
			privVal,
			thisBlock.Header.ChainID,
			0,
			thisBlock.Header.Height,
			0,
			2,
			blockID,
			time.Now(),
		)
		require.NoError(t, err)
		seenExtCommit = &types.ExtendedCommit{
			Height:             vote.Height,
			Round:              vote.Round,
			BlockID:            blockID,
			ExtendedSignatures: []types.ExtendedCommitSig{vote.ExtendedCommitSig()},
		}

		state, err = blockExec.ApplyBlock(ctx, state, blockID, thisBlock)
		require.NoError(t, err)

		blockStore.SaveBlock(thisBlock, thisParts, seenExtCommit)
	}

	rts.peerChans[nodeID] = make(chan p2p.PeerUpdate)
	rts.peerUpdates[nodeID] = p2p.NewPeerUpdates(rts.peerChans[nodeID], 1)
	rts.network.Nodes[nodeID].PeerManager.Register(ctx, rts.peerUpdates[nodeID])

	chCreator := func(ctx context.Context, chdesc *p2p.ChannelDescriptor) (*p2p.Channel, error) {
		return rts.blockSyncChannels[nodeID], nil
	}
	rts.reactors[nodeID] = NewReactor(
		rts.logger.With("nodeID", nodeID),
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

	require.NoError(t, rts.reactors[nodeID].Start(ctx))
	require.True(t, rts.reactors[nodeID].IsRunning())
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

	rts := setup(ctx, t, genDoc, privVals, []int64{maxBlockHeight, 0})

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

//@jmalicevic ToDO
// a) Add tests that verify whether faulty peer is properly detected
// 1. block at H + 1 is faulty
// 2. block at H + 2 is faulty (the validator set does not match)
// b) Add test to verify we replace a peer with a new one if we detect misbehavior
func TestReactor_NonGenesisSync(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.ResetTestRoot(t.TempDir(), "block_sync_reactor_test")
	require.NoError(t, err)
	defer os.RemoveAll(cfg.RootDir)

	valSet, privVals := factory.ValidatorSet(ctx, t, 4, 30)
	genDoc := factory.GenesisDoc(cfg, time.Now(), valSet.Validators, factory.ConsensusParams())
	// Has to be 100 + the max starting height because we calculate the sync rate based on every 100 blocks processed
	// Otherwise the test will time out
	maxBlockHeight := int64(151)

	rts := setup(ctx, t, genDoc, privVals, []int64{maxBlockHeight, 2, 50, 0})
	require.Equal(t, maxBlockHeight, rts.reactors[rts.nodes[0]].store.Height())
	rts.start(ctx, t)

	require.Eventually(
		t,
		func() bool {
			matching := true
			for idx := range rts.nodes {
				if idx == 0 {
					continue
				}
				matching = matching && rts.reactors[rts.nodes[idx]].GetRemainingSyncTime() > time.Nanosecond &&
					rts.reactors[rts.nodes[idx]].pool.getLastSyncRate() > 0.0001

				if !matching {
					height, _, _ := rts.reactors[rts.nodes[idx]].pool.GetStatus()
					t.Logf("%d %d %s %f", height, idx, rts.reactors[rts.nodes[idx]].GetRemainingSyncTime(), rts.reactors[rts.nodes[idx]].pool.getLastSyncRate())
				}
			}
			return matching
		},
		30*time.Second,
		10*time.Millisecond,
		"expected node to be partially synced",
	)
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

	rts := setup(ctx, t, genDoc, privVals, []int64{maxBlockHeight, 0})
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

	rts := setup(ctx, t, genDoc, privVals, []int64{maxBlockHeight, 0})

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

	rts := setup(ctx, t, genDoc, privVals, []int64{maxBlockHeight, 0, 0, 0, 0})

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
