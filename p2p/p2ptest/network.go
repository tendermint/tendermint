package p2ptest

import (
	"math/rand"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
)

// Network sets up an in-memory network that can be used for high-level P2P
// testing. It creates an arbitrary number of nodes that are connected to each
// other, and can open channels across all nodes with custom reactors.
type Network struct {
	Nodes map[p2p.NodeID]*Node

	logger        log.Logger
	memoryNetwork *p2p.MemoryNetwork
}

// NetworkOptions is an argument structure to parameterize the
// MakeNetwork function.
type NetworkOptions struct {
	NumNodes   int
	BufferSize int
}

func (opts *NetworkOptions) setDefaults() {
	if opts.BufferSize == 0 {
		opts.BufferSize = 1
	}
}

// MakeNetwork creates a test network with the given number of nodes and
// connects them to each other.
func MakeNetwork(t *testing.T, opts NetworkOptions) *Network {
	opts.setDefaults()
	logger := log.TestingLogger()
	network := &Network{
		Nodes:         map[p2p.NodeID]*Node{},
		logger:        logger,
		memoryNetwork: p2p.NewMemoryNetwork(logger, opts.BufferSize),
	}

	for i := 0; i < opts.NumNodes; i++ {
		node := network.MakeNode(t)
		network.Nodes[node.NodeID] = node
	}

	return network
}

// Start starts the network by setting up a list of node addresses to dial in
// addition to creating a peer update subscription for each node. Finally, all
// nodes are connected to each other.
func (n *Network) Start(t *testing.T) {
	// Set up a list of node addresses to dial, and a peer update subscription
	// for each node.
	dialQueue := []p2p.NodeAddress{}
	subs := map[p2p.NodeID]*p2p.PeerUpdates{}
	for _, node := range n.Nodes {
		dialQueue = append(dialQueue, node.NodeAddress)
		subs[node.NodeID] = node.PeerManager.Subscribe()
		defer subs[node.NodeID].Close()
	}

	// For each node, dial the nodes that it still doesn't have a connection to
	// (either inbound or outbound), and wait for both sides to confirm the
	// connection via the subscriptions.
	for i, sourceAddress := range dialQueue {
		sourceNode := n.Nodes[sourceAddress.NodeID]
		sourceSub := subs[sourceAddress.NodeID]

		for _, targetAddress := range dialQueue[i+1:] { // nodes <i already connected
			targetNode := n.Nodes[targetAddress.NodeID]
			targetSub := subs[targetAddress.NodeID]
			require.NoError(t, sourceNode.PeerManager.Add(targetAddress))

			select {
			case peerUpdate := <-sourceSub.Updates():
				require.Equal(t, p2p.PeerUpdate{
					NodeID: targetNode.NodeID,
					Status: p2p.PeerStatusUp,
				}, peerUpdate)
			case <-time.After(time.Second):
				require.Fail(t, "timed out waiting for peer", "%v dialing %v",
					sourceNode.NodeID, targetNode.NodeID)
			}

			select {
			case peerUpdate := <-targetSub.Updates():
				require.Equal(t, p2p.PeerUpdate{
					NodeID: sourceNode.NodeID,
					Status: p2p.PeerStatusUp,
				}, peerUpdate)
			case <-time.After(time.Second):
				require.Fail(t, "timed out waiting for peer", "%v accepting %v",
					targetNode.NodeID, sourceNode.NodeID)
			}

			// Add the address to the target as well, so it's able to dial the
			// source back if that's even necessary.
			require.NoError(t, targetNode.PeerManager.Add(sourceAddress))
		}
	}
}

// NodeIDs returns the network's node IDs.
func (n *Network) NodeIDs() []p2p.NodeID {
	ids := []p2p.NodeID{}
	for id := range n.Nodes {
		ids = append(ids, id)
	}
	return ids
}

// MakeChannels makes a channel on all nodes and returns them, automatically
// doing error checks and cleanups.
func (n *Network) MakeChannels(
	t *testing.T,
	chID p2p.ChannelID,
	messageType proto.Message,
	size int,
) map[p2p.NodeID]*p2p.Channel {
	channels := map[p2p.NodeID]*p2p.Channel{}
	for _, node := range n.Nodes {
		channels[node.NodeID] = node.MakeChannel(t, chID, messageType, size)
	}
	return channels
}

// MakeChannelsNoCleanup makes a channel on all nodes and returns them,
// automatically doing error checks. The caller must ensure proper cleanup of
// all the channels.
func (n *Network) MakeChannelsNoCleanup(
	t *testing.T,
	chID p2p.ChannelID,
	messageType proto.Message,
	size int,
) map[p2p.NodeID]*p2p.Channel {
	channels := map[p2p.NodeID]*p2p.Channel{}
	for _, node := range n.Nodes {
		channels[node.NodeID] = node.MakeChannelNoCleanup(t, chID, messageType, size)
	}
	return channels
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

// Remove removes a node from the network, stopping it and waiting for all other
// nodes to pick up the disconnection.
func (n *Network) Remove(t *testing.T, id p2p.NodeID) {
	require.Contains(t, n.Nodes, id)
	node := n.Nodes[id]
	delete(n.Nodes, id)

	subs := []*p2p.PeerUpdates{}
	for _, peer := range n.Nodes {
		sub := peer.PeerManager.Subscribe()
		defer sub.Close()
		subs = append(subs, sub)
	}

	require.NoError(t, node.Transport.Close())
	if node.Router.IsRunning() {
		require.NoError(t, node.Router.Stop())
	}
	node.PeerManager.Close()

	for _, sub := range subs {
		RequireUpdate(t, sub, p2p.PeerUpdate{
			NodeID: node.NodeID,
			Status: p2p.PeerStatusDown,
		})
	}
}

// Node is a node in a Network, with a Router and a PeerManager.
type Node struct {
	NodeID      p2p.NodeID
	NodeInfo    p2p.NodeInfo
	NodeAddress p2p.NodeAddress
	PrivKey     crypto.PrivKey
	Router      *p2p.Router
	PeerManager *p2p.PeerManager
	Transport   *p2p.MemoryTransport
}

// MakeNode creates a new Node configured for the network with a
// running peer manager, but does not add it to the existing
// network. Callers are responsible for updating peering relationships.
func (n *Network) MakeNode(t *testing.T) *Node {
	privKey := ed25519.GenPrivKey()
	nodeID := p2p.NodeIDFromPubKey(privKey.PubKey())
	nodeInfo := p2p.NodeInfo{
		NodeID:     nodeID,
		ListenAddr: "0.0.0.0:0", // FIXME: We have to fake this for now.
		Moniker:    string(nodeID),
	}

	transport := n.memoryNetwork.CreateTransport(nodeID)
	require.Len(t, transport.Endpoints(), 1, "transport not listening on 1 endpoint")

	peerManager, err := p2p.NewPeerManager(nodeID, dbm.NewMemDB(), p2p.PeerManagerOptions{
		MinRetryTime:    10 * time.Millisecond,
		MaxRetryTime:    100 * time.Millisecond,
		RetryTimeJitter: time.Millisecond,
	})
	require.NoError(t, err)

	router, err := p2p.NewRouter(
		n.logger,
		p2p.NopMetrics(),
		nodeInfo,
		privKey,
		peerManager,
		[]p2p.Transport{transport},
		p2p.RouterOptions{},
	)
	require.NoError(t, err)
	require.NoError(t, router.Start())

	t.Cleanup(func() {
		if router.IsRunning() {
			require.NoError(t, router.Stop())
		}
		peerManager.Close()
		require.NoError(t, transport.Close())
	})

	return &Node{
		NodeID:      nodeID,
		NodeInfo:    nodeInfo,
		NodeAddress: transport.Endpoints()[0].NodeAddress(nodeID),
		PrivKey:     privKey,
		Router:      router,
		PeerManager: peerManager,
		Transport:   transport,
	}
}

// MakeChannel opens a channel, with automatic error handling and cleanup. On
// test cleanup, it also checks that the channel is empty, to make sure
// all expected messages have been asserted.
func (n *Node) MakeChannel(t *testing.T, chID p2p.ChannelID, messageType proto.Message, size int) *p2p.Channel {
	channel, err := n.Router.OpenChannel(chID, messageType, size)
	require.NoError(t, err)
	t.Cleanup(func() {
		RequireEmpty(t, channel)
		channel.Close()
	})
	return channel
}

// MakeChannelNoCleanup opens a channel, with automatic error handling. The
// caller must ensure proper cleanup of the channel.
func (n *Node) MakeChannelNoCleanup(
	t *testing.T,
	chID p2p.ChannelID,
	messageType proto.Message,
	size int,
) *p2p.Channel {

	channel, err := n.Router.OpenChannel(chID, messageType, size)
	require.NoError(t, err)
	return channel
}

// MakePeerUpdates opens a peer update subscription, with automatic cleanup.
// It checks that all updates have been consumed during cleanup.
func (n *Node) MakePeerUpdates(t *testing.T) *p2p.PeerUpdates {
	t.Helper()
	sub := n.PeerManager.Subscribe()
	t.Cleanup(func() {
		t.Helper()
		RequireNoUpdates(t, sub)
		sub.Close()
	})

	return sub
}

// MakePeerUpdatesNoRequireEmpty opens a peer update subscription, with automatic cleanup.
// It does *not* check that all updates have been consumed, but will
// close the update channel.
func (n *Node) MakePeerUpdatesNoRequireEmpty(t *testing.T) *p2p.PeerUpdates {
	sub := n.PeerManager.Subscribe()
	t.Cleanup(func() {
		sub.Close()
	})

	return sub
}
