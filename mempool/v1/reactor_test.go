package v1

import (
	"encoding/hex"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log/term"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/p2p/mock"

	cfg "github.com/tendermint/tendermint/config"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	memproto "github.com/tendermint/tendermint/proto/tendermint/mempool"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
)

const (
	numTxs  = 1000
	timeout = 120 * time.Second // ridiculously high because CircleCI is slow
)

type peerState struct {
	height int64
}

func (ps peerState) GetHeight() int64 {
	return ps.height
}

// Send a bunch of txs to the first reactor's mempool and wait for them all to
// be received in the others.
func TestReactorBroadcastTxsMessage(t *testing.T) {
	config := cfg.TestConfig()
	// if there were more than two reactors, the order of transactions could not be
	// asserted in waitForTxsOnReactors (due to transactions gossiping). If we
	// replace Connect2Switches (full mesh) with a func, which connects first
	// reactor to others and nothing else, this test should also pass with >2 reactors.
	const N = 2
	reactors := makeAndConnectReactors(config, N)
	defer func() {
		for _, r := range reactors {
			if err := r.Stop(); err != nil {
				assert.NoError(t, err)
			}
		}
	}()
	for _, r := range reactors {
		for _, peer := range r.Switch.Peers().List() {
			peer.Set(types.PeerStateKey, peerState{1})
		}
	}

	txs := checkTxs(t, reactors[0].mempool, numTxs, mempool.UnknownPeerID)
	transactions := make(types.Txs, len(txs))
	for idx, tx := range txs {
		transactions[idx] = tx.tx
	}

	waitForTxsOnReactors(t, transactions, reactors)
}

func TestMempoolVectors(t *testing.T) {
	testCases := []struct {
		testName string
		tx       []byte
		expBytes string
	}{
		{"tx 1", []byte{123}, "0a030a017b"},
		{"tx 2", []byte("proto encoding in mempool"), "0a1b0a1970726f746f20656e636f64696e6720696e206d656d706f6f6c"},
	}

	for _, tc := range testCases {
		tc := tc

		msg := memproto.Message{
			Sum: &memproto.Message_Txs{
				Txs: &memproto.Txs{Txs: [][]byte{tc.tx}},
			},
		}
		bz, err := msg.Marshal()
		require.NoError(t, err, tc.testName)

		require.Equal(t, tc.expBytes, hex.EncodeToString(bz), tc.testName)
	}
}

func TestLegacyReactorReceiveBasic(t *testing.T) {
	config := cfg.TestConfig()
	// if there were more than two reactors, the order of transactions could not be
	// asserted in waitForTxsOnReactors (due to transactions gossiping). If we
	// replace Connect2Switches (full mesh) with a func, which connects first
	// reactor to others and nothing else, this test should also pass with >2 reactors.
	const N = 1
	reactors := makeAndConnectReactors(config, N)
	var (
		reactor = reactors[0]
		peer    = mock.NewPeer(nil)
	)
	defer func() {
		err := reactor.Stop()
		assert.NoError(t, err)
	}()

	reactor.InitPeer(peer)
	reactor.AddPeer(peer)
	m := &memproto.Txs{}
	wm := m.Wrap()
	msg, err := proto.Marshal(wm)
	assert.NoError(t, err)

	assert.NotPanics(t, func() {
		reactor.Receive(mempool.MempoolChannel, peer, msg)
	})
}

func makeAndConnectReactors(config *cfg.Config, n int) []*Reactor {
	reactors := make([]*Reactor, n)
	logger := mempoolLogger()
	for i := 0; i < n; i++ {
		app := kvstore.NewApplication()
		cc := proxy.NewLocalClientCreator(app)
		mempool, cleanup := newMempoolWithApp(cc)
		defer cleanup()

		reactors[i] = NewReactor(config.Mempool, mempool) // so we dont start the consensus states
		reactors[i].SetLogger(logger.With("validator", i))
	}

	p2p.MakeConnectedSwitches(config.P2P, n, func(i int, s *p2p.Switch) *p2p.Switch {
		s.AddReactor("MEMPOOL", reactors[i])
		return s

	}, p2p.Connect2Switches)
	return reactors
}

// mempoolLogger is a TestingLogger which uses a different
// color for each validator ("validator" key must exist).
func mempoolLogger() log.Logger {
	return log.TestingLoggerWithColorFn(func(keyvals ...interface{}) term.FgBgColor {
		for i := 0; i < len(keyvals)-1; i += 2 {
			if keyvals[i] == "validator" {
				return term.FgBgColor{Fg: term.Color(uint8(keyvals[i+1].(int) + 1))}
			}
		}
		return term.FgBgColor{}
	})
}

func newMempoolWithApp(cc proxy.ClientCreator) (*TxMempool, func()) {
	conf := cfg.ResetTestRoot("mempool_test")

	mp, cu := newMempoolWithAppAndConfig(cc, conf)
	return mp, cu
}

func newMempoolWithAppAndConfig(cc proxy.ClientCreator, conf *cfg.Config) (*TxMempool, func()) {
	appConnMem, _ := cc.NewABCIClient()
	appConnMem.SetLogger(log.TestingLogger().With("module", "abci-client", "connection", "mempool"))
	err := appConnMem.Start()
	if err != nil {
		panic(err)
	}

	mp := NewTxMempool(log.TestingLogger(), conf.Mempool, appConnMem, 0)

	return mp, func() { os.RemoveAll(conf.RootDir) }
}

func waitForTxsOnReactors(t *testing.T, txs types.Txs, reactors []*Reactor) {
	// wait for the txs in all mempools
	wg := new(sync.WaitGroup)
	for i, reactor := range reactors {
		wg.Add(1)
		go func(r *Reactor, reactorIndex int) {
			defer wg.Done()
			waitForTxsOnReactor(t, txs, r, reactorIndex)
		}(reactor, i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	timer := time.After(timeout)
	select {
	case <-timer:
		t.Fatal("Timed out waiting for txs")
	case <-done:
	}
}

func waitForTxsOnReactor(t *testing.T, txs types.Txs, reactor *Reactor, reactorIndex int) {
	mempool := reactor.mempool
	for mempool.Size() < len(txs) {
		time.Sleep(time.Millisecond * 100)
	}

	reapedTxs := mempool.ReapMaxTxs(len(txs))
	for i, tx := range txs {
		assert.Equalf(t, tx, reapedTxs[i],
			"txs at index %d on reactor %d don't match: %v vs %v", i, reactorIndex, tx, reapedTxs[i])
	}
}
