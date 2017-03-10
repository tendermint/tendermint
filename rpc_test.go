package rpc

import (
	"bytes"
	crand "crypto/rand"
	"fmt"
	"math/rand"
	"net/http"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	client "github.com/tendermint/go-rpc/client"
	server "github.com/tendermint/go-rpc/server"
	types "github.com/tendermint/go-rpc/types"
	wire "github.com/tendermint/go-wire"
)

// Client and Server should work over tcp or unix sockets
const (
	tcpAddr = "tcp://0.0.0.0:46657"

	unixSocket = "/tmp/go-rpc.sock"
	unixAddr   = "unix:///tmp/go-rpc.sock"

	websocketEndpoint = "/websocket/endpoint"
)

// Define a type for results and register concrete versions
type Result interface{}

type ResultStatus struct {
	Value string
}

type ResultBytes struct {
	Value []byte
}

var _ = wire.RegisterInterface(
	struct{ Result }{},
	wire.ConcreteType{&ResultStatus{}, 0x1},
	wire.ConcreteType{&ResultBytes{}, 0x2},
)

// Define some routes
var Routes = map[string]*server.RPCFunc{
	"status":    server.NewRPCFunc(StatusResult, "arg"),
	"status_ws": server.NewWSRPCFunc(StatusWSResult, "arg"),
	"bytes":     server.NewRPCFunc(BytesResult, "arg"),
}

// an rpc function
func StatusResult(v string) (Result, error) {
	return &ResultStatus{v}, nil
}

func StatusWSResult(wsCtx types.WSRPCContext, v string) (Result, error) {
	return &ResultStatus{v}, nil
}

func BytesResult(v []byte) (Result, error) {
	return &ResultBytes{v}, nil
}

// launch unix and tcp servers
func init() {
	cmd := exec.Command("rm", "-f", unixSocket)
	err := cmd.Start()
	if err != nil {
		panic(err)
	}
	err = cmd.Wait()

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

func status(cl client.HTTPClient, val string) (string, error) {
	params := map[string]interface{}{
		"arg": val,
	}
	var result Result
	if _, err := cl.Call("status", params, &result); err != nil {
		return "", err
	}
	return result.(*ResultStatus).Value, nil
}

func testWithHTTPClient(t *testing.T, cl client.HTTPClient) {
	val := "acbd"
	got, err := status(cl, val)
	require.Nil(t, err)
	assert.Equal(t, got, val)
}

func testWithWSClient(t *testing.T, cl *client.WSClient) {
	val := "acbd"
	params := map[string]interface{}{
		"arg": val,
	}
	err := cl.WriteJSON(types.RPCRequest{
		JSONRPC: "2.0",
		ID:      "",
		Method:  "status",
		Params:  params,
	})
	require.Nil(t, err)

	select {
	case msg := <-cl.ResultsCh:
		result := new(Result)
		wire.ReadJSONPtr(result, msg, &err)
		require.Nil(t, err)
		got := (*result).(*ResultStatus).Value
		assert.Equal(t, got, val)
	case err := <-cl.ErrorsCh:
		t.Fatal(err)
	}
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
	got, err := status(cl, val)
	require.Nil(t, err)
	assert.Equal(t, got, val)
}

func TestQuotedStringArg(t *testing.T) {
	cl := client.NewURIClient(tcpAddr)
	// should NOT be unquoted
	val := "\"abc\""
	got, err := status(cl, val)
	require.Nil(t, err)
	assert.Equal(t, got, val)
}

func randBytes(t *testing.T) []byte {
	n := rand.Intn(10) + 2
	buf := make([]byte, n)
	_, err := crand.Read(buf)
	require.Nil(t, err)
	return bytes.Replace(buf, []byte("="), []byte{100}, -1)
}

func TestByteSliceViaJSONRPC(t *testing.T) {
	cl := client.NewJSONRPCClient(unixAddr)

	val := randBytes(t)
	params := map[string]interface{}{
		"arg": val,
	}
	var result Result
	_, err := cl.Call("bytes", params, &result)
	require.Nil(t, err)
	got := result.(*ResultBytes).Value
	assert.Equal(t, got, val)
}

func TestWSNewWSRPCFunc(t *testing.T) {
	cl := client.NewWSClient(unixAddr, websocketEndpoint)
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
		Method:  "status_ws",
		Params:  params,
	})
	require.Nil(t, err)

	select {
	case msg := <-cl.ResultsCh:
		result := new(Result)
		wire.ReadJSONPtr(result, msg, &err)
		require.Nil(t, err)
		got := (*result).(*ResultStatus).Value
		assert.Equal(t, got, val)
	case err := <-cl.ErrorsCh:
		t.Fatal(err)
	}
}
