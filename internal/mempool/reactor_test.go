package mempool

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/require"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
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
	mempoolChannels map[types.NodeID]p2p.Channel
	mempools        map[types.NodeID]*TxMempool
	kvstores        map[types.NodeID]*kvstore.Application

	peerChans   map[types.NodeID]chan p2p.PeerUpdate
	peerUpdates map[types.NodeID]*p2p.PeerUpdates

	nodes []types.NodeID
}

func setupReactors(ctx context.Context, t *testing.T, logger log.Logger, numNodes int, chBuf uint) *reactorTestSuite {
	t.Helper()

	cfg, err := config.ResetTestRoot(t.TempDir(), strings.ReplaceAll(t.Name(), "/", "|"))
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(cfg.RootDir) })

	rts := &reactorTestSuite{
		logger:          log.NewNopLogger().With("testCase", t.Name()),
		network:         p2ptest.MakeNetwork(ctx, t, p2ptest.NetworkOptions{NumNodes: numNodes}),
		reactors:        make(map[types.NodeID]*Reactor, numNodes),
		mempoolChannels: make(map[types.NodeID]p2p.Channel, numNodes),
		mempools:        make(map[types.NodeID]*TxMempool, numNodes),
		kvstores:        make(map[types.NodeID]*kvstore.Application, numNodes),
		peerChans:       make(map[types.NodeID]chan p2p.PeerUpdate, numNodes),
		peerUpdates:     make(map[types.NodeID]*p2p.PeerUpdates, numNodes),
	}

	chDesc := getChannelDescriptor(cfg.Mempool)
	rts.mempoolChannels = rts.network.MakeChannelsNoCleanup(ctx, t, chDesc)

	for nodeID := range rts.network.Nodes {
		rts.kvstores[nodeID] = kvstore.NewApplication()

		client := abciclient.NewLocalClient(logger, rts.kvstores[nodeID])
		require.NoError(t, client.Start(ctx))
		t.Cleanup(client.Wait)

		mempool := setup(t, client, 1<<20)
		rts.mempools[nodeID] = mempool

		rts.peerChans[nodeID] = make(chan p2p.PeerUpdate, chBuf)
		rts.peerUpdates[nodeID] = p2p.NewPeerUpdates(rts.peerChans[nodeID], 1)
		rts.network.Nodes[nodeID].PeerManager.Register(ctx, rts.peerUpdates[nodeID])

		chCreator := func(ctx context.Context, chDesc *p2p.ChannelDescriptor) (p2p.Channel, error) {
			return rts.mempoolChannels[nodeID], nil
		}

		rts.reactors[nodeID] = NewReactor(
			rts.logger.With("nodeID", nodeID),
			cfg.Mempool,
			mempool,
			chCreator,
			func(ctx context.Context) *p2p.PeerUpdates { return rts.peerUpdates[nodeID] },
		)
		rts.nodes = append(rts.nodes, nodeID)

		require.NoError(t, rts.reactors[nodeID].Start(ctx))
		require.True(t, rts.reactors[nodeID].IsRunning())
	}

	require.Len(t, rts.reactors, numNodes)

	t.Cleanup(func() {
		for nodeID := range rts.reactors {
			if rts.reactors[nodeID].IsRunning() {
				rts.reactors[nodeID].Stop()
				rts.reactors[nodeID].Wait()
				require.False(t, rts.reactors[nodeID].IsRunning())
			}

		}
	})

	t.Cleanup(leaktest.Check(t))

	return rts
}

func (rts *reactorTestSuite) start(ctx context.Context, t *testing.T) {
	t.Helper()
	rts.network.Start(ctx, t)

	require.Len(t,
		rts.network.RandomNode().PeerManager.Peers(),
		len(rts.nodes)-1,
		"network does not have expected number of nodes")
}

func (rts *reactorTestSuite) waitForTxns(t *testing.T, txs []types.Tx, ids ...types.NodeID) {
	t.Helper()

	// ensure that the transactions get fully broadcast to the
	// rest of the network
	wg := &sync.WaitGroup{}
	for name, pool := range rts.mempools {
		if !p2ptest.NodeInSlice(name, ids) {
			continue
		}
		if len(txs) == pool.Size() {
			continue
		}

		wg.Add(1)
		go func(name types.NodeID, pool *TxMempool) {
			defer wg.Done()
			require.Eventually(t, func() bool { return len(txs) == pool.Size() },
				time.Minute,
				250*time.Millisecond,
				"node=%q, ntx=%d, size=%d", name, len(txs), pool.Size(),
			)
		}(name, pool)
	}
	wg.Wait()
}

func TestReactorBroadcastDoesNotPanic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const numNodes = 2

	logger := log.NewNopLogger()
	rts := setupReactors(ctx, t, logger, numNodes, 0)

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
	primaryMempool.Lock()
	primaryMempool.insertTx(firstTx)
	primaryMempool.Unlock()

	// run the router
	rts.start(ctx, t)

	go primaryReactor.broadcastTxRoutine(ctx, secondary, rts.mempoolChannels[primary])

	wg := &sync.WaitGroup{}
	for i := 0; i < 50; i++ {
		next := &WrappedTx{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			primaryMempool.Lock()
			defer primaryMempool.Unlock()
			primaryMempool.insertTx(next)
		}()
	}

	primaryReactor.Stop()
	wg.Wait()
}

func TestReactorBroadcastTxs(t *testing.T) {
	numTxs := 512
	numNodes := 4
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	rts := setupReactors(ctx, t, logger, numNodes, uint(numTxs))

	primary := rts.nodes[0]
	secondaries := rts.nodes[1:]

	txs := checkTxs(ctx, t, rts.reactors[primary].mempool, numTxs, UnknownPeerID)

	require.Equal(t, numTxs, rts.reactors[primary].mempool.Size())

	rts.start(ctx, t)

	// Wait till all secondary suites (reactor) received all mempool txs from the
	// primary suite (node).
	rts.waitForTxns(t, convertTex(txs), secondaries...)
}

// regression test for https://github.com/tendermint/tendermint/issues/5408
func TestReactorConcurrency(t *testing.T) {
	numTxs := 10
	numNodes := 2

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()
	rts := setupReactors(ctx, t, logger, numNodes, 0)

	primary := rts.nodes[0]
	secondary := rts.nodes[1]

	rts.start(ctx, t)

	var wg sync.WaitGroup

	for i := 0; i < runtime.NumCPU()*2; i++ {
		wg.Add(2)

		// 1. submit a bunch of txs
		// 2. update the whole mempool

		txs := checkTxs(ctx, t, rts.reactors[primary].mempool, numTxs, UnknownPeerID)
		go func() {
			defer wg.Done()

			mempool := rts.mempools[primary]

			mempool.Lock()
			defer mempool.Unlock()

			deliverTxResponses := make([]*abci.ExecTxResult, len(txs))
			for i := range txs {
				deliverTxResponses[i] = &abci.ExecTxResult{Code: 0}
			}

			require.NoError(t, mempool.Update(ctx, 1, convertTex(txs), deliverTxResponses, nil, nil, true))
		}()

		// 1. submit a bunch of txs
		// 2. update none
		_ = checkTxs(ctx, t, rts.reactors[secondary].mempool, numTxs, UnknownPeerID)
		go func() {
			defer wg.Done()

			mempool := rts.mempools[secondary]

			mempool.Lock()
			defer mempool.Unlock()

			err := mempool.Update(ctx, 1, []types.Tx{}, make([]*abci.ExecTxResult, 0), nil, nil, true)
			require.NoError(t, err)
		}()
	}

	wg.Wait()
}

func TestReactorNoBroadcastToSender(t *testing.T) {
	numTxs := 1000
	numNodes := 2

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()
	rts := setupReactors(ctx, t, logger, numNodes, uint(numTxs))

	primary := rts.nodes[0]
	secondary := rts.nodes[1]

	peerID := uint16(1)
	_ = checkTxs(ctx, t, rts.mempools[primary], numTxs, peerID)

	rts.start(ctx, t)

	time.Sleep(100 * time.Millisecond)

	require.Eventually(t, func() bool {
		return rts.mempools[secondary].Size() == 0
	}, time.Minute, 100*time.Millisecond)
}

func TestReactor_MaxTxBytes(t *testing.T) {
	numNodes := 2
	cfg := config.TestConfig()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	rts := setupReactors(ctx, t, logger, numNodes, 0)

	primary := rts.nodes[0]
	secondary := rts.nodes[1]

	// Broadcast a tx, which has the max size and ensure it's received by the
	// second reactor.
	tx1 := tmrand.Bytes(cfg.Mempool.MaxTxBytes)
	err := rts.reactors[primary].mempool.CheckTx(
		ctx,
		tx1,
		nil,
		TxInfo{
			SenderID: UnknownPeerID,
		},
	)
	require.NoError(t, err)

	rts.start(ctx, t)

	rts.reactors[primary].mempool.Flush()
	rts.reactors[secondary].mempool.Flush()

	// broadcast a tx, which is beyond the max size and ensure it's not sent
	tx2 := tmrand.Bytes(cfg.Mempool.MaxTxBytes + 1)
	err = rts.mempools[primary].CheckTx(ctx, tx2, nil, TxInfo{SenderID: UnknownPeerID})
	require.Error(t, err)
}

func TestDontExhaustMaxActiveIDs(t *testing.T) {
	// we're creating a single node network, but not starting the
	// network.

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()
	rts := setupReactors(ctx, t, logger, 1, MaxActiveIDs+1)

	nodeID := rts.nodes[0]

	peerID, err := types.NewNodeID("0011223344556677889900112233445566778899")
	require.NoError(t, err)

	// ensure the reactor does not panic (i.e. exhaust active IDs)
	for i := 0; i < MaxActiveIDs+1; i++ {
		rts.peerChans[nodeID] <- p2p.PeerUpdate{
			Status: p2p.PeerStatusUp,
			NodeID: peerID,
		}

		require.NoError(t, rts.mempoolChannels[nodeID].Send(ctx, p2p.Envelope{
			To: peerID,
			Message: &protomem.Txs{
				Txs: [][]byte{},
			},
		}))
	}
}

func TestMempoolIDsPanicsIfNodeRequestsOvermaxActiveIDs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	// 0 is already reserved for UnknownPeerID
	ids := NewMempoolIDs()

	for i := 0; i < MaxActiveIDs-1; i++ {
		peerID, err := types.NewNodeID(fmt.Sprintf("%040d", i))
		require.NoError(t, err)
		ids.ReserveForPeer(peerID)
	}

	peerID, err := types.NewNodeID(fmt.Sprintf("%040d", MaxActiveIDs-1))
	require.NoError(t, err)
	require.Panics(t, func() {
		ids.ReserveForPeer(peerID)
	})
}

func TestBroadcastTxForPeerStopsWhenPeerStops(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	rts := setupReactors(ctx, t, logger, 2, 2)

	primary := rts.nodes[0]
	secondary := rts.nodes[1]

	rts.start(ctx, t)

	// disconnect peer
	rts.peerChans[primary] <- p2p.PeerUpdate{
		Status: p2p.PeerStatusDown,
		NodeID: secondary,
	}
	time.Sleep(500 * time.Millisecond)

	txs := checkTxs(ctx, t, rts.reactors[primary].mempool, 4, UnknownPeerID)
	require.Equal(t, 4, len(txs))
	require.Equal(t, 4, rts.mempools[primary].Size())
	require.Equal(t, 0, rts.mempools[secondary].Size())
}
