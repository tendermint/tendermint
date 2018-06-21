package rpcserver_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	amino "github.com/tendermint/go-amino"
	rs "github.com/tendermint/tendermint/rpc/lib/server"
	types "github.com/tendermint/tendermint/rpc/lib/types"
	"github.com/tendermint/tmlibs/log"
)

//////////////////////////////////////////////////////////////////////////////
// HTTP REST API
// TODO

//////////////////////////////////////////////////////////////////////////////
// JSON-RPC over HTTP

func testMux() *http.ServeMux {
	funcMap := map[string]*rs.RPCFunc{
		"c": rs.NewRPCFunc(func(s string, i int) (string, error) { return "foo", nil }, "s,i"),
	}
	cdc := amino.NewCodec()
	mux := http.NewServeMux()
	buf := new(bytes.Buffer)
	logger := log.NewTMLogger(buf)
	rs.RegisterRPCFuncs(mux, funcMap, cdc, logger)

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
		{`{"method": "c", "id": "0", "params": ["a", "b"]}`, "invalid character"},
		{`{"method": "c", "id": "0", "params": [1, 1]}`, "of type string"},

		// good
		{`{"jsonrpc": "2.0", "method": "c", "id": "0", "params": null}`, ""},
		{`{"method": "c", "id": "0", "params": {}}`, ""},
		{`{"method": "c", "id": "0", "params": ["a", "10"]}`, ""},
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

func TestUnknownRPCPath(t *testing.T) {
	mux := testMux()
	req, _ := http.NewRequest("GET", "http://localhost/unknownrpcpath", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	res := rec.Result()

	// Always expecting back a 404 error
	require.Equal(t, http.StatusNotFound, res.StatusCode, "should always return 404")
}

//////////////////////////////////////////////////////////////////////////////
// JSON-RPC over WEBSOCKETS

func TestWebsocketManagerHandler(t *testing.T) {
	s := newWSServer()
	defer s.Close()

	// check upgrader works
	d := websocket.Dialer{}
	c, dialResp, err := d.Dial("ws://"+s.Listener.Addr().String()+"/websocket", nil)
	require.NoError(t, err)

	if got, want := dialResp.StatusCode, http.StatusSwitchingProtocols; got != want {
		t.Errorf("dialResp.StatusCode = %q, want %q", got, want)
	}

	// check basic functionality works
	req, err := types.MapToRequest(amino.NewCodec(), "TestWebsocketManager", "c", map[string]interface{}{"s": "a", "i": 10})
	require.NoError(t, err)
	err = c.WriteJSON(req)
	require.NoError(t, err)

	var resp types.RPCResponse
	err = c.ReadJSON(&resp)
	require.NoError(t, err)
	require.Nil(t, resp.Error)
}

func newWSServer() *httptest.Server {
	funcMap := map[string]*rs.RPCFunc{
		"c": rs.NewWSRPCFunc(func(wsCtx types.WSRPCContext, s string, i int) (string, error) { return "foo", nil }, "s,i"),
	}
	wm := rs.NewWebsocketManager(funcMap, amino.NewCodec())
	wm.SetLogger(log.TestingLogger())

	mux := http.NewServeMux()
	mux.HandleFunc("/websocket", wm.WebsocketHandler)

	return httptest.NewServer(mux)
}
