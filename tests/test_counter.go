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

func startClient() *tmspcli.TMSPClient {
	// Start client
	client, err := tmspcli.NewTMSPClient("tcp://127.0.0.1:46658")
	if err != nil {
		panic("connecting to counter_app: " + err.Error())
	}
	return client
}

func setOption(client *tmspcli.TMSPClient, key, value string) {
	log, err := client.SetOptionSync(key, value)
	if err != nil {
		panic(Fmt("setting %v=%v: %v\nlog: %v", key, value, err, log))
	}
}

func commit(client *tmspcli.TMSPClient, hashExp []byte) {
	hash, log, err := client.CommitSync()
	if err != nil {
		panic(Fmt("committing %v\nlog: %v", err, log))
	}
	if !bytes.Equal(hash, hashExp) {
		panic(Fmt("Commit hash was unexpected. Got %X expected %X",
			hash, hashExp))
	}
}

func appendTx(client *tmspcli.TMSPClient, txBytes []byte, codeExp types.CodeType, dataExp []byte) {
	code, data, log, err := client.AppendTxSync(txBytes)
	if err != nil {
		panic(Fmt("appending tx %X: %v\nlog: %v", txBytes, err, log))
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

func checkTx(client *tmspcli.TMSPClient, txBytes []byte, codeExp types.CodeType, dataExp []byte) {
	code, data, log, err := client.CheckTxSync(txBytes)
	if err != nil {
		panic(Fmt("checking tx %X: %v\nlog: %v", txBytes, err, log))
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
