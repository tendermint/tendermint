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
	cfg, err := rpctest.CreateConfig(t.Name())
	require.NoError(t, err)
	cfg.BaseConfig.DBBackend = "goleveldb"

	t.Log(cfg.DBDir())

	ctx, cancel := context.WithCancel(context.Background())
	app, err := e2e.NewApplication(e2e.DefaultConfig(dir))
	require.NoError(t, err)
	node, _, err := rpctest.StartTendermint(ctx, cfg, app)
	require.NoError(t, err)

	time.Sleep(3 * time.Second)
	cancel()
	node.Wait()

	require.NoError(t, app.Rollback())
	height, _, err := commands.RollbackState(cfg)
	require.NoError(t, err)

	newCtx, newCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer newCancel()
	node, _, err = rpctest.StartTendermint(ctx, cfg, app)
	require.NoError(t, err)

	client, err := local.New(node.(local.NodeService))
	require.NoError(t, err)

	ticker := time.NewTicker(200 * time.Millisecond)
	for {
		select {
		case <-newCtx.Done():
			t.Fatal("failed to make progress after 20 seconds")
		case <-ticker.C:
			status, err := client.Status(newCtx)
			require.NoError(t, err)

			if status.SyncInfo.LatestBlockHeight > height {
				break
			}
		}
	}

}
