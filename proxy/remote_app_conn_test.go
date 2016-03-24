package proxy

import (
	"strings"
	"testing"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/tmsp/example/dummy"
	"github.com/tendermint/tmsp/server"
)

func TestEcho(t *testing.T) {
	sockPath := Fmt("unix:///tmp/echo_%v.sock", RandStr(6))

	// Start server
	s, err := server.NewServer(sockPath, dummy.NewDummyApplication())
	if err != nil {
		Exit(err.Error())
	}
	defer s.Stop()
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
	s, err := server.NewServer(sockPath, dummy.NewDummyApplication())
	if err != nil {
		Exit(err.Error())
	}
	defer s.Stop()
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
	s, err := server.NewServer(sockPath, dummy.NewDummyApplication())
	if err != nil {
		Exit(err.Error())
	}
	defer s.Stop()
	// Start client
	proxy, err := NewRemoteAppConn(sockPath)
	if err != nil {
		Exit(err.Error())
	} else {
		t.Log("Connected")
	}
	res := proxy.InfoSync()
	if res.IsErr() {
		t.Errorf("Unexpected error: %v", err)
	}
	if string(res.Data) != "size:0" {
		t.Error("Expected ResponseInfo with one element 'size:0' but got something else")
	}
}
