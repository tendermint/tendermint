package v1

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	tmsync "github.com/tendermint/tendermint/internal/libs/sync"
	"github.com/tendermint/tendermint/internal/mempool"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/internal/p2p/p2ptest"
	"github.com/tendermint/tendermint/libs/log"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	protomem "github.com/tendermint/tendermint/proto/tendermint/mempool"
	"github.com/tendermint/tendermint/types"
)

type reactorTestSuite struct {
	network *p2ptest.Network
	logger  log.Logger

	reactors        map[types.NodeID]*Reactor
	mempoolChannels map[types.NodeID]*p2p.Channel
	mempools        map[types.NodeID]*TxMempool
	kvstores        map[types.NodeID]*kvstore.Application

	peerChans   map[types.NodeID]chan p2p.PeerUpdate
	peerUpdates map[types.NodeID]*p2p.PeerUpdates

	nodes []types.NodeID
}

func setupReactors(t *testing.T, numNodes int, chBuf uint) *reactorTestSuite {
	t.Helper()

	cfg := config.ResetTestRoot(strings.ReplaceAll(t.Name(), "/", "|"))
	t.Cleanup(func() {
		os.RemoveAll(cfg.RootDir)
	})

	rts := &reactorTestSuite{
		logger:          log.TestingLogger().With("testCase", t.Name()),
		network:         p2ptest.MakeNetwork(t, p2ptest.NetworkOptions{NumNodes: numNodes}),
		reactors:        make(map[types.NodeID]*Reactor, numNodes),
		mempoolChannels: make(map[types.NodeID]*p2p.Channel, numNodes),
		mempools:        make(map[types.NodeID]*TxMempool, numNodes),
		kvstores:        make(map[types.NodeID]*kvstore.Application, numNodes),
		peerChans:       make(map[types.NodeID]chan p2p.PeerUpdate, numNodes),
		peerUpdates:     make(map[types.NodeID]*p2p.PeerUpdates, numNodes),
	}

	chDesc := GetChannelDescriptor(cfg.Mempool)
	rts.mempoolChannels = rts.network.MakeChannelsNoCleanup(t, chDesc)

	for nodeID := range rts.network.Nodes {
		rts.kvstores[nodeID] = kvstore.NewApplication()

		mempool := setup(t, 0)
		rts.mempools[nodeID] = mempool

		rts.peerChans[nodeID] = make(chan p2p.PeerUpdate)
		rts.peerUpdates[nodeID] = p2p.NewPeerUpdates(rts.peerChans[nodeID], 1)
		rts.network.Nodes[nodeID].PeerManager.Register(rts.peerUpdates[nodeID])

		rts.reactors[nodeID] = NewReactor(
			rts.logger.With("nodeID", nodeID),
			cfg.Mempool,
			rts.network.Nodes[nodeID].PeerManager,
			mempool,
			rts.mempoolChannels[nodeID],
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

// NOTE: these are noops just as shims for hte legacy tests. we might
// want to implement them.
func (rts *reactorTestSuite) assertMempoolChannelsDrained(t *testing.T) {
	t.Helper()

	for id, r := range rts.reactors {
		require.NoError(t, r.Stop(), "stopping reactor %s", id)
		r.Wait()
		require.False(t, r.IsRunning(), "reactor %s did not stop", id)
	}

	for _, mch := range rts.mempoolChannels {
		require.Empty(t, mch.Out, "checking channel %q (len=%d)", mch.ID, len(mch.Out))
	}
}

func (rts *reactorTestSuite) waitForTxns(t *testing.T, txs []types.Tx, ids ...types.NodeID) {
	t.Helper()

	// NOTE: in the legacy version of these tests, there was more
	// complex logic that waited for the mempool to drain. The
	// tests pass without that (modulo adding a simple one second
	// sleep, in a test. This is likely because trasactions are
	// less likely to "hang out" in a buffer in the new reactor,
	// which is what this prevented in the legacy tests.
}

func TestReactorBroadcastDoesNotPanic(t *testing.T) {
	numNodes := 2
	rts := setupReactors(t, numNodes, 0)

	observePanic := func(r interface{}) {
		t.Fatal("panic detected in reactor")
	}

	primary := rts.nodes[0]
	secondary := rts.nodes[1]
	primaryReactor := rts.reactors[primary]
	primaryMempool := primaryReactor.mempool
	secondaryReactor := rts.reactors[secondary]

	primaryReactor.observePanic = observePanic
	secondaryReactor.observePanic = observePanic

	firstTx := &WrappedTx{}
	primaryMempool.insertTx(firstTx)

	// run the router
	rts.start(t)

	closer := tmsync.NewCloser()
	primaryReactor.peerWG.Add(1)
	go primaryReactor.broadcastTxRoutine(secondary, closer)

	wg := &sync.WaitGroup{}
	for i := 0; i < 50; i++ {
		next := &WrappedTx{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			primaryMempool.insertTx(next)
		}()
	}

	err := primaryReactor.Stop()
	require.NoError(t, err)
	primaryReactor.peerWG.Wait()
	wg.Wait()
}

func TestReactorBroadcastTxs(t *testing.T) {
	numTxs := 1000
	numNodes := 10

	rts := setupReactors(t, numNodes, 0)

	primary := rts.nodes[0]
	secondaries := rts.nodes[1:]

	txs := checkTxs(t, rts.reactors[primary].mempool, numTxs, mempool.UnknownPeerID)

	// run the router
	rts.start(t)

	// Wait till all secondary suites (reactor) received all mempool txs from the
	// primary suite (node).
	rts.waitForTxns(t, convertTex(txs), secondaries...)

	time.Sleep(time.Second)

	for _, pool := range rts.mempools {
		require.Equal(t, len(txs), pool.Size())
	}

	rts.assertMempoolChannelsDrained(t)
}

// regression test for https://github.com/tendermint/tendermint/issues/5408
func TestReactorConcurrency(t *testing.T) {
	numTxs := 5
	numNodes := 2

	rts := setupReactors(t, numNodes, 0)

	primary := rts.nodes[0]
	secondary := rts.nodes[1]

	rts.start(t)

	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(2)

		// 1. submit a bunch of txs
		// 2. update the whole mempool

		txs := checkTxs(t, rts.reactors[primary].mempool, numTxs, mempool.UnknownPeerID)
		go func() {
			defer wg.Done()

			mempool := rts.mempools[primary]

			mempool.Lock()
			defer mempool.Unlock()

			deliverTxResponses := make([]*abci.ResponseDeliverTx, len(txs))
			for i := range txs {
				deliverTxResponses[i] = &abci.ResponseDeliverTx{Code: 0}
			}

			require.NoError(t, mempool.Update(1, convertTex(txs), deliverTxResponses, nil, nil))
		}()

		// 1. submit a bunch of txs
		// 2. update none
		_ = checkTxs(t, rts.reactors[secondary].mempool, numTxs, mempool.UnknownPeerID)
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

	rts := setupReactors(t, numNodes, uint(numTxs))

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

func TestReactor_MaxTxBytes(t *testing.T) {
	numNodes := 2
	cfg := config.TestConfig()

	rts := setupReactors(t, numNodes, 0)

	primary := rts.nodes[0]
	secondary := rts.nodes[1]

	// Broadcast a tx, which has the max size and ensure it's received by the
	// second reactor.
	tx1 := tmrand.Bytes(cfg.Mempool.MaxTxBytes)
	err := rts.reactors[primary].mempool.CheckTx(
		context.Background(),
		tx1,
		nil,
		mempool.TxInfo{
			SenderID: mempool.UnknownPeerID,
		},
	)
	require.NoError(t, err)

	rts.start(t)

	// Wait till all secondary suites (reactor) received all mempool txs from the
	// primary suite (node).
	rts.waitForTxns(t, []types.Tx{tx1}, secondary)

	rts.reactors[primary].mempool.Flush()
	rts.reactors[secondary].mempool.Flush()

	// broadcast a tx, which is beyond the max size and ensure it's not sent
	tx2 := tmrand.Bytes(cfg.Mempool.MaxTxBytes + 1)
	err = rts.mempools[primary].CheckTx(context.Background(), tx2, nil, mempool.TxInfo{SenderID: mempool.UnknownPeerID})
	require.Error(t, err)

	rts.assertMempoolChannelsDrained(t)
}

func TestDontExhaustMaxActiveIDs(t *testing.T) {
	// we're creating a single node network, but not starting the
	// network.
	rts := setupReactors(t, 1, mempool.MaxActiveIDs+1)

	nodeID := rts.nodes[0]

	peerID, err := types.NewNodeID("0011223344556677889900112233445566778899")
	require.NoError(t, err)

	// ensure the reactor does not panic (i.e. exhaust active IDs)
	for i := 0; i < mempool.MaxActiveIDs+1; i++ {
		rts.peerChans[nodeID] <- p2p.PeerUpdate{
			Status: p2p.PeerStatusUp,
			NodeID: peerID,
		}

		rts.mempoolChannels[nodeID].Out <- p2p.Envelope{
			To: peerID,
			Message: &protomem.Txs{
				Txs: [][]byte{},
			},
		}
	}

	require.Eventually(
		t,
		func() bool {
			for _, mch := range rts.mempoolChannels {
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
	ids := mempool.NewMempoolIDs()

	peerID, err := types.NewNodeID("0011223344556677889900112233445566778899")
	require.NoError(t, err)

	for i := 0; i < mempool.MaxActiveIDs-1; i++ {
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

	rts := setupReactors(t, 2, 0)

	primary := rts.nodes[0]
	secondary := rts.nodes[1]

	rts.start(t)

	// disconnect peer
	rts.peerChans[primary] <- p2p.PeerUpdate{
		Status: p2p.PeerStatusDown,
		NodeID: secondary,
	}
}
