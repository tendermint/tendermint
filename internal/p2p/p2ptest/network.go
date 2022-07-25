package p2ptest

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

// Network sets up an in-memory network that can be used for high-level P2P
// testing. It creates an arbitrary number of nodes that are connected to each
// other, and can open channels across all nodes with custom reactors.
type Network struct {
	Nodes map[types.NodeID]*Node

	logger        log.Logger
	memoryNetwork *p2p.MemoryNetwork
	cancel        context.CancelFunc
}

// NetworkOptions is an argument structure to parameterize the
// MakeNetwork function.
type NetworkOptions struct {
	NumNodes   int
	BufferSize int
	NodeOpts   NodeOptions
}

type NodeOptions struct {
	MaxPeers     uint16
	MaxConnected uint16
}

func (opts *NetworkOptions) setDefaults() {
	if opts.BufferSize == 0 {
		opts.BufferSize = 1
	}
}

// MakeNetwork creates a test network with the given number of nodes and
// connects them to each other.
func MakeNetwork(ctx context.Context, t *testing.T, opts NetworkOptions) *Network {
	opts.setDefaults()
	logger := log.NewNopLogger()
	network := &Network{
		Nodes:         map[types.NodeID]*Node{},
		logger:        logger,
		memoryNetwork: p2p.NewMemoryNetwork(logger, opts.BufferSize),
	}

	for i := 0; i < opts.NumNodes; i++ {
		node := network.MakeNode(ctx, t, opts.NodeOpts)
		network.Nodes[node.NodeID] = node
	}

	return network
}

// Start starts the network by setting up a list of node addresses to dial in
// addition to creating a peer update subscription for each node. Finally, all
// nodes are connected to each other.
func (n *Network) Start(ctx context.Context, t *testing.T) {
	ctx, n.cancel = context.WithCancel(ctx)
	t.Cleanup(n.cancel)

	// Set up a list of node addresses to dial, and a peer update subscription
	// for each node.
	dialQueue := []p2p.NodeAddress{}
	subs := map[types.NodeID]*p2p.PeerUpdates{}
	subctx, subcancel := context.WithCancel(ctx)
	defer subcancel()
	for _, node := range n.Nodes {
		dialQueue = append(dialQueue, node.NodeAddress)
		subs[node.NodeID] = node.PeerManager.Subscribe(subctx)
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
			added, err := sourceNode.PeerManager.Add(targetAddress)
			require.NoError(t, err)
			require.True(t, added)

			select {
			case <-ctx.Done():
				require.Fail(t, "operation canceled")
			case peerUpdate := <-sourceSub.Updates():
				require.Equal(t, targetNode.NodeID, peerUpdate.NodeID)
				require.Equal(t, p2p.PeerStatusUp, peerUpdate.Status)
			case <-time.After(3 * time.Second):
				require.Fail(t, "timed out waiting for peer", "%v dialing %v",
					sourceNode.NodeID, targetNode.NodeID)
			}

			select {
			case <-ctx.Done():
				require.Fail(t, "operation canceled")
			case peerUpdate := <-targetSub.Updates():
				peerUpdate.Channels = nil
				require.Equal(t, p2p.PeerUpdate{
					NodeID: sourceNode.NodeID,
					Status: p2p.PeerStatusUp,
				}, peerUpdate)
			case <-time.After(3 * time.Second):
				require.Fail(t, "timed out waiting for peer", "%v accepting %v",
					targetNode.NodeID, sourceNode.NodeID)
			}

			// Add the address to the target as well, so it's able to dial the
			// source back if that's even necessary.
			added, err = targetNode.PeerManager.Add(sourceAddress)
			require.NoError(t, err)
			require.True(t, added)
		}
	}
}

// NodeIDs returns the network's node IDs.
func (n *Network) NodeIDs() []types.NodeID {
	ids := []types.NodeID{}
	for id := range n.Nodes {
		ids = append(ids, id)
	}
	return ids
}

// MakeChannels makes a channel on all nodes and returns them, automatically
// doing error checks and cleanups.
func (n *Network) MakeChannels(
	ctx context.Context,
	t *testing.T,
	chDesc *p2p.ChannelDescriptor,
) map[types.NodeID]p2p.Channel {
	channels := map[types.NodeID]p2p.Channel{}
	for _, node := range n.Nodes {
		channels[node.NodeID] = node.MakeChannel(ctx, t, chDesc)
	}
	return channels
}

// MakeChannelsNoCleanup makes a channel on all nodes and returns them,
// automatically doing error checks. The caller must ensure proper cleanup of
// all the channels.
func (n *Network) MakeChannelsNoCleanup(
	ctx context.Context,
	t *testing.T,
	chDesc *p2p.ChannelDescriptor,
) map[types.NodeID]p2p.Channel {
	channels := map[types.NodeID]p2p.Channel{}
	for _, node := range n.Nodes {
		channels[node.NodeID] = node.MakeChannelNoCleanup(ctx, t, chDesc)
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
func (n *Network) Peers(id types.NodeID) []*Node {
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
func (n *Network) Remove(ctx context.Context, t *testing.T, id types.NodeID) {
	require.Contains(t, n.Nodes, id)
	node := n.Nodes[id]
	delete(n.Nodes, id)

	subs := []*p2p.PeerUpdates{}
	subctx, subcancel := context.WithCancel(ctx)
	defer subcancel()
	for _, peer := range n.Nodes {
		sub := peer.PeerManager.Subscribe(subctx)
		subs = append(subs, sub)
	}

	require.NoError(t, node.Transport.Close())
	node.cancel()
	if node.Router.IsRunning() {
		node.Router.Stop()
		node.Router.Wait()
	}

	for _, sub := range subs {
		RequireUpdate(t, sub, p2p.PeerUpdate{
			NodeID: node.NodeID,
			Status: p2p.PeerStatusDown,
		})
	}
}

// Node is a node in a Network, with a Router and a PeerManager.
type Node struct {
	NodeID      types.NodeID
	NodeInfo    types.NodeInfo
	NodeAddress p2p.NodeAddress
	PrivKey     crypto.PrivKey
	Router      *p2p.Router
	PeerManager *p2p.PeerManager
	Transport   *p2p.MemoryTransport

	cancel context.CancelFunc
}

// MakeNode creates a new Node configured for the network with a
// running peer manager, but does not add it to the existing
// network. Callers are responsible for updating peering relationships.
func (n *Network) MakeNode(ctx context.Context, t *testing.T, opts NodeOptions) *Node {
	ctx, cancel := context.WithCancel(ctx)

	privKey := ed25519.GenPrivKey()
	nodeID := types.NodeIDFromPubKey(privKey.PubKey())
	nodeInfo := types.NodeInfo{
		NodeID:     nodeID,
		ListenAddr: "0.0.0.0:0", // FIXME: We have to fake this for now.
		Moniker:    string(nodeID),
	}

	transport := n.memoryNetwork.CreateTransport(nodeID)
	ep, err := transport.Endpoint()
	require.NoError(t, err)
	require.NotNil(t, ep, "transport not listening an endpoint")

	peerManager, err := p2p.NewPeerManager(nodeID, dbm.NewMemDB(), p2p.PeerManagerOptions{
		MinRetryTime:             10 * time.Millisecond,
		DisconnectCooldownPeriod: 10 * time.Millisecond,
		MaxRetryTime:             100 * time.Millisecond,
		RetryTimeJitter:          time.Millisecond,
		MaxPeers:                 opts.MaxPeers,
		MaxConnected:             opts.MaxConnected,
		Metrics:                  p2p.NopMetrics(),
	})
	require.NoError(t, err)

	router, err := p2p.NewRouter(
		n.logger,
		p2p.NopMetrics(),
		privKey,
		peerManager,
		func() *types.NodeInfo { return &nodeInfo },
		transport,
		ep,
		p2p.RouterOptions{},
	)

	require.NoError(t, err)
	require.NoError(t, router.Start(ctx))

	t.Cleanup(func() {
		if router.IsRunning() {
			router.Stop()
			router.Wait()
		}
		require.NoError(t, transport.Close())
		cancel()
	})

	return &Node{
		NodeID:      nodeID,
		NodeInfo:    nodeInfo,
		NodeAddress: ep.NodeAddress(nodeID),
		PrivKey:     privKey,
		Router:      router,
		PeerManager: peerManager,
		Transport:   transport,
		cancel:      cancel,
	}
}

// MakeChannel opens a channel, with automatic error handling and cleanup. On
// test cleanup, it also checks that the channel is empty, to make sure
// all expected messages have been asserted.
func (n *Node) MakeChannel(
	ctx context.Context,
	t *testing.T,
	chDesc *p2p.ChannelDescriptor,
) p2p.Channel {
	ctx, cancel := context.WithCancel(ctx)
	channel, err := n.Router.OpenChannel(ctx, chDesc)
	require.NoError(t, err)
	t.Cleanup(func() {
		RequireEmpty(ctx, t, channel)
		cancel()
	})
	return channel
}

// MakeChannelNoCleanup opens a channel, with automatic error handling. The
// caller must ensure proper cleanup of the channel.
func (n *Node) MakeChannelNoCleanup(
	ctx context.Context,
	t *testing.T,
	chDesc *p2p.ChannelDescriptor,
) p2p.Channel {
	channel, err := n.Router.OpenChannel(ctx, chDesc)
	require.NoError(t, err)
	return channel
}

// MakePeerUpdates opens a peer update subscription, with automatic cleanup.
// It checks that all updates have been consumed during cleanup.
func (n *Node) MakePeerUpdates(ctx context.Context, t *testing.T) *p2p.PeerUpdates {
	t.Helper()
	sub := n.PeerManager.Subscribe(ctx)
	t.Cleanup(func() {
		RequireNoUpdates(ctx, t, sub)
	})

	return sub
}

// MakePeerUpdatesNoRequireEmpty opens a peer update subscription, with automatic cleanup.
// It does *not* check that all updates have been consumed, but will
// close the update channel.
func (n *Node) MakePeerUpdatesNoRequireEmpty(ctx context.Context, t *testing.T) *p2p.PeerUpdates {
	return n.PeerManager.Subscribe(ctx)
}

func MakeChannelDesc(chID p2p.ChannelID) *p2p.ChannelDescriptor {
	return &p2p.ChannelDescriptor{
		ID:                  chID,
		MessageType:         &Message{},
		Priority:            5,
		SendQueueCapacity:   10,
		RecvMessageCapacity: 10,
	}
}
