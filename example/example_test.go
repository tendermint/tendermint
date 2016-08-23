package nilapp

import (
	"fmt"
	"net"
	"reflect"
	"testing"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/tmsp/client"
	"github.com/tendermint/tmsp/example/dummy"
	nilapp "github.com/tendermint/tmsp/example/nil"
	"github.com/tendermint/tmsp/server"
	"github.com/tendermint/tmsp/types"
)

func TestDummy(t *testing.T) {
	fmt.Println("### Testing Dummy")
	testStream(t, dummy.NewDummyApplication())
}

func TestNilApp(t *testing.T) {
	fmt.Println("### Testing NilApp")
	testStream(t, nilapp.NewNilApplication())
}

func TestGRPC(t *testing.T) {
	fmt.Println("### Testing GRPC")
	testGRPCSync(t, types.NewGRPCApplication(nilapp.NewNilApplication()))
}

func testStream(t *testing.T, app types.Application) {

	numAppendTxs := 200000

	// Start the listener
	server, err := server.NewSocketServer("unix://test.sock", app)
	if err != nil {
		Exit(Fmt("Error starting socket server: %v", err.Error()))
	}
	defer server.Stop()

	// Connect to the socket
	client, err := tmspcli.NewSocketClient("unix://test.sock", false)
	if err != nil {
		Exit(Fmt("Error starting socket client: %v", err.Error()))
	}
	client.Start()
	defer client.Stop()

	done := make(chan struct{})
	counter := 0
	client.SetResponseCallback(func(req *types.Request, res *types.Response) {
		// Process response
		switch r := res.Value.(type) {
		case *types.Response_AppendTx:
			counter += 1
			if r.AppendTx.Code != types.CodeType_OK {
				t.Error("AppendTx failed with ret_code", r.AppendTx.Code)
			}
			if counter > numAppendTxs {
				t.Fatalf("Too many AppendTx responses. Got %d, expected %d", counter, numAppendTxs)
			}
			if counter == numAppendTxs {
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
	for counter := 0; counter < numAppendTxs; counter++ {
		// Send request
		reqRes := client.AppendTxAsync([]byte("test"))
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
	return Connect(addr)
}

func testGRPCSync(t *testing.T, app *types.GRPCApplication) {

	numAppendTxs := 2000

	// Start the listener
	server, err := server.NewGRPCServer("unix://test.sock", app)
	if err != nil {
		Exit(Fmt("Error starting GRPC server: %v", err.Error()))
	}
	defer server.Stop()

	// Connect to the socket
	conn, err := grpc.Dial("unix://test.sock", grpc.WithInsecure(), grpc.WithDialer(dialerFunc))
	if err != nil {
		Exit(Fmt("Error dialing GRPC server: %v", err.Error()))
	}
	defer conn.Close()

	client := types.NewTMSPApplicationClient(conn)

	// Write requests
	for counter := 0; counter < numAppendTxs; counter++ {
		// Send request
		response, err := client.AppendTx(context.Background(), &types.RequestAppendTx{[]byte("test")})
		if err != nil {
			t.Fatalf("Error in GRPC AppendTx: %v", err.Error())
		}
		counter += 1
		if response.Code != types.CodeType_OK {
			t.Error("AppendTx failed with ret_code", response.Code)
		}
		if counter > numAppendTxs {
			t.Fatal("Too many AppendTx responses")
		}
		t.Log("response", counter)
		if counter == numAppendTxs {
			go func() {
				time.Sleep(time.Second * 2) // Wait for a bit to allow counter overflow
			}()
		}

	}
}
