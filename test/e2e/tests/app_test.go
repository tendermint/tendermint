package e2e_test

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
	"github.com/tendermint/tendermint/types"
)

// Tests that any initial state given in genesis has made it into the app.
func TestApp_InitialState(t *testing.T) {
	testNode(t, func(t *testing.T, node e2e.Node) {
		if node.Stateless() {
			return
		}
		if len(node.Testnet.InitialState) == 0 {
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

// Tests that the app hash (as reported by the app) matches the last
// block and the node sync status.
func TestApp_Hash(t *testing.T) {
	testNode(t, func(t *testing.T, node e2e.Node) {
		if node.Mode == e2e.ModeSeed {
			return
		}

		client, err := node.Client()
		require.NoError(t, err)
		info, err := client.ABCIInfo(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, info.Response.LastBlockAppHash, "expected app to return app hash")

		block, err := client.Block(ctx, nil)
		require.NoError(t, err)
		require.EqualValues(t, info.Response.LastBlockAppHash, block.Block.AppHash,
			"app hash does not match last block's app hash")

		status, err := client.Status(ctx)
		require.NoError(t, err)
		require.EqualValues(t, info.Response.LastBlockAppHash, status.SyncInfo.LatestAppHash,
			"app hash does not match node status")
	})
}

// Tests that we can set a value and retrieve it.
func TestApp_Tx(t *testing.T) {
	testNode(t, func(t *testing.T, node e2e.Node) {
		if node.Mode == e2e.ModeSeed {
			return
		}

		client, err := node.Client()
		require.NoError(t, err)

		// Generate a random value, to prevent duplicate tx errors when
		// manually running the test multiple times for a testnet.
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		bz := make([]byte, 32)
		_, err = r.Read(bz)
		require.NoError(t, err)

		key := fmt.Sprintf("testapp-tx-%v", node.Name)
		value := fmt.Sprintf("%x", bz)
		tx := types.Tx(fmt.Sprintf("%v=%v", key, value))

		_, err = client.BroadcastTxSync(ctx, tx)
		require.NoError(t, err)

		hash := tx.Hash()
		waitTime := 30 * time.Second
		require.Eventuallyf(t, func() bool {
			txResp, err := client.Tx(ctx, hash, false)
			return err == nil && bytes.Equal(txResp.Tx, tx)
		}, waitTime, time.Second,
			"submitted tx wasn't committed after %v", waitTime,
		)

		// NOTE: we don't test abci query of the light client
		if node.Mode == e2e.ModeLight {
			return
		}

		abciResp, err := client.ABCIQuery(ctx, "", []byte(key))
		require.NoError(t, err)
		assert.Equal(t, key, string(abciResp.Response.Key))
		assert.Equal(t, value, string(abciResp.Response.Value))

	})
}
