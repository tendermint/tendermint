package example

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/tendermint/tendermint/libs/log"
	tmnet "github.com/tendermint/tendermint/libs/net"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/code"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	abciserver "github.com/tendermint/tendermint/abci/server"
	"github.com/tendermint/tendermint/abci/types"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func TestKVStore(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := log.NewTestingLogger(t)

	logger.Info("### Testing KVStore")
	testStream(ctx, t, logger, kvstore.NewApplication())
}

func TestBaseApp(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := log.NewTestingLogger(t)

	logger.Info("### Testing BaseApp")
	testStream(ctx, t, logger, types.NewBaseApplication())
}

func TestGRPC(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewTestingLogger(t)

	logger.Info("### Testing GRPC")
	testGRPCSync(ctx, t, logger, types.NewGRPCApplication(types.NewBaseApplication()))
}

func testStream(ctx context.Context, t *testing.T, logger log.Logger, app types.Application) {
	t.Helper()

	const numDeliverTxs = 20000
	socketFile := fmt.Sprintf("test-%08x.sock", rand.Int31n(1<<30))
	defer os.Remove(socketFile)
	socket := fmt.Sprintf("unix://%v", socketFile)
	// Start the listener
	server := abciserver.NewSocketServer(logger.With("module", "abci-server"), socket, app)
	t.Cleanup(server.Wait)
	err := server.Start(ctx)
	require.NoError(t, err)

	// Connect to the socket
	client := abciclient.NewSocketClient(logger.With("module", "abci-client"), socket, false)
	t.Cleanup(client.Wait)

	err = client.Start(ctx)
	require.NoError(t, err)

	done := make(chan struct{})
	counter := 0
	client.SetResponseCallback(func(req *types.Request, res *types.Response) {
		// Process response
		switch r := res.Value.(type) {
		case *types.Response_FinalizeBlock:
			for _, tx := range r.FinalizeBlock.Txs {
				counter++
				if tx.Code != code.CodeTypeOK {
					t.Error("DeliverTx failed with ret_code", tx.Code)
				}
				if counter > numDeliverTxs {
					t.Fatalf("Too many DeliverTx responses. Got %d, expected %d", counter, numDeliverTxs)
				}
				if counter == numDeliverTxs {
					go func() {
						time.Sleep(time.Second * 1) // Wait for a bit to allow counter overflow
						close(done)
					}()
					return
				}
			}
		case *types.Response_Flush:
			// ignore
		default:
			t.Error("Unexpected response type", reflect.TypeOf(res.Value))
		}
	})

	// Write requests
	for counter := 0; counter < numDeliverTxs; counter++ {
		// Send request
		tx := []byte("test")
		_, err = client.FinalizeBlockAsync(ctx, types.RequestFinalizeBlock{Txs: [][]byte{tx}})
		require.NoError(t, err)

		// Sometimes send flush messages
		if counter%128 == 0 {
			err = client.Flush(ctx)
			require.NoError(t, err)
		}
	}

	// Send final flush message
	_, err = client.FlushAsync(ctx)
	require.NoError(t, err)

	<-done
}

//-------------------------
// test grpc

func dialerFunc(ctx context.Context, addr string) (net.Conn, error) {
	return tmnet.Connect(addr)
}

func testGRPCSync(ctx context.Context, t *testing.T, logger log.Logger, app types.ABCIApplicationServer) {
	t.Helper()
	numDeliverTxs := 2000
	socketFile := fmt.Sprintf("/tmp/test-%08x.sock", rand.Int31n(1<<30))
	defer os.Remove(socketFile)
	socket := fmt.Sprintf("unix://%v", socketFile)

	// Start the listener
	server := abciserver.NewGRPCServer(logger.With("module", "abci-server"), socket, app)

	require.NoError(t, server.Start(ctx))
	t.Cleanup(func() { server.Wait() })

	// Connect to the socket
	conn, err := grpc.Dial(socket,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialerFunc),
	)
	require.NoError(t, err, "Error dialing GRPC server")

	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Error(err)
		}
	})

	client := types.NewABCIApplicationClient(conn)

	// Write requests
	for counter := 0; counter < numDeliverTxs; counter++ {
		// Send request
		txt := []byte("test")
		response, err := client.FinalizeBlock(ctx, &types.RequestFinalizeBlock{Txs: [][]byte{txt}})
		require.NoError(t, err, "Error in GRPC FinalizeBlock")

		counter++
		for _, tx := range response.Txs {
			if tx.Code != code.CodeTypeOK {
				t.Error("DeliverTx failed with ret_code", tx.Code)
			}
			if counter > numDeliverTxs {
				t.Fatal("Too many DeliverTx responses")
			}
			t.Log("response", counter)
			if counter == numDeliverTxs {
				go func() {
					time.Sleep(time.Second * 1) // Wait for a bit to allow counter overflow
				}()
			}
		}

	}
}
