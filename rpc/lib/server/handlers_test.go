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
	"github.com/tendermint/tendermint/libs/log"
	rs "github.com/tendermint/tendermint/rpc/lib/server"
	types "github.com/tendermint/tendermint/rpc/lib/types"
)

//////////////////////////////////////////////////////////////////////////////
// HTTP REST API
// TODO

//////////////////////////////////////////////////////////////////////////////
// JSON-RPC over HTTP

func testMux() *http.ServeMux {
	funcMap := map[string]*rs.RPCFunc{
		"c": rs.NewRPCFunc(func(ctx *types.Context, s string, i int) (string, error) { return "foo", nil }, "s,i"),
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
		payload    string
		wantErr    string
		expectedId interface{}
	}{
		// bad
		{`{"jsonrpc": "2.0", "id": "0"}`, "Method not found", types.JSONRPCStringID("0")},
		{`{"jsonrpc": "2.0", "method": "y", "id": "0"}`, "Method not found", types.JSONRPCStringID("0")},
		{`{"method": "c", "id": "0", "params": a}`, "invalid character", types.JSONRPCStringID("")}, // id not captured in JSON parsing failures
		{`{"method": "c", "id": "0", "params": ["a"]}`, "got 1", types.JSONRPCStringID("0")},
		{`{"method": "c", "id": "0", "params": ["a", "b"]}`, "invalid character", types.JSONRPCStringID("0")},
		{`{"method": "c", "id": "0", "params": [1, 1]}`, "of type string", types.JSONRPCStringID("0")},

		// good
		{`{"jsonrpc": "2.0", "method": "c", "id": "0", "params": null}`, "", types.JSONRPCStringID("0")},
		{`{"method": "c", "id": "0", "params": {}}`, "", types.JSONRPCStringID("0")},
		{`{"method": "c", "id": "0", "params": ["a", "10"]}`, "", types.JSONRPCStringID("0")},
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
		assert.Equal(t, tt.expectedId, recv.ID, "#%d: expected ID not matched in RPCResponse", i)
		if tt.wantErr == "" {
			assert.Nil(t, recv.Error, "#%d: not expecting an error", i)
		} else {
			assert.True(t, recv.Error.Code < 0, "#%d: not expecting a positive JSONRPC code", i)
			// The wanted error is either in the message or the data
			assert.Contains(t, recv.Error.Message+recv.Error.Data, tt.wantErr, "#%d: expected substring", i)
		}
	}
}

func TestJSONRPCID(t *testing.T) {
	mux := testMux()
	tests := []struct {
		payload    string
		wantErr    bool
		expectedId interface{}
	}{
		// good id
		{`{"jsonrpc": "2.0", "method": "c", "id": "0", "params": ["a", "10"]}`, false, types.JSONRPCStringID("0")},
		{`{"jsonrpc": "2.0", "method": "c", "id": "abc", "params": ["a", "10"]}`, false, types.JSONRPCStringID("abc")},
		{`{"jsonrpc": "2.0", "method": "c", "id": 0, "params": ["a", "10"]}`, false, types.JSONRPCIntID(0)},
		{`{"jsonrpc": "2.0", "method": "c", "id": 1, "params": ["a", "10"]}`, false, types.JSONRPCIntID(1)},
		{`{"jsonrpc": "2.0", "method": "c", "id": 1.3, "params": ["a", "10"]}`, false, types.JSONRPCIntID(1)},
		{`{"jsonrpc": "2.0", "method": "c", "id": -1, "params": ["a", "10"]}`, false, types.JSONRPCIntID(-1)},
		{`{"jsonrpc": "2.0", "method": "c", "id": null, "params": ["a", "10"]}`, false, nil},

		// bad id
		{`{"jsonrpc": "2.0", "method": "c", "id": {}, "params": ["a", "10"]}`, true, nil},
		{`{"jsonrpc": "2.0", "method": "c", "id": [], "params": ["a", "10"]}`, true, nil},
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
		err = json.Unmarshal(blob, recv)
		assert.Nil(t, err, "#%d: expecting successful parsing of an RPCResponse:\nblob: %s", i, blob)
		if !tt.wantErr {
			assert.NotEqual(t, recv, new(types.RPCResponse), "#%d: not expecting a blank RPCResponse", i)
			assert.Equal(t, tt.expectedId, recv.ID, "#%d: expected ID not matched in RPCResponse", i)
			assert.Nil(t, recv.Error, "#%d: not expecting an error", i)
		} else {
			assert.True(t, recv.Error.Code < 0, "#%d: not expecting a positive JSONRPC code", i)
		}
	}
}

func TestRPCNotification(t *testing.T) {
	mux := testMux()
	body := strings.NewReader(`{"jsonrpc": "2.0", "id": ""}`)
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

func TestRPCNotificationInBatch(t *testing.T) {
	mux := testMux()
	tests := []struct {
		payload     string
		expectCount int
	}{
		{
			`[
				{"jsonrpc": "2.0","id": ""},
				{"jsonrpc": "2.0","method":"c","id":"abc","params":["a","10"]}
			 ]`,
			1,
		},
		{
			`[
				{"jsonrpc": "2.0","id": ""},
				{"jsonrpc": "2.0","method":"c","id":"abc","params":["a","10"]},
				{"jsonrpc": "2.0","id": ""},
				{"jsonrpc": "2.0","method":"c","id":"abc","params":["a","10"]}
			 ]`,
			2,
		},
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

		var responses []types.RPCResponse
		// try to unmarshal an array first
		err = json.Unmarshal(blob, &responses)
		if err != nil {
			// if we were actually expecting an array, but got an error
			if tt.expectCount > 1 {
				t.Errorf("#%d: expected an array, couldn't unmarshal it\nblob: %s", i, blob)
				continue
			} else {
				// we were expecting an error here, so let's unmarshal a single response
				var response types.RPCResponse
				err = json.Unmarshal(blob, &response)
				if err != nil {
					t.Errorf("#%d: expected successful parsing of an RPCResponse\nblob: %s", i, blob)
					continue
				}
				// have a single-element result
				responses = []types.RPCResponse{response}
			}
		}
		if tt.expectCount != len(responses) {
			t.Errorf("#%d: expected %d response(s), but got %d\nblob: %s", i, tt.expectCount, len(responses), blob)
			continue
		}
		for _, response := range responses {
			assert.NotEqual(t, response, new(types.RPCResponse), "#%d: not expecting a blank RPCResponse", i)
		}
	}
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
	req, err := types.MapToRequest(amino.NewCodec(), types.JSONRPCStringID("TestWebsocketManager"), "c", map[string]interface{}{"s": "a", "i": 10})
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
		"c": rs.NewWSRPCFunc(func(ctx *types.Context, s string, i int) (string, error) { return "foo", nil }, "s,i"),
	}
	wm := rs.NewWebsocketManager(funcMap, amino.NewCodec())
	wm.SetLogger(log.TestingLogger())

	mux := http.NewServeMux()
	mux.HandleFunc("/websocket", wm.WebsocketHandler)

	return httptest.NewServer(mux)
}
