package commands_test

// import (
// 	"context"
// 	"fmt"
// 	"testing"
// 	"time"

// 	"github.com/stretchr/testify/require"

// 	"github.com/tendermint/tendermint/cmd/tendermint/commands"
// 	"github.com/tendermint/tendermint/config"
// 	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
// 	tmnet "github.com/tendermint/tendermint/libs/net"
// 	rpctest "github.com/tendermint/tendermint/rpc/test"
// 	e2e "github.com/tendermint/tendermint/test/e2e/app"
// )

// func TestRollbackIntegration(t *testing.T) {
// 	var height int64
// 	dir := t.TempDir()
// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()
// 	cfg := createConfig(t.Name())
// 	app, err := e2e.NewApplication(e2e.DefaultConfig(dir))

// 	t.Run("First run", func(t *testing.T) {
// 		require.NoError(t, err)
// 		node := rpctest.StartTendermint(app, rpctest.SuppressStdout)

// 		time.Sleep(3 * time.Second)
// 		require.NoError(t, node.Stop())
// 		node.Wait()
// 		require.False(t, node.IsRunning())
// 	})

// 	t.Run("Rollback", func(t *testing.T) {
// 		require.NoError(t, app.Rollback())
// 		height, _, err = commands.RollbackState(cfg)
// 		require.NoError(t, err)

// 	})

// 	t.Run("Restart", func(t *testing.T) {
// 		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
// 		defer cancel()
// 		node2 := rpctest.StartTendermint(app, rpctest.SuppressStdout)
// 		defer func(){
// 			rpctest.StopTendermint(node2)
// 		}()

// 		client, err := rpchttp.New(cfg.RPC.ListenAddress, "/websocket")
// 		require.NoError(t, err)

// 		ticker := time.NewTicker(200 * time.Millisecond)
// 		for {
// 			select {
// 			case <-ctx.Done():
// 				t.Fatalf("failed to make progress after 20 seconds. Min height: %d", height)
// 			case <-ticker.C:
// 				status, err := client.Status(ctx)
// 				require.NoError(t, err)

// 				if status.SyncInfo.LatestBlockHeight > height {
// 					return
// 				}
// 			}
// 		}
// 	})

// }

// func createConfig(testName string) *config.Config {
// 	c := config.ResetTestRoot(testName)

// 	p2pAddr, rpcAddr := makeAddrs()
// 	c.P2P.ListenAddress = p2pAddr
// 	c.RPC.ListenAddress = rpcAddr
// 	c.Consensus.WalPath = "rpc-test"
// 	c.BaseConfig.DBBackend = "goleveldb"
// 	c.RPC.CORSAllowedOrigins = []string{"https://tendermint.com/"}
// 	return c
// }

// func makeAddrs() (p2pAddr, rpcAddr string) {
// 	const addrTemplate = "tcp://127.0.0.1:%d"
// 	return fmt.Sprintf(addrTemplate, randPort()), fmt.Sprintf(addrTemplate, randPort())
// }

// func randPort() int {
// 	port, err := tmnet.GetFreePort()
// 	if err != nil {
// 		panic(err)
// 	}
// 	return port
// }