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
	checkFrequency = 200 * time.Millisecond
)

type reactorTestSuite struct {
	network *p2ptest.Network
	logger  log.Logger

	reactors   map[p2p.NodeID]*pex.ReactorV2
	pexChnnels map[p2p.NodeID]*p2p.Channel

	peerChans   map[p2p.NodeID]chan p2p.PeerUpdate
	peerUpdates map[p2p.NodeID]*p2p.PeerUpdates

	nodes []p2p.NodeID
	mocks []p2p.NodeID
}

func setup(t *testing.T, mockNodes, realNodes int, chBuf uint) *reactorTestSuite {
	t.Helper()

	totalNodes := mockNodes + realNodes

	rts := &reactorTestSuite{
		logger:      log.TestingLogger().With("testCase", t.Name()),
		network:     p2ptest.MakeNetwork(t, p2ptest.NetworkOptions{NumNodes: totalNodes}),
		reactors:    make(map[p2p.NodeID]*pex.ReactorV2, realNodes),
		pexChnnels:  make(map[p2p.NodeID]*p2p.Channel, totalNodes),
		peerChans:   make(map[p2p.NodeID]chan p2p.PeerUpdate, totalNodes),
		peerUpdates: make(map[p2p.NodeID]*p2p.PeerUpdates, totalNodes),
	}

	// NOTE: we don't assert that the channels get drained after stopping the
	// reactor
	rts.pexChnnels = rts.network.MakeChannelsNoCleanup(t, p2p.ChannelID(pex.PexChannel), new(proto.PexMessage), int(chBuf))

	idx := 0
	for nodeID := range rts.network.Nodes {
		rts.peerChans[nodeID] = make(chan p2p.PeerUpdate, int(chBuf))
		rts.peerUpdates[nodeID] = p2p.NewPeerUpdates(rts.peerChans[nodeID], int(chBuf))
		rts.network.Nodes[nodeID].PeerManager.Register(rts.peerUpdates[nodeID])

		if idx < realNodes {
			rts.reactors[nodeID] = pex.NewReactorV2(
				rts.logger.With("nodeID", nodeID),
				rts.network.Nodes[nodeID].PeerManager,
				rts.pexChnnels[nodeID],
				rts.peerUpdates[nodeID],
			)
			rts.nodes = append(rts.nodes, nodeID)
		} else {
			rts.mocks = append(rts.mocks, nodeID)
		}

		idx++
	}

	require.Len(t, rts.reactors, realNodes)

	t.Cleanup(func() {
		for nodeID, reactor := range rts.reactors {
			if reactor.IsRunning() {
				require.NoError(t, reactor.Stop())
				require.False(t, reactor.IsRunning())
			}
			rts.pexChnnels[nodeID].Close()
		}
		for _, nodeID := range rts.mocks {
			rts.pexChnnels[nodeID].Close()
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

func (r *reactorTestSuite) size() int {
	return len(r.network.Nodes)
}

func (r *reactorTestSuite) listenForRequest(t *testing.T, node p2p.NodeID, waitPeriod time.Duration) {
	select {
	case msg := <-r.pexChnnels[node].In:
		require.Equal(t, msg.Message, &proto.PexRequest{})

	case <-time.After(waitPeriod):
		require.Fail(t, "timed out listening for PEX request",
			"node=%q waitPeriod=%s", node, waitPeriod)
	}
}

func (r *reactorTestSuite) sendRequestAndWaitForResponse(
	t *testing.T,
	fromNode, toNode p2p.NodeID,
	waitPeriod time.Duration,
	addresses []proto.PexAddress,
) {
	r.pexChnnels[fromNode].Out <- p2p.Envelope{
		To:      toNode,
		Message: &proto.PexRequest{},
	}

	select {
	case msg := <-r.pexChnnels[fromNode].In:
		require.Equal(t, &proto.PexResponse{Addresses: addresses}, msg.Message)
	case <-time.After(waitPeriod):
		require.Fail(t, "timed out waiting for PEX response",
			"node=%q, waitPeriod=%s", toNode, waitPeriod)
	}
}

func (r *reactorTestSuite) requireNumberOfPeers(
	t *testing.T,
	nodeIndex, numPeers int,
	waitPeriod time.Duration,
) {
	nodeID := r.nodes[nodeIndex]
	actualNumPeers := 0
	iterations := int(waitPeriod / checkFrequency)
	for i := 0; i < iterations; i++ {
		connectedPeers := r.network.Nodes[r.nodes[nodeIndex]].PeerManager.Peers()
		actualNumPeers = len(connectedPeers)
		if len(connectedPeers) >= numPeers {
			return
		}
		time.Sleep(checkFrequency)
	}
	require.Fail(t, "peer failed to connect with the asserted amount of peers",
		"node=%q, waitPeriod=%s expected=%d actual=%d",
		nodeID, waitPeriod, numPeers, actualNumPeers)
}

func (r *reactorTestSuite) connectAll(t *testing.T) {
	r.connectN(t, r.size()-1)
}

// connects all nodes with n other nodes
func (r *reactorTestSuite) connectN(t *testing.T, n int) {
	if n >= r.size() {
		require.Fail(t, "connectN: n must be less than the size of the network - 1")
	}

	allNodes := append(r.mocks, r.nodes...)

	for i := 0; i < r.size()-1; i++ {
		for j := 0; j < n; j++ {
			r.connectPeers(t, allNodes[i], allNodes[(i+j+1)%r.size()])
		}
	}
}

// connects node1 to node2
func (r *reactorTestSuite) connectPeers(t *testing.T, node1, node2 p2p.NodeID) {
	t.Helper()
	r.logger.Debug("connecting peers", "node1", node1, "node2", node2)

	require.NotEqual(t, node1, node2, "cannot connect to self")

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
	r.logger.Info("source address", "address", sourceAddress)
	targetAddress := n2.NodeAddress
	r.logger.Info("target address", "address", targetAddress)

	added, err := n1.PeerManager.Add(targetAddress)
	require.NoError(t, err)
	require.True(t, added)

	select {
	case peerUpdate := <-targetSub.Updates():
		require.Equal(t, p2p.PeerUpdate{
			NodeID: node1,
			Status: p2p.PeerStatusUp,
		}, peerUpdate)
	case <-time.After(time.Second):
		require.Fail(t, "timed out waiting for peer", "%v accepting %v",
			node2, node1)
	}

	select {
	case peerUpdate := <-sourceSub.Updates():
		require.Equal(t, p2p.PeerUpdate{
			NodeID: node2,
			Status: p2p.PeerStatusUp,
		}, peerUpdate)
	case <-time.After(time.Second):
		require.Fail(t, "timed out waiting for peer", "%v dialing %v",
			node1, node2)
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

func TestReactorBasic(t *testing.T) {
	// start a network with one mock reactor and one "real" reactor
	testNet := setup(t, 1, 1, 1)
	testNet.start(t)
	testNet.connectAll(t)
	// assert that the mock node receives a request
	testNet.listenForRequest(t, testNet.mocks[0], 10*time.Second)
	// assert that when a mock node sends a request it receives a response
	testNet.sendRequestAndWaitForResponse(t, testNet.mocks[0], testNet.nodes[0], 10*time.Second, []proto.PexAddress(nil))
}

func TestReactorConnectFullNetwork(t *testing.T) {
	netSize := 5
	testNet := setup(t, 0, netSize, 1)
	testNet.start(t)
	// make every node only connected with one other node
	testNet.connectN(t, 1)
	// assert that all nodes add each other in the network
	for idx := 0; idx < len(testNet.nodes); idx++ {
		testNet.requireNumberOfPeers(t, idx, netSize-1, 10*time.Second)
	}
}

func TestReactorSendsRequestsTooOften(t *testing.T) {

}

func TestReactorSendsResponseWithoutRequest(t *testing.T) {

}

func TestReactorSendsTooManyPeers(t *testing.T) {

}

func TestReactorSendsResponsesWithLargeDelay(t *testing.T) {

}

func TestReactorSmallPeerStoreInALargeNetwork(t *testing.T) {

}

func TestReactorLargePeerStoreInASmallNetwork(t *testing.T) {

}

func TestReactorWithNetworkGrowth(t *testing.T) {

}
