package rpc

import (
	"net/http"
	"testing"
	"time"

	"github.com/tendermint/go-rpc/client"
	"github.com/tendermint/go-rpc/server"
	"github.com/tendermint/go-wire"
)

// Client and Server should work over tcp or unix sockets
var (
	tcpAddr  = "0.0.0.0:46657"
	unixAddr = "/tmp/go-rpc.sock" // NOTE: must remove file for test to run again
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
var Routes = map[string]*rpcserver.RPCFunc{
	"status": rpcserver.NewRPCFunc(StatusResult, "arg"),
}

// an rpc function
func StatusResult(v string) (Result, error) {
	return &ResultStatus{v}, nil
}

// launch unix and tcp servers
func init() {
	mux := http.NewServeMux()
	rpcserver.RegisterRPCFuncs(mux, Routes)
	go func() {
		_, err := rpcserver.StartHTTPServer(tcpAddr, mux)
		if err != nil {
			panic(err)
		}
	}()

	mux = http.NewServeMux()
	rpcserver.RegisterRPCFuncs(mux, Routes)
	go func() {
		_, err := rpcserver.StartHTTPServer(unixAddr, mux)
		if err != nil {
			panic(err)
		}
	}()

	// wait for servers to start
	time.Sleep(time.Second * 2)

}

func testURI(t *testing.T, cl *rpcclient.ClientURI) {
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

func testJSONRPC(t *testing.T, cl *rpcclient.ClientJSONRPC) {
	val := "acbd"
	params := []interface{}{val}
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

func TestURI_TCP(t *testing.T) {
	cl := rpcclient.NewClientURI(tcpAddr)
	testURI(t, cl)
}

func TestURI_UNIX(t *testing.T) {
	cl := rpcclient.NewClientURI(unixAddr)
	testURI(t, cl)
}

func TestJSONRPC_TCP(t *testing.T) {
	cl := rpcclient.NewClientJSONRPC(tcpAddr)
	testJSONRPC(t, cl)
}

func TestJSONRPC_UNIX(t *testing.T) {
	cl := rpcclient.NewClientJSONRPC(unixAddr)
	testJSONRPC(t, cl)
}
