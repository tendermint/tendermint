package e2e_test

import (
	"context"
	"os"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	rpctypes "github.com/tendermint/tendermint/rpc/coretypes"
	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
	"github.com/tendermint/tendermint/types"
)

func init() {
	// This can be used to manually specify a testnet manifest and/or node to
	// run tests against. The testnet must have been started by the runner first.
	// os.Setenv("E2E_MANIFEST", "networks/ci.toml")
	// os.Setenv("E2E_NODE", "validator01")
}

var (
	testnetCache    = map[string]e2e.Testnet{}
	testnetCacheMtx = sync.Mutex{}
	blocksCache     = map[string][]*types.Block{}
	blocksCacheMtx  = sync.Mutex{}
)

// testNode runs tests for testnet nodes. The callback function is
// given a single stateful node to test, running as a subtest in
// parallel with other subtests.
//
// The testnet manifest must be given as the envvar E2E_MANIFEST. If not set,
// these tests are skipped so that they're not picked up during normal unit
// test runs. If E2E_NODE is also set, only the specified node is tested,
// otherwise all nodes are tested.
func testNode(t *testing.T, testFunc func(context.Context, *testing.T, e2e.Node)) {
	t.Helper()

	testnet := loadTestnet(t)
	nodes := testnet.Nodes

	if name := os.Getenv("E2E_NODE"); name != "" {
		node := testnet.LookupNode(name)
		require.NotNil(t, node, "node %q not found in testnet %q", name, testnet.Name)
		nodes = []*e2e.Node{node}
	} else {
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].Name < nodes[j].Name
		})
	}

	for _, node := range nodes {
		node := *node

		if node.Stateless() {
			continue
		}

		t.Run(node.Name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			testFunc(ctx, t, node)
		})
	}
}

// loadTestnet loads the testnet based on the E2E_MANIFEST envvar.
func loadTestnet(t *testing.T) e2e.Testnet {
	t.Helper()

	manifest := os.Getenv("E2E_MANIFEST")
	if manifest == "" {
		t.Skip("E2E_MANIFEST not set, not an end-to-end test run")
	}

	testnetCacheMtx.Lock()
	defer testnetCacheMtx.Unlock()
	if testnet, ok := testnetCache[manifest]; ok {
		return testnet
	}

	testnet, err := e2e.LoadTestnet(manifest)
	require.NoError(t, err)
	testnetCache[manifest] = *testnet
	return *testnet
}

// fetchBlockChain fetches a complete, up-to-date block history from
// the freshest testnet archive node.
func fetchBlockChain(ctx context.Context, t *testing.T) []*types.Block {
	t.Helper()

	testnet := loadTestnet(t)

	// Find the freshest archive node
	var (
		client *rpchttp.HTTP
		status *rpctypes.ResultStatus
	)
	for _, node := range testnet.ArchiveNodes() {
		c, err := node.Client()
		require.NoError(t, err)
		s, err := c.Status(ctx)
		require.NoError(t, err)
		if status == nil || s.SyncInfo.LatestBlockHeight > status.SyncInfo.LatestBlockHeight {
			client = c
			status = s
		}
	}
	require.NotNil(t, client, "couldn't find an archive node")

	// Fetch blocks. Look for existing block history in the block cache, and
	// extend it with any new blocks that have been produced.
	blocksCacheMtx.Lock()
	defer blocksCacheMtx.Unlock()

	from := status.SyncInfo.EarliestBlockHeight
	to := status.SyncInfo.LatestBlockHeight
	blocks, ok := blocksCache[testnet.Name]
	if !ok {
		blocks = make([]*types.Block, 0, to-from+1)
	}
	if len(blocks) > 0 {
		from = blocks[len(blocks)-1].Height + 1
	}

	for h := from; h <= to; h++ {
		resp, err := client.Block(ctx, &(h))
		require.NoError(t, err)
		require.NotNil(t, resp.Block)
		require.Equal(t, h, resp.Block.Height, "unexpected block height %v", resp.Block.Height)
		blocks = append(blocks, resp.Block)
	}
	require.NotEmpty(t, blocks, "blockchain does not contain any blocks")
	blocksCache[testnet.Name] = blocks

	return blocks
}
