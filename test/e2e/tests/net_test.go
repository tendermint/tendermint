package e2e_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
	"github.com/tendermint/tendermint/types"
)

// Tests that all nodes have peered with each other, regardless of discovery method.
func TestNet_Peers(t *testing.T) {
	testNode(t, func(ctx context.Context, t *testing.T, node e2e.Node) {
		client, err := node.Client()
		require.NoError(t, err)
		netInfo, err := client.NetInfo(ctx)
		require.NoError(t, err)

		// FIXME: https://github.com/tendermint/tendermint/issues/8848
		// We should be able to assert that we can discover all peers in a network
		expectedPeers := len(node.Testnet.Nodes)
		peers := make(map[string]*e2e.Node, 0)
		seen := map[string]bool{}
		for _, n := range node.Testnet.Nodes {
			// we never save light client addresses as they use RPC or ourselves
			if n.Mode == e2e.ModeLight || n.Name == node.Name {
				expectedPeers--
				continue
			}
			peers[string(types.NodeIDFromPubKey(n.NodeKey.PubKey()))] = n
			seen[n.Name] = false
		}

		require.GreaterOrEqual(t, netInfo.NPeers, expectedPeers-1,
			"node is not fully meshed with peers")

		for _, peerInfo := range netInfo.Peers {
			id := string(peerInfo.ID)
			peer, ok := peers[id]
			require.True(t, ok, "unknown node %v", id)
			require.Contains(t, peerInfo.URL, peer.IP.String(),
				"unexpected IP address for peer %v", id)
			seen[peer.Name] = true
		}

		// FIXME: https://github.com/tendermint/tendermint/issues/8848
		// We should be able to assert that we can discover all peers in a network
		// for name := range seen {
		// 	require.True(t, seen[name], "node %v not peered with %v", node.Name, name)
		// }
	})
}
