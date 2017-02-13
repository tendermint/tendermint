package example

import (
	"fmt"
	"log"
	"net"
	"reflect"
	"testing"
	"time"

	"google.golang.org/grpc"

	"golang.org/x/net/context"

	"github.com/tendermint/abci/client"
	"github.com/tendermint/abci/example/dummy"
	"github.com/tendermint/abci/server"
	"github.com/tendermint/abci/types"
	cmn "github.com/tendermint/go-common"
)

func TestDummy(t *testing.T) {
	fmt.Println("### Testing Dummy")
	testStream(t, dummy.NewDummyApplication())
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

	numDeliverTxs := 200000

	// Start the listener
	server, err := server.NewSocketServer("unix://test.sock", app)
	if err != nil {
		log.Fatal(fmt.Sprintf("Error starting socket server: %v", err.Error()))
	}
	defer server.Stop()

	// Connect to the socket
	client, err := abcicli.NewSocketClient("unix://test.sock", false)
	if err != nil {
		log.Fatal(fmt.Sprintf("Error starting socket client: %v", err.Error()))
	}
	client.Start()
	defer client.Stop()

	done := make(chan struct{})
	counter := 0
	client.SetResponseCallback(func(req *types.Request, res *types.Response) {
		// Process response
		switch r := res.Value.(type) {
		case *types.Response_DeliverTx:
			counter++
			if r.DeliverTx.Code != types.CodeType_OK {
				t.Error("DeliverTx failed with ret_code", r.DeliverTx.Code)
			}
			if counter > numDeliverTxs {
				t.Fatalf("Too many DeliverTx responses. Got %d, expected %d", counter, numDeliverTxs)
			}
			if counter == numDeliverTxs {
				go func() {
					time.Sleep(time.Second * 2) // Wait for a bit to allow counter overflow
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
		reqRes := client.DeliverTxAsync([]byte("test"))
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

func dialerFunc(addr string, timeout time.Duration) (net.Conn, error) {
	return cmn.Connect(addr)
}

func testGRPCSync(t *testing.T, app *types.GRPCApplication) {

	numDeliverTxs := 2000

	// Start the listener
	server, err := server.NewGRPCServer("unix://test.sock", app)
	if err != nil {
		log.Fatal(fmt.Sprintf("Error starting GRPC server: %v", err.Error()))
	}
	defer server.Stop()

	// Connect to the socket
	conn, err := grpc.Dial("unix://test.sock", grpc.WithInsecure(), grpc.WithDialer(dialerFunc))
	if err != nil {
		log.Fatal(fmt.Sprintf("Error dialing GRPC server: %v", err.Error()))
	}
	defer conn.Close()

	client := types.NewABCIApplicationClient(conn)

	// Write requests
	for counter := 0; counter < numDeliverTxs; counter++ {
		// Send request
		response, err := client.DeliverTx(context.Background(), &types.RequestDeliverTx{[]byte("test")})
		if err != nil {
			t.Fatalf("Error in GRPC DeliverTx: %v", err.Error())
		}
		counter++
		if response.Code != types.CodeType_OK {
			t.Error("DeliverTx failed with ret_code", response.Code)
		}
		if counter > numDeliverTxs {
			t.Fatal("Too many DeliverTx responses")
		}
		t.Log("response", counter)
		if counter == numDeliverTxs {
			go func() {
				time.Sleep(time.Second * 2) // Wait for a bit to allow counter overflow
			}()
		}

	}
}
