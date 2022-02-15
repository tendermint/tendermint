package proxy

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	"github.com/tendermint/tendermint/abci/server"
	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	tmrand "github.com/tendermint/tendermint/libs/rand"
)

//----------------------------------------

type appConnTestI interface {
	Echo(context.Context, string) (*types.ResponseEcho, error)
	Flush(context.Context) error
	Info(context.Context, types.RequestInfo) (*types.ResponseInfo, error)
}

type appConnTest struct {
	appConn abciclient.Client
}

func newAppConnTest(appConn abciclient.Client) appConnTestI {
	return &appConnTest{appConn}
}

func (app *appConnTest) Echo(ctx context.Context, msg string) (*types.ResponseEcho, error) {
	return app.appConn.Echo(ctx, msg)
}

func (app *appConnTest) Flush(ctx context.Context) error {
	return app.appConn.Flush(ctx)
}

func (app *appConnTest) Info(ctx context.Context, req types.RequestInfo) (*types.ResponseInfo, error) {
	return app.appConn.Info(ctx, req)
}

//----------------------------------------

var SOCKET = "socket"

func TestEcho(t *testing.T) {
	sockPath := fmt.Sprintf("unix:///tmp/echo_%v.sock", tmrand.Str(6))
	logger := log.TestingLogger()
	clientCreator := abciclient.NewRemoteCreator(logger, sockPath, SOCKET, true)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server
	s := server.NewSocketServer(logger.With("module", "abci-server"), sockPath, kvstore.NewApplication())
	require.NoError(t, s.Start(ctx), "error starting socket server")
	t.Cleanup(func() { cancel(); s.Wait() })

	// Start client
	cli, err := clientCreator(logger.With("module", "abci-client"))
	require.NoError(t, err, "Error creating ABCI client:")

	require.NoError(t, cli.Start(ctx), "Error starting ABCI client")

	proxy := newAppConnTest(cli)
	t.Log("Connected")

	for i := 0; i < 1000; i++ {
		_, err = proxy.Echo(ctx, fmt.Sprintf("echo-%v", i))
		if err != nil {
			t.Error(err)
		}
		// flush sometimes
		if i%128 == 0 {
			if err := proxy.Flush(ctx); err != nil {
				t.Error(err)
			}
		}
	}
	if err := proxy.Flush(ctx); err != nil {
		t.Error(err)
	}
}

func BenchmarkEcho(b *testing.B) {
	b.StopTimer() // Initialize
	sockPath := fmt.Sprintf("unix:///tmp/echo_%v.sock", tmrand.Str(6))
	logger := log.TestingLogger()
	clientCreator := abciclient.NewRemoteCreator(logger, sockPath, SOCKET, true)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server
	s := server.NewSocketServer(logger.With("module", "abci-server"), sockPath, kvstore.NewApplication())
	require.NoError(b, s.Start(ctx), "Error starting socket server")
	b.Cleanup(func() { cancel(); s.Wait() })

	// Start client
	cli, err := clientCreator(logger.With("module", "abci-client"))
	require.NoError(b, err, "Error creating ABCI client")

	require.NoError(b, cli.Start(ctx), "Error starting ABCI client")

	proxy := newAppConnTest(cli)
	b.Log("Connected")
	echoString := strings.Repeat(" ", 200)
	b.StartTimer() // Start benchmarking tests

	for i := 0; i < b.N; i++ {
		_, err = proxy.Echo(ctx, echoString)
		if err != nil {
			b.Error(err)
		}
		// flush sometimes
		if i%128 == 0 {
			if err := proxy.Flush(ctx); err != nil {
				b.Error(err)
			}
		}
	}
	if err := proxy.Flush(ctx); err != nil {
		b.Error(err)
	}

	b.StopTimer()
	// info := proxy.Info(types.RequestInfo{""})
	// b.Log("N: ", b.N, info)
}

func TestInfo(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sockPath := fmt.Sprintf("unix:///tmp/echo_%v.sock", tmrand.Str(6))
	logger := log.TestingLogger()
	clientCreator := abciclient.NewRemoteCreator(logger, sockPath, SOCKET, true)

	// Start server
	s := server.NewSocketServer(logger.With("module", "abci-server"), sockPath, kvstore.NewApplication())
	require.NoError(t, s.Start(ctx), "Error starting socket server")
	t.Cleanup(func() { cancel(); s.Wait() })

	// Start client
	cli, err := clientCreator(logger.With("module", "abci-client"))
	require.NoError(t, err, "Error creating ABCI client")

	require.NoError(t, cli.Start(ctx), "Error starting ABCI client")

	proxy := newAppConnTest(cli)
	t.Log("Connected")

	resInfo, err := proxy.Info(ctx, RequestInfo)
	require.NoError(t, err)

	if resInfo.Data != "{\"size\":0}" {
		t.Error("Expected ResponseInfo with one element '{\"size\":0}' but got something else")
	}
}
