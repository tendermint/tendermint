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

type reactorTestSuite struct {
	network *p2ptest.Network
	logger  log.Logger

	reactors   map[p2p.NodeID]*pex.ReactorV2
	pexChnnels map[p2p.NodeID]*p2p.Channel

	peerChans   map[p2p.NodeID]chan p2p.PeerUpdate
	peerUpdates map[p2p.NodeID]*p2p.PeerUpdates

	nodes []p2p.NodeID
}

func setup(t *testing.T, numNodes int, chBuf uint) *reactorTestSuite {
	t.Helper()

	rts := &reactorTestSuite{
		logger:      log.TestingLogger().With("testCase", t.Name()),
		network:     p2ptest.MakeNetwork(t, p2ptest.NetworkOptions{NumNodes: numNodes}),
		reactors:    make(map[p2p.NodeID]*pex.ReactorV2, numNodes),
		pexChnnels:  make(map[p2p.NodeID]*p2p.Channel, numNodes),
		peerChans:   make(map[p2p.NodeID]chan p2p.PeerUpdate, numNodes),
		peerUpdates: make(map[p2p.NodeID]*p2p.PeerUpdates, numNodes),
	}

	rts.pexChnnels = rts.network.MakeChannels(t, p2p.ChannelID(pex.PexChannel), new(proto.PexMessage), int(chBuf))

	for nodeID := range rts.network.Nodes {
		rts.peerChans[nodeID] = make(chan p2p.PeerUpdate)
		rts.peerUpdates[nodeID] = p2p.NewPeerUpdates(rts.peerChans[nodeID], 1)
		rts.network.Nodes[nodeID].PeerManager.Register(rts.peerUpdates[nodeID])

		rts.reactors[nodeID] = pex.NewReactorV2(
			rts.logger.With("nodeID", nodeID),
			rts.network.Nodes[nodeID].PeerManager,
			rts.pexChnnels[nodeID],
			rts.peerUpdates[nodeID],
		)

		rts.nodes = append(rts.nodes, nodeID)

	}
	require.Len(t, rts.reactors, numNodes)

	return rts
}

// starts up the pex reactors for each node
func (r *reactorTestSuite) start(t *testing.T) {
	for _, reactor := range r.reactors {
		require.NoError(t, reactor.Start())
		require.True(t, reactor.IsRunning())
	}

	t.Cleanup(func() {
		for _, reactor := range r.reactors {
			if reactor.IsRunning() {
				require.NoError(t, reactor.Stop())
				require.False(t, reactor.IsRunning())
			}
		}
	})
}

func (r *reactorTestSuite) listenForRequest(t *testing.T, node p2p.NodeID, waitPeriod time.Duration) {
	select {
	case msg := <-r.pexChnnels[node].In:
		require.Equal(t, msg.Message, &proto.PexRequest{})

	case <-time.After(waitPeriod):
		require.Fail(t, "timed out listening for PEX request", "node", node, "wait")
	}
}

func (r *reactorTestSuite) waitForResponse(t *testing.T, node p2p.NodeID, waitPeriod time.Duration, addresses []proto.PexAddress) {
	select {
	case msg := <-r.pexChnnels[node].In:
		require.Equal(t, msg.Message, &proto.PexResponse{Addresses: addresses})

	case <-time.After(waitPeriod):
		require.Fail(t, "timed out listening for PEX request", "node", node, "wait")
	}
}

func (r *reactorTestSuite) connectAll(t *testing.T) {
	r.connectN(t, len(r.nodes)-1)
}

// connects all nodes with n other nodes
func (r *reactorTestSuite) connectN(t *testing.T, n int) {
	if n >= len(r.nodes) {
		require.Fail(t, "connectN: n must be less than the size of the network - 1")
	}

	for i := 0; i < len(r.nodes)-1; i++ {
		for j := 0; j < n; j++ {
			r.connectPeers(t, r.nodes[i], r.nodes[(i+j+1)%len(r.nodes)])
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

	sourceAddress := r.network.Nodes[node1].NodeAddress
	targetAddress := r.network.Nodes[node2].NodeAddress

	added, err := n1.PeerManager.Add(targetAddress)
	require.NoError(t, err)
	require.True(t, added)

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

	added, err = n2.PeerManager.Add(sourceAddress)
	require.NoError(t, err)
	require.True(t, added)
}

func (r *reactorTestSuite) pexAddresses(t *testing.T, nodeIndices []int) []proto.PexAddress {
	var addresses []proto.PexAddress
	for _, i := range nodeIndices {
		if i < len(r.nodes) {
			require.Fail(t, "index for pex address is greater than number of nodes")
		}
		nodeAddrs := r.network.Nodes[r.nodes[i]].NodeAddress
		ctx, cancel := context.WithTimeout(context.Background(), 3 * time.Second)
		endpoints, err := nodeAddrs.Resolve(ctx)
		cancel()
		require.NoError(t, err)
		for _, endpoint := range endpoints {
			if endpoint.IP != nil {
				addresses = append(addresses, proto.PexAddress{
					ID: string(nodeAddrs.NodeID),
					IP: endpoint.IP.String(),
					Port: uint32(endpoint.Port),
				})
			}
		}

	}
	return addresses
}

func TestPEXBasic(t *testing.T) {
	testNet := setup(t, 2, 1)
	testNet.start(t)
	testNet.connectAll(t)
	// node 0 receives a request
	testNet.listenForRequest(t, testNet.nodes[0], time.Second)
	// node 1 receives a response (which contains no addresses lol because who
	// else is in the network)
	testNet.waitForResponse(t, testNet.nodes[1], time.Second, []proto.PexAddress{})
}

func TestPEXConnectFullNetwork(t *testing.T) { 

}

func TestPEXSendsRequestsTooOften(t *testing.T) {

}

func TestPEXSendsResponseWithoutRequest(t *testing.T) {

}

func TestPEXSendsTooManyPeers(t *testing.T) {

}

func TestPEXSendsResponsesWithLargeDelay(t *testing.T) {

}

func TestPEXSmallPeerStoreInALargeNetwork(t *testing.T) {

}

func TestPEXLargePeerStoreInASmallNetwork(t *testing.T) {

}

func TestPEXWithNetworkGrowth(t *testing.T) {

}
