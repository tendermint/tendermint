package p2ptest

import (
	"math/rand"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
)

// StringMessage is a simple message containing a string-typed Value field.
type StringMessage = gogotypes.StringValue

// Network sets up an in-memory network that can be used for high-level P2P
// testing. It creates an arbitrary number of nodes that are connected to each
// other, and can open channels across all nodes with custom reactors.
type Network struct {
	Nodes         map[p2p.NodeID]*Node
	memoryNetwork *p2p.MemoryNetwork
}

// MakeNetwork creates a test network with the given number of nodes and
// connects them to each other.
func MakeNetwork(t *testing.T, nodes int) *Network {
	network := &Network{
		Nodes:         map[p2p.NodeID]*Node{},
		memoryNetwork: p2p.NewMemoryNetwork(log.TestingLogger()),
	}
	for i := 0; i < nodes; i++ {
		node := MakeNode(t, network)
		network.Nodes[node.NodeID] = node
	}

	// Set up a list of node addresses to dial, and a peer update subscription
	// for each node.
	dialQueue := []p2p.NodeAddress{}
	subs := map[p2p.NodeID]*p2p.PeerUpdates{}
	for _, node := range network.Nodes {
		dialQueue = append(dialQueue, node.NodeAddress)
		subs[node.NodeID] = node.PeerManager.Subscribe()
		defer subs[node.NodeID].Close()
	}

	// For each node, dial the nodes that it still doesn't have a connection to
	// (either inbound or outbound), and wait for both sides to confirm the
	// connection via the subscriptions.
	for i, sourceAddress := range dialQueue {
		sourceNode := network.Nodes[sourceAddress.NodeID]
		sourceSub := subs[sourceAddress.NodeID]
		for _, targetAddress := range dialQueue[i+1:] { // nodes <i already connected
			targetNode := network.Nodes[targetAddress.NodeID]
			targetSub := subs[targetAddress.NodeID]
			require.NoError(t, sourceNode.PeerManager.Add(targetAddress))

			select {
			case peerUpdate := <-sourceSub.Updates():
				require.Equal(t, p2p.PeerUpdate{
					NodeID: targetNode.NodeID,
					Status: p2p.PeerStatusUp,
				}, peerUpdate)
			case <-time.After(time.Second):
				require.Fail(t, "timed out waiting for %v to dial %v", sourceNode.NodeID, targetNode.NodeID)
			}

			select {
			case peerUpdate := <-targetSub.Updates():
				require.Equal(t, p2p.PeerUpdate{
					NodeID: sourceNode.NodeID,
					Status: p2p.PeerStatusUp,
				}, peerUpdate)
			case <-time.After(time.Second):
				require.Fail(t, "timed out waiting for %v to accept %v", targetNode.NodeID, sourceNode.NodeID)
			}

			// Add the address to the target as well, so it's able to dial the
			// source back if that's even necessary.
			require.NoError(t, targetNode.PeerManager.Add(sourceAddress))
		}
	}

	return network
}

// MakeChannelAtNode opens a channel across all nodes in the network, calling
// the given onPeers function for each node except the given node ID, and returns
// the channel at the given node ID.
func (n *Network) MakeChannelAtNode(
	t *testing.T,
	nodeID p2p.NodeID,
	chID p2p.ChannelID,
	messageType proto.Message,
	onPeers func(*testing.T, p2p.NodeID, *p2p.Channel),
) *p2p.Channel {

	// First create the channel on the "local" (specified) node.
	require.Contains(t, n.Nodes, nodeID)
	node := n.Nodes[nodeID]
	channel, err := node.Router.OpenChannel(chID, messageType)
	require.NoError(t, err)
	t.Cleanup(channel.Close)

	// Then create the channel on all "remote" (unspecified) peers,
	// calling onPeers with the channel.
	for _, peer := range n.Nodes {
		if peer.NodeID == nodeID {
			continue
		}

		channel, err := peer.Router.OpenChannel(chID, messageType)
		require.NoError(t, err)
		t.Cleanup(channel.Close)

		onPeers(t, peer.NodeID, channel)
	}

	return channel
}

// RandomNode returns a random node.
func (n *Network) RandomNode() *Node {
	nodes := make([]*Node, 0, len(n.Nodes))
	for _, node := range n.Nodes {
		nodes = append(nodes, node)
	}
	return nodes[rand.Intn(len(nodes))] // nolint:gosec
}

// Peers returns a node's peers (i.e. everyone except itself).
func (n *Network) Peers(id p2p.NodeID) []*Node {
	peers := make([]*Node, 0, len(n.Nodes)-1)
	for _, peer := range n.Nodes {
		if peer.NodeID != id {
			peers = append(peers, peer)
		}
	}
	return peers
}

// Node is a node in a TestNetwork, with a Router and a PeerManager.
type Node struct {
	NodeID      p2p.NodeID
	NodeInfo    p2p.NodeInfo
	NodeAddress p2p.NodeAddress
	PrivKey     crypto.PrivKey
	Router      *p2p.Router
	PeerManager *p2p.PeerManager
}

// NewNode creates a new TestNode.
func MakeNode(t *testing.T, network *Network) *Node {
	logger := log.TestingLogger()
	privKey := ed25519.GenPrivKey()
	nodeID := p2p.NodeIDFromPubKey(privKey.PubKey())
	nodeInfo := p2p.NodeInfo{
		NodeID:     nodeID,
		ListenAddr: "0.0.0.0:0", // FIXME: We have to fake this for now.
		Moniker:    string(nodeID),
	}

	transport := network.memoryNetwork.CreateTransport(nodeID)
	endpoints := transport.Endpoints()
	require.Len(t, endpoints, 1)

	peerManager, err := p2p.NewPeerManager(nodeID, dbm.NewMemDB(), p2p.PeerManagerOptions{})
	require.NoError(t, err)

	router, err := p2p.NewRouter(logger, nodeInfo, privKey, peerManager, []p2p.Transport{transport}, p2p.RouterOptions{})
	require.NoError(t, err)
	err = router.Start()
	require.NoError(t, err)

	t.Cleanup(func() {
		peerManager.Close()
		require.NoError(t, transport.Close())
		if router.IsRunning() {
			require.NoError(t, router.Stop())
		}
	})

	return &Node{
		NodeID:      nodeID,
		NodeInfo:    nodeInfo,
		NodeAddress: endpoints[0].NodeAddress(nodeID),
		PrivKey:     privKey,
		Router:      router,
		PeerManager: peerManager,
	}
}
