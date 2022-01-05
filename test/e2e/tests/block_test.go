package e2e_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
)

// Tests that block headers are identical across nodes where present.
func TestBlock_Header(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	blocks := fetchBlockChain(ctx, t)
	testNode(t, func(ctx context.Context, t *testing.T, node e2e.Node) {
		client, err := node.Client()
		require.NoError(t, err)
		status, err := client.Status(ctx)
		require.NoError(t, err)

		first := status.SyncInfo.EarliestBlockHeight
		last := status.SyncInfo.LatestBlockHeight
		if node.RetainBlocks > 0 {
			first++ // avoid race conditions with block pruning
		}

		for _, block := range blocks {
			if block.Header.Height < first {
				continue
			}
			// the first blocks after state sync come from the backfill process
			// and are therefore not complete
			if node.StateSync != e2e.StateSyncDisabled && block.Header.Height <= first+e2e.EvidenceAgeHeight+1 {
				continue
			}
			if block.Header.Height > last {
				break
			}
			resp, err := client.Block(ctx, &block.Header.Height)
			require.NoError(t, err)

			require.Equal(t, block, resp.Block,
				"block mismatch for height %d", block.Header.Height)

			require.NoError(t, resp.Block.ValidateBasic(),
				"block at height %d is invalid", block.Header.Height)
		}
	})
}

// Tests that the node contains the expected block range.
func TestBlock_Range(t *testing.T) {
	testNode(t, func(ctx context.Context, t *testing.T, node e2e.Node) {
		client, err := node.Client()
		require.NoError(t, err)
		status, err := client.Status(ctx)
		require.NoError(t, err)

		first := status.SyncInfo.EarliestBlockHeight
		last := status.SyncInfo.LatestBlockHeight

		switch {
		// if the node state synced we ignore any assertions because it's hard to know how far back
		// the node ran reverse sync for
		case node.StateSync != e2e.StateSyncDisabled:
			break
		case node.RetainBlocks > 0 && int64(node.RetainBlocks) < (last-node.Testnet.InitialHeight+1):
			// Delta handles race conditions in reading first/last heights.
			assert.InDelta(t, node.RetainBlocks, last-first+1, 1,
				"node not pruning expected blocks")

		default:
			assert.Equal(t, node.Testnet.InitialHeight, first,
				"node's first block should be network's initial height")
		}

		for h := first; h <= last; h++ {
			if node.StateSync != e2e.StateSyncDisabled && h <= first+e2e.EvidenceAgeHeight+1 {
				continue
			}
			resp, err := client.Block(ctx, &(h))
			if err != nil && node.RetainBlocks > 0 && h == first {
				// Ignore errors in first block if node is pruning blocks due to race conditions.
				continue
			}
			require.NoError(t, err)
			require.NotNil(t, resp.Block)
			assert.Equal(t, h, resp.Block.Height)
		}

		for h := node.Testnet.InitialHeight; h < first; h++ {
			_, err := client.Block(ctx, &(h))
			require.Error(t, err)
		}
	})
}
