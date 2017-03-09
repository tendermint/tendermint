package rpc

import (
	"bytes"
	crand "crypto/rand"
	"math/rand"
	"net/http"
	"os/exec"
	"testing"
	"time"

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

func testURI(t *testing.T, cl *client.ClientURI) {
	val := "acbd"
	params := map[string]interface{}{
		"arg": val,
	}
	var result Result
	_, err := cl.Call("status", params, &result)
	if err != nil {
		t.Fatal(err)
	}
	got := result.(*ResultStatus).Value
	if got != val {
		t.Fatalf("Got: %v   ....   Expected: %v \n", got, val)
	}
}

func testJSONRPC(t *testing.T, cl *client.ClientJSONRPC) {
	val := "acbd"
	params := map[string]interface{}{
		"arg": val,
	}
	var result Result
	_, err := cl.Call("status", params, &result)
	if err != nil {
		t.Fatal(err)
	}
	got := result.(*ResultStatus).Value
	if got != val {
		t.Fatalf("Got: %v   ....   Expected: %v \n", got, val)
	}
}

func testWS(t *testing.T, cl *client.WSClient) {
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
	if err != nil {
		t.Fatal(err)
	}

	select {
	case msg := <-cl.ResultsCh:
		result := new(Result)
		wire.ReadJSONPtr(result, msg, &err)
		if err != nil {
			t.Fatal(err)
		}
		got := (*result).(*ResultStatus).Value
		if got != val {
			t.Fatalf("Got: %v   ....   Expected: %v \n", got, val)
		}
	case err := <-cl.ErrorsCh:
		t.Fatal(err)
	}
}

//-------------

func TestURI_TCP(t *testing.T) {
	cl := client.NewClientURI(tcpAddr)
	testURI(t, cl)
}

func TestURI_UNIX(t *testing.T) {
	cl := client.NewClientURI(unixAddr)
	testURI(t, cl)
}

func TestJSONRPC_TCP(t *testing.T) {
	cl := client.NewClientJSONRPC(tcpAddr)
	testJSONRPC(t, cl)
}

func TestJSONRPC_UNIX(t *testing.T) {
	cl := client.NewClientJSONRPC(unixAddr)
	testJSONRPC(t, cl)
}

func TestWS_TCP(t *testing.T) {
	cl := client.NewWSClient(tcpAddr, websocketEndpoint)
	_, err := cl.Start()
	if err != nil {
		t.Fatal(err)
	}
	testWS(t, cl)
}

func TestWS_UNIX(t *testing.T) {
	cl := client.NewWSClient(unixAddr, websocketEndpoint)
	_, err := cl.Start()
	if err != nil {
		t.Fatal(err)
	}
	testWS(t, cl)
}

func TestHexStringArg(t *testing.T) {
	cl := client.NewClientURI(tcpAddr)
	// should NOT be handled as hex
	val := "0xabc"
	params := map[string]interface{}{
		"arg": val,
	}
	var result Result
	_, err := cl.Call("status", params, &result)
	if err != nil {
		t.Fatal(err)
	}
	got := result.(*ResultStatus).Value
	if got != val {
		t.Fatalf("Got: %v   ....   Expected: %v \n", got, val)
	}
}

func TestQuotedStringArg(t *testing.T) {
	cl := client.NewClientURI(tcpAddr)
	// should NOT be unquoted
	val := "\"abc\""
	params := map[string]interface{}{
		"arg": val,
	}
	var result Result
	_, err := cl.Call("status", params, &result)
	if err != nil {
		t.Fatal(err)
	}
	got := result.(*ResultStatus).Value
	if got != val {
		t.Fatalf("Got: %v   ....   Expected: %v \n", got, val)
	}
}

func randBytes(t *testing.T) []byte {
	n := rand.Intn(10) + 2
	buf := make([]byte, n)
	_, err := crand.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.Replace(buf, []byte("="), []byte{100}, -1)
}

func TestByteSliceViaJSONRPC(t *testing.T) {
	cl := client.NewClientJSONRPC(unixAddr)

	val := randBytes(t)
	params := map[string]interface{}{
		"arg": val,
	}
	var result Result
	_, err := cl.Call("bytes", params, &result)
	if err != nil {
		t.Fatal(err)
	}
	got := result.(*ResultBytes).Value
	if bytes.Compare(got, val) != 0 {
		t.Fatalf("Got: %v   ....   Expected: %v \n", got, val)
	}
}

func TestWSNewWSRPCFunc(t *testing.T) {
	cl := client.NewWSClient(unixAddr, websocketEndpoint)
	_, err := cl.Start()
	if err != nil {
		t.Fatal(err)
	}
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
	if err != nil {
		t.Fatal(err)
	}

	select {
	case msg := <-cl.ResultsCh:
		result := new(Result)
		wire.ReadJSONPtr(result, msg, &err)
		if err != nil {
			t.Fatal(err)
		}
		got := (*result).(*ResultStatus).Value
		if got != val {
			t.Fatalf("Got: %v   ....   Expected: %v \n", got, val)
		}
	case err := <-cl.ErrorsCh:
		t.Fatal(err)
	}
}
