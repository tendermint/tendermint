/*
package tests contain integration tests and helper functions for testing
the RPC interface

In particular, it allows us to spin up a tendermint node in process, with
a live RPC server, which we can use to verify our rpc calls.  It provides
all data structures, enabling us to do more complex tests (like node_test.go)
that introspect the blocks themselves to validate signatures and the like.

It currently only spins up one node, it would be interesting to expand it
to multiple nodes to see the real effects of validating partially signed
blocks.
*/
package rpctest

import (
	"os"
	"testing"

	"github.com/tendermint/abci/example/dummy"
	nm "github.com/tendermint/tendermint/node"
)

var node *nm.Node

func TestMain(m *testing.M) {
	// start a tendermint node (and merkleeyes) in the background to test against
	app := dummy.NewDummyApplication()
	node = StartTendermint(app)
	code := m.Run()

	// and shut down proper at the end
	node.Stop()
	node.Wait()
	os.Exit(code)
}
