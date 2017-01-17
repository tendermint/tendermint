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
	appProc := startApp(abciApp)
	defer appProc.StopProcess(true)
	client := startClient(abciType)
	defer client.Stop()

	setOption(client, "serial", "on")
	commit(client, nil)
	deliverTx(client, []byte("abc"), types.CodeType_BadNonce, nil)
	commit(client, nil)
	deliverTx(client, []byte{0x00}, types.CodeType_OK, nil)
	commit(client, []byte{0, 0, 0, 0, 0, 0, 0, 1})
	deliverTx(client, []byte{0x00}, types.CodeType_BadNonce, nil)
	deliverTx(client, []byte{0x01}, types.CodeType_OK, nil)
	deliverTx(client, []byte{0x00, 0x02}, types.CodeType_OK, nil)
	deliverTx(client, []byte{0x00, 0x03}, types.CodeType_OK, nil)
	deliverTx(client, []byte{0x00, 0x00, 0x04}, types.CodeType_OK, nil)
	deliverTx(client, []byte{0x00, 0x00, 0x06}, types.CodeType_BadNonce, nil)
	commit(client, []byte{0, 0, 0, 0, 0, 0, 0, 5})
}
