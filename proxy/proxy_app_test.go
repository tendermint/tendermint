package proxy

import (
	"bytes"
	"strings"
	"testing"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-logio"
	example "github.com/tendermint/tmsp/example"
	"github.com/tendermint/tmsp/server"
)

func TestEcho(t *testing.T) {
	sockPath := Fmt("unix:///tmp/echo_%v.sock", RandStr(6))
	_, err := server.StartListener(sockPath, example.NewDummyApplication())
	if err != nil {
		Exit(err.Error())
	}
	conn, err := Connect(sockPath)
	if err != nil {
		Exit(err.Error())
	} else {
		t.Log("Connected")
	}

	logBuffer := bytes.NewBuffer(nil)
	logConn := logio.NewLoggedConn(conn, logBuffer)
	proxy := NewProxyApp(logConn, 10)
	proxy.Start()

	for i := 0; i < 1000; i++ {
		proxy.EchoAsync(Fmt("echo-%v", i))
	}
	proxy.FlushSync()

	if proxy.reqSent.Len() != 1001 {
		t.Error(Fmt("Expected 1001 requests sent, got %v",
			proxy.reqSent.Len()))
	}
	if proxy.resReceived.Len() != 1001 {
		t.Error(Fmt("Expected 1001 responses received, got %v",
			proxy.resReceived.Len()))
	}
	if t.Failed() {
		logio.PrintReader(logBuffer)
	}
}

func BenchmarkEcho(b *testing.B) {
	b.StopTimer() // Initialize
	sockPath := Fmt("unix:///tmp/echo_%v.sock", RandStr(6))
	_, err := server.StartListener(sockPath, example.NewDummyApplication())
	if err != nil {
		Exit(err.Error())
	}
	conn, err := Connect(sockPath)
	if err != nil {
		Exit(err.Error())
	} else {
		b.Log("Connected")
	}

	proxy := NewProxyApp(conn, 10)
	proxy.Start()
	echoString := strings.Repeat(" ", 200)
	b.StartTimer() // Start benchmarking tests

	for i := 0; i < b.N; i++ {
		proxy.EchoAsync(echoString)
	}
	proxy.FlushSync()

	b.StopTimer()
	// info := proxy.InfoSync()
	//b.Log("N: ", b.N, info)
}

func TestInfo(t *testing.T) {
	sockPath := Fmt("unix:///tmp/echo_%v.sock", RandStr(6))
	_, err := server.StartListener(sockPath, example.NewDummyApplication())
	if err != nil {
		Exit(err.Error())
	}
	conn, err := Connect(sockPath)
	if err != nil {
		Exit(err.Error())
	} else {
		t.Log("Connected")
	}

	logBuffer := bytes.NewBuffer(nil)
	logConn := logio.NewLoggedConn(conn, logBuffer)
	proxy := NewProxyApp(logConn, 10)
	proxy.Start()
	data := proxy.InfoSync()

	if data[0] != "size:0" {
		t.Error("Expected ResponseInfo with one element 'size:0' but got something else")
	}
}
