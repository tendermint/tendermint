package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
)

// Tests that all nodes have peered with each other.
func TestNet_Peers(t *testing.T) {
	testNode(t, func(t *testing.T, node e2e.Node) {
		client, err := node.Client()
		require.NoError(t, err)
		netInfo, err := client.NetInfo(ctx)
		require.NoError(t, err)

		require.Equal(t, len(node.Testnet.Nodes)-1, netInfo.NPeers)

		seen := map[string]bool{}
		for _, n := range node.Testnet.Nodes {
			seen[n.Name] = (n.Name == node.Name) // we've clearly seen ourself
		}
		for _, peerInfo := range netInfo.Peers {
			peer := node.Testnet.LookupNode(peerInfo.NodeInfo.Moniker)
			require.NotNil(t, peer, "unknown node %v", peerInfo.NodeInfo.Moniker)
			require.Equal(t, peer.IP.String(), peerInfo.RemoteIP,
				"unexpected IP address for peer %v", peer.Name)
			seen[peerInfo.NodeInfo.Moniker] = true
		}

		for name := range seen {
			require.True(t, seen[name], "node %v not peered with %v", node.Name, name)
		}
	})
}
