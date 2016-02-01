package example

import (
	"testing"
	"time"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/tmsp/server"
	"github.com/tendermint/tmsp/types"
)

func TestStream(t *testing.T) {

	numAppendTxs := 200000

	// Start the listener
	_, err := server.StartListener("tcp://127.0.0.1:46658", NewDummyApplication())
	if err != nil {
		Exit(err.Error())
	}

	// Connect to the socket
	conn, err := Connect("tcp://127.0.0.1:46658")
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
			switch res.Type {
			case types.MessageType_AppendTx:
				counter += 1
				if types.RetCode(res.Code) != types.RetCodeOK {
					t.Error("AppendTx failed with ret_code", res.Code)
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
			case types.MessageType_Flush:
				// ignore
			default:
				t.Error("Unexpected response type", res.Type)
			}
		}
	}()

	// Write requests
	for counter := 0; counter < numAppendTxs; counter++ {
		// Send request
		var req = types.RequestAppendTx([]byte("test"))
		err := types.WriteMessage(req, conn)
		if err != nil {
			t.Fatal(err.Error())
		}

		// Sometimes send flush messages
		if counter%123 == 0 {
			t.Log("flush")
			err := types.WriteMessage(types.RequestFlush(), conn)
			if err != nil {
				t.Fatal(err.Error())
			}
		}
	}

	// Send final flush message
	err = types.WriteMessage(types.RequestFlush(), conn)
	if err != nil {
		t.Fatal(err.Error())
	}

	<-done
}
