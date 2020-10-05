package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
)

// Tests that any initial state given in genesis has made it into the app.
func TestApp_InitialState(t *testing.T) {
	testNode(t, func(t *testing.T, node e2e.Node) {
		switch {
		case node.Mode == e2e.ModeSeed:
			return
		case len(node.Testnet.InitialState) == 0:
			return
		}

		client, err := node.Client()
		require.NoError(t, err)
		for k, v := range node.Testnet.InitialState {
			resp, err := client.ABCIQuery(ctx, "", []byte(k))
			require.NoError(t, err)
			assert.Equal(t, k, string(resp.Response.Key))
			assert.Equal(t, v, string(resp.Response.Value))
		}
	})
}
