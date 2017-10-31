package rpcserver_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rs "github.com/tendermint/tendermint/rpc/lib/server"
	types "github.com/tendermint/tendermint/rpc/lib/types"
	"github.com/tendermint/tmlibs/log"
)

func testMux() *http.ServeMux {
	funcMap := map[string]*rs.RPCFunc{
		"c": rs.NewRPCFunc(func(s string, i int) (string, error) { return "foo", nil }, "s,i"),
	}
	mux := http.NewServeMux()
	buf := new(bytes.Buffer)
	logger := log.NewTMLogger(buf)
	rs.RegisterRPCFuncs(mux, funcMap, logger)

	return mux
}

func statusOK(code int) bool { return code >= 200 && code <= 299 }

// Ensure that nefarious/unintended inputs to `params`
// do not crash our RPC handlers.
// See Issue https://github.com/tendermint/tendermint/issues/708.
func TestRPCParams(t *testing.T) {
	mux := testMux()
	tests := []struct {
		payload string
		wantErr string
	}{
		// bad
		{`{"jsonrpc": "2.0", "id": "0"}`, "Method not found"},
		{`{"jsonrpc": "2.0", "method": "y", "id": "0"}`, "Method not found"},
		{`{"method": "c", "id": "0", "params": a}`, "invalid character"},
		{`{"method": "c", "id": "0", "params": ["a"]}`, "got 1"},
		{`{"method": "c", "id": "0", "params": ["a", "b"]}`, "of type int"},
		{`{"method": "c", "id": "0", "params": [1, 1]}`, "of type string"},

		// good
		{`{"jsonrpc": "2.0", "method": "c", "id": "0", "params": null}`, ""},
		{`{"method": "c", "id": "0", "params": {}}`, ""},
		{`{"method": "c", "id": "0", "params": ["a", 10]}`, ""},
	}

	for i, tt := range tests {
		req, _ := http.NewRequest("POST", "http://localhost/", strings.NewReader(tt.payload))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		res := rec.Result()
		// Always expecting back a JSONRPCResponse
		assert.True(t, statusOK(res.StatusCode), "#%d: should always return 2XX", i)
		blob, err := ioutil.ReadAll(res.Body)
		if err != nil {
			t.Errorf("#%d: err reading body: %v", i, err)
			continue
		}

		recv := new(types.RPCResponse)
		assert.Nil(t, json.Unmarshal(blob, recv), "#%d: expecting successful parsing of an RPCResponse:\nblob: %s", i, blob)
		assert.NotEqual(t, recv, new(types.RPCResponse), "#%d: not expecting a blank RPCResponse", i)

		if tt.wantErr == "" {
			assert.Nil(t, recv.Error, "#%d: not expecting an error", i)
		} else {
			assert.True(t, recv.Error.Code < 0, "#%d: not expecting a positive JSONRPC code", i)
			// The wanted error is either in the message or the data
			assert.Contains(t, recv.Error.Message+recv.Error.Data, tt.wantErr, "#%d: expected substring", i)
		}
	}
}

func TestRPCNotification(t *testing.T) {
	mux := testMux()
	body := strings.NewReader(`{"jsonrpc": "2.0"}`)
	req, _ := http.NewRequest("POST", "http://localhost/", body)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	res := rec.Result()

	// Always expecting back a JSONRPCResponse
	require.True(t, statusOK(res.StatusCode), "should always return 2XX")
	blob, err := ioutil.ReadAll(res.Body)
	require.Nil(t, err, "reading from the body should not give back an error")
	require.Equal(t, len(blob), 0, "a notification SHOULD NOT be responded to by the server")
}
