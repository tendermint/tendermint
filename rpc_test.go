package rpc

import (
	"net/http"
	"testing"
	"time"

	client "github.com/tendermint/go-rpc/client"
	server "github.com/tendermint/go-rpc/server"
	types "github.com/tendermint/go-rpc/types"
	wire "github.com/tendermint/go-wire"
)

// Client and Server should work over tcp or unix sockets
const (
	tcpAddr  = "tcp://0.0.0.0:46657"
	unixAddr = "unix:///tmp/go-rpc.sock" // NOTE: must remove file for test to run again

	websocketEndpoint = "/websocket/endpoint"
)

// Define a type for results and register concrete versions
type Result interface{}

type ResultStatus struct {
	Value string
}

var _ = wire.RegisterInterface(
	struct{ Result }{},
	wire.ConcreteType{&ResultStatus{}, 0x1},
)

// Define some routes
var Routes = map[string]*server.RPCFunc{
	"status": server.NewRPCFunc(StatusResult, "arg"),
}

// an rpc function
func StatusResult(v string) (Result, error) {
	return &ResultStatus{v}, nil
}

// launch unix and tcp servers
func init() {

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

	msg := <-cl.ResultsCh
	result := new(Result)
	wire.ReadJSONPtr(result, msg, &err)
	if err != nil {
		t.Fatal(err)
	}
	got := (*result).(*ResultStatus).Value
	if got != val {
		t.Fatalf("Got: %v   ....   Expected: %v \n", got, val)
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
