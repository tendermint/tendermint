package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fortytw2/leaktest"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/log"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

func TestWebsocketManagerHandler(t *testing.T) {
	logger := log.NewNopLogger()

	s := newWSServer(t, logger)
	defer s.Close()

	t.Cleanup(leaktest.Check(t))

	// check upgrader works
	d := websocket.Dialer{}
	c, dialResp, err := d.Dial("ws://"+s.Listener.Addr().String()+"/websocket", nil)
	require.NoError(t, err)

	if got, want := dialResp.StatusCode, http.StatusSwitchingProtocols; got != want {
		t.Errorf("dialResp.StatusCode = %q, want %q", got, want)
	}

	// check basic functionality works
	req := rpctypes.NewRequest(1001)
	require.NoError(t, req.SetMethodAndParams("c", map[string]interface{}{"s": "a", "i": 10}))
	require.NoError(t, c.WriteJSON(req))

	var resp rpctypes.RPCResponse
	err = c.ReadJSON(&resp)
	require.NoError(t, err)
	require.Nil(t, resp.Error)
	dialResp.Body.Close()
}

func newWSServer(t *testing.T, logger log.Logger) *httptest.Server {
	type args struct {
		S string      `json:"s"`
		I json.Number `json:"i"`
	}
	funcMap := map[string]*RPCFunc{
		"c": NewWSRPCFunc(func(context.Context, *args) (string, error) { return "foo", nil }),
	}
	wm := NewWebsocketManager(logger, funcMap)

	mux := http.NewServeMux()
	mux.HandleFunc("/websocket", wm.WebsocketHandler)

	srv := httptest.NewServer(mux)

	t.Cleanup(srv.Close)

	return srv
}
