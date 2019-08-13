package example

import (
	"fmt"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"google.golang.org/grpc"

	"golang.org/x/net/context"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"

	abcicli "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/code"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	abciserver "github.com/tendermint/tendermint/abci/server"
	"github.com/tendermint/tendermint/abci/types"
)

func TestKVStore(t *testing.T) {
	fmt.Println("### Testing KVStore")
	testStream(t, kvstore.NewKVStoreApplication())
}

func TestBaseApp(t *testing.T) {
	fmt.Println("### Testing BaseApp")
	testStream(t, types.NewBaseApplication())
}

func TestGRPC(t *testing.T) {
	fmt.Println("### Testing GRPC")
	testGRPCSync(t, types.NewGRPCApplication(types.NewBaseApplication()))
}

func testStream(t *testing.T, app types.Application) {
	numDeliverTxs := 20000

	// Start the listener
	server := abciserver.NewSocketServer("unix://test.sock", app)
	server.SetLogger(log.TestingLogger().With("module", "abci-server"))
	if err := server.Start(); err != nil {
		require.NoError(t, err, "Error starting socket server")
	}
	defer server.Stop()

	// Connect to the socket
	client := abcicli.NewSocketClient("unix://test.sock", false)
	client.SetLogger(log.TestingLogger().With("module", "abci-client"))
	if err := client.Start(); err != nil {
		t.Fatalf("Error starting socket client: %v", err.Error())
	}
	defer client.Stop()

	done := make(chan struct{})
	counter := 0
	client.SetResponseCallback(func(req *types.Request, res *types.Response) {
		// Process response
		switch r := res.Value.(type) {
		case *types.Response_DeliverTx:
			counter++
			if r.DeliverTx.Code != code.CodeTypeOK {
				t.Error("DeliverTx failed with ret_code", r.DeliverTx.Code)
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
		case *types.Response_Flush:
			// ignore
		default:
			t.Error("Unexpected response type", reflect.TypeOf(res.Value))
		}
	})

	// Write requests
	for counter := 0; counter < numDeliverTxs; counter++ {
		// Send request
		reqRes := client.DeliverTxAsync(types.RequestDeliverTx{Tx: []byte("test")})
		_ = reqRes
		// check err ?

		// Sometimes send flush messages
		if counter%123 == 0 {
			client.FlushAsync()
			// check err ?
		}
	}

	// Send final flush message
	client.FlushAsync()

	<-done
}

//-------------------------
// test grpc

func dialerFunc(ctx context.Context, addr string) (net.Conn, error) {
	return cmn.Connect(addr)
}

func testGRPCSync(t *testing.T, app *types.GRPCApplication) {
	numDeliverTxs := 2000

	// Start the listener
	server := abciserver.NewGRPCServer("unix://test.sock", app)
	server.SetLogger(log.TestingLogger().With("module", "abci-server"))
	if err := server.Start(); err != nil {
		t.Fatalf("Error starting GRPC server: %v", err.Error())
	}
	defer server.Stop()

	// Connect to the socket
	conn, err := grpc.Dial("unix://test.sock", grpc.WithInsecure(), grpc.WithContextDialer(dialerFunc))
	if err != nil {
		t.Fatalf("Error dialing GRPC server: %v", err.Error())
	}
	defer conn.Close()

	client := types.NewABCIApplicationClient(conn)

	// Write requests
	for counter := 0; counter < numDeliverTxs; counter++ {
		// Send request
		response, err := client.DeliverTx(context.Background(), &types.RequestDeliverTx{Tx: []byte("test")})
		if err != nil {
			t.Fatalf("Error in GRPC DeliverTx: %v", err.Error())
		}
		counter++
		if response.Code != code.CodeTypeOK {
			t.Error("DeliverTx failed with ret_code", response.Code)
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
