package rpc

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

	"github.com/go-kit/kit/log/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	amino "github.com/tendermint/go-amino"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"

	client "github.com/tendermint/tendermint/rpc/lib/client"
	server "github.com/tendermint/tendermint/rpc/lib/server"
	types "github.com/tendermint/tendermint/rpc/lib/types"
)

// Client and Server should work over tcp or unix sockets
const (
	tcpAddr = "tcp://0.0.0.0:47768"

	unixSocket = "/tmp/rpc_test.sock"
	unixAddr   = "unix://" + unixSocket

	websocketEndpoint = "/websocket/endpoint"
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
	Value cmn.HexBytes `json:"value"`
}

// Define some routes
var Routes = map[string]*server.RPCFunc{
	"echo":            server.NewRPCFunc(EchoResult, "arg"),
	"echo_ws":         server.NewWSRPCFunc(EchoWSResult, "arg"),
	"echo_bytes":      server.NewRPCFunc(EchoBytesResult, "arg"),
	"echo_data_bytes": server.NewRPCFunc(EchoDataBytesResult, "arg"),
	"echo_int":        server.NewRPCFunc(EchoIntResult, "arg"),
}

// Amino codec required to encode/decode everything above.
var RoutesCdc = amino.NewCodec()

func EchoResult(v string) (*ResultEcho, error) {
	return &ResultEcho{v}, nil
}

func EchoWSResult(wsCtx types.WSRPCContext, v string) (*ResultEcho, error) {
	return &ResultEcho{v}, nil
}

func EchoIntResult(v int) (*ResultEchoInt, error) {
	return &ResultEchoInt{v}, nil
}

func EchoBytesResult(v []byte) (*ResultEchoBytes, error) {
	return &ResultEchoBytes{v}, nil
}

func EchoDataBytesResult(v cmn.HexBytes) (*ResultEchoDataBytes, error) {
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
	server.RegisterRPCFuncs(mux, Routes, RoutesCdc, tcpLogger)
	wm := server.NewWebsocketManager(Routes, RoutesCdc, server.ReadWait(5*time.Second), server.PingPeriod(1*time.Second))
	wm.SetLogger(tcpLogger)
	mux.HandleFunc(websocketEndpoint, wm.WebsocketHandler)
	listener1, err := server.Listen(tcpAddr, server.Config{})
	if err != nil {
		panic(err)
	}
	go server.StartHTTPServer(listener1, mux, tcpLogger)

	unixLogger := logger.With("socket", "unix")
	mux2 := http.NewServeMux()
	server.RegisterRPCFuncs(mux2, Routes, RoutesCdc, unixLogger)
	wm = server.NewWebsocketManager(Routes, RoutesCdc)
	wm.SetLogger(unixLogger)
	mux2.HandleFunc(websocketEndpoint, wm.WebsocketHandler)
	listener2, err := server.Listen(unixAddr, server.Config{})
	if err != nil {
		panic(err)
	}
	go server.StartHTTPServer(listener2, mux2, unixLogger)

	// wait for servers to start
	time.Sleep(time.Second * 2)
}

func echoViaHTTP(cl client.HTTPClient, val string) (string, error) {
	params := map[string]interface{}{
		"arg": val,
	}
	result := new(ResultEcho)
	if _, err := cl.Call("echo", params, result); err != nil {
		return "", err
	}
	return result.Value, nil
}

func echoIntViaHTTP(cl client.HTTPClient, val int) (int, error) {
	params := map[string]interface{}{
		"arg": val,
	}
	result := new(ResultEchoInt)
	if _, err := cl.Call("echo_int", params, result); err != nil {
		return 0, err
	}
	return result.Value, nil
}

func echoBytesViaHTTP(cl client.HTTPClient, bytes []byte) ([]byte, error) {
	params := map[string]interface{}{
		"arg": bytes,
	}
	result := new(ResultEchoBytes)
	if _, err := cl.Call("echo_bytes", params, result); err != nil {
		return []byte{}, err
	}
	return result.Value, nil
}

func echoDataBytesViaHTTP(cl client.HTTPClient, bytes cmn.HexBytes) (cmn.HexBytes, error) {
	params := map[string]interface{}{
		"arg": bytes,
	}
	result := new(ResultEchoDataBytes)
	if _, err := cl.Call("echo_data_bytes", params, result); err != nil {
		return []byte{}, err
	}
	return result.Value, nil
}

func testWithHTTPClient(t *testing.T, cl client.HTTPClient) {
	val := "acbd"
	got, err := echoViaHTTP(cl, val)
	require.Nil(t, err)
	assert.Equal(t, got, val)

	val2 := randBytes(t)
	got2, err := echoBytesViaHTTP(cl, val2)
	require.Nil(t, err)
	assert.Equal(t, got2, val2)

	val3 := cmn.HexBytes(randBytes(t))
	got3, err := echoDataBytesViaHTTP(cl, val3)
	require.Nil(t, err)
	assert.Equal(t, got3, val3)

	val4 := cmn.RandIntn(10000)
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
	val := "acbd"
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
		cl1 := client.NewURIClient(addr)
		fmt.Printf("=== testing server on %s using URI client", addr)
		testWithHTTPClient(t, cl1)

		cl2 := client.NewJSONRPCClient(addr)
		fmt.Printf("=== testing server on %s using JSONRPC client", addr)
		testWithHTTPClient(t, cl2)

		cl3 := client.NewWSClient(addr, websocketEndpoint)
		cl3.SetLogger(log.TestingLogger())
		err := cl3.Start()
		require.Nil(t, err)
		fmt.Printf("=== testing server on %s using WS client", addr)
		testWithWSClient(t, cl3)
		cl3.Stop()
	}
}

func TestHexStringArg(t *testing.T) {
	cl := client.NewURIClient(tcpAddr)
	// should NOT be handled as hex
	val := "0xabc"
	got, err := echoViaHTTP(cl, val)
	require.Nil(t, err)
	assert.Equal(t, got, val)
}

func TestQuotedStringArg(t *testing.T) {
	cl := client.NewURIClient(tcpAddr)
	// should NOT be unquoted
	val := "\"abc\""
	got, err := echoViaHTTP(cl, val)
	require.Nil(t, err)
	assert.Equal(t, got, val)
}

func TestWSNewWSRPCFunc(t *testing.T) {
	cl := client.NewWSClient(tcpAddr, websocketEndpoint)
	cl.SetLogger(log.TestingLogger())
	err := cl.Start()
	require.Nil(t, err)
	defer cl.Stop()

	val := "acbd"
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
	cl := client.NewWSClient(tcpAddr, websocketEndpoint)
	cl.SetLogger(log.TestingLogger())
	err := cl.Start()
	require.Nil(t, err)
	defer cl.Stop()

	val := "acbd"
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
	cl := client.NewWSClient(tcpAddr, websocketEndpoint)
	cl.SetLogger(log.TestingLogger())
	err := cl.Start()
	require.Nil(t, err)
	defer cl.Stop()

	time.Sleep(6 * time.Second)
}

func randBytes(t *testing.T) []byte {
	n := cmn.RandIntn(10) + 2
	buf := make([]byte, n)
	_, err := crand.Read(buf)
	require.Nil(t, err)
	return bytes.Replace(buf, []byte("="), []byte{100}, -1)
}
