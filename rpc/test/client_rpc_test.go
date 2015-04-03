package rpc

import (
	"testing"
)

//--------------------------------------------------------------------------------
// Test the HTTP client

func TestHTTPStatus(t *testing.T) {
	testStatus(t, "HTTP")
}

func TestHTTPGenPriv(t *testing.T) {
	testGenPriv(t, "HTTP")
}

func TestHTTPGetAccount(t *testing.T) {
	testGetAccount(t, "HTTP")
}

func TestHTTPSignedTx(t *testing.T) {
	testSignedTx(t, "HTTP")
}

func TestHTTPBroadcastTx(t *testing.T) {
	testBroadcastTx(t, "HTTP")
}

func TestHTTPGetStorage(t *testing.T) {
	testGetStorage(t, "HTTP")
}

func TestHTTPCallCode(t *testing.T) {
	testCallCode(t, "HTTP")
}

func TestHTTPCallContract(t *testing.T) {
	testCall(t, "HTTP")
}

//--------------------------------------------------------------------------------
// Test the JSONRPC client

func TestJSONStatus(t *testing.T) {
	testStatus(t, "JSONRPC")
}

func TestJSONGenPriv(t *testing.T) {
	testGenPriv(t, "JSONRPC")
}

func TestJSONGetAccount(t *testing.T) {
	testGetAccount(t, "JSONRPC")
}

func TestJSONSignedTx(t *testing.T) {
	testSignedTx(t, "JSONRPC")
}

func TestJSONBroadcastTx(t *testing.T) {
	testBroadcastTx(t, "JSONRPC")
}

func TestJSONGetStorage(t *testing.T) {
	testGetStorage(t, "JSONRPC")
}

func TestJSONCallCode(t *testing.T) {
	testCallCode(t, "JSONRPC")
}

func TestJSONCallContract(t *testing.T) {
	testCall(t, "JSONRPC")
}
