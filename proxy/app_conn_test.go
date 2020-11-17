package proxy

import (
	"context"
	"fmt"
	"strings"
	"testing"

	abcicli "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/abci/server"
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	tmrand "github.com/tendermint/tendermint/libs/rand"
)

//----------------------------------------

type appConnTestI interface {
	EchoAsync(string) (*abcicli.ReqRes, error)
	FlushSync(context.Context) error
	InfoSync(context.Context, types.RequestInfo) (*types.ResponseInfo, error)
}

type appConnTest struct {
	appConn abcicli.Client
}

func newAppConnTest(appConn abcicli.Client) appConnTestI {
	return &appConnTest{appConn}
}

func (app *appConnTest) EchoAsync(msg string) (*abcicli.ReqRes, error) {
	return app.appConn.EchoAsync(msg)
}

func (app *appConnTest) FlushSync(ctx context.Context) error {
	return app.appConn.FlushSync(ctx)
}

func (app *appConnTest) InfoSync(ctx context.Context, req types.RequestInfo) (*types.ResponseInfo, error) {
	return app.appConn.InfoSync(ctx, req)
}

//----------------------------------------

var SOCKET = "socket"

func TestEcho(t *testing.T) {
	sockPath := fmt.Sprintf("unix:///tmp/echo_%v.sock", tmrand.Str(6))
	clientCreator := NewRemoteClientCreator(sockPath, SOCKET, true)

	// Start server
	s := server.NewSocketServer(sockPath, kvstore.NewApplication())
	s.SetLogger(log.TestingLogger().With("module", "abci-server"))
	if err := s.Start(); err != nil {
		t.Fatalf("Error starting socket server: %v", err.Error())
	}
	t.Cleanup(func() {
		if err := s.Stop(); err != nil {
			t.Error(err)
		}
	})

	// Start client
	cli, err := clientCreator.NewABCIClient()
	if err != nil {
		t.Fatalf("Error creating ABCI client: %v", err.Error())
	}
	cli.SetLogger(log.TestingLogger().With("module", "abci-client"))
	if err := cli.Start(); err != nil {
		t.Fatalf("Error starting ABCI client: %v", err.Error())
	}

	proxy := newAppConnTest(cli)
	t.Log("Connected")

	for i := 0; i < 1000; i++ {
		_, err = proxy.EchoAsync(fmt.Sprintf("echo-%v", i))
		if err != nil {
			t.Error(err)
		}
	}
	if err := proxy.FlushSync(context.Background()); err != nil {
		t.Error(err)
	}
}

func BenchmarkEcho(b *testing.B) {
	b.StopTimer() // Initialize
	sockPath := fmt.Sprintf("unix:///tmp/echo_%v.sock", tmrand.Str(6))
	clientCreator := NewRemoteClientCreator(sockPath, SOCKET, true)

	// Start server
	s := server.NewSocketServer(sockPath, kvstore.NewApplication())
	s.SetLogger(log.TestingLogger().With("module", "abci-server"))
	if err := s.Start(); err != nil {
		b.Fatalf("Error starting socket server: %v", err.Error())
	}
	b.Cleanup(func() {
		if err := s.Stop(); err != nil {
			b.Error(err)
		}
	})

	// Start client
	cli, err := clientCreator.NewABCIClient()
	if err != nil {
		b.Fatalf("Error creating ABCI client: %v", err.Error())
	}
	cli.SetLogger(log.TestingLogger().With("module", "abci-client"))
	if err := cli.Start(); err != nil {
		b.Fatalf("Error starting ABCI client: %v", err.Error())
	}

	proxy := newAppConnTest(cli)
	b.Log("Connected")
	echoString := strings.Repeat(" ", 200)
	b.StartTimer() // Start benchmarking tests

	for i := 0; i < b.N; i++ {
		_, err = proxy.EchoAsync(echoString)
		if err != nil {
			b.Error(err)
		}
	}
	if err := proxy.FlushSync(context.Background()); err != nil {
		b.Error(err)
	}

	b.StopTimer()
	// info := proxy.InfoSync(types.RequestInfo{""})
	// b.Log("N: ", b.N, info)
}

func TestInfo(t *testing.T) {
	sockPath := fmt.Sprintf("unix:///tmp/echo_%v.sock", tmrand.Str(6))
	clientCreator := NewRemoteClientCreator(sockPath, SOCKET, true)

	// Start server
	s := server.NewSocketServer(sockPath, kvstore.NewApplication())
	s.SetLogger(log.TestingLogger().With("module", "abci-server"))
	if err := s.Start(); err != nil {
		t.Fatalf("Error starting socket server: %v", err.Error())
	}
	t.Cleanup(func() {
		if err := s.Stop(); err != nil {
			t.Error(err)
		}
	})

	// Start client
	cli, err := clientCreator.NewABCIClient()
	if err != nil {
		t.Fatalf("Error creating ABCI client: %v", err.Error())
	}
	cli.SetLogger(log.TestingLogger().With("module", "abci-client"))
	if err := cli.Start(); err != nil {
		t.Fatalf("Error starting ABCI client: %v", err.Error())
	}

	proxy := newAppConnTest(cli)
	t.Log("Connected")

	resInfo, err := proxy.InfoSync(context.Background(), RequestInfo)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if resInfo.Data != "{\"size\":0}" {
		t.Error("Expected ResponseInfo with one element '{\"size\":0}' but got something else")
	}
}
