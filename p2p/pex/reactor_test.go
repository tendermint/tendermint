package pex_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/p2ptest"
	"github.com/tendermint/tendermint/p2p/pex"
	proto "github.com/tendermint/tendermint/proto/tendermint/p2p"
)

const (
	checkFrequency    = 200 * time.Millisecond
	defaultBufferSize = 2
	defaultWait       = 10 * time.Second

	firstNode  = 0
	secondNode = 1
	thirdNode  = 2
	fourthNode = 3
)

func TestReactorBasic(t *testing.T) {
	// start a network with one mock reactor and one "real" reactor
	testNet := setup(t, testOptions{
		MockNodes:  1,
		TotalNodes: 2,
	})
	testNet.connectAll(t)
	testNet.start(t)

	// assert that the mock node receives a request from the real node
	testNet.listenForRequest(t, secondNode, firstNode, defaultWait)

	// assert that when a mock node sends a request it receives a response (and
	// the correct one)
	testNet.sendRequest(t, firstNode, secondNode, true)
	testNet.listenForResponse(t, secondNode, firstNode, defaultWait, []proto.PexAddressV2(nil))
}

func TestReactorConnectFullNetwork(t *testing.T) {
	netSize := 10
	testNet := setup(t, testOptions{
		TotalNodes: netSize,
	})

	// make every node be only connected with one other node (it actually ends up
	// being two because of two way connections but oh well)
	testNet.connectN(t, 1)
	testNet.start(t)

	// assert that all nodes add each other in the network
	for idx := 0; idx < len(testNet.nodes); idx++ {
		testNet.requireNumberOfPeers(t, idx, netSize-1, defaultWait)
	}
}

func TestReactorSendsRequestsTooOften(t *testing.T) {
	testNet := setup(t, testOptions{
		MockNodes:  1,
		TotalNodes: 3,
	})
	testNet.connectAll(t)
	testNet.start(t)

	// firstNode sends two requests to the secondNode
	testNet.sendRequest(t, firstNode, secondNode, true)
	testNet.sendRequest(t, firstNode, secondNode, true)

	// assert that the secondNode evicts the first node (although they reconnect
	// straight away again)
	testNet.listenForPeerUpdate(t, secondNode, firstNode, p2p.PeerStatusDown, defaultWait)

	// firstNode should still receive the address of the thirdNode by the secondNode
	expectedAddrs := testNet.getV2AddressesFor([]int{thirdNode})
	testNet.listenForResponse(t, secondNode, firstNode, defaultWait, expectedAddrs)

}

func TestReactorSendsResponseWithoutRequest(t *testing.T) {
	testNet := setup(t, testOptions{
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
	testNet.listenForPeerUpdate(t, secondNode, firstNode, p2p.PeerStatusDown, defaultWait)
}

func TestReactorNeverSendsTooManyPeers(t *testing.T) {
	testNet := setup(t, testOptions{
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
	testNet.pingAndlistenForNAddresses(t, secondNode, firstNode, defaultWait, 100)
}

func TestReactorErrorsOnReceivingTooManyPeers(t *testing.T) {
	testNet := setup(t, testOptions{
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

	// now we send a response with more than 100 peers
	testNet.sendResponse(t, firstNode, secondNode, nodes, true)
	// secondNode should evict the firstNode
	testNet.listenForPeerUpdate(t, secondNode, firstNode, p2p.PeerStatusDown, defaultWait)
}

func TestReactorSmallPeerStoreInALargeNetwork(t *testing.T) {
	testNet := setup(t, testOptions{
		TotalNodes:   20,
		MaxPeers:     10,
		MaxConnected: 10,
		BufferSize:   10,
	})
	testNet.connectN(t, 1)
	testNet.start(t)

	// test that all nodes reach full capacity
	for _, nodeID := range testNet.nodes {
		require.Eventually(t, func() bool {
			// nolint:scopelint
			return testNet.network.Nodes[nodeID].PeerManager.PeerRatio() > 0
		}, defaultWait, checkFrequency)
	}

	// once a node is at full capacity, it shouldn't send messages as often
	for idx, nodeID := range testNet.nodes {
		if idx >= len(testNet.mocks) {
			require.Eventually(t, func() bool {
				// nolint:scopelint
				reactor := testNet.reactors[nodeID]
				return reactor.NextRequestTime().After(time.Now().Add(5 * time.Minute))
			}, defaultWait, checkFrequency)
		}
	}
}

func TestReactorLargePeerStoreInASmallNetwork(t *testing.T) {
	testNet := setup(t, testOptions{
		TotalNodes:   20,
		MaxPeers:     100,
		MaxConnected: 100,
		BufferSize:   10,
	})
	testNet.connectN(t, 1)
	testNet.start(t)

	// assert that all nodes add each other in the network
	for idx := 0; idx < len(testNet.nodes); idx++ {
		testNet.requireNumberOfPeers(t, idx, len(testNet.nodes)-1, defaultWait)
	}
}

func TestReactorWithNetworkGrowth(t *testing.T) {
	testNet := setup(t, testOptions{
		TotalNodes: 10,
		BufferSize: 10,
	})
	testNet.connectAll(t)
	testNet.start(t)

	// assert that all nodes add each other in the network
	for idx := 0; idx < len(testNet.nodes); idx++ {
		testNet.requireNumberOfPeers(t, idx, len(testNet.nodes)-1, defaultWait)
	}

	// now we inject 10 more nodes
	testNet.addNodes(t, 10)
	for i := 10; i < testNet.total; i++ {
		node := testNet.nodes[i]
		require.NoError(t, testNet.reactors[node].Start())
		require.True(t, testNet.reactors[node].IsRunning())
		// we connect all new nodes to a single entry point and check that the
		// node can distribute the addresses to all the others
		testNet.connectPeers(t, 0, i)
	}
	require.Len(t, testNet.reactors, 20)

	// assert that all nodes add each other in the network
	for idx := 0; idx < len(testNet.nodes); idx++ {
		testNet.requireNumberOfPeers(t, idx, len(testNet.nodes)-1, defaultWait)
	}
}

func TestReactorIntegrationWithLegacyHandleRequest(t *testing.T) {
	testNet := setup(t, testOptions{
		MockNodes:  1,
		TotalNodes: 3,
	})
	testNet.connectAll(t)
	testNet.start(t)
	t.Log(testNet.nodes)

	// mock node sends a V1 Pex message to the second node
	testNet.sendRequest(t, firstNode, secondNode, false)
	addrs := testNet.getAddressesFor(t, []int{thirdNode})
	testNet.listenForLegacyResponse(t, secondNode, firstNode, defaultWait, addrs)
}

func TestReactorIntegrationWithLegacyHandleResponse(t *testing.T) {
	testNet := setup(t, testOptions{
		MockNodes:  1,
		TotalNodes: 4,
		BufferSize: 4,
	})
	testNet.connectPeers(t, firstNode, secondNode)
	testNet.connectPeers(t, firstNode, thirdNode)
	testNet.connectPeers(t, firstNode, fourthNode)
	testNet.start(t)

	testNet.listenForRequest(t, secondNode, firstNode, defaultWait)
	// send a v1 response instead
	testNet.sendResponse(t, firstNode, secondNode, []int{thirdNode, fourthNode}, false)
	testNet.requireNumberOfPeers(t, secondNode, len(testNet.nodes)-1, defaultWait)
}

type reactorTestSuite struct {
	network *p2ptest.Network
	logger  log.Logger

	reactors    map[p2p.NodeID]*pex.ReactorV2
	pexChannels map[p2p.NodeID]*p2p.Channel

	peerChans   map[p2p.NodeID]chan p2p.PeerUpdate
	peerUpdates map[p2p.NodeID]*p2p.PeerUpdates

	nodes []p2p.NodeID
	mocks []p2p.NodeID
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
func setup(t *testing.T, opts testOptions) *reactorTestSuite {
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
		reactors:    make(map[p2p.NodeID]*pex.ReactorV2, realNodes),
		pexChannels: make(map[p2p.NodeID]*p2p.Channel, opts.TotalNodes),
		peerChans:   make(map[p2p.NodeID]chan p2p.PeerUpdate, opts.TotalNodes),
		peerUpdates: make(map[p2p.NodeID]*p2p.PeerUpdates, opts.TotalNodes),
		total:       opts.TotalNodes,
		opts:        opts,
	}

	// NOTE: we don't assert that the channels get drained after stopping the
	// reactor
	rts.pexChannels = rts.network.MakeChannelsNoCleanup(
		t, p2p.ChannelID(pex.PexChannel), new(proto.PexMessage), chBuf,
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
			t, p2p.ChannelID(pex.PexChannel), new(proto.PexMessage), r.opts.BufferSize,
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
	node p2p.NodeID,
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
		_, ok := msg.Message.(*proto.PexRequestV2)
		return ok && msg.From == from
	}
	assertion := func(t *testing.T, msg p2p.Envelope) bool {
		require.Equal(t, &proto.PexRequestV2{}, msg.Message)
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
		_, ok := msg.Message.(*proto.PexResponseV2)
		return ok && msg.From == from
	}
	assertion := func(t *testing.T, msg p2p.Envelope) bool {
		m, ok := msg.Message.(*proto.PexResponseV2)
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
	addresses []proto.PexAddressV2,
) {
	r.logger.Info("Listening for response", "from", fromNode, "to", toNode)
	to, from := r.checkNodePair(t, toNode, fromNode)
	conditional := func(msg p2p.Envelope) bool {
		_, ok := msg.Message.(*proto.PexResponseV2)
		r.logger.Info("message", msg, "ok", ok)
		return ok && msg.From == from
	}
	assertion := func(t *testing.T, msg p2p.Envelope) bool {
		require.Equal(t, &proto.PexResponseV2{Addresses: addresses}, msg.Message)
		return true
	}
	r.listenFor(t, to, conditional, assertion, waitPeriod)
}

func (r *reactorTestSuite) listenForLegacyResponse(
	t *testing.T,
	fromNode, toNode int,
	waitPeriod time.Duration,
	addresses []proto.PexAddress,
) {
	r.logger.Info("Listening for response", "from", fromNode, "to", toNode)
	to, from := r.checkNodePair(t, toNode, fromNode)
	conditional := func(msg p2p.Envelope) bool {
		_, ok := msg.Message.(*proto.PexResponse)
		return ok && msg.From == from
	}
	assertion := func(t *testing.T, msg p2p.Envelope) bool {
		require.Equal(t, &proto.PexResponse{Addresses: addresses}, msg.Message)
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

func (r *reactorTestSuite) getV2AddressesFor(nodes []int) []proto.PexAddressV2 {
	addresses := make([]proto.PexAddressV2, len(nodes))
	for idx, node := range nodes {
		nodeID := r.nodes[node]
		addresses[idx] = proto.PexAddressV2{
			URL: r.network.Nodes[nodeID].NodeAddress.String(),
		}
	}
	return addresses
}

func (r *reactorTestSuite) getAddressesFor(t *testing.T, nodes []int) []proto.PexAddress {
	addresses := make([]proto.PexAddress, len(nodes))
	for idx, node := range nodes {
		nodeID := r.nodes[node]
		nodeAddrs := r.network.Nodes[nodeID].NodeAddress
		endpoints, err := nodeAddrs.Resolve(context.Background())
		require.NoError(t, err)
		require.Len(t, endpoints, 1)
		addresses[idx] = proto.PexAddress{
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
			Message: &proto.PexRequestV2{},
		}
	} else {
		r.pexChannels[from].Out <- p2p.Envelope{
			To:      to,
			Message: &proto.PexRequest{},
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
			Message: &proto.PexResponseV2{
				Addresses: addrs,
			},
		}
	} else {
		addrs := r.getAddressesFor(t, withNodes)
		r.pexChannels[from].Out <- p2p.Envelope{
			To: to,
			Message: &proto.PexResponse{
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
	case <-time.After(time.Second):
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
	case <-time.After(time.Second):
		require.Fail(t, "timed out waiting for peer", "%v dialing %v",
			sourceNode, targetNode)
	}

	added, err = n2.PeerManager.Add(sourceAddress)
	require.NoError(t, err)
	require.True(t, added)
}

// nolint: unused
func (r *reactorTestSuite) pexAddresses(t *testing.T, nodeIndices []int) []proto.PexAddress {
	var addresses []proto.PexAddress
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
				addresses = append(addresses, proto.PexAddress{
					ID:   string(nodeAddrs.NodeID),
					IP:   endpoint.IP.String(),
					Port: uint32(endpoint.Port),
				})
			}
		}

	}
	return addresses
}

func (r *reactorTestSuite) checkNodePair(t *testing.T, first, second int) (p2p.NodeID, p2p.NodeID) {
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
