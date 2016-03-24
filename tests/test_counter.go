package main

import (
	"bytes"
	"fmt"
	"os"
	"time"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-process"
	"github.com/tendermint/tmsp/client"
	"github.com/tendermint/tmsp/types"
)

func main() {

	// Run tests
	testBasic()

	fmt.Println("Success!")
}

func testBasic() {
	fmt.Println("Running basic tests")
	appProc := startApp()
	defer appProc.StopProcess(true)
	client := startClient()
	defer client.Stop()

	setOption(client, "serial", "on")
	commit(client, nil)
	appendTx(client, []byte("abc"), types.CodeType_BadNonce, nil)
	commit(client, nil)
	appendTx(client, []byte{0x00}, types.CodeType_OK, nil)
	commit(client, []byte{0, 0, 0, 0, 0, 0, 0, 1})
	appendTx(client, []byte{0x00}, types.CodeType_BadNonce, nil)
	appendTx(client, []byte{0x01}, types.CodeType_OK, nil)
	appendTx(client, []byte{0x00, 0x02}, types.CodeType_OK, nil)
	appendTx(client, []byte{0x00, 0x03}, types.CodeType_OK, nil)
	appendTx(client, []byte{0x00, 0x00, 0x04}, types.CodeType_OK, nil)
	appendTx(client, []byte{0x00, 0x00, 0x06}, types.CodeType_BadNonce, nil)
	commit(client, []byte{0, 0, 0, 0, 0, 0, 0, 5})
}

//----------------------------------------

func startApp() *process.Process {
	counterApp := os.Getenv("COUNTER_APP")
	if counterApp == "" {
		panic("No COUNTER_APP specified")
	}

	// Start the app
	//outBuf := NewBufferCloser(nil)
	proc, err := process.StartProcess("counter_app",
		"bash",
		[]string{"-c", counterApp},
		nil,
		os.Stdout,
	)
	if err != nil {
		panic("running counter_app: " + err.Error())
	}

	// TODO a better way to handle this?
	time.Sleep(time.Second)

	return proc
}

func startClient() tmspcli.Client {
	// Start client
	client, err := tmspcli.NewClient("tcp://127.0.0.1:46658", true)
	if err != nil {
		panic("connecting to counter_app: " + err.Error())
	}
	return client
}

func setOption(client tmspcli.Client, key, value string) {
	res := client.SetOptionSync(key, value)
	_, _, log := res.Code, res.Data, res.Log
	if res.IsErr() {
		panic(Fmt("setting %v=%v: \nlog: %v", key, value, log))
	}
}

func commit(client tmspcli.Client, hashExp []byte) {
	res := client.CommitSync()
	_, data, log := res.Code, res.Data, res.Log
	if res.IsErr() {
		panic(Fmt("committing %v\nlog: %v", log))
	}
	if !bytes.Equal(res.Data, hashExp) {
		panic(Fmt("Commit hash was unexpected. Got %X expected %X",
			data, hashExp))
	}
}

func appendTx(client tmspcli.Client, txBytes []byte, codeExp types.CodeType, dataExp []byte) {
	res := client.AppendTxSync(txBytes)
	code, data, log := res.Code, res.Data, res.Log
	if res.IsErr() {
		panic(Fmt("appending tx %X: %v\nlog: %v", txBytes, log))
	}
	if code != codeExp {
		panic(Fmt("AppendTx response code was unexpected. Got %v expected %v. Log: %v",
			code, codeExp, log))
	}
	if !bytes.Equal(data, dataExp) {
		panic(Fmt("AppendTx response data was unexpected. Got %X expected %X",
			data, dataExp))
	}
}

func checkTx(client tmspcli.Client, txBytes []byte, codeExp types.CodeType, dataExp []byte) {
	res := client.CheckTxSync(txBytes)
	code, data, log := res.Code, res.Data, res.Log
	if res.IsErr() {
		panic(Fmt("checking tx %X: %v\nlog: %v", txBytes, log))
	}
	if code != codeExp {
		panic(Fmt("CheckTx response code was unexpected. Got %v expected %v. Log: %v",
			code, codeExp, log))
	}
	if !bytes.Equal(data, dataExp) {
		panic(Fmt("CheckTx response data was unexpected. Got %X expected %X",
			data, dataExp))
	}
}
