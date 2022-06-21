package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/log"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

func testMux() *http.ServeMux {
	type testArgs struct {
		S string      `json:"s"`
		I json.Number `json:"i"`
	}
	type blockArgs struct {
		H json.Number `json:"h"`
	}
	funcMap := map[string]*RPCFunc{
		"c":     NewRPCFunc(func(ctx context.Context, arg *testArgs) (string, error) { return "foo", nil }),
		"block": NewRPCFunc(func(ctx context.Context, arg *blockArgs) (string, error) { return "block", nil }),
	}
	mux := http.NewServeMux()
	logger := log.NewNopLogger()
	RegisterRPCFuncs(mux, funcMap, logger)

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
		expectedID string
	}{
		// bad
		{`{"jsonrpc": "2.0", "id": "0"}`, "Method not found", `"0"`},
		{`{"jsonrpc": "2.0", "method": "y", "id": "0"}`, "Method not found", `"0"`},
		// id not captured in JSON parsing failures
		{`{"method": "c", "id": "0", "params": a}`, "invalid character", ""},
		{`{"method": "c", "id": "0", "params": ["a"]}`, "got 1", `"0"`},
		{`{"method": "c", "id": "0", "params": ["a", "b"]}`, "invalid number", `"0"`},
		{`{"method": "c", "id": "0", "params": [1, 1]}`, "of type string", `"0"`},

		// no ID - notification
		// {`{"jsonrpc": "2.0", "method": "c", "params": ["a", "10"]}`, false, nil},

		// good
		{`{"jsonrpc": "2.0", "method": "c", "id": "0", "params": null}`, "", `"0"`},
		{`{"method": "c", "id": "0", "params": {}}`, "", `"0"`},
		{`{"method": "c", "id": "0", "params": ["a", "10"]}`, "", `"0"`},
	}

	for i, tt := range tests {
		req, _ := http.NewRequest("POST", "http://localhost/", strings.NewReader(tt.payload))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		res := rec.Result()

		// Always expecting back a JSONRPCResponse
		assert.NotZero(t, res.StatusCode, "#%d: should always return code", i)
		blob, err := io.ReadAll(res.Body)
		require.NoError(t, err, "#%d: reading body", i)
		require.NoError(t, res.Body.Close())

		recv := new(rpctypes.RPCResponse)
		assert.Nil(t, json.Unmarshal(blob, recv), "#%d: expecting successful parsing of an RPCResponse:\nblob: %s", i, blob)
		assert.NotEqual(t, recv, new(rpctypes.RPCResponse), "#%d: not expecting a blank RPCResponse", i)
		assert.Equal(t, tt.expectedID, recv.ID(), "#%d: expected ID not matched in RPCResponse", i)
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
		expectedID string
	}{
		// good id
		{`{"jsonrpc": "2.0", "method": "c", "id": "0", "params": ["a", "10"]}`, false, `"0"`},
		{`{"jsonrpc": "2.0", "method": "c", "id": "abc", "params": ["a", "10"]}`, false, `"abc"`},
		{`{"jsonrpc": "2.0", "method": "c", "id": 0, "params": ["a", "10"]}`, false, `0`},
		{`{"jsonrpc": "2.0", "method": "c", "id": 1, "params": ["a", "10"]}`, false, `1`},
		{`{"jsonrpc": "2.0", "method": "c", "id": -1, "params": ["a", "10"]}`, false, `-1`},

		// bad id
		{`{"jsonrpc": "2.0", "method": "c", "id": {}, "params": ["a", "10"]}`, true, ""},  // object
		{`{"jsonrpc": "2.0", "method": "c", "id": [], "params": ["a", "10"]}`, true, ""},  // array
		{`{"jsonrpc": "2.0", "method": "c", "id": 1.3, "params": ["a", "10"]}`, true, ""}, // fractional
	}

	for i, tt := range tests {
		req, _ := http.NewRequest("POST", "http://localhost/", strings.NewReader(tt.payload))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		res := rec.Result()
		// Always expecting back a JSONRPCResponse
		assert.NotZero(t, res.StatusCode, "#%d: should always return code", i)
		blob, err := io.ReadAll(res.Body)
		if err != nil {
			t.Errorf("#%d: err reading body: %v", i, err)
			continue
		}
		res.Body.Close()

		recv := new(rpctypes.RPCResponse)
		err = json.Unmarshal(blob, recv)
		assert.NoError(t, err, "#%d: expecting successful parsing of an RPCResponse:\nblob: %s", i, blob)
		if !tt.wantErr {
			assert.NotEqual(t, recv, new(rpctypes.RPCResponse), "#%d: not expecting a blank RPCResponse", i)
			assert.Equal(t, tt.expectedID, recv.ID(), "#%d: expected ID not matched in RPCResponse", i)
			assert.Nil(t, recv.Error, "#%d: not expecting an error", i)
		} else {
			assert.True(t, recv.Error.Code < 0, "#%d: not expecting a positive JSONRPC code", i)
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
	blob, err := io.ReadAll(res.Body)
	res.Body.Close()
	require.NoError(t, err, "reading from the body should not give back an error")
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
				{"jsonrpc": "2.0"},
				{"jsonrpc": "2.0","method":"c","id":"abc","params":["a","10"]}
			 ]`,
			1,
		},
		{
			`[
				{"jsonrpc": "2.0"},
				{"jsonrpc": "2.0","method":"c","id":"abc","params":["a","10"]},
				{"jsonrpc": "2.0"},
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
		blob, err := io.ReadAll(res.Body)
		if err != nil {
			t.Errorf("#%d: err reading body: %v", i, err)
			continue
		}
		res.Body.Close()

		var responses []rpctypes.RPCResponse
		// try to unmarshal an array first
		err = json.Unmarshal(blob, &responses)
		if err != nil {
			// if we were actually expecting an array, but got an error
			if tt.expectCount > 1 {
				t.Errorf("#%d: expected an array, couldn't unmarshal it\nblob: %s", i, blob)
				continue
			} else {
				// we were expecting an error here, so let's unmarshal a single response
				var response rpctypes.RPCResponse
				err = json.Unmarshal(blob, &response)
				if err != nil {
					t.Errorf("#%d: expected successful parsing of an RPCResponse\nblob: %s", i, blob)
					continue
				}
				// have a single-element result
				responses = []rpctypes.RPCResponse{response}
			}
		}
		if tt.expectCount != len(responses) {
			t.Errorf("#%d: expected %d response(s), but got %d\nblob: %s", i, tt.expectCount, len(responses), blob)
			continue
		}
		for _, response := range responses {
			assert.NotEqual(t, response, new(rpctypes.RPCResponse), "#%d: not expecting a blank RPCResponse", i)
		}
	}
}

func TestUnknownRPCPath(t *testing.T) {
	mux := testMux()
	req, _ := http.NewRequest("GET", "http://localhost/unknownrpcpath", strings.NewReader(""))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	res := rec.Result()

	// Always expecting back a 404 error
	require.Equal(t, http.StatusNotFound, res.StatusCode, "should always return 404")
	res.Body.Close()
}

func TestRPCResponseCache(t *testing.T) {
	mux := testMux()
	body := strings.NewReader(`{"jsonrpc": "2.0","method":"block","id": 0, "params": ["1"]}`)
	req, _ := http.NewRequest("Get", "http://localhost/", body)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	res := rec.Result()

	// Always expecting back a JSONRPCResponse
	require.True(t, statusOK(res.StatusCode), "should always return 2XX")
	require.Equal(t, "", res.Header.Get("Cache-control"))

	_, err := io.ReadAll(res.Body)
	res.Body.Close()
	require.NoError(t, err, "reading from the body should not give back an error")

	// send a request with default height.
	body = strings.NewReader(`{"jsonrpc": "2.0","method":"block","id": 0, "params": ["0"]}`)
	req, _ = http.NewRequest("Get", "http://localhost/", body)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	res = rec.Result()

	// Always expecting back a JSONRPCResponse
	require.True(t, statusOK(res.StatusCode), "should always return 2XX")
	require.Equal(t, "", res.Header.Get("Cache-control"))

	_, err = io.ReadAll(res.Body)
	res.Body.Close()
	require.NoError(t, err, "reading from the body should not give back an error")
}
