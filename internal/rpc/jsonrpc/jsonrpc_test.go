package jsonrpc

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/go-kit/log/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/libs/log"
	tmrand "github.com/tendermint/tendermint/libs/rand"

	client "github.com/tendermint/tendermint/rpc/jsonrpc/client"
	server "github.com/tendermint/tendermint/rpc/jsonrpc/server"
	types "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

// Client and Server should work over tcp or unix sockets
const (
	tcpAddr = "tcp://127.0.0.1:47768"

	unixSocket = "/tmp/rpc_test.sock"
	unixAddr   = "unix://" + unixSocket

	websocketEndpoint = "/websocket/endpoint"

	testVal = "acbd"
)

var (
	ctx = context.Background()
)

type ResultEcho struct {
	Value string `json:"value"`
}

type ResultEchoInt struct {
	Value int `json:"value"`
}

type ResultEchoBytes struct {
	Value []byte `json:"value"`
}

type ResultEchoDataBytes struct {
	Value tmbytes.HexBytes `json:"value"`
}

// Define some routes
var Routes = map[string]*server.RPCFunc{
	"echo":            server.NewRPCFunc(EchoResult, "arg"),
	"echo_ws":         server.NewWSRPCFunc(EchoWSResult, "arg"),
	"echo_bytes":      server.NewRPCFunc(EchoBytesResult, "arg"),
	"echo_data_bytes": server.NewRPCFunc(EchoDataBytesResult, "arg"),
	"echo_int":        server.NewRPCFunc(EchoIntResult, "arg"),
}

func EchoResult(ctx *types.Context, v string) (*ResultEcho, error) {
	return &ResultEcho{v}, nil
}

func EchoWSResult(ctx *types.Context, v string) (*ResultEcho, error) {
	return &ResultEcho{v}, nil
}

func EchoIntResult(ctx *types.Context, v int) (*ResultEchoInt, error) {
	return &ResultEchoInt{v}, nil
}

func EchoBytesResult(ctx *types.Context, v []byte) (*ResultEchoBytes, error) {
	return &ResultEchoBytes{v}, nil
}

func EchoDataBytesResult(ctx *types.Context, v tmbytes.HexBytes) (*ResultEchoDataBytes, error) {
	return &ResultEchoDataBytes{v}, nil
}

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	os.Exit(code)
}

var colorFn = func(keyvals ...interface{}) term.FgBgColor {
	for i := 0; i < len(keyvals)-1; i += 2 {
		if keyvals[i] == "socket" {
			if keyvals[i+1] == "tcp" {
				return term.FgBgColor{Fg: term.DarkBlue}
			} else if keyvals[i+1] == "unix" {
				return term.FgBgColor{Fg: term.DarkCyan}
			}
		}
	}
	return term.FgBgColor{}
}

// launch unix and tcp servers
func setup() {
	logger := log.NewTMLoggerWithColorFn(log.NewSyncWriter(os.Stdout), colorFn)

	cmd := exec.Command("rm", "-f", unixSocket)
	err := cmd.Start()
	if err != nil {
		panic(err)
	}
	if err = cmd.Wait(); err != nil {
		panic(err)
	}

	tcpLogger := logger.With("socket", "tcp")
	mux := http.NewServeMux()
	server.RegisterRPCFuncs(mux, Routes, tcpLogger)
	wm := server.NewWebsocketManager(Routes, server.ReadWait(5*time.Second), server.PingPeriod(1*time.Second))
	wm.SetLogger(tcpLogger)
	mux.HandleFunc(websocketEndpoint, wm.WebsocketHandler)
	config := server.DefaultConfig()
	listener1, err := server.Listen(tcpAddr, config)
	if err != nil {
		panic(err)
	}
	go func() {
		if err := server.Serve(listener1, mux, tcpLogger, config); err != nil {
			panic(err)
		}
	}()

	unixLogger := logger.With("socket", "unix")
	mux2 := http.NewServeMux()
	server.RegisterRPCFuncs(mux2, Routes, unixLogger)
	wm = server.NewWebsocketManager(Routes)
	wm.SetLogger(unixLogger)
	mux2.HandleFunc(websocketEndpoint, wm.WebsocketHandler)
	listener2, err := server.Listen(unixAddr, config)
	if err != nil {
		panic(err)
	}
	go func() {
		if err := server.Serve(listener2, mux2, unixLogger, config); err != nil {
			panic(err)
		}
	}()

	// wait for servers to start
	time.Sleep(time.Second * 2)
}

func echoViaHTTP(cl client.Caller, val string) (string, error) {
	params := map[string]interface{}{
		"arg": val,
	}
	result := new(ResultEcho)
	if _, err := cl.Call(ctx, "echo", params, result); err != nil {
		return "", err
	}
	return result.Value, nil
}

func echoIntViaHTTP(cl client.Caller, val int) (int, error) {
	params := map[string]interface{}{
		"arg": val,
	}
	result := new(ResultEchoInt)
	if _, err := cl.Call(ctx, "echo_int", params, result); err != nil {
		return 0, err
	}
	return result.Value, nil
}

func echoBytesViaHTTP(cl client.Caller, bytes []byte) ([]byte, error) {
	params := map[string]interface{}{
		"arg": bytes,
	}
	result := new(ResultEchoBytes)
	if _, err := cl.Call(ctx, "echo_bytes", params, result); err != nil {
		return []byte{}, err
	}
	return result.Value, nil
}

func echoDataBytesViaHTTP(cl client.Caller, bytes tmbytes.HexBytes) (tmbytes.HexBytes, error) {
	params := map[string]interface{}{
		"arg": bytes,
	}
	result := new(ResultEchoDataBytes)
	if _, err := cl.Call(ctx, "echo_data_bytes", params, result); err != nil {
		return []byte{}, err
	}
	return result.Value, nil
}

func testWithHTTPClient(t *testing.T, cl client.HTTPClient) {
	val := testVal
	got, err := echoViaHTTP(cl, val)
	require.Nil(t, err)
	assert.Equal(t, got, val)

	val2 := randBytes(t)
	got2, err := echoBytesViaHTTP(cl, val2)
	require.Nil(t, err)
	assert.Equal(t, got2, val2)

	val3 := tmbytes.HexBytes(randBytes(t))
	got3, err := echoDataBytesViaHTTP(cl, val3)
	require.Nil(t, err)
	assert.Equal(t, got3, val3)

	val4 := tmrand.Intn(10000)
	got4, err := echoIntViaHTTP(cl, val4)
	require.Nil(t, err)
	assert.Equal(t, got4, val4)
}

func echoViaWS(cl *client.WSClient, val string) (string, error) {
	params := map[string]interface{}{
		"arg": val,
	}
	err := cl.Call(context.Background(), "echo", params)
	if err != nil {
		return "", err
	}

	msg := <-cl.ResponsesCh
	if msg.Error != nil {
		return "", err

	}
	result := new(ResultEcho)
	err = json.Unmarshal(msg.Result, result)
	if err != nil {
		return "", nil
	}
	return result.Value, nil
}

func echoBytesViaWS(cl *client.WSClient, bytes []byte) ([]byte, error) {
	params := map[string]interface{}{
		"arg": bytes,
	}
	err := cl.Call(context.Background(), "echo_bytes", params)
	if err != nil {
		return []byte{}, err
	}

	msg := <-cl.ResponsesCh
	if msg.Error != nil {
		return []byte{}, msg.Error

	}
	result := new(ResultEchoBytes)
	err = json.Unmarshal(msg.Result, result)
	if err != nil {
		return []byte{}, nil
	}
	return result.Value, nil
}

func testWithWSClient(t *testing.T, cl *client.WSClient) {
	val := testVal
	got, err := echoViaWS(cl, val)
	require.Nil(t, err)
	assert.Equal(t, got, val)

	val2 := randBytes(t)
	got2, err := echoBytesViaWS(cl, val2)
	require.Nil(t, err)
	assert.Equal(t, got2, val2)
}

//-------------

func TestServersAndClientsBasic(t *testing.T) {
	serverAddrs := [...]string{tcpAddr, unixAddr}
	for _, addr := range serverAddrs {
		cl1, err := client.NewURI(addr)
		require.Nil(t, err)
		fmt.Printf("=== testing server on %s using URI client", addr)
		testWithHTTPClient(t, cl1)

		cl2, err := client.New(addr)
		require.Nil(t, err)
		fmt.Printf("=== testing server on %s using JSONRPC client", addr)
		testWithHTTPClient(t, cl2)

		cl3, err := client.NewWS(addr, websocketEndpoint)
		require.Nil(t, err)
		cl3.SetLogger(log.TestingLogger())
		err = cl3.Start()
		require.Nil(t, err)
		fmt.Printf("=== testing server on %s using WS client", addr)
		testWithWSClient(t, cl3)
		err = cl3.Stop()
		require.NoError(t, err)
	}
}

func TestHexStringArg(t *testing.T) {
	cl, err := client.NewURI(tcpAddr)
	require.Nil(t, err)
	// should NOT be handled as hex
	val := "0xabc"
	got, err := echoViaHTTP(cl, val)
	require.Nil(t, err)
	assert.Equal(t, got, val)
}

func TestQuotedStringArg(t *testing.T) {
	cl, err := client.NewURI(tcpAddr)
	require.Nil(t, err)
	// should NOT be unquoted
	val := "\"abc\""
	got, err := echoViaHTTP(cl, val)
	require.Nil(t, err)
	assert.Equal(t, got, val)
}

func TestWSNewWSRPCFunc(t *testing.T) {
	cl, err := client.NewWS(tcpAddr, websocketEndpoint)
	require.Nil(t, err)
	cl.SetLogger(log.TestingLogger())
	err = cl.Start()
	require.Nil(t, err)
	t.Cleanup(func() {
		if err := cl.Stop(); err != nil {
			t.Error(err)
		}
	})

	val := testVal
	params := map[string]interface{}{
		"arg": val,
	}
	err = cl.Call(context.Background(), "echo_ws", params)
	require.Nil(t, err)

	msg := <-cl.ResponsesCh
	if msg.Error != nil {
		t.Fatal(err)
	}
	result := new(ResultEcho)
	err = json.Unmarshal(msg.Result, result)
	require.Nil(t, err)
	got := result.Value
	assert.Equal(t, got, val)
}

func TestWSHandlesArrayParams(t *testing.T) {
	cl, err := client.NewWS(tcpAddr, websocketEndpoint)
	require.Nil(t, err)
	cl.SetLogger(log.TestingLogger())
	err = cl.Start()
	require.Nil(t, err)
	t.Cleanup(func() {
		if err := cl.Stop(); err != nil {
			t.Error(err)
		}
	})

	val := testVal
	params := []interface{}{val}
	err = cl.CallWithArrayParams(context.Background(), "echo_ws", params)
	require.Nil(t, err)

	msg := <-cl.ResponsesCh
	if msg.Error != nil {
		t.Fatalf("%+v", err)
	}
	result := new(ResultEcho)
	err = json.Unmarshal(msg.Result, result)
	require.Nil(t, err)
	got := result.Value
	assert.Equal(t, got, val)
}

// TestWSClientPingPong checks that a client & server exchange pings
// & pongs so connection stays alive.
func TestWSClientPingPong(t *testing.T) {
	cl, err := client.NewWS(tcpAddr, websocketEndpoint)
	require.Nil(t, err)
	cl.SetLogger(log.TestingLogger())
	err = cl.Start()
	require.Nil(t, err)
	t.Cleanup(func() {
		if err := cl.Stop(); err != nil {
			t.Error(err)
		}
	})

	time.Sleep(6 * time.Second)
}

func randBytes(t *testing.T) []byte {
	n := tmrand.Intn(10) + 2
	buf := make([]byte, n)
	_, err := crand.Read(buf)
	require.Nil(t, err)
	return bytes.ReplaceAll(buf, []byte("="), []byte{100})
}
