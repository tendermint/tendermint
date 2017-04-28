package rpc

import (
	"bytes"
	crand "crypto/rand"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/go-wire/data"
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

// Define a type for results and register concrete versions
type ResultInner interface{}

type Result struct {
	ResultInner `json:"unwrap"`
}

func (r Result) MarshalJSON() ([]byte, error) {
	return resultMapper.ToJSON(r.ResultInner)
}

func (r *Result) UnmarshalJSON(data []byte) (err error) {
	parsed, err := resultMapper.FromJSON(data)
	if err == nil && parsed != nil {
		r.ResultInner = parsed.(ResultInner)
	}
	return
}

func (r Result) Unwrap() ResultInner {
	tmrI := r.ResultInner
	for wrap, ok := tmrI.(Result); ok; wrap, ok = tmrI.(Result) {
		tmrI = wrap.ResultInner
	}
	return tmrI
}

func (r Result) Empty() bool {
	return r.ResultInner == nil
}

type ResultEcho struct {
	Value string
}

type ResultEchoInt struct {
	Value int
}

type ResultEchoBytes struct {
	Value []byte
}

type ResultEchoDataBytes struct {
	Value data.Bytes
}

var resultMapper = data.NewMapper(Result{}).
	RegisterImplementation(&ResultEcho{}, "echo", 0x1).
	RegisterImplementation(&ResultEchoBytes{}, "echo_bytes", 0x2).
	RegisterImplementation(&ResultEchoDataBytes{}, "echo_data_bytes", 0x3).
	RegisterImplementation(&ResultEchoInt{}, "echo_int", 0x4)

// Define some routes
var Routes = map[string]*server.RPCFunc{
	"echo":            server.NewRPCFunc(EchoResult, "arg"),
	"echo_ws":         server.NewWSRPCFunc(EchoWSResult, "arg"),
	"echo_bytes":      server.NewRPCFunc(EchoBytesResult, "arg"),
	"echo_data_bytes": server.NewRPCFunc(EchoDataBytesResult, "arg"),
	"echo_int":        server.NewRPCFunc(EchoIntResult, "arg"),
}

func EchoResult(v string) (Result, error) {
	return Result{&ResultEcho{v}}, nil
}

func EchoWSResult(wsCtx types.WSRPCContext, v string) (Result, error) {
	return Result{&ResultEcho{v}}, nil
}

func EchoIntResult(v int) (Result, error) {
	return Result{&ResultEchoInt{v}}, nil
}

func EchoBytesResult(v []byte) (Result, error) {
	return Result{&ResultEchoBytes{v}}, nil
}

func EchoDataBytesResult(v data.Bytes) (Result, error) {
	return Result{&ResultEchoDataBytes{v}}, nil
}

// launch unix and tcp servers
func init() {
	cmd := exec.Command("rm", "-f", unixSocket)
	err := cmd.Start()
	if err != nil {
		panic(err)
	}
	if err = cmd.Wait(); err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	server.RegisterRPCFuncs(mux, Routes)
	wm := server.NewWebsocketManager(Routes, nil)
	mux.HandleFunc(websocketEndpoint, wm.WebsocketHandler)
	go func() {
		_, err := server.StartHTTPServer(tcpAddr, mux)
		if err != nil {
			panic(err)
		}
	}()

	mux2 := http.NewServeMux()
	server.RegisterRPCFuncs(mux2, Routes)
	wm = server.NewWebsocketManager(Routes, nil)
	mux2.HandleFunc(websocketEndpoint, wm.WebsocketHandler)
	go func() {
		_, err := server.StartHTTPServer(unixAddr, mux2)
		if err != nil {
			panic(err)
		}
	}()

	// wait for servers to start
	time.Sleep(time.Second * 2)
}

func echoViaHTTP(cl client.HTTPClient, val string) (string, error) {
	params := map[string]interface{}{
		"arg": val,
	}
	var result Result
	if _, err := cl.Call("echo", params, &result); err != nil {
		return "", err
	}
	return result.Unwrap().(*ResultEcho).Value, nil
}

func echoIntViaHTTP(cl client.HTTPClient, val int) (int, error) {
	params := map[string]interface{}{
		"arg": val,
	}
	var result Result
	if _, err := cl.Call("echo_int", params, &result); err != nil {
		return 0, err
	}
	return result.Unwrap().(*ResultEchoInt).Value, nil
}

func echoBytesViaHTTP(cl client.HTTPClient, bytes []byte) ([]byte, error) {
	params := map[string]interface{}{
		"arg": bytes,
	}
	var result Result
	if _, err := cl.Call("echo_bytes", params, &result); err != nil {
		return []byte{}, err
	}
	return result.Unwrap().(*ResultEchoBytes).Value, nil
}

func echoDataBytesViaHTTP(cl client.HTTPClient, bytes data.Bytes) (data.Bytes, error) {
	params := map[string]interface{}{
		"arg": bytes,
	}
	var result Result
	if _, err := cl.Call("echo_data_bytes", params, &result); err != nil {
		return []byte{}, err
	}
	return result.Unwrap().(*ResultEchoDataBytes).Value, nil
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

	val3 := data.Bytes(randBytes(t))
	got3, err := echoDataBytesViaHTTP(cl, val3)
	require.Nil(t, err)
	assert.Equal(t, got3, val3)

	/*
		val4 := rand.Intn(10000)
		got4, err := echoIntViaHTTP(cl, val4)
		require.Nil(t, err)
		assert.Equal(t, got4, val4)
	*/
}

func echoViaWS(cl *client.WSClient, val string) (string, error) {
	params := map[string]interface{}{
		"arg": val,
	}
	err := cl.Call("echo", params)
	if err != nil {
		return "", err
	}

	select {
	case msg := <-cl.ResultsCh:
		result := new(Result)
		err = json.Unmarshal(msg, result)
		if err != nil {
			return "", nil
		}
		return result.Unwrap().(*ResultEcho).Value, nil
	case err := <-cl.ErrorsCh:
		return "", err
	}
}

func echoBytesViaWS(cl *client.WSClient, bytes []byte) ([]byte, error) {
	params := map[string]interface{}{
		"arg": bytes,
	}
	err := cl.Call("echo_bytes", params)
	if err != nil {
		return []byte{}, err
	}

	select {
	case msg := <-cl.ResultsCh:
		result := new(Result)
		err = json.Unmarshal(msg, result)
		if err != nil {
			return []byte{}, nil
		}
		return result.Unwrap().(*ResultEchoBytes).Value, nil
	case err := <-cl.ErrorsCh:
		return []byte{}, err
	}
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
		fmt.Printf("=== testing server on %s using %v client", addr, cl1)
		testWithHTTPClient(t, cl1)

		cl2 := client.NewJSONRPCClient(tcpAddr)
		fmt.Printf("=== testing server on %s using %v client", addr, cl2)
		testWithHTTPClient(t, cl2)

		cl3 := client.NewWSClient(tcpAddr, websocketEndpoint)
		_, err := cl3.Start()
		require.Nil(t, err)
		fmt.Printf("=== testing server on %s using %v client", addr, cl3)
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
	_, err := cl.Start()
	require.Nil(t, err)
	defer cl.Stop()

	val := "acbd"
	params := map[string]interface{}{
		"arg": val,
	}
	err = cl.WriteJSON(types.RPCRequest{
		JSONRPC: "2.0",
		ID:      "",
		Method:  "echo_ws",
		Params:  params,
	})
	require.Nil(t, err)

	select {
	case msg := <-cl.ResultsCh:
		result := new(Result)
		err = json.Unmarshal(msg, result)
		require.Nil(t, err)
		got := result.Unwrap().(*ResultEcho).Value
		assert.Equal(t, got, val)
	case err := <-cl.ErrorsCh:
		t.Fatal(err)
	}
}

func TestWSHandlesArrayParams(t *testing.T) {
	cl := client.NewWSClient(tcpAddr, websocketEndpoint)
	_, err := cl.Start()
	require.Nil(t, err)
	defer cl.Stop()

	val := "acbd"
	params := []interface{}{val}
	err = cl.WriteJSON(types.RPCRequest{
		JSONRPC: "2.0",
		ID:      "",
		Method:  "echo_ws",
		Params:  params,
	})
	require.Nil(t, err)

	select {
	case msg := <-cl.ResultsCh:
		result := new(Result)
		err = json.Unmarshal(msg, result)
		require.Nil(t, err)
		got := result.Unwrap().(*ResultEcho).Value
		assert.Equal(t, got, val)
	case err := <-cl.ErrorsCh:
		t.Fatalf("%+v", err)
	}
}

func randBytes(t *testing.T) []byte {
	n := rand.Intn(10) + 2
	buf := make([]byte, n)
	_, err := crand.Read(buf)
	require.Nil(t, err)
	return bytes.Replace(buf, []byte("="), []byte{100}, -1)
}
