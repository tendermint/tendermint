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
	peerUpdates   *p2p.PeerUpdatesCh
}

func setup(t *testing.T, cfg *cfg.MempoolConfig, logger log.Logger, chBuf uint) *reactorTestSuite {
	t.Helper()

	pID := make([]byte, 16)
	_, err := rng.Read(pID)
	require.NoError(t, err)

	peerUpdatesCh := make(chan p2p.PeerUpdate, chBuf)

	rts := &reactorTestSuite{
		mempoolInCh:      make(chan p2p.Envelope, chBuf),
		mempoolOutCh:     make(chan p2p.Envelope, chBuf),
		mempoolPeerErrCh: make(chan p2p.PeerError, chBuf),
		peerUpdatesCh:    peerUpdatesCh,
		peerUpdates:      p2p.NewPeerUpdates(peerUpdatesCh),
		peerID:           p2p.NodeID(fmt.Sprintf("%x", pID)),
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

func simulateRouter(wg *sync.WaitGroup, primary *reactorTestSuite, suites []*reactorTestSuite, numOut int) {
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
			PeerID: suite.peerID,
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

	peerID := uint16(1)
	_ = checkTxs(t, primary.reactor.mempool, numTxs, peerID)

	primary.peerUpdatesCh <- p2p.PeerUpdate{
		Status: p2p.PeerStatusUp,
		PeerID: secondary.peerID,
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

// ============================================================================

// func TestReactor_MaxTxBytes(t *testing.T) {
// 	config := cfg.TestConfig()

// 	const N = 2
// 	reactors := makeAndConnectReactors(config, N)
// 	defer func() {
// 		for _, r := range reactors {
// 			if err := r.Stop(); err != nil {
// 				assert.NoError(t, err)
// 			}
// 		}
// 	}()
// 	for _, r := range reactors {
// 		for _, peer := range r.Switch.Peers().List() {
// 			peer.Set(types.PeerStateKey, peerState{1})
// 		}
// 	}

// 	// Broadcast a tx, which has the max size
// 	// => ensure it's received by the second reactor.
// 	tx1 := tmrand.Bytes(config.Mempool.MaxTxBytes)
// 	err := reactors[0].mempool.CheckTx(tx1, nil, TxInfo{SenderID: UnknownPeerID})
// 	require.NoError(t, err)
// 	waitForTxsOnReactors(t, []types.Tx{tx1}, reactors)

// 	reactors[0].mempool.Flush()
// 	reactors[1].mempool.Flush()

// 	// Broadcast a tx, which is beyond the max size
// 	// => ensure it's not sent
// 	tx2 := tmrand.Bytes(config.Mempool.MaxTxBytes + 1)
// 	err = reactors[0].mempool.CheckTx(tx2, nil, TxInfo{SenderID: UnknownPeerID})
// 	require.Error(t, err)
// }

// func TestBroadcastTxForPeerStopsWhenPeerStops(t *testing.T) {
// 	if testing.Short() {
// 		t.Skip("skipping test in short mode.")
// 	}

// 	config := cfg.TestConfig()
// 	const N = 2
// 	reactors := makeAndConnectReactors(config, N)
// 	defer func() {
// 		for _, r := range reactors {
// 			if err := r.Stop(); err != nil {
// 				assert.NoError(t, err)
// 			}
// 		}
// 	}()

// 	// stop peer
// 	sw := reactors[1].Switch
// 	sw.StopPeerForError(sw.Peers().List()[0], errors.New("some reason"))

// 	// check that we are not leaking any go-routines
// 	// i.e. broadcastTxRoutine finishes when peer is stopped
// 	leaktest.CheckTimeout(t, 10*time.Second)()
// }

// func TestBroadcastTxForPeerStopsWhenReactorStops(t *testing.T) {
// 	if testing.Short() {
// 		t.Skip("skipping test in short mode.")
// 	}

// 	config := cfg.TestConfig()
// 	const N = 2
// 	reactors := makeAndConnectReactors(config, N)

// 	// stop reactors
// 	for _, r := range reactors {
// 		if err := r.Stop(); err != nil {
// 			assert.NoError(t, err)
// 		}
// 	}

// 	// check that we are not leaking any go-routines
// 	// i.e. broadcastTxRoutine finishes when reactor is stopped
// 	leaktest.CheckTimeout(t, 10*time.Second)()
// }

// func TestMempoolIDsBasic(t *testing.T) {
// 	ids := newMempoolIDs()

// 	peer := mock.NewPeer(net.IP{127, 0, 0, 1})

// 	ids.ReserveForPeer(peer)
// 	assert.EqualValues(t, 1, ids.GetForPeer(peer))
// 	ids.Reclaim(peer)

// 	ids.ReserveForPeer(peer)
// 	assert.EqualValues(t, 2, ids.GetForPeer(peer))
// 	ids.Reclaim(peer)
// }

// func TestMempoolIDsPanicsIfNodeRequestsOvermaxActiveIDs(t *testing.T) {
// 	if testing.Short() {
// 		return
// 	}

// 	// 0 is already reserved for UnknownPeerID
// 	ids := newMempoolIDs()

// 	for i := 0; i < maxActiveIDs-1; i++ {
// 		peer := mock.NewPeer(net.IP{127, 0, 0, 1})
// 		ids.ReserveForPeer(peer)
// 	}

// 	assert.Panics(t, func() {
// 		peer := mock.NewPeer(net.IP{127, 0, 0, 1})
// 		ids.ReserveForPeer(peer)
// 	})
// }

// func TestDontExhaustMaxActiveIDs(t *testing.T) {
// 	config := cfg.TestConfig()
// 	const N = 1
// 	reactors := makeAndConnectReactors(config, N)
// 	defer func() {
// 		for _, r := range reactors {
// 			if err := r.Stop(); err != nil {
// 				assert.NoError(t, err)
// 			}
// 		}
// 	}()
// 	reactor := reactors[0]

// 	for i := 0; i < maxActiveIDs+1; i++ {
// 		peer := mock.NewPeer(nil)
// 		reactor.Receive(MempoolChannel, peer, []byte{0x1, 0x2, 0x3})
// 		reactor.AddPeer(peer)
// 	}
// }

// // mempoolLogger is a TestingLogger which uses a different
// // color for each validator ("validator" key must exist).
// func mempoolLogger() log.Logger {
// 	return log.TestingLoggerWithColorFn(func(keyvals ...interface{}) term.FgBgColor {
// 		for i := 0; i < len(keyvals)-1; i += 2 {
// 			if keyvals[i] == "validator" {
// 				return term.FgBgColor{Fg: term.Color(uint8(keyvals[i+1].(int) + 1))}
// 			}
// 		}
// 		return term.FgBgColor{}
// 	})
// }

// // connect N mempool reactors through N switches
// func makeAndConnectReactors(config *cfg.Config, n int) []*Reactor {
// 	reactors := make([]*Reactor, n)
// 	logger := mempoolLogger()
// 	for i := 0; i < n; i++ {
// 		app := kvstore.NewApplication()
// 		cc := proxy.NewLocalClientCreator(app)
// 		mempool, cleanup := newMempoolWithApp(cc)
// 		defer cleanup()

// 		reactors[i] = NewReactor(config.Mempool, mempool) // so we dont start the consensus states
// 		reactors[i].SetLogger(logger.With("validator", i))
// 	}

// 	p2p.MakeConnectedSwitches(config.P2P, n, func(i int, s *p2p.Switch) *p2p.Switch {
// 		s.AddReactor("MEMPOOL", reactors[i])
// 		return s

// 	}, p2p.Connect2Switches)
// 	return reactors
// }

// func waitForTxsOnReactors(t *testing.T, txs types.Txs, reactors []*Reactor) {
// 	// wait for the txs in all mempools
// 	wg := new(sync.WaitGroup)
// 	for i, reactor := range reactors {
// 		wg.Add(1)
// 		go func(r *Reactor, reactorIndex int) {
// 			defer wg.Done()
// 			waitForTxsOnReactor(t, txs, r, reactorIndex)
// 		}(reactor, i)
// 	}

// 	done := make(chan struct{})
// 	go func() {
// 		wg.Wait()
// 		close(done)
// 	}()

// 	timer := time.After(timeout)
// 	select {
// 	case <-timer:
// 		t.Fatal("Timed out waiting for txs")
// 	case <-done:
// 	}
// }

// func waitForTxsOnReactor(t *testing.T, txs types.Txs, reactor *Reactor, reactorIndex int) {
// 	mempool := reactor.mempool
// 	for mempool.Size() < len(txs) {
// 		time.Sleep(time.Millisecond * 100)
// 	}

// 	reapedTxs := mempool.ReapMaxTxs(len(txs))
// 	for i, tx := range txs {
// 		assert.Equalf(t, tx, reapedTxs[i],
// 			"txs at index %d on reactor %d don't match: %v vs %v", i, reactorIndex, tx, reapedTxs[i])
// 	}
// }

// // ensure no txs on reactor after some timeout
// func ensureNoTxs(t *testing.T, reactor *Reactor, timeout time.Duration) {
// 	time.Sleep(timeout) // wait for the txs in all mempools
// 	assert.Zero(t, reactor.mempool.Size())
// }
