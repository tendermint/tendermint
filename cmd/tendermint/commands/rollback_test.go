package commands_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/cmd/tendermint/commands"
	"github.com/tendermint/tendermint/rpc/client/local"
	rpctest "github.com/tendermint/tendermint/rpc/test"
	e2e "github.com/tendermint/tendermint/test/e2e/app"
)

func TestRollbackIntegration(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg, err := rpctest.CreateConfig(t.Name())
	require.NoError(t, err)
	cfg.BaseConfig.DBBackend = "goleveldb"

	subctx, subcancel := context.WithCancel(ctx)
	defer subcancel()
	app, err := e2e.NewApplication(e2e.DefaultConfig(dir))
	require.NoError(t, err)
	node, _, err := rpctest.StartTendermint(subctx, cfg, app, rpctest.SuppressStdout)
	require.NoError(t, err)

	time.Sleep(3 * time.Second)
	subcancel()
	node.Wait()
	require.False(t, node.IsRunning())

	require.NoError(t, app.Rollback())
	height, _, err := commands.RollbackState(cfg)
	require.NoError(t, err)

	subctx2, subcancel2 := context.WithTimeout(ctx, 10*time.Second)
	defer subcancel2()
	node2, _, err2 := rpctest.StartTendermint(subctx2, cfg, app, rpctest.SuppressStdout)
	require.NoError(t, err2)

	client, err := local.New(node2.(local.NodeService))
	require.NoError(t, err)

	ticker := time.NewTicker(200 * time.Millisecond)
	for {
		select {
		case <-subctx2.Done():
			t.Fatalf("failed to make progress after 20 seconds. Min height: %d", height)
		case <-ticker.C:
			status, err := client.Status(subctx2)
			require.NoError(t, err)

			if status.SyncInfo.LatestBlockHeight > height {
				return
			}
		}
	}

}
