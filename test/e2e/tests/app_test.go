package e2e_test

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/rpc/client/http"
	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
	"github.com/tendermint/tendermint/types"
)

const (
	randomSeed = 4827085738
)

// Tests that any initial state given in genesis has made it into the app.
func TestApp_InitialState(t *testing.T) {
	testNode(t, func(ctx context.Context, t *testing.T, node e2e.Node) {
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
	testNode(t, func(ctx context.Context, t *testing.T, node e2e.Node) {
		client, err := node.Client()
		require.NoError(t, err)
		info, err := client.ABCIInfo(ctx)
		require.NoError(t, err)
		require.NotEmpty(t, info.Response.LastBlockAppHash, "expected app to return app hash")

		status, err := client.Status(ctx)
		require.NoError(t, err)
		require.NotZero(t, status.SyncInfo.LatestBlockHeight)

		block, err := client.Block(ctx, &info.Response.LastBlockHeight)
		require.NoError(t, err)

		if info.Response.LastBlockHeight == block.Block.Height {
			require.Equal(t,
				fmt.Sprintf("%x", info.Response.LastBlockAppHash),
				fmt.Sprintf("%x", block.Block.AppHash.Bytes()),
				"app hash does not match last block's app hash")
		}

		require.True(t, status.SyncInfo.LatestBlockHeight >= info.Response.LastBlockHeight,
			"status out of sync with application")
	})
}

// Tests that the app and blockstore have and report the same height.
func TestApp_Height(t *testing.T) {
	testNode(t, func(ctx context.Context, t *testing.T, node e2e.Node) {
		client, err := node.Client()
		require.NoError(t, err)
		info, err := client.ABCIInfo(ctx)
		require.NoError(t, err)
		require.NotZero(t, info.Response.LastBlockHeight)

		status, err := client.Status(ctx)
		require.NoError(t, err)
		require.NotZero(t, status.SyncInfo.LatestBlockHeight)

		block, err := client.Block(ctx, &info.Response.LastBlockHeight)
		require.NoError(t, err)

		require.Equal(t, info.Response.LastBlockHeight, block.Block.Height)

		require.True(t, status.SyncInfo.LatestBlockHeight >= info.Response.LastBlockHeight,
			"status out of sync with application")
	})
}

// Tests that we can set a value and retrieve it.
func TestApp_Tx(t *testing.T) {
	type broadcastFunc func(context.Context, types.Tx) error

	testCases := []struct {
		Name        string
		WaitTime    time.Duration
		BroadcastTx func(client *http.HTTP) broadcastFunc
		ShouldSkip  bool
	}{
		{
			Name:     "Sync",
			WaitTime: time.Minute,
			BroadcastTx: func(client *http.HTTP) broadcastFunc {
				return func(ctx context.Context, tx types.Tx) error {
					_, err := client.BroadcastTxSync(ctx, tx)
					return err
				}
			},
		},
		{
			Name:     "Commit",
			WaitTime: 15 * time.Second,
			// TODO: turn this check back on if it can
			// return reliably. Currently these calls have
			// a hard timeout of 10s (server side
			// configured). The Sync check is probably
			// safe.
			ShouldSkip: true,
			BroadcastTx: func(client *http.HTTP) broadcastFunc {
				return func(ctx context.Context, tx types.Tx) error {
					_, err := client.BroadcastTxCommit(ctx, tx)
					return err
				}
			},
		},
		{
			Name:     "Async",
			WaitTime: 90 * time.Second,
			// TODO: turn this check back on if there's a
			// way to avoid failures in the case that the
			// transaction doesn't make it into the
			// mempool. (retries?)
			ShouldSkip: true,
			BroadcastTx: func(client *http.HTTP) broadcastFunc {
				return func(ctx context.Context, tx types.Tx) error {
					_, err := client.BroadcastTxAsync(ctx, tx)
					return err
				}
			},
		},
	}

	r := rand.New(rand.NewSource(randomSeed))
	for idx, test := range testCases {
		if test.ShouldSkip {
			continue
		}
		t.Run(test.Name, func(t *testing.T) {
			test := testCases[idx]
			testNode(t, func(ctx context.Context, t *testing.T, node e2e.Node) {
				client, err := node.Client()
				require.NoError(t, err)

				key := fmt.Sprintf("testapp-tx-%v", node.Name)
				value := tmrand.StrFromSource(r, 32)
				tx := types.Tx(fmt.Sprintf("%v=%v", key, value))

				require.NoError(t, test.BroadcastTx(client)(ctx, tx))

				hash := tx.Hash()

				require.Eventuallyf(t, func() bool {
					txResp, err := client.Tx(ctx, hash, false)
					return err == nil && bytes.Equal(txResp.Tx, tx)
				},
					test.WaitTime, // timeout
					time.Second,   // interval
					"submitted tx %X wasn't committed after %v",
					hash, test.WaitTime,
				)

				abciResp, err := client.ABCIQuery(ctx, "", []byte(key))
				require.NoError(t, err)
				assert.Equal(t, key, string(abciResp.Response.Key))
				assert.Equal(t, value, string(abciResp.Response.Value))
			})

		})

	}

}
