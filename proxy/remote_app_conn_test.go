package proxy

import (
	"strings"
	"testing"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/tmsp/example/golang"
	"github.com/tendermint/tmsp/server"
)

func TestEcho(t *testing.T) {
	sockPath := Fmt("unix:///tmp/echo_%v.sock", RandStr(6))

	// Start server
	_, err := server.StartListener(sockPath, example.NewDummyApplication())
	if err != nil {
		Exit(err.Error())
	}
	// Start client
	proxy, err := NewRemoteAppConn(sockPath)
	if err != nil {
		Exit(err.Error())
	} else {
		t.Log("Connected")
	}

	for i := 0; i < 1000; i++ {
		proxy.EchoAsync(Fmt("echo-%v", i))
	}
	proxy.FlushSync()
}

func BenchmarkEcho(b *testing.B) {
	b.StopTimer() // Initialize
	sockPath := Fmt("unix:///tmp/echo_%v.sock", RandStr(6))
	// Start server
	_, err := server.StartListener(sockPath, example.NewDummyApplication())
	if err != nil {
		Exit(err.Error())
	}
	// Start client
	proxy, err := NewRemoteAppConn(sockPath)
	if err != nil {
		Exit(err.Error())
	} else {
		b.Log("Connected")
	}
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
	// Start server
	_, err := server.StartListener(sockPath, example.NewDummyApplication())
	if err != nil {
		Exit(err.Error())
	}
	// Start client
	proxy, err := NewRemoteAppConn(sockPath)
	if err != nil {
		Exit(err.Error())
	} else {
		t.Log("Connected")
	}
	proxy.Start()
	data, err := proxy.InfoSync()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if data != "size:0" {
		t.Error("Expected ResponseInfo with one element 'size:0' but got something else")
	}
}
