package main

import (
	"fmt"
	"os"

	"github.com/tendermint/abci/types"
)

var abciType string

func init() {
	abciType = os.Getenv("ABCI")
	if abciType == "" {
		abciType = "socket"
	}
}

func main() {
	testCounter()
}

func testCounter() {
	abciApp := os.Getenv("ABCI_APP")
	if abciApp == "" {
		panic("No ABCI_APP specified")
	}

	fmt.Printf("Running %s test with abci=%s\n", abciApp, abciType)
	appProc := StartApp(abciApp)
	defer appProc.StopProcess(true)
	client := StartClient(abciType)
	defer client.Stop()

	SetOption(client, "serial", "on")
	Commit(client, nil)
	DeliverTx(client, []byte("abc"), types.CodeType_BadNonce, nil)
	Commit(client, nil)
	DeliverTx(client, []byte{0x00}, types.CodeType_OK, nil)
	Commit(client, []byte{0, 0, 0, 0, 0, 0, 0, 1})
	DeliverTx(client, []byte{0x00}, types.CodeType_BadNonce, nil)
	DeliverTx(client, []byte{0x01}, types.CodeType_OK, nil)
	DeliverTx(client, []byte{0x00, 0x02}, types.CodeType_OK, nil)
	DeliverTx(client, []byte{0x00, 0x03}, types.CodeType_OK, nil)
	DeliverTx(client, []byte{0x00, 0x00, 0x04}, types.CodeType_OK, nil)
	DeliverTx(client, []byte{0x00, 0x00, 0x06}, types.CodeType_BadNonce, nil)
	Commit(client, []byte{0, 0, 0, 0, 0, 0, 0, 5})
}
