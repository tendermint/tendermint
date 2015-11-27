package example

import (
	// "fmt"
	"testing"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tmsp/server"
	"github.com/tendermint/tmsp/types"
)

func TestStream(t *testing.T) {

	// Start the listener
	_, err := server.StartListener("tcp://127.0.0.1:8080", NewDummyApplication())
	if err != nil {
		Exit(err.Error())
	}

	// Connect to the socket
	conn, err := Connect("tcp://127.0.0.1:8080")
	if err != nil {
		Exit(err.Error())
	}

	// Read response data
	go func() {
		for {
			var n int
			var err error
			var res types.Response
			wire.ReadBinaryPtr(&res, conn, 0, &n, &err)
			if err != nil {
				Exit(err.Error())
			}
			// fmt.Println("Read", n)
		}
	}()

	// Write requests
	for {
		var n int
		var err error
		var req types.Request = types.RequestAppendTx{TxBytes: []byte("test")}
		wire.WriteBinary(req, conn, &n, &err)
		if err != nil {
			Exit(err.Error())
		}
		// fmt.Println("Wrote", n)
	}
}
