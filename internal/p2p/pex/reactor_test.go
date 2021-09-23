package pex_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/internal/p2p/p2ptest"
	"github.com/tendermint/tendermint/internal/p2p/pex"
	"github.com/tendermint/tendermint/libs/log"
	p2pproto "github.com/tendermint/tendermint/proto/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

const (
	checkFrequency    = 500 * time.Millisecond
	defaultBufferSize = 2
	shortWait         = 10 * time.Second
	longWait          = 60 * time.Second

	firstNode  = 0
	secondNode = 1
	thirdNode  = 2
	fourthNode = 3
)

func TestReactorBasic(t *testing.T) {
	// start a network with one mock reactor and one "real" reactor
	testNet := setupNetwork(t, testOptions{
		MockNodes:  1,
		TotalNodes: 2,
	})
	testNet.connectAll(t)
	testNet.start(t)

	// assert that the mock node receives a request from the real node
	testNet.listenForRequest(t, secondNode, firstNode, shortWait)

	// assert that when a mock node sends a request it receives a response (and
	// the correct one)
	testNet.sendRequest(t, firstNode, secondNode, true)
	testNet.listenForResponse(t, secondNode, firstNode, shortWait, []p2pproto.PexAddressV2(nil))
}

func TestReactorConnectFullNetwork(t *testing.T) {
	testNet := setupNetwork(t, testOptions{
		TotalNodes: 4,
	})

	// make every node be only connected with one other node (it actually ends up
	// being two because of two way connections but oh well)
	testNet.connectN(t, 1)
	testNet.start(t)

	// assert that all nodes add each other in the network
	for idx := 0; idx < len(testNet.nodes); idx++ {
		testNet.requireNumberOfPeers(t, idx, len(testNet.nodes)-1, longWait)
	}
}

func TestReactorSendsRequestsTooOften(t *testing.T) {
	r := setupSingle(t)

	badNode := newNodeID(t, "b")

	r.pexInCh <- p2p.Envelope{
		From:    badNode,
		Message: &p2pproto.PexRequestV2{},
	}

	resp := <-r.pexOutCh
	msg, ok := resp.Message.(*p2pproto.PexResponseV2)
	require.True(t, ok)
	require.Empty(t, msg.Addresses)

	r.pexInCh <- p2p.Envelope{
		From:    badNode,
		Message: &p2pproto.PexRequestV2{},
	}

	peerErr := <-r.pexErrCh
	require.Error(t, peerErr.Err)
	require.Empty(t, r.pexOutCh)
	require.Contains(t, peerErr.Err.Error(), "peer sent a request too close after a prior one")
	require.Equal(t, badNode, peerErr.NodeID)
}

func TestReactorSendsResponseWithoutRequest(t *testing.T) {
	testNet := setupNetwork(t, testOptions{
		MockNodes:  1,
		TotalNodes: 3,
	})
	testNet.connectAll(t)
	testNet.start(t)

	// firstNode sends the secondNode an unrequested response
	// NOTE: secondNode will send a request by default during startup so we send
	// two responses to counter that.
	testNet.sendResponse(t, firstNode, secondNode, []int{thirdNode}, true)
	testNet.sendResponse(t, firstNode, secondNode, []int{thirdNode}, true)

	// secondNode should evict the firstNode
	testNet.listenForPeerUpdate(t, secondNode, firstNode, p2p.PeerStatusDown, shortWait)
}

func TestReactorNeverSendsTooManyPeers(t *testing.T) {
	testNet := setupNetwork(t, testOptions{
		MockNodes:  1,
		TotalNodes: 2,
	})
	testNet.connectAll(t)
	testNet.start(t)

	testNet.addNodes(t, 110)
	nodes := make([]int, 110)
	for i := 0; i < len(nodes); i++ {
		nodes[i] = i + 2
	}
	testNet.addAddresses(t, secondNode, nodes)

	// first we check that even although we have 110 peers, honest pex reactors
	// only send 100 (test if secondNode sends firstNode 100 addresses)
	testNet.pingAndlistenForNAddresses(t, secondNode, firstNode, shortWait, 100)
}

func TestReactorErrorsOnReceivingTooManyPeers(t *testing.T) {
	r := setupSingle(t)
	peer := p2p.NodeAddress{Protocol: p2p.MemoryProtocol, NodeID: randomNodeID(t)}
	added, err := r.manager.Add(peer)
	require.NoError(t, err)
	require.True(t, added)

	addresses := make([]p2pproto.PexAddressV2, 101)
	for i := 0; i < len(addresses); i++ {
		nodeAddress := p2p.NodeAddress{Protocol: p2p.MemoryProtocol, NodeID: randomNodeID(t)}
		addresses[i] = p2pproto.PexAddressV2{
			URL: nodeAddress.String(),
		}
	}

	r.peerCh <- p2p.PeerUpdate{
		NodeID: peer.NodeID,
		Status: p2p.PeerStatusUp,
	}

	select {
	// wait for a request and then send a response with too many addresses
	case req := <-r.pexOutCh:
		if _, ok := req.Message.(*p2pproto.PexRequestV2); !ok {
			t.Fatal("expected v2 pex request")
		}
		r.pexInCh <- p2p.Envelope{
			From: peer.NodeID,
			Message: &p2pproto.PexResponseV2{
				Addresses: addresses,
			},
		}

	case <-time.After(10 * time.Second):
		t.Fatal("pex failed to send a request within 10 seconds")
	}

	peerErr := <-r.pexErrCh
	require.Error(t, peerErr.Err)
	require.Empty(t, r.pexOutCh)
	require.Contains(t, peerErr.Err.Error(), "peer sent too many addresses")
	require.Equal(t, peer.NodeID, peerErr.NodeID)
}

func TestReactorSmallPeerStoreInALargeNetwork(t *testing.T) {
	testNet := setupNetwork(t, testOptions{
		TotalNodes:   8,
		MaxPeers:     4,
		MaxConnected: 3,
		BufferSize:   8,
	})
	testNet.connectN(t, 1)
	testNet.start(t)

	// test that all nodes reach full capacity
	for _, nodeID := range testNet.nodes {
		require.Eventually(t, func() bool {
			// nolint:scopelint
			return testNet.network.Nodes[nodeID].PeerManager.PeerRatio() >= 0.9
		}, longWait, checkFrequency)
	}
}

func TestReactorLargePeerStoreInASmallNetwork(t *testing.T) {
	testNet := setupNetwork(t, testOptions{
		TotalNodes:   3,
		MaxPeers:     25,
		MaxConnected: 25,
		BufferSize:   5,
	})
	testNet.connectN(t, 1)
	testNet.start(t)

	// assert that all nodes add each other in the network
	for idx := 0; idx < len(testNet.nodes); idx++ {
		testNet.requireNumberOfPeers(t, idx, len(testNet.nodes)-1, longWait)
	}
}

func TestReactorWithNetworkGrowth(t *testing.T) {
	testNet := setupNetwork(t, testOptions{
		TotalNodes: 5,
		BufferSize: 5,
	})
	testNet.connectAll(t)
	testNet.start(t)

	// assert that all nodes add each other in the network
	for idx := 0; idx < len(testNet.nodes); idx++ {
		testNet.requireNumberOfPeers(t, idx, len(testNet.nodes)-1, shortWait)
	}

	// now we inject 10 more nodes
	testNet.addNodes(t, 10)
	for i := 5; i < testNet.total; i++ {
		node := testNet.nodes[i]
		require.NoError(t, testNet.reactors[node].Start())
		require.True(t, testNet.reactors[node].IsRunning())
		// we connect all new nodes to a single entry point and check that the
		// node can distribute the addresses to all the others
		testNet.connectPeers(t, 0, i)
	}
	require.Len(t, testNet.reactors, 15)

	// assert that all nodes add each other in the network
	for idx := 0; idx < len(testNet.nodes); idx++ {
		testNet.requireNumberOfPeers(t, idx, len(testNet.nodes)-1, longWait)
	}
}

func TestReactorIntegrationWithLegacyHandleRequest(t *testing.T) {
	testNet := setupNetwork(t, testOptions{
		MockNodes:  1,
		TotalNodes: 3,
	})
	testNet.connectAll(t)
	testNet.start(t)
	t.Log(testNet.nodes)

	// mock node sends a V1 Pex message to the second node
	testNet.sendRequest(t, firstNode, secondNode, false)
	addrs := testNet.getAddressesFor(t, []int{thirdNode})
	testNet.listenForLegacyResponse(t, secondNode, firstNode, shortWait, addrs)
}

func TestReactorIntegrationWithLegacyHandleResponse(t *testing.T) {
	testNet := setupNetwork(t, testOptions{
		MockNodes:  1,
		TotalNodes: 4,
		BufferSize: 4,
	})
	testNet.connectPeers(t, firstNode, secondNode)
	testNet.connectPeers(t, firstNode, thirdNode)
	testNet.connectPeers(t, firstNode, fourthNode)
	testNet.start(t)

	testNet.listenForRequest(t, secondNode, firstNode, shortWait)
	// send a v1 response instead
	testNet.sendResponse(t, firstNode, secondNode, []int{thirdNode, fourthNode}, false)
	testNet.requireNumberOfPeers(t, secondNode, len(testNet.nodes)-1, shortWait)
}

type singleTestReactor struct {
	reactor  *pex.ReactorV2
	pexInCh  chan p2p.Envelope
	pexOutCh chan p2p.Envelope
	pexErrCh chan p2p.PeerError
	pexCh    *p2p.Channel
	peerCh   chan p2p.PeerUpdate
	manager  *p2p.PeerManager
}

func setupSingle(t *testing.T) *singleTestReactor {
	t.Helper()
	nodeID := newNodeID(t, "a")
	chBuf := 2
	pexInCh := make(chan p2p.Envelope, chBuf)
	pexOutCh := make(chan p2p.Envelope, chBuf)
	pexErrCh := make(chan p2p.PeerError, chBuf)
	pexCh := p2p.NewChannel(
		p2p.ChannelID(pex.PexChannel),
		new(p2pproto.PexMessage),
		pexInCh,
		pexOutCh,
		pexErrCh,
	)

	peerCh := make(chan p2p.PeerUpdate, chBuf)
	peerUpdates := p2p.NewPeerUpdates(peerCh, chBuf)
	peerManager, err := p2p.NewPeerManager(nodeID, dbm.NewMemDB(), p2p.PeerManagerOptions{})
	require.NoError(t, err)

	reactor := pex.NewReactorV2(log.TestingLogger(), peerManager, pexCh, peerUpdates)
	require.NoError(t, reactor.Start())
	t.Cleanup(func() {
		err := reactor.Stop()
		if err != nil {
			t.Fatal(err)
		}
		pexCh.Close()
		peerUpdates.Close()
	})

	return &singleTestReactor{
		reactor:  reactor,
		pexInCh:  pexInCh,
		pexOutCh: pexOutCh,
		pexErrCh: pexErrCh,
		pexCh:    pexCh,
		peerCh:   peerCh,
		manager:  peerManager,
	}
}

type reactorTestSuite struct {
	network *p2ptest.Network
	logger  log.Logger

	reactors    map[types.NodeID]*pex.ReactorV2
	pexChannels map[types.NodeID]*p2p.Channel

	peerChans   map[types.NodeID]chan p2p.PeerUpdate
	peerUpdates map[types.NodeID]*p2p.PeerUpdates

	nodes []types.NodeID
	mocks []types.NodeID
	total int
	opts  testOptions
}

type testOptions struct {
	MockNodes    int
	TotalNodes   int
	BufferSize   int
	MaxPeers     uint16
	MaxConnected uint16
}

// setup setups a test suite with a network of nodes. Mocknodes represent the
// hollow nodes that the test can listen and send on
func setupNetwork(t *testing.T, opts testOptions) *reactorTestSuite {
	t.Helper()

	require.Greater(t, opts.TotalNodes, opts.MockNodes)
	if opts.BufferSize == 0 {
		opts.BufferSize = defaultBufferSize
	}
	networkOpts := p2ptest.NetworkOptions{
		NumNodes:   opts.TotalNodes,
		BufferSize: opts.BufferSize,
		NodeOpts: p2ptest.NodeOptions{
			MaxPeers:     opts.MaxPeers,
			MaxConnected: opts.MaxConnected,
		},
	}
	chBuf := opts.BufferSize
	realNodes := opts.TotalNodes - opts.MockNodes

	rts := &reactorTestSuite{
		logger:      log.TestingLogger().With("testCase", t.Name()),
		network:     p2ptest.MakeNetwork(t, networkOpts),
		reactors:    make(map[types.NodeID]*pex.ReactorV2, realNodes),
		pexChannels: make(map[types.NodeID]*p2p.Channel, opts.TotalNodes),
		peerChans:   make(map[types.NodeID]chan p2p.PeerUpdate, opts.TotalNodes),
		peerUpdates: make(map[types.NodeID]*p2p.PeerUpdates, opts.TotalNodes),
		total:       opts.TotalNodes,
		opts:        opts,
	}

	// NOTE: we don't assert that the channels get drained after stopping the
	// reactor
	rts.pexChannels = rts.network.MakeChannelsNoCleanup(
		t, pex.ChannelDescriptor(), new(p2pproto.PexMessage), chBuf,
	)

	idx := 0
	for nodeID := range rts.network.Nodes {
		rts.peerChans[nodeID] = make(chan p2p.PeerUpdate, chBuf)
		rts.peerUpdates[nodeID] = p2p.NewPeerUpdates(rts.peerChans[nodeID], chBuf)
		rts.network.Nodes[nodeID].PeerManager.Register(rts.peerUpdates[nodeID])

		// the first nodes in the array are always mock nodes
		if idx < opts.MockNodes {
			rts.mocks = append(rts.mocks, nodeID)
		} else {
			rts.reactors[nodeID] = pex.NewReactorV2(
				rts.logger.With("nodeID", nodeID),
				rts.network.Nodes[nodeID].PeerManager,
				rts.pexChannels[nodeID],
				rts.peerUpdates[nodeID],
			)
		}
		rts.nodes = append(rts.nodes, nodeID)

		idx++
	}

	require.Len(t, rts.reactors, realNodes)

	t.Cleanup(func() {
		for nodeID, reactor := range rts.reactors {
			if reactor.IsRunning() {
				require.NoError(t, reactor.Stop())
				require.False(t, reactor.IsRunning())
			}
			rts.pexChannels[nodeID].Close()
			rts.peerUpdates[nodeID].Close()
		}
		for _, nodeID := range rts.mocks {
			rts.pexChannels[nodeID].Close()
			rts.peerUpdates[nodeID].Close()
		}
	})

	return rts
}

// starts up the pex reactors for each node
func (r *reactorTestSuite) start(t *testing.T) {
	t.Helper()

	for _, reactor := range r.reactors {
		require.NoError(t, reactor.Start())
		require.True(t, reactor.IsRunning())
	}
}

func (r *reactorTestSuite) addNodes(t *testing.T, nodes int) {
	t.Helper()

	for i := 0; i < nodes; i++ {
		node := r.network.MakeNode(t, p2ptest.NodeOptions{
			MaxPeers:     r.opts.MaxPeers,
			MaxConnected: r.opts.MaxConnected,
		})
		r.network.Nodes[node.NodeID] = node
		nodeID := node.NodeID
		r.pexChannels[nodeID] = node.MakeChannelNoCleanup(
			t, pex.ChannelDescriptor(), new(p2pproto.PexMessage), r.opts.BufferSize,
		)
		r.peerChans[nodeID] = make(chan p2p.PeerUpdate, r.opts.BufferSize)
		r.peerUpdates[nodeID] = p2p.NewPeerUpdates(r.peerChans[nodeID], r.opts.BufferSize)
		r.network.Nodes[nodeID].PeerManager.Register(r.peerUpdates[nodeID])
		r.reactors[nodeID] = pex.NewReactorV2(
			r.logger.With("nodeID", nodeID),
			r.network.Nodes[nodeID].PeerManager,
			r.pexChannels[nodeID],
			r.peerUpdates[nodeID],
		)
		r.nodes = append(r.nodes, nodeID)
		r.total++
	}
}

func (r *reactorTestSuite) listenFor(
	t *testing.T,
	node types.NodeID,
	conditional func(msg p2p.Envelope) bool,
	assertion func(t *testing.T, msg p2p.Envelope) bool,
	waitPeriod time.Duration,
) {
	timesUp := time.After(waitPeriod)
	for {
		select {
		case envelope := <-r.pexChannels[node].In:
			if conditional(envelope) && assertion(t, envelope) {
				return
			}
		case <-timesUp:
			require.Fail(t, "timed out waiting for message",
				"node=%v, waitPeriod=%s", node, waitPeriod)
		}
	}
}

func (r *reactorTestSuite) listenForRequest(t *testing.T, fromNode, toNode int, waitPeriod time.Duration) {
	r.logger.Info("Listening for request", "from", fromNode, "to", toNode)
	to, from := r.checkNodePair(t, toNode, fromNode)
	conditional := func(msg p2p.Envelope) bool {
		_, ok := msg.Message.(*p2pproto.PexRequestV2)
		return ok && msg.From == from
	}
	assertion := func(t *testing.T, msg p2p.Envelope) bool {
		require.Equal(t, &p2pproto.PexRequestV2{}, msg.Message)
		return true
	}
	r.listenFor(t, to, conditional, assertion, waitPeriod)
}

func (r *reactorTestSuite) pingAndlistenForNAddresses(
	t *testing.T,
	fromNode, toNode int,
	waitPeriod time.Duration,
	addresses int,
) {
	r.logger.Info("Listening for addresses", "from", fromNode, "to", toNode)
	to, from := r.checkNodePair(t, toNode, fromNode)
	conditional := func(msg p2p.Envelope) bool {
		_, ok := msg.Message.(*p2pproto.PexResponseV2)
		return ok && msg.From == from
	}
	assertion := func(t *testing.T, msg p2p.Envelope) bool {
		m, ok := msg.Message.(*p2pproto.PexResponseV2)
		if !ok {
			require.Fail(t, "expected pex response v2")
			return true
		}
		// assert the same amount of addresses
		if len(m.Addresses) == addresses {
			return true
		}
		// if we didn't get the right length, we wait and send the
		// request again
		time.Sleep(300 * time.Millisecond)
		r.sendRequest(t, toNode, fromNode, true)
		return false
	}
	r.sendRequest(t, toNode, fromNode, true)
	r.listenFor(t, to, conditional, assertion, waitPeriod)
}

func (r *reactorTestSuite) listenForResponse(
	t *testing.T,
	fromNode, toNode int,
	waitPeriod time.Duration,
	addresses []p2pproto.PexAddressV2,
) {
	r.logger.Info("Listening for response", "from", fromNode, "to", toNode)
	to, from := r.checkNodePair(t, toNode, fromNode)
	conditional := func(msg p2p.Envelope) bool {
		_, ok := msg.Message.(*p2pproto.PexResponseV2)
		r.logger.Info("message", msg, "ok", ok)
		return ok && msg.From == from
	}
	assertion := func(t *testing.T, msg p2p.Envelope) bool {
		require.Equal(t, &p2pproto.PexResponseV2{Addresses: addresses}, msg.Message)
		return true
	}
	r.listenFor(t, to, conditional, assertion, waitPeriod)
}

func (r *reactorTestSuite) listenForLegacyResponse(
	t *testing.T,
	fromNode, toNode int,
	waitPeriod time.Duration,
	addresses []p2pproto.PexAddress,
) {
	r.logger.Info("Listening for response", "from", fromNode, "to", toNode)
	to, from := r.checkNodePair(t, toNode, fromNode)
	conditional := func(msg p2p.Envelope) bool {
		_, ok := msg.Message.(*p2pproto.PexResponse)
		return ok && msg.From == from
	}
	assertion := func(t *testing.T, msg p2p.Envelope) bool {
		require.Equal(t, &p2pproto.PexResponse{Addresses: addresses}, msg.Message)
		return true
	}
	r.listenFor(t, to, conditional, assertion, waitPeriod)
}

func (r *reactorTestSuite) listenForPeerUpdate(
	t *testing.T,
	onNode, withNode int,
	status p2p.PeerStatus,
	waitPeriod time.Duration,
) {
	on, with := r.checkNodePair(t, onNode, withNode)
	sub := r.network.Nodes[on].PeerManager.Subscribe()
	defer sub.Close()
	timesUp := time.After(waitPeriod)
	for {
		select {
		case peerUpdate := <-sub.Updates():
			if peerUpdate.NodeID == with {
				require.Equal(t, status, peerUpdate.Status)
				return
			}

		case <-timesUp:
			require.Fail(t, "timed out waiting for peer status", "%v with status %v",
				with, status)
			return
		}
	}
}

func (r *reactorTestSuite) getV2AddressesFor(nodes []int) []p2pproto.PexAddressV2 {
	addresses := make([]p2pproto.PexAddressV2, len(nodes))
	for idx, node := range nodes {
		nodeID := r.nodes[node]
		addresses[idx] = p2pproto.PexAddressV2{
			URL: r.network.Nodes[nodeID].NodeAddress.String(),
		}
	}
	return addresses
}

func (r *reactorTestSuite) getAddressesFor(t *testing.T, nodes []int) []p2pproto.PexAddress {
	addresses := make([]p2pproto.PexAddress, len(nodes))
	for idx, node := range nodes {
		nodeID := r.nodes[node]
		nodeAddrs := r.network.Nodes[nodeID].NodeAddress
		endpoints, err := nodeAddrs.Resolve(context.Background())
		require.NoError(t, err)
		require.Len(t, endpoints, 1)
		addresses[idx] = p2pproto.PexAddress{
			ID:   string(nodeAddrs.NodeID),
			IP:   endpoints[0].IP.String(),
			Port: uint32(endpoints[0].Port),
		}
	}
	return addresses
}

func (r *reactorTestSuite) sendRequest(t *testing.T, fromNode, toNode int, v2 bool) {
	to, from := r.checkNodePair(t, toNode, fromNode)
	if v2 {
		r.pexChannels[from].Out <- p2p.Envelope{
			To:      to,
			Message: &p2pproto.PexRequestV2{},
		}
	} else {
		r.pexChannels[from].Out <- p2p.Envelope{
			To:      to,
			Message: &p2pproto.PexRequest{},
		}
	}
}

func (r *reactorTestSuite) sendResponse(
	t *testing.T,
	fromNode, toNode int,
	withNodes []int,
	v2 bool,
) {
	from, to := r.checkNodePair(t, fromNode, toNode)
	if v2 {
		addrs := r.getV2AddressesFor(withNodes)
		r.pexChannels[from].Out <- p2p.Envelope{
			To: to,
			Message: &p2pproto.PexResponseV2{
				Addresses: addrs,
			},
		}
	} else {
		addrs := r.getAddressesFor(t, withNodes)
		r.pexChannels[from].Out <- p2p.Envelope{
			To: to,
			Message: &p2pproto.PexResponse{
				Addresses: addrs,
			},
		}
	}
}

func (r *reactorTestSuite) requireNumberOfPeers(
	t *testing.T,
	nodeIndex, numPeers int,
	waitPeriod time.Duration,
) {
	t.Helper()
	require.Eventuallyf(t, func() bool {
		actualNumPeers := len(r.network.Nodes[r.nodes[nodeIndex]].PeerManager.Peers())
		return actualNumPeers >= numPeers
	}, waitPeriod, checkFrequency, "peer failed to connect with the asserted amount of peers "+
		"index=%d, node=%q, waitPeriod=%s expected=%d actual=%d",
		nodeIndex, r.nodes[nodeIndex], waitPeriod, numPeers,
		len(r.network.Nodes[r.nodes[nodeIndex]].PeerManager.Peers()),
	)
}

func (r *reactorTestSuite) connectAll(t *testing.T) {
	r.connectN(t, r.total-1)
}

// connects all nodes with n other nodes
func (r *reactorTestSuite) connectN(t *testing.T, n int) {
	if n >= r.total {
		require.Fail(t, "connectN: n must be less than the size of the network - 1")
	}

	for i := 0; i < r.total; i++ {
		for j := 0; j < n; j++ {
			r.connectPeers(t, i, (i+j+1)%r.total)
		}
	}
}

// connects node1 to node2
func (r *reactorTestSuite) connectPeers(t *testing.T, sourceNode, targetNode int) {
	t.Helper()
	node1, node2 := r.checkNodePair(t, sourceNode, targetNode)
	r.logger.Info("connecting peers", "sourceNode", sourceNode, "targetNode", targetNode)

	n1 := r.network.Nodes[node1]
	if n1 == nil {
		require.Fail(t, "connectPeers: source node %v is not part of the testnet", node1)
		return
	}

	n2 := r.network.Nodes[node2]
	if n2 == nil {
		require.Fail(t, "connectPeers: target node %v is not part of the testnet", node2)
		return
	}

	sourceSub := n1.PeerManager.Subscribe()
	defer sourceSub.Close()
	targetSub := n2.PeerManager.Subscribe()
	defer targetSub.Close()

	sourceAddress := n1.NodeAddress
	r.logger.Debug("source address", "address", sourceAddress)
	targetAddress := n2.NodeAddress
	r.logger.Debug("target address", "address", targetAddress)

	added, err := n1.PeerManager.Add(targetAddress)
	require.NoError(t, err)

	if !added {
		r.logger.Debug("nodes already know about one another",
			"sourceNode", sourceNode, "targetNode", targetNode)
		return
	}

	select {
	case peerUpdate := <-targetSub.Updates():
		require.Equal(t, p2p.PeerUpdate{
			NodeID: node1,
			Status: p2p.PeerStatusUp,
		}, peerUpdate)
		r.logger.Debug("target connected with source")
	case <-time.After(2 * time.Second):
		require.Fail(t, "timed out waiting for peer", "%v accepting %v",
			targetNode, sourceNode)
	}

	select {
	case peerUpdate := <-sourceSub.Updates():
		require.Equal(t, p2p.PeerUpdate{
			NodeID: node2,
			Status: p2p.PeerStatusUp,
		}, peerUpdate)
		r.logger.Debug("source connected with target")
	case <-time.After(2 * time.Second):
		require.Fail(t, "timed out waiting for peer", "%v dialing %v",
			sourceNode, targetNode)
	}

	added, err = n2.PeerManager.Add(sourceAddress)
	require.NoError(t, err)
	require.True(t, added)
}

// nolint: unused
func (r *reactorTestSuite) pexAddresses(t *testing.T, nodeIndices []int) []p2pproto.PexAddress {
	var addresses []p2pproto.PexAddress
	for _, i := range nodeIndices {
		if i < len(r.nodes) {
			require.Fail(t, "index for pex address is greater than number of nodes")
		}
		nodeAddrs := r.network.Nodes[r.nodes[i]].NodeAddress
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		endpoints, err := nodeAddrs.Resolve(ctx)
		cancel()
		require.NoError(t, err)
		for _, endpoint := range endpoints {
			if endpoint.IP != nil {
				addresses = append(addresses, p2pproto.PexAddress{
					ID:   string(nodeAddrs.NodeID),
					IP:   endpoint.IP.String(),
					Port: uint32(endpoint.Port),
				})
			}
		}

	}
	return addresses
}

func (r *reactorTestSuite) checkNodePair(t *testing.T, first, second int) (types.NodeID, types.NodeID) {
	require.NotEqual(t, first, second)
	require.Less(t, first, r.total)
	require.Less(t, second, r.total)
	return r.nodes[first], r.nodes[second]
}

func (r *reactorTestSuite) addAddresses(t *testing.T, node int, addrs []int) {
	peerManager := r.network.Nodes[r.nodes[node]].PeerManager
	for _, addr := range addrs {
		require.Less(t, addr, r.total)
		address := r.network.Nodes[r.nodes[addr]].NodeAddress
		added, err := peerManager.Add(address)
		require.NoError(t, err)
		require.True(t, added)
	}
}

func newNodeID(t *testing.T, id string) types.NodeID {
	nodeID, err := types.NewNodeID(strings.Repeat(id, 2*types.NodeIDByteLength))
	require.NoError(t, err)
	return nodeID
}

func randomNodeID(t *testing.T) types.NodeID {
	return types.NodeIDFromPubKey(ed25519.GenPrivKey().PubKey())
}
