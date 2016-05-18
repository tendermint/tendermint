package nilapp

import (
	"net"
	"reflect"
	"testing"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/tmsp/server"
	"github.com/tendermint/tmsp/types"
)

func TestStream(t *testing.T) {

	numAppendTxs := 200000

	// Start the listener
	server, err := server.NewSocketServer("unix://test.sock", NewNilApplication())
	if err != nil {
		Exit(err.Error())
	}
	defer server.Stop()

	// Connect to the socket
	conn, err := Connect("unix://test.sock")
	if err != nil {
		Exit(err.Error())
	}

	// Read response data
	done := make(chan struct{})
	go func() {
		counter := 0
		for {

			var res = &types.Response{}
			err := types.ReadMessage(conn, res)
			if err != nil {
				Exit(err.Error())
			}

			// Process response
			switch r := res.Value.(type) {
			case *types.Response_AppendTx:
				counter += 1
				if r.AppendTx.Code != types.CodeType_OK {
					t.Error("AppendTx failed with ret_code", r.AppendTx.Code)
				}
				if counter > numAppendTxs {
					t.Fatal("Too many AppendTx responses")
				}
				t.Log("response", counter)
				if counter == numAppendTxs {
					go func() {
						time.Sleep(time.Second * 2) // Wait for a bit to allow counter overflow
						close(done)
					}()
				}
			case *types.Response_Flush:
				// ignore
			default:
				t.Error("Unexpected response type", reflect.TypeOf(res.Value))
			}
		}
	}()

	// Write requests
	for counter := 0; counter < numAppendTxs; counter++ {
		// Send request
		var req = types.ToRequestAppendTx([]byte("test"))
		err := types.WriteMessage(req, conn)
		if err != nil {
			t.Fatal(err.Error())
		}

		// Sometimes send flush messages
		if counter%123 == 0 {
			t.Log("flush")
			err := types.WriteMessage(types.ToRequestFlush(), conn)
			if err != nil {
				t.Fatal(err.Error())
			}
		}
	}

	// Send final flush message
	err = types.WriteMessage(types.ToRequestFlush(), conn)
	if err != nil {
		t.Fatal(err.Error())
	}

	<-done
}

//-------------------------
// test grpc

func dialerFunc(addr string, timeout time.Duration) (net.Conn, error) {
	return Connect(addr)
}

func TestGRPCSync(t *testing.T) {

	numAppendTxs := 2000

	// Start the listener
	server, err := server.NewGRPCServer("unix://test.sock", types.NewGRPCApplication(NewNilApplication()))
	if err != nil {
		Exit(err.Error())
	}
	defer server.Stop()

	// Connect to the socket
	conn, err := grpc.Dial("unix://test.sock", grpc.WithInsecure(), grpc.WithDialer(dialerFunc))
	if err != nil {
		Exit(err.Error())
	}
	defer conn.Close()

	client := types.NewTMSPApplicationClient(conn)

	// Write requests
	for counter := 0; counter < numAppendTxs; counter++ {
		// Send request
		response, err := client.AppendTx(context.Background(), &types.RequestAppendTx{[]byte("test")})
		if err != nil {
			t.Fatal(err.Error())
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
