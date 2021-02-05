package mempool

import (
	"fmt"
	"math/rand"
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
	protomem "github.com/tendermint/tendermint/proto/tendermint/mempool"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
)

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

type reactorTestSuite struct {
	reactor *Reactor

	peerID p2p.NodeID

	mempoolChannel   *p2p.Channel
	mempoolInCh      chan p2p.Envelope
	mempoolOutCh     chan p2p.Envelope
	mempoolPeerErrCh chan p2p.PeerError

	peerUpdatesCh chan p2p.PeerUpdate
	peerUpdates   *p2p.PeerUpdates
}

func setup(t *testing.T, cfg *cfg.MempoolConfig, logger log.Logger, chBuf uint) *reactorTestSuite {
	t.Helper()

	pID := make([]byte, 20)
	_, err := rng.Read(pID)
	require.NoError(t, err)

	peerID, err := p2p.NewNodeID(fmt.Sprintf("%x", pID))
	require.NoError(t, err)

	peerUpdatesCh := make(chan p2p.PeerUpdate, chBuf)

	rts := &reactorTestSuite{
		mempoolInCh:      make(chan p2p.Envelope, chBuf),
		mempoolOutCh:     make(chan p2p.Envelope, chBuf),
		mempoolPeerErrCh: make(chan p2p.PeerError, chBuf),
		peerUpdatesCh:    peerUpdatesCh,
		peerUpdates:      p2p.NewPeerUpdates(peerUpdatesCh),
		peerID:           peerID,
	}

	rts.mempoolChannel = p2p.NewChannel(
		MempoolChannel,
		new(protomem.Message),
		rts.mempoolInCh,
		rts.mempoolOutCh,
		rts.mempoolPeerErrCh,
	)

	app := kvstore.NewApplication()
	cc := proxy.NewLocalClientCreator(app)
	mempool, memCleanup := newMempoolWithApp(cc)

	mempool.SetLogger(logger)

	rts.reactor = NewReactor(
		logger,
		cfg,
		nil,
		mempool,
		rts.mempoolChannel,
		rts.peerUpdates,
	)

	require.NoError(t, rts.reactor.Start())
	require.True(t, rts.reactor.IsRunning())

	t.Cleanup(func() {
		memCleanup()
		require.NoError(t, rts.reactor.Stop())
		require.False(t, rts.reactor.IsRunning())
	})

	return rts
}

func simulateRouter(
	wg *sync.WaitGroup,
	primary *reactorTestSuite,
	suites []*reactorTestSuite,
	numOut int,
) {

	wg.Add(1)

	// create a mapping for efficient suite lookup by peer ID
	suitesByPeerID := make(map[p2p.NodeID]*reactorTestSuite)
	for _, suite := range suites {
		suitesByPeerID[suite.peerID] = suite
	}

	// Simulate a router by listening for all outbound envelopes and proxying the
	// envelope to the respective peer (suite).
	go func() {
		for i := 0; i < numOut; i++ {
			envelope := <-primary.mempoolOutCh
			other := suitesByPeerID[envelope.To]

			other.mempoolInCh <- p2p.Envelope{
				From:    primary.peerID,
				To:      envelope.To,
				Message: envelope.Message,
			}
		}

		wg.Done()
	}()
}

func waitForTxs(t *testing.T, txs types.Txs, suites ...*reactorTestSuite) {
	t.Helper()

	wg := new(sync.WaitGroup)

	for _, suite := range suites {
		wg.Add(1)

		go func(s *reactorTestSuite) {
			mempool := s.reactor.mempool
			for mempool.Size() < len(txs) {
				time.Sleep(time.Millisecond * 100)
			}

			reapedTxs := mempool.ReapMaxTxs(len(txs))
			for i, tx := range txs {
				require.Equalf(
					t, tx, reapedTxs[i],
					"txs at index %d in reactor mempool mismatch; got: %v, expected: %v", i, tx, reapedTxs[i],
				)
			}

			wg.Done()
		}(suite)
	}

	wg.Wait()
}

func TestReactorBroadcastTxs(t *testing.T) {
	numTxs := 1000
	numNodes := 10
	config := cfg.TestConfig()

	testSuites := make([]*reactorTestSuite, numNodes)
	for i := 0; i < len(testSuites); i++ {
		logger := log.TestingLogger().With("node", i)
		testSuites[i] = setup(t, config.Mempool, logger, 0)
	}

	// ignore all peer errors
	for _, suite := range testSuites {
		go func(s *reactorTestSuite) {
			// drop all errors on the mempool channel
			for range s.mempoolPeerErrCh {
			}
		}(suite)
	}

	primary := testSuites[0]
	secondaries := testSuites[1:]

	// Simulate a router by listening for all outbound envelopes and proxying the
	// envelopes to the respective peer (suite).
	wg := new(sync.WaitGroup)
	simulateRouter(wg, primary, testSuites, numTxs*len(secondaries))

	txs := checkTxs(t, primary.reactor.mempool, numTxs, UnknownPeerID)

	// Add each secondary suite (node) as a peer to the primary suite (node). This
	// will cause the primary to gossip all mempool txs to the secondaries.
	for _, suite := range secondaries {
		primary.peerUpdatesCh <- p2p.PeerUpdate{
			Status: p2p.PeerStatusUp,
			NodeID: suite.peerID,
		}
	}

	// Wait till all secondary suites (reactor) received all mempool txs from the
	// primary suite (node).
	waitForTxs(t, txs, secondaries...)

	for _, suite := range testSuites {
		require.Equal(t, len(txs), suite.reactor.mempool.Size())
	}

	wg.Wait()

	// ensure all channels are drained
	for _, suite := range testSuites {
		require.Empty(t, suite.mempoolOutCh)
	}
}

// regression test for https://github.com/tendermint/tendermint/issues/5408
func TestReactorConcurrency(t *testing.T) {
	numTxs := 5
	numNodes := 2
	config := cfg.TestConfig()

	testSuites := make([]*reactorTestSuite, numNodes)
	for i := 0; i < len(testSuites); i++ {
		logger := log.TestingLogger().With("node", i)
		testSuites[i] = setup(t, config.Mempool, logger, 0)
	}

	primary := testSuites[0]
	secondary := testSuites[1]

	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(2)

		// 1. submit a bunch of txs
		// 2. update the whole mempool
		txs := checkTxs(t, primary.reactor.mempool, numTxs, UnknownPeerID)
		go func() {
			defer wg.Done()

			primary.reactor.mempool.Lock()
			defer primary.reactor.mempool.Unlock()

			deliverTxResponses := make([]*abci.ResponseDeliverTx, len(txs))
			for i := range txs {
				deliverTxResponses[i] = &abci.ResponseDeliverTx{Code: 0}
			}

			err := primary.reactor.mempool.Update(1, txs, deliverTxResponses, nil, nil)
			require.NoError(t, err)
		}()

		// 1. submit a bunch of txs
		// 2. update none
		_ = checkTxs(t, secondary.reactor.mempool, numTxs, UnknownPeerID)
		go func() {
			defer wg.Done()

			secondary.reactor.mempool.Lock()
			defer secondary.reactor.mempool.Unlock()

			err := secondary.reactor.mempool.Update(1, []types.Tx{}, make([]*abci.ResponseDeliverTx, 0), nil, nil)
			require.NoError(t, err)
		}()

		// flush the mempool
		secondary.reactor.mempool.Flush()
	}

	wg.Wait()
}

func TestReactorNoBroadcastToSender(t *testing.T) {
	numTxs := 1000
	numNodes := 2
	config := cfg.TestConfig()

	testSuites := make([]*reactorTestSuite, numNodes)
	for i := 0; i < len(testSuites); i++ {
		logger := log.TestingLogger().With("node", i)
		testSuites[i] = setup(t, config.Mempool, logger, uint(numTxs))
	}

	primary := testSuites[0]
	secondary := testSuites[1]

	// ignore all peer errors
	for _, suite := range testSuites {
		go func(s *reactorTestSuite) {
			// drop all errors on the mempool channel
			for range s.mempoolPeerErrCh {
			}
		}(suite)
	}

	peerID := uint16(1)
	_ = checkTxs(t, primary.reactor.mempool, numTxs, peerID)

	primary.peerUpdatesCh <- p2p.PeerUpdate{
		Status: p2p.PeerStatusUp,
		NodeID: secondary.peerID,
	}

	time.Sleep(100 * time.Millisecond)

	require.Eventually(t, func() bool {
		return secondary.reactor.mempool.Size() == 0
	}, time.Minute, 100*time.Millisecond)

	// ensure all channels are drained
	for _, suite := range testSuites {
		require.Empty(t, suite.mempoolOutCh)
	}
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

	testSuites := make([]*reactorTestSuite, numNodes)
	for i := 0; i < len(testSuites); i++ {
		logger := log.TestingLogger().With("node", i)
		testSuites[i] = setup(t, config.Mempool, logger, 0)
	}

	// ignore all peer errors
	for _, suite := range testSuites {
		go func(s *reactorTestSuite) {
			// drop all errors on the mempool channel
			for range s.mempoolPeerErrCh {
			}
		}(suite)
	}

	primary := testSuites[0]
	secondary := testSuites[1]

	// Simulate a router by listening for all outbound envelopes and proxying the
	// envelopes to the respective peer (suite).
	wg := new(sync.WaitGroup)
	simulateRouter(wg, primary, testSuites, 1)

	// Broadcast a tx, which has the max size and ensure it's received by the
	// second reactor.
	tx1 := tmrand.Bytes(config.Mempool.MaxTxBytes)
	err := primary.reactor.mempool.CheckTx(tx1, nil, TxInfo{SenderID: UnknownPeerID})
	require.NoError(t, err)

	primary.peerUpdatesCh <- p2p.PeerUpdate{
		Status: p2p.PeerStatusUp,
		NodeID: secondary.peerID,
	}

	// Wait till all secondary suites (reactor) received all mempool txs from the
	// primary suite (node).
	waitForTxs(t, []types.Tx{tx1}, secondary)

	primary.reactor.mempool.Flush()
	secondary.reactor.mempool.Flush()

	// broadcast a tx, which is beyond the max size and ensure it's not sent
	tx2 := tmrand.Bytes(config.Mempool.MaxTxBytes + 1)
	err = primary.reactor.mempool.CheckTx(tx2, nil, TxInfo{SenderID: UnknownPeerID})
	require.Error(t, err)

	wg.Wait()

	// ensure all channels are drained
	for _, suite := range testSuites {
		require.Empty(t, suite.mempoolOutCh)
	}
}

func TestDontExhaustMaxActiveIDs(t *testing.T) {
	config := cfg.TestConfig()
	reactor := setup(t, config.Mempool, log.TestingLogger().With("node", 0), 0)

	go func() {
		// drop all messages on the mempool channel
		for range reactor.mempoolOutCh {
		}
	}()

	go func() {
		// drop all errors on the mempool channel
		for range reactor.mempoolPeerErrCh {
		}
	}()

	peerID, err := p2p.NewNodeID("0011223344556677889900112233445566778899")
	require.NoError(t, err)

	// ensure the reactor does not panic (i.e. exhaust active IDs)
	for i := 0; i < maxActiveIDs+1; i++ {
		reactor.peerUpdatesCh <- p2p.PeerUpdate{
			Status: p2p.PeerStatusUp,
			NodeID: peerID,
		}
		reactor.mempoolOutCh <- p2p.Envelope{
			To: peerID,
			Message: &protomem.Txs{
				Txs: [][]byte{},
			},
		}
	}

	require.Empty(t, reactor.mempoolOutCh)
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

	testSuites := []*reactorTestSuite{
		setup(t, config.Mempool, log.TestingLogger().With("node", 0), 0),
		setup(t, config.Mempool, log.TestingLogger().With("node", 1), 0),
	}

	primary := testSuites[0]
	secondary := testSuites[1]

	// ignore all peer errors
	for _, suite := range testSuites {
		go func(s *reactorTestSuite) {
			// drop all errors on the mempool channel
			for range s.mempoolPeerErrCh {
			}
		}(suite)
	}

	// connect peer
	primary.peerUpdatesCh <- p2p.PeerUpdate{
		Status: p2p.PeerStatusUp,
		NodeID: secondary.peerID,
	}

	// disconnect peer
	primary.peerUpdatesCh <- p2p.PeerUpdate{
		Status: p2p.PeerStatusDown,
		NodeID: secondary.peerID,
	}
}
