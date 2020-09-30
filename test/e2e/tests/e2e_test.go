package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	e2e "github.com/tendermint/tendermint/test/e2e/pkg"
)

func init() {
	// This can be used to manually specify a testnet manifest and/or node to
	// run tests against. The testnet must have been started by the runner first.
	//os.Setenv("E2E_MANIFEST", "networks/simple.toml")
	//os.Setenv("E2E_NODE", "validator01")
}

var (
	ctx = context.Background()
)

// testNode runs tests for testnet nodes. The callback function is given a
// single node to test, running as a subtest in parallel with other subtests.
//
// The testnet manifest must be given as the envvar E2E_MANIFEST. If not set,
// these tests are skipped so that they're not picked up during normal unit
// test runs. If E2E_NODE is also set, only the specified node is tested,
// otherwise all nodes are tested.
func testNode(t *testing.T, testFunc func(*testing.T, e2e.Node)) {
	manifest := os.Getenv("E2E_MANIFEST")
	if manifest == "" {
		t.Skip("E2E_MANIFEST not set, not an end-to-end test run")
	}
	if !filepath.IsAbs(manifest) {
		manifest = filepath.Join("..", manifest)
	}

	testnet, err := e2e.LoadTestnet(manifest)
	require.NoError(t, err)
	nodes := testnet.Nodes

	if name := os.Getenv("E2E_NODE"); name != "" {
		node := testnet.LookupNode(name)
		require.NotNil(t, node, "node %q not found in testnet %q", name, testnet.Name)
		nodes = []*e2e.Node{node}
	}

	for _, node := range nodes {
		node := *node
		t.Run(node.Name, func(t *testing.T) {
			t.Parallel()
			testFunc(t, node)
		})
	}
}
