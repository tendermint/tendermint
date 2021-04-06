package mempool

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/p2ptest"
	protomem "github.com/tendermint/tendermint/proto/tendermint/mempool"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
)

type reactorTestSuite struct {
	network *p2ptest.Network
	logger  log.Logger

	reactors       map[p2p.NodeID]*Reactor
	mempoolChnnels map[p2p.NodeID]*p2p.Channel
	mempools       map[p2p.NodeID]*CListMempool
	kvstores       map[p2p.NodeID]*kvstore.Application

	peerChans   map[p2p.NodeID]chan p2p.PeerUpdate
	peerUpdates map[p2p.NodeID]*p2p.PeerUpdates

	nodes []p2p.NodeID
}

func setup(t *testing.T, cfg *cfg.MempoolConfig, numNodes int, chBuf uint) *reactorTestSuite {
	t.Helper()

	rts := &reactorTestSuite{
		logger:         log.TestingLogger().With("testCase", t.Name()),
		network:        p2ptest.MakeNetwork(t, p2ptest.NetworkOptions{NumNodes: numNodes}),
		reactors:       make(map[p2p.NodeID]*Reactor, numNodes),
		mempoolChnnels: make(map[p2p.NodeID]*p2p.Channel, numNodes),
		mempools:       make(map[p2p.NodeID]*CListMempool, numNodes),
		kvstores:       make(map[p2p.NodeID]*kvstore.Application, numNodes),
		peerChans:      make(map[p2p.NodeID]chan p2p.PeerUpdate, numNodes),
		peerUpdates:    make(map[p2p.NodeID]*p2p.PeerUpdates, numNodes),
	}

	rts.mempoolChnnels = rts.network.MakeChannelsNoCleanup(t, MempoolChannel, new(protomem.Message), int(chBuf))

	for nodeID := range rts.network.Nodes {
		rts.kvstores[nodeID] = kvstore.NewApplication()
		cc := proxy.NewLocalClientCreator(rts.kvstores[nodeID])

		mempool, memCleanup := newMempoolWithApp(cc)
		t.Cleanup(memCleanup)
		mempool.SetLogger(rts.logger)
		rts.mempools[nodeID] = mempool

		rts.peerChans[nodeID] = make(chan p2p.PeerUpdate)
		rts.peerUpdates[nodeID] = p2p.NewPeerUpdates(rts.peerChans[nodeID], 1)
		rts.network.Nodes[nodeID].PeerManager.Register(rts.peerUpdates[nodeID])

		rts.reactors[nodeID] = NewReactor(
			rts.logger.With("nodeID", nodeID),
			cfg,
			rts.network.Nodes[nodeID].PeerManager,
			mempool,
			rts.mempoolChnnels[nodeID],
			rts.peerUpdates[nodeID],
		)

		rts.nodes = append(rts.nodes, nodeID)

		require.NoError(t, rts.reactors[nodeID].Start())
		require.True(t, rts.reactors[nodeID].IsRunning())
	}

	require.Len(t, rts.reactors, numNodes)

	t.Cleanup(func() {
		for nodeID := range rts.reactors {
			if rts.reactors[nodeID].IsRunning() {
				require.NoError(t, rts.reactors[nodeID].Stop())
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

func (rts *reactorTestSuite) assertMempoolChannelsDrained(t *testing.T) {
	t.Helper()

	for id, r := range rts.reactors {
		require.NoError(t, r.Stop(), "stopping reactor %s", id)
		r.Wait()
		require.False(t, r.IsRunning(), "reactor %s did not stop", id)
	}

	for _, mch := range rts.mempoolChnnels {
		require.Empty(t, mch.Out, "checking channel %q (len=%d)", mch.ID, len(mch.Out))
	}
}

func (rts *reactorTestSuite) waitForTxns(t *testing.T, txs types.Txs, ids ...p2p.NodeID) {
	t.Helper()

	fn := func(pool *CListMempool) {
		for pool.Size() < len(txs) {
			time.Sleep(50 * time.Millisecond)
		}

		reapedTxs := pool.ReapMaxTxs(len(txs))
		require.Equal(t, len(txs), len(reapedTxs))
		for i, tx := range txs {
			require.Equalf(t,
				tx,
				reapedTxs[i],
				"txs at index %d in reactor mempool mismatch; got: %v, expected: %v", i, tx, reapedTxs[i],
			)
		}
	}

	if len(ids) == 1 {
		fn(rts.reactors[ids[0]].mempool)
		return
	}

	wg := &sync.WaitGroup{}
	for id := range rts.mempools {
		if len(ids) > 0 && !p2ptest.NodeInSlice(id, ids) {
			continue
		}

		wg.Add(1)
		func(nid p2p.NodeID) { defer wg.Done(); fn(rts.reactors[nid].mempool) }(id)
	}

	wg.Wait()
}

func TestReactorBroadcastTxs(t *testing.T) {
	numTxs := 1000
	numNodes := 10
	config := cfg.TestConfig()

	rts := setup(t, config.Mempool, numNodes, 0)

	primary := rts.nodes[0]
	secondaries := rts.nodes[1:]

	txs := checkTxs(t, rts.reactors[primary].mempool, numTxs, UnknownPeerID)

	// run the router
	rts.start(t)

	// Wait till all secondary suites (reactor) received all mempool txs from the
	// primary suite (node).
	rts.waitForTxns(t, txs, secondaries...)

	for _, pool := range rts.mempools {
		require.Equal(t, len(txs), pool.Size())
	}

	rts.assertMempoolChannelsDrained(t)
}

// regression test for https://github.com/tendermint/tendermint/issues/5408
func TestReactorConcurrency(t *testing.T) {
	numTxs := 5
	numNodes := 2
	config := cfg.TestConfig()

	rts := setup(t, config.Mempool, numNodes, 0)

	primary := rts.nodes[0]
	secondary := rts.nodes[1]

	rts.start(t)

	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(2)

		// 1. submit a bunch of txs
		// 2. update the whole mempool

		txs := checkTxs(t, rts.reactors[primary].mempool, numTxs, UnknownPeerID)
		go func() {
			defer wg.Done()

			mempool := rts.mempools[primary]

			mempool.Lock()
			defer mempool.Unlock()

			deliverTxResponses := make([]*abci.ResponseDeliverTx, len(txs))
			for i := range txs {
				deliverTxResponses[i] = &abci.ResponseDeliverTx{Code: 0}
			}

			require.NoError(t, mempool.Update(1, txs, deliverTxResponses, nil, nil))
		}()

		// 1. submit a bunch of txs
		// 2. update none
		_ = checkTxs(t, rts.reactors[secondary].mempool, numTxs, UnknownPeerID)
		go func() {
			defer wg.Done()

			mempool := rts.mempools[secondary]

			mempool.Lock()
			defer mempool.Unlock()

			err := mempool.Update(1, []types.Tx{}, make([]*abci.ResponseDeliverTx, 0), nil, nil)
			require.NoError(t, err)
		}()

		// flush the mempool
		rts.mempools[secondary].Flush()
	}

	wg.Wait()
}

func TestReactorNoBroadcastToSender(t *testing.T) {
	numTxs := 1000
	numNodes := 2
	config := cfg.TestConfig()

	rts := setup(t, config.Mempool, numNodes, uint(numTxs))

	primary := rts.nodes[0]
	secondary := rts.nodes[1]

	peerID := uint16(1)
	_ = checkTxs(t, rts.mempools[primary], numTxs, peerID)

	rts.start(t)

	time.Sleep(100 * time.Millisecond)

	require.Eventually(t, func() bool {
		return rts.mempools[secondary].Size() == 0
	}, time.Minute, 100*time.Millisecond)

	rts.assertMempoolChannelsDrained(t)
}

func TestMempoolIDsBasic(t *testing.T) {
	ids := newMempoolIDs()

	peerID, err := p2p.NewNodeID("0011223344556677889900112233445566778899")
	require.NoError(t, err)

	ids.ReserveForPeer(peerID)
	require.EqualValues(t, 1, ids.GetForPeer(peerID))
	ids.Reclaim(peerID)

	ids.ReserveForPeer(peerID)
	require.EqualValues(t, 2, ids.GetForPeer(peerID))
	ids.Reclaim(peerID)
}

func TestReactor_MaxTxBytes(t *testing.T) {
	numNodes := 2
	config := cfg.TestConfig()

	rts := setup(t, config.Mempool, numNodes, 0)

	primary := rts.nodes[0]
	secondary := rts.nodes[1]

	// Broadcast a tx, which has the max size and ensure it's received by the
	// second reactor.
	tx1 := tmrand.Bytes(config.Mempool.MaxTxBytes)
	err := rts.reactors[primary].mempool.CheckTx(tx1, nil, TxInfo{SenderID: UnknownPeerID})
	require.NoError(t, err)

	rts.start(t)

	// Wait till all secondary suites (reactor) received all mempool txs from the
	// primary suite (node).
	rts.waitForTxns(t, []types.Tx{tx1}, secondary)

	rts.reactors[primary].mempool.Flush()
	rts.reactors[secondary].mempool.Flush()

	// broadcast a tx, which is beyond the max size and ensure it's not sent
	tx2 := tmrand.Bytes(config.Mempool.MaxTxBytes + 1)
	err = rts.mempools[primary].CheckTx(tx2, nil, TxInfo{SenderID: UnknownPeerID})
	require.Error(t, err)

	rts.assertMempoolChannelsDrained(t)
}

func TestDontExhaustMaxActiveIDs(t *testing.T) {
	config := cfg.TestConfig()

	// we're creating a single node network, but not starting the
	// network.
	rts := setup(t, config.Mempool, 1, maxActiveIDs+1)

	nodeID := rts.nodes[0]

	peerID, err := p2p.NewNodeID("0011223344556677889900112233445566778899")
	require.NoError(t, err)

	// ensure the reactor does not panic (i.e. exhaust active IDs)
	for i := 0; i < maxActiveIDs+1; i++ {
		rts.peerChans[nodeID] <- p2p.PeerUpdate{
			Status: p2p.PeerStatusUp,
			NodeID: peerID,
		}

		rts.mempoolChnnels[nodeID].Out <- p2p.Envelope{
			To: peerID,
			Message: &protomem.Txs{
				Txs: [][]byte{},
			},
		}
	}

	require.Eventually(
		t,
		func() bool {
			for _, mch := range rts.mempoolChnnels {
				if len(mch.Out) > 0 {
					return false
				}
			}

			return true
		},
		time.Minute,
		10*time.Millisecond,
	)

	rts.assertMempoolChannelsDrained(t)
}

func TestMempoolIDsPanicsIfNodeRequestsOvermaxActiveIDs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	// 0 is already reserved for UnknownPeerID
	ids := newMempoolIDs()

	peerID, err := p2p.NewNodeID("0011223344556677889900112233445566778899")
	require.NoError(t, err)

	for i := 0; i < maxActiveIDs-1; i++ {
		ids.ReserveForPeer(peerID)
	}

	require.Panics(t, func() {
		ids.ReserveForPeer(peerID)
	})
}

func TestBroadcastTxForPeerStopsWhenPeerStops(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	config := cfg.TestConfig()

	rts := setup(t, config.Mempool, 2, 0)

	primary := rts.nodes[0]
	secondary := rts.nodes[1]

	rts.start(t)

	// disconnect peer
	rts.peerChans[primary] <- p2p.PeerUpdate{
		Status: p2p.PeerStatusDown,
		NodeID: secondary,
	}
}
