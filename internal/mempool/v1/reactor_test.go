package v1

import (
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/config"
	tmsync "github.com/tendermint/tendermint/internal/libs/sync"
	"github.com/tendermint/tendermint/internal/mempool"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/internal/p2p/p2ptest"
	"github.com/tendermint/tendermint/libs/log"
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

	cfg, err := config.ResetTestRoot(strings.ReplaceAll(t.Name(), "/", "|"))
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(cfg.RootDir) })

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

	chDesc := p2p.ChannelDescriptor{ID: byte(mempool.MempoolChannel)}
	rts.mempoolChannels = rts.network.MakeChannelsNoCleanup(t, chDesc, new(protomem.Message), int(chBuf))

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
			primaryMempool.Lock()
			primaryMempool.insertTx(next)
			primaryMempool.Unlock()
		}()
	}

	err := primaryReactor.Stop()
	require.NoError(t, err)
	primaryReactor.peerWG.Wait()
	wg.Wait()
}
