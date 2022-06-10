package e2e_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
)

// Tests that all nodes have peered with each other, regardless of discovery method.
func TestNet_Peers(t *testing.T) {
	testNode(t, func(ctx context.Context, t *testing.T, node e2e.Node) {
		client, err := node.Client()
		require.NoError(t, err)
		netInfo, err := client.NetInfo(ctx)
		require.NoError(t, err)

		expectedPeers := len(node.Testnet.Nodes) - 1
		seen := map[string]bool{}
		for _, n := range node.Testnet.Nodes {
			// we never save light client addresses as they use RPC
			if n.Mode == e2e.ModeLight {
				expectedPeers--
				continue
			}
			seen[n.Name] = (n.Name == node.Name) // we've clearly seen ourself
		}

		require.Equal(t, expectedPeers, netInfo.NPeers,
			"node is not fully meshed with peers")

		for _, peerInfo := range netInfo.Peers {
			id := peerInfo.ID
			peer := node.Testnet.LookupNode(string(id))
			require.NotNil(t, peer, "unknown node %v", id)
			require.Contains(t, peerInfo.URL, peer.IP.String(),
				"unexpected IP address for peer %v", id)
			seen[string(id)] = true
		}

		for name := range seen {
			require.True(t, seen[name], "node %v not peered with %v", node.Name, name)
		}
	})
}
