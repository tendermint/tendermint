package client_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/node"
	rpctest "github.com/tendermint/tendermint/rpc/test"
)

func NodeSuite(t *testing.T) *node.Node {
	t.Helper()

	dir, err := ioutil.TempDir("/tmp", fmt.Sprint("rpc-client-test-", t.Name()))
	require.NoError(t, err)

	app := kvstore.NewPersistentKVStoreApplication(dir)
	n := rpctest.StartTendermint(app)

	t.Cleanup(func() {
		// and shut down proper at the end
		rpctest.StopTendermint(n)
		app.Close()

		_ = os.RemoveAll(dir)
	})

	return n
}
