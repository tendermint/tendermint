package proxy

import (
	"strings"
	"testing"

	. "github.com/tendermint/go-common"
	tmspcli "github.com/tendermint/tmsp/client"
	"github.com/tendermint/tmsp/example/dummy"
	"github.com/tendermint/tmsp/server"
	"github.com/tendermint/tmsp/types"
)

//----------------------------------------

type AppConnTest interface {
	EchoAsync(string) *tmspcli.ReqRes
	FlushSync() error
	InfoSync() (res types.Result)
}

type appConnTest struct {
	appConn tmspcli.Client
}

func NewAppConnTest(appConn tmspcli.Client) AppConnTest {
	return &appConnTest{appConn}
}

func (app *appConnTest) EchoAsync(msg string) *tmspcli.ReqRes {
	return app.appConn.EchoAsync(msg)
}

func (app *appConnTest) FlushSync() error {
	return app.appConn.FlushSync()
}

func (app *appConnTest) InfoSync() types.Result {
	return app.appConn.InfoSync()
}

//----------------------------------------

var SOCKET = "socket"

func TestEcho(t *testing.T) {
	sockPath := Fmt("unix:///tmp/echo_%v.sock", RandStr(6))
	clientCreator := NewRemoteClientCreator(sockPath, SOCKET, true)

	// Start server
	s, err := server.NewSocketServer(sockPath, dummy.NewDummyApplication())
	if err != nil {
		Exit(err.Error())
	}
	defer s.Stop()
	// Start client
	cli, err := clientCreator.NewTMSPClient()
	if err != nil {
		Exit(err.Error())
	}
	proxy := NewAppConnTest(cli)
	t.Log("Connected")

	for i := 0; i < 1000; i++ {
		proxy.EchoAsync(Fmt("echo-%v", i))
	}
	proxy.FlushSync()
}

func BenchmarkEcho(b *testing.B) {
	b.StopTimer() // Initialize
	sockPath := Fmt("unix:///tmp/echo_%v.sock", RandStr(6))
	clientCreator := NewRemoteClientCreator(sockPath, SOCKET, true)
	// Start server
	s, err := server.NewSocketServer(sockPath, dummy.NewDummyApplication())
	if err != nil {
		Exit(err.Error())
	}
	defer s.Stop()
	// Start client
	cli, err := clientCreator.NewTMSPClient()
	if err != nil {
		Exit(err.Error())
	}
	proxy := NewAppConnTest(cli)
	b.Log("Connected")
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
	clientCreator := NewRemoteClientCreator(sockPath, SOCKET, true)
	// Start server
	s, err := server.NewSocketServer(sockPath, dummy.NewDummyApplication())
	if err != nil {
		Exit(err.Error())
	}
	defer s.Stop()
	// Start client
	cli, err := clientCreator.NewTMSPClient()
	if err != nil {
		Exit(err.Error())
	}
	proxy := NewAppConnTest(cli)
	t.Log("Connected")

	res := proxy.InfoSync()
	if res.IsErr() {
		t.Errorf("Unexpected error: %v", err)
	}
	if string(res.Data) != "size:0" {
		t.Error("Expected ResponseInfo with one element 'size:0' but got something else")
	}
}
