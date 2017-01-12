package main

import (
	"fmt"
	"os"

	"github.com/tendermint/tmsp/types"
)

var tmspType string

func init() {
	tmspType = os.Getenv("TMSP")
	if tmspType == "" {
		tmspType = "socket"
	}
}

func main() {
	testCounter()
}

func testCounter() {
	tmspApp := os.Getenv("TMSP_APP")
	if tmspApp == "" {
		panic("No TMSP_APP specified")
	}

	fmt.Printf("Running %s test with tmsp=%s\n", tmspApp, tmspType)
	appProc := StartApp(tmspApp)
	defer appProc.StopProcess(true)
	client := StartClient(tmspType)
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
