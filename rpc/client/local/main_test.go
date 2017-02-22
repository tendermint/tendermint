package local_test

import (
	"os"
	"testing"

	meapp "github.com/tendermint/merkleeyes/app"
	nm "github.com/tendermint/tendermint/node"
	rpctest "github.com/tendermint/tendermint/rpc/test"
)

var node *nm.Node

func TestMain(m *testing.M) {
	// configure a node, but don't start the server
	app := meapp.NewMerkleEyesApp("", 100)
	node = rpctest.StartTendermint(app)

	code := m.Run()
	// and shut down proper at the end
	os.Exit(code)
}
