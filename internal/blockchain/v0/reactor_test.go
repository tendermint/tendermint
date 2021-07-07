package v0

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	cfg "github.com/tendermint/tendermint/config"
	cons "github.com/tendermint/tendermint/internal/consensus"
	"github.com/tendermint/tendermint/internal/mempool/mock"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/internal/p2p/p2ptest"
	"github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/libs/log"
	bcproto "github.com/tendermint/tendermint/proto/tendermint/blockchain"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	sf "github.com/tendermint/tendermint/state/test/factory"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

type reactorTestSuite struct {
	network *p2ptest.Network
	logger  log.Logger
	nodes   []types.NodeID

	reactors map[types.NodeID]*Reactor
	app      map[types.NodeID]proxy.AppConns

	blockchainChannels map[types.NodeID]*p2p.Channel
	peerChans          map[types.NodeID]chan p2p.PeerUpdate
	peerUpdates        map[types.NodeID]*p2p.PeerUpdates

	fastSync bool
}

func setup(
	t *testing.T,
	genDoc *types.GenesisDoc,
	privVal types.PrivValidator,
	maxBlockHeights []int64,
	chBuf uint,
) *reactorTestSuite {
	t.Helper()

	numNodes := len(maxBlockHeights)
	require.True(t, numNodes >= 1,
		"must specify at least one block height (nodes)")

	rts := &reactorTestSuite{
		logger:             log.TestingLogger().With("module", "blockchain", "testCase", t.Name()),
		network:            p2ptest.MakeNetwork(t, p2ptest.NetworkOptions{NumNodes: numNodes}),
		nodes:              make([]types.NodeID, 0, numNodes),
		reactors:           make(map[types.NodeID]*Reactor, numNodes),
		app:                make(map[types.NodeID]proxy.AppConns, numNodes),
		blockchainChannels: make(map[types.NodeID]*p2p.Channel, numNodes),
		peerChans:          make(map[types.NodeID]chan p2p.PeerUpdate, numNodes),
		peerUpdates:        make(map[types.NodeID]*p2p.PeerUpdates, numNodes),
		fastSync:           true,
	}

	chDesc := p2p.ChannelDescriptor{ID: byte(BlockchainChannel)}
	rts.blockchainChannels = rts.network.MakeChannelsNoCleanup(t, chDesc, new(bcproto.Message), int(chBuf))

	i := 0
	for nodeID := range rts.network.Nodes {
		rts.addNode(t, nodeID, genDoc, privVal, maxBlockHeights[i])
		i++
	}

	t.Cleanup(func() {
		for _, nodeID := range rts.nodes {
			rts.peerUpdates[nodeID].Close()

			if rts.reactors[nodeID].IsRunning() {
				require.NoError(t, rts.reactors[nodeID].Stop())
				require.NoError(t, rts.app[nodeID].Stop())
				require.False(t, rts.reactors[nodeID].IsRunning())
			}
		}
	})

	return rts
}

func (rts *reactorTestSuite) addNode(t *testing.T,
	nodeID types.NodeID,
	genDoc *types.GenesisDoc,
	privVal types.PrivValidator,
	maxBlockHeight int64,
) {
	t.Helper()

	rts.nodes = append(rts.nodes, nodeID)
	rts.app[nodeID] = proxy.NewAppConns(proxy.NewLocalClientCreator(&abci.BaseApplication{}))
	require.NoError(t, rts.app[nodeID].Start())

	blockDB := dbm.NewMemDB()
	stateDB := dbm.NewMemDB()
	stateStore := sm.NewStore(stateDB)
	blockStore := store.NewBlockStore(blockDB)

	state, err := sm.MakeGenesisState(genDoc)
	require.NoError(t, err)
	require.NoError(t, stateStore.Save(state))

	blockExec := sm.NewBlockExecutor(
		stateStore,
		log.TestingLogger(),
		rts.app[nodeID].Consensus(),
		mock.Mempool{},
		sm.EmptyEvidencePool{},
		blockStore,
	)

	for blockHeight := int64(1); blockHeight <= maxBlockHeight; blockHeight++ {
		lastCommit := types.NewCommit(blockHeight-1, 0, types.BlockID{}, nil)

		if blockHeight > 1 {
			lastBlockMeta := blockStore.LoadBlockMeta(blockHeight - 1)
			lastBlock := blockStore.LoadBlock(blockHeight - 1)

			vote, err := factory.MakeVote(
				privVal,
				lastBlock.Header.ChainID, 0,
				lastBlock.Header.Height, 0, 2,
				lastBlockMeta.BlockID,
				time.Now(),
			)
			require.NoError(t, err)

			lastCommit = types.NewCommit(
				vote.Height,
				vote.Round,
				lastBlockMeta.BlockID,
				[]types.CommitSig{vote.CommitSig()},
			)
		}

		thisBlock := sf.MakeBlock(state, blockHeight, lastCommit)
		thisParts := thisBlock.MakePartSet(types.BlockPartSizeBytes)
		blockID := types.BlockID{Hash: thisBlock.Hash(), PartSetHeader: thisParts.Header()}

		state, err = blockExec.ApplyBlock(state, blockID, thisBlock)
		require.NoError(t, err)

		blockStore.SaveBlock(thisBlock, thisParts, lastCommit)
	}

	rts.peerChans[nodeID] = make(chan p2p.PeerUpdate)
	rts.peerUpdates[nodeID] = p2p.NewPeerUpdates(rts.peerChans[nodeID], 1)
	rts.network.Nodes[nodeID].PeerManager.Register(rts.peerUpdates[nodeID])
	rts.reactors[nodeID], err = NewReactor(
		rts.logger.With("nodeID", nodeID),
		state.Copy(),
		blockExec,
		blockStore,
		nil,
		rts.blockchainChannels[nodeID],
		rts.peerUpdates[nodeID],
		rts.fastSync,
		cons.NopMetrics())
	require.NoError(t, err)

	require.NoError(t, rts.reactors[nodeID].Start())
	require.True(t, rts.reactors[nodeID].IsRunning())
}

func (rts *reactorTestSuite) start(t *testing.T) {
	t.Helper()
	rts.network.Start(t)
	require.Len(t,
		rts.network.RandomNode().PeerManager.Peers(),
		len(rts.nodes)-1,
		"network does not have expected number of nodes")
}

func TestReactor_AbruptDisconnect(t *testing.T) {
	config := cfg.ResetTestRoot("blockchain_reactor_test")
	defer os.RemoveAll(config.RootDir)

	genDoc, privVals := factory.RandGenesisDoc(config, 1, false, 30)
	maxBlockHeight := int64(64)

	rts := setup(t, genDoc, privVals[0], []int64{maxBlockHeight, 0}, 0)

	require.Equal(t, maxBlockHeight, rts.reactors[rts.nodes[0]].store.Height())

	rts.start(t)

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
	rts.network.Nodes[rts.nodes[1]].PeerManager.Disconnected(rts.nodes[0])
}

func TestReactor_SyncTime(t *testing.T) {
	config := cfg.ResetTestRoot("blockchain_reactor_test")
	defer os.RemoveAll(config.RootDir)

	genDoc, privVals := factory.RandGenesisDoc(config, 1, false, 30)
	maxBlockHeight := int64(101)

	rts := setup(t, genDoc, privVals[0], []int64{maxBlockHeight, 0}, 0)
	require.Equal(t, maxBlockHeight, rts.reactors[rts.nodes[0]].store.Height())
	rts.start(t)

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
	config := cfg.ResetTestRoot("blockchain_reactor_test")
	defer os.RemoveAll(config.RootDir)

	genDoc, privVals := factory.RandGenesisDoc(config, 1, false, 30)
	maxBlockHeight := int64(65)

	rts := setup(t, genDoc, privVals[0], []int64{maxBlockHeight, 0}, 0)

	require.Equal(t, maxBlockHeight, rts.reactors[rts.nodes[0]].store.Height())

	rts.start(t)

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

	config := cfg.ResetTestRoot("blockchain_reactor_test")
	defer os.RemoveAll(config.RootDir)

	maxBlockHeight := int64(48)
	genDoc, privVals := factory.RandGenesisDoc(config, 1, false, 30)

	rts := setup(t, genDoc, privVals[0], []int64{maxBlockHeight, 0, 0, 0, 0}, 1000)

	require.Equal(t, maxBlockHeight, rts.reactors[rts.nodes[0]].store.Height())

	rts.start(t)

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
	otherGenDoc, otherPrivVals := factory.RandGenesisDoc(config, 1, false, 30)
	newNode := rts.network.MakeNode(t, p2ptest.NodeOptions{
		MaxPeers:     uint16(len(rts.nodes) + 1),
		MaxConnected: uint16(len(rts.nodes) + 1),
	})
	rts.addNode(t, newNode.NodeID, otherGenDoc, otherPrivVals[0], maxBlockHeight)

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
