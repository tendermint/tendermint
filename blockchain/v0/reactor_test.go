package v0

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/mempool/mock"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/p2ptest"
	bcproto "github.com/tendermint/tendermint/proto/tendermint/blockchain"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

type reactorTestSuite struct {
	network *p2ptest.Network
	logger  log.Logger
	nodes   []p2p.NodeID

	reactors map[p2p.NodeID]*Reactor
	app      map[p2p.NodeID]proxy.AppConns

	blockchainChannels map[p2p.NodeID]*p2p.Channel
	peerChans          map[p2p.NodeID]chan p2p.PeerUpdate
	peerUpdates        map[p2p.NodeID]*p2p.PeerUpdates

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
		nodes:              make([]p2p.NodeID, 0, numNodes),
		reactors:           make(map[p2p.NodeID]*Reactor, numNodes),
		app:                make(map[p2p.NodeID]proxy.AppConns, numNodes),
		blockchainChannels: make(map[p2p.NodeID]*p2p.Channel, numNodes),
		peerChans:          make(map[p2p.NodeID]chan p2p.PeerUpdate, numNodes),
		peerUpdates:        make(map[p2p.NodeID]*p2p.PeerUpdates, numNodes),
		fastSync:           true,
	}

	rts.blockchainChannels = rts.network.MakeChannelsNoCleanup(t, BlockchainChannel, new(bcproto.Message), int(chBuf))

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
	nodeID p2p.NodeID,
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

	state, err := stateStore.LoadFromDBOrGenesisDoc(genDoc)
	require.NoError(t, err)

	db := dbm.NewMemDB()
	stateStore = sm.NewStore(db)

	blockExec := sm.NewBlockExecutor(
		stateStore,
		log.TestingLogger(),
		rts.app[nodeID].Consensus(),
		mock.Mempool{},
		sm.EmptyEvidencePool{},
	)
	require.NoError(t, stateStore.Save(state))

	for blockHeight := int64(1); blockHeight <= maxBlockHeight; blockHeight++ {
		lastCommit := types.NewCommit(blockHeight-1, 0, types.BlockID{}, nil)

		if blockHeight > 1 {
			lastBlockMeta := blockStore.LoadBlockMeta(blockHeight - 1)
			lastBlock := blockStore.LoadBlock(blockHeight - 1)

			vote, err := types.MakeVote(
				lastBlock.Header.Height,
				lastBlockMeta.BlockID,
				state.Validators,
				privVal,
				lastBlock.Header.ChainID,
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

		thisBlock := makeBlock(blockHeight, state, lastCommit)
		thisParts := thisBlock.MakePartSet(types.BlockPartSizeBytes)
		blockID := types.BlockID{Hash: thisBlock.Hash(), PartSetHeader: thisParts.Header()}

		state, _, err = blockExec.ApplyBlock(state, blockID, thisBlock)
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
		rts.fastSync)
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

	genDoc, privVals := randGenesisDoc(config, 1, false, 30)
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
	require.NoError(t, rts.network.Nodes[rts.nodes[1]].PeerManager.Disconnected(rts.nodes[0]))
}

func TestReactor_NoBlockResponse(t *testing.T) {
	config := cfg.ResetTestRoot("blockchain_reactor_test")
	defer os.RemoveAll(config.RootDir)

	genDoc, privVals := randGenesisDoc(config, 1, false, 30)
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
	genDoc, privVals := randGenesisDoc(config, 1, false, 30)

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
	otherGenDoc, otherPrivVals := randGenesisDoc(config, 1, false, 30)
	newNode := rts.network.MakeNode(t)
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
