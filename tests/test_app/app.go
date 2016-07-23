package main

import (
	"bytes"
	"os"
	"time"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-process"
	"github.com/tendermint/tmsp/client"
	"github.com/tendermint/tmsp/types"
)

//----------------------------------------

func StartApp(tmspApp string) *process.Process {
	// Start the app
	//outBuf := NewBufferCloser(nil)
	proc, err := process.StartProcess("tmsp_app",
		"bash",
		[]string{"-c", tmspApp},
		nil,
		os.Stdout,
	)
	if err != nil {
		panic("running tmsp_app: " + err.Error())
	}

	// TODO a better way to handle this?
	time.Sleep(time.Second)

	return proc
}

func StartClient(tmspType string) tmspcli.Client {
	// Start client
	client, err := tmspcli.NewClient("tcp://127.0.0.1:46658", tmspType, true)
	if err != nil {
		panic("connecting to tmsp_app: " + err.Error())
	}
	return client
}

func SetOption(client tmspcli.Client, key, value string) {
	res := client.SetOptionSync(key, value)
	_, _, log := res.Code, res.Data, res.Log
	if res.IsErr() {
		panic(Fmt("setting %v=%v: \nlog: %v", key, value, log))
	}
}

func Commit(client tmspcli.Client, hashExp []byte) {
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

func AppendTx(client tmspcli.Client, txBytes []byte, codeExp types.CodeType, dataExp []byte) {
	res := client.AppendTxSync(txBytes)
	code, data, log := res.Code, res.Data, res.Log
	if code != codeExp {
		panic(Fmt("AppendTx response code was unexpected. Got %v expected %v. Log: %v",
			code, codeExp, log))
	}
	if !bytes.Equal(data, dataExp) {
		panic(Fmt("AppendTx response data was unexpected. Got %X expected %X",
			data, dataExp))
	}
}

func CheckTx(client tmspcli.Client, txBytes []byte, codeExp types.CodeType, dataExp []byte) {
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
