package proxy

import (
	"strings"
	"testing"

	abcicli "github.com/tendermint/abci/client"
	"github.com/tendermint/abci/example/dummy"
	"github.com/tendermint/abci/server"
	"github.com/tendermint/abci/types"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"
)

//----------------------------------------

type AppConnTest interface {
	EchoAsync(string) *abcicli.ReqRes
	FlushSync() error
	InfoSync() (types.ResponseInfo, error)
}

type appConnTest struct {
	appConn abcicli.Client
}

func NewAppConnTest(appConn abcicli.Client) AppConnTest {
	return &appConnTest{appConn}
}

func (app *appConnTest) EchoAsync(msg string) *abcicli.ReqRes {
	return app.appConn.EchoAsync(msg)
}

func (app *appConnTest) FlushSync() error {
	return app.appConn.FlushSync()
}

func (app *appConnTest) InfoSync() (types.ResponseInfo, error) {
	return app.appConn.InfoSync()
}

//----------------------------------------

var SOCKET = "socket"

func TestEcho(t *testing.T) {
	sockPath := cmn.Fmt("unix:///tmp/echo_%v.sock", cmn.RandStr(6))
	clientCreator := NewRemoteClientCreator(sockPath, SOCKET, true)

	// Start server
	s := server.NewSocketServer(sockPath, dummy.NewDummyApplication())
	s.SetLogger(log.TestingLogger().With("module", "abci-server"))
	if _, err := s.Start(); err != nil {
		t.Fatalf("Error starting socket server: %v", err.Error())
	}
	defer s.Stop()

	// Start client
	cli, err := clientCreator.NewABCIClient()
	if err != nil {
		t.Fatalf("Error creating ABCI client: %v", err.Error())
	}
	cli.SetLogger(log.TestingLogger().With("module", "abci-client"))
	if _, err := cli.Start(); err != nil {
		t.Fatalf("Error starting ABCI client: %v", err.Error())
	}

	proxy := NewAppConnTest(cli)
	t.Log("Connected")

	for i := 0; i < 1000; i++ {
		proxy.EchoAsync(cmn.Fmt("echo-%v", i))
	}
	proxy.FlushSync()
}

func BenchmarkEcho(b *testing.B) {
	b.StopTimer() // Initialize
	sockPath := cmn.Fmt("unix:///tmp/echo_%v.sock", cmn.RandStr(6))
	clientCreator := NewRemoteClientCreator(sockPath, SOCKET, true)

	// Start server
	s := server.NewSocketServer(sockPath, dummy.NewDummyApplication())
	s.SetLogger(log.TestingLogger().With("module", "abci-server"))
	if _, err := s.Start(); err != nil {
		b.Fatalf("Error starting socket server: %v", err.Error())
	}
	defer s.Stop()

	// Start client
	cli, err := clientCreator.NewABCIClient()
	if err != nil {
		b.Fatalf("Error creating ABCI client: %v", err.Error())
	}
	cli.SetLogger(log.TestingLogger().With("module", "abci-client"))
	if _, err := cli.Start(); err != nil {
		b.Fatalf("Error starting ABCI client: %v", err.Error())
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
	sockPath := cmn.Fmt("unix:///tmp/echo_%v.sock", cmn.RandStr(6))
	clientCreator := NewRemoteClientCreator(sockPath, SOCKET, true)

	// Start server
	s := server.NewSocketServer(sockPath, dummy.NewDummyApplication())
	s.SetLogger(log.TestingLogger().With("module", "abci-server"))
	if _, err := s.Start(); err != nil {
		t.Fatalf("Error starting socket server: %v", err.Error())
	}
	defer s.Stop()

	// Start client
	cli, err := clientCreator.NewABCIClient()
	if err != nil {
		t.Fatalf("Error creating ABCI client: %v", err.Error())
	}
	cli.SetLogger(log.TestingLogger().With("module", "abci-client"))
	if _, err := cli.Start(); err != nil {
		t.Fatalf("Error starting ABCI client: %v", err.Error())
	}

	proxy := NewAppConnTest(cli)
	t.Log("Connected")

	resInfo, err := proxy.InfoSync()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if string(resInfo.Data) != "{\"size\":0}" {
		t.Error("Expected ResponseInfo with one element '{\"size\":0}' but got something else")
	}
}
