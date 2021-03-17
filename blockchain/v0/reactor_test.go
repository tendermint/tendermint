package v0

import (
	"math/rand"
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

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

type reactorTestSuite struct {
	network *p2ptest.Network
	logger  log.Logger
	nodes   []p2p.NodeID

	reactors map[p2p.NodeID]*Reactor
	app      map[p2p.NodeID]proxy.AppConns

	blockchainChannels map[p2p.NodeID]*p2p.Channel
	peerChans          map[p2p.NodeID]chan p2p.PeerUpdate
	peerUpdates        map[p2p.NodeID]*p2p.PeerUpdates
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
		network:            p2ptest.MakeNetwork(t, numNodes),
		nodes:              make([]p2p.NodeID, 0, numNodes),
		reactors:           make(map[p2p.NodeID]*Reactor, numNodes),
		app:                make(map[p2p.NodeID]proxy.AppConns, numNodes),
		blockchainChannels: make(map[p2p.NodeID]*p2p.Channel, numNodes),
		peerChans:          make(map[p2p.NodeID]chan p2p.PeerUpdate, numNodes),
		peerUpdates:        make(map[p2p.NodeID]*p2p.PeerUpdates, numNodes),
	}

	rts.network.MakeChannelsNoCleanup(t, BlockchainChannel, new(bcproto.Message), int(chBuf))

	const fastSync = true

	i := 0
	for nodeID := range rts.network.Nodes {
		rts.nodes = append(rts.nodes, nodeID)
		app := proxy.NewAppConns(proxy.NewLocalClientCreator(&abci.BaseApplication{}))
		require.NoError(t, app.Start())

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
			app.Consensus(),
			mock.Mempool{},
			sm.EmptyEvidencePool{},
		)
		require.NoError(t, stateStore.Save(state))

		for blockHeight := int64(1); blockHeight <= maxBlockHeights[i]; blockHeight++ {
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
		rts.peerUpdates[nodeID] = p2p.NewPeerUpdates(rts.peerChans[nodeID])
		rts.network.Nodes[nodeID].PeerManager.Register(rts.peerUpdates[nodeID])
		rts.reactors[nodeID], err = NewReactor(
			rts.logger.With("nodeID", nodeID),
			state.Copy(),
			blockExec,
			blockStore,
			nil,
			rts.blockchainChannels[nodeID],
			rts.peerUpdates[nodeID],
			fastSync)
		require.NoError(t, err)

		require.NoError(t, rts.reactors[nodeID].Start())
		require.True(t, rts.reactors[nodeID].IsRunning())

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
	testSuites := []*reactorTestSuite{
		setup(t, genDoc, privVals, maxBlockHeight, 0),
		setup(t, genDoc, privVals, 0, 0),
	}

	require.Equal(t, maxBlockHeight, testSuites[0].reactor.store.Height())

	for _, s := range testSuites {
		simulateRouter(s, testSuites, true)

		// connect reactor to every other reactor
		for _, ss := range testSuites {
			if s.peerID != ss.peerID {
				s.peerUpdatesCh <- p2p.PeerUpdate{
					Status: p2p.PeerStatusUp,
					NodeID: ss.peerID,
				}
			}
		}
	}

	secondaryPool := testSuites[1].reactor.pool
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
	testSuites[1].peerUpdatesCh <- p2p.PeerUpdate{
		Status: p2p.PeerStatusDown,
		NodeID: testSuites[0].peerID,
	}
}

func TestReactor_NoBlockResponse(t *testing.T) {
	config := cfg.ResetTestRoot("blockchain_reactor_test")
	defer os.RemoveAll(config.RootDir)

	genDoc, privVals := randGenesisDoc(config, 1, false, 30)
	maxBlockHeight := int64(65)
	testSuites := []*reactorTestSuite{
		setup(t, genDoc, privVals, maxBlockHeight, 0),
		setup(t, genDoc, privVals, 0, 0),
	}

	require.Equal(t, maxBlockHeight, testSuites[0].reactor.store.Height())

	for _, s := range testSuites {
		simulateRouter(s, testSuites, true)

		// connect reactor to every other reactor
		for _, ss := range testSuites {
			if s.peerID != ss.peerID {
				s.peerUpdatesCh <- p2p.PeerUpdate{
					Status: p2p.PeerStatusUp,
					NodeID: ss.peerID,
				}
			}
		}
	}

	testCases := []struct {
		height   int64
		existent bool
	}{
		{maxBlockHeight + 2, false},
		{10, true},
		{1, true},
		{100, false},
	}

	secondaryPool := testSuites[1].reactor.pool
	require.Eventually(
		t,
		func() bool { return secondaryPool.MaxPeerHeight() > 0 && secondaryPool.IsCaughtUp() },
		10*time.Second,
		10*time.Millisecond,
		"expected node to be fully synced",
	)

	for _, tc := range testCases {
		block := testSuites[1].reactor.store.LoadBlock(tc.height)
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

	testSuites := []*reactorTestSuite{
		setup(t, genDoc, privVals, maxBlockHeight, 1000), // fully synced node
		setup(t, genDoc, privVals, 0, 1000),
		setup(t, genDoc, privVals, 0, 1000),
		setup(t, genDoc, privVals, 0, 1000),
		setup(t, genDoc, privVals, 0, 1000), // new node
	}

	require.Equal(t, maxBlockHeight, testSuites[0].reactor.store.Height())

	for _, s := range testSuites[:len(testSuites)-1] {
		simulateRouter(s, testSuites, true)

		// connect reactor to every other reactor except the new node
		for _, ss := range testSuites[:len(testSuites)-1] {
			if s.peerID != ss.peerID {
				s.peerUpdatesCh <- p2p.PeerUpdate{
					Status: p2p.PeerStatusUp,
					NodeID: ss.peerID,
				}
			}
		}
	}

	require.Eventually(
		t,
		func() bool {
			caughtUp := true
			for _, s := range testSuites[1 : len(testSuites)-1] {
				if s.reactor.pool.MaxPeerHeight() == 0 || !s.reactor.pool.IsCaughtUp() {
					caughtUp = false
				}
			}

			return caughtUp
		},
		10*time.Minute,
		10*time.Millisecond,
		"expected all nodes to be fully synced",
	)

	for _, s := range testSuites[:len(testSuites)-1] {
		require.Len(t, s.reactor.pool.peers, 3)
	}

	// Mark testSuites[3] as an invalid peer which will cause newSuite to disconnect
	// from this peer.
	//
	// XXX: This causes a potential race condition.
	// See: https://github.com/tendermint/tendermint/issues/6005
	otherGenDoc, otherPrivVals := randGenesisDoc(config, 1, false, 30)
	otherSuite := setup(t, otherGenDoc, otherPrivVals, maxBlockHeight, 0)
	testSuites[3].reactor.store = otherSuite.reactor.store

	// add a fake peer just so we do not wait for the consensus ticker to timeout
	otherSuite.reactor.pool.SetPeerRange("00ff", 10, 10)

	// start the new peer's faux router
	newSuite := testSuites[len(testSuites)-1]
	simulateRouter(newSuite, testSuites, false)

	// connect all nodes to the new peer
	for _, s := range testSuites[:len(testSuites)-1] {
		newSuite.peerUpdatesCh <- p2p.PeerUpdate{
			Status: p2p.PeerStatusUp,
			NodeID: s.peerID,
		}
	}

	// wait for the new peer to catch up and become fully synced
	require.Eventually(
		t,
		func() bool { return newSuite.reactor.pool.MaxPeerHeight() > 0 && newSuite.reactor.pool.IsCaughtUp() },
		10*time.Minute,
		10*time.Millisecond,
		"expected new node to be fully synced",
	)

	require.Eventuallyf(
		t,
		func() bool { return len(newSuite.reactor.pool.peers) < len(testSuites)-1 },
		10*time.Minute,
		10*time.Millisecond,
		"invalid number of peers; expected < %d, got: %d",
		len(testSuites)-1,
		len(newSuite.reactor.pool.peers),
	)
}
