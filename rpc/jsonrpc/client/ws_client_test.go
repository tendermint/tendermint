package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

const wsCallTimeout = 5 * time.Second

type myTestHandler struct {
	closeConnAfterRead bool
	mtx                sync.RWMutex
	t                  *testing.T
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func (h *myTestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	require.NoError(h.t, err)

	defer conn.Close()
	for {
		messageType, in, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var req rpctypes.RPCRequest
		err = json.Unmarshal(in, &req)
		require.NoError(h.t, err)

		func() {
			h.mtx.RLock()
			defer h.mtx.RUnlock()

			if h.closeConnAfterRead {
				require.NoError(h.t, conn.Close())
			}
		}()

		res := json.RawMessage(`{}`)

		emptyRespBytes, err := json.Marshal(req.MakeResponse(res))
		require.NoError(h.t, err)
		if err := conn.WriteMessage(messageType, emptyRespBytes); err != nil {
			return
		}
	}
}

func TestWSClientReconnectsAfterReadFailure(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	// start server
	h := &myTestHandler{t: t}
	s := httptest.NewServer(h)
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := startClient(ctx, t, "//"+s.Listener.Addr().String())

	go handleResponses(ctx, t, c)

	h.mtx.Lock()
	h.closeConnAfterRead = true
	h.mtx.Unlock()

	// results in WS read error, no send retry because write succeeded
	call(ctx, t, "a", c)

	// expect to reconnect almost immediately
	time.Sleep(10 * time.Millisecond)
	h.mtx.Lock()
	h.closeConnAfterRead = false
	h.mtx.Unlock()

	// should succeed
	call(ctx, t, "b", c)
}

func TestWSClientReconnectsAfterWriteFailure(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	// start server
	h := &myTestHandler{t: t}
	s := httptest.NewServer(h)
	defer s.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := startClient(ctx, t, "//"+s.Listener.Addr().String())

	go handleResponses(ctx, t, c)

	// hacky way to abort the connection before write
	if err := c.conn.Close(); err != nil {
		t.Error(err)
	}

	// results in WS write error, the client should resend on reconnect
	call(ctx, t, "a", c)

	// expect to reconnect almost immediately
	time.Sleep(10 * time.Millisecond)

	// should succeed
	call(ctx, t, "b", c)
}

func TestWSClientReconnectFailure(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	// start server
	h := &myTestHandler{t: t}
	s := httptest.NewServer(h)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := startClient(ctx, t, "//"+s.Listener.Addr().String())

	go func() {
		for {
			select {
			case <-c.ResponsesCh:
			case <-ctx.Done():
				return
			}
		}
	}()

	// hacky way to abort the connection before write
	if err := c.conn.Close(); err != nil {
		t.Error(err)
	}
	s.Close()

	// results in WS write error
	// provide timeout to avoid blocking
	cctx, cancel := context.WithTimeout(ctx, wsCallTimeout)
	defer cancel()
	if err := c.Call(cctx, "a", make(map[string]interface{})); err != nil {
		t.Error(err)
	}

	// expect to reconnect almost immediately
	time.Sleep(10 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		// client should block on this
		call(ctx, t, "b", c)
		close(done)
	}()

	// test that client blocks on the second send
	select {
	case <-done:
		t.Fatal("client should block on calling 'b' during reconnect")
	case <-time.After(5 * time.Second):
		t.Log("All good")
	}
}

func TestNotBlockingOnStop(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	s := httptest.NewServer(&myTestHandler{t: t})
	defer s.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := startClient(ctx, t, "//"+s.Listener.Addr().String())
	require.NoError(t, c.Call(ctx, "a", make(map[string]interface{})))

	time.Sleep(200 * time.Millisecond) // give service routines time to start ⚠️
	done := make(chan struct{})
	go func() {
		cancel()
		if assert.NoError(t, c.Stop()) {
			close(done)
		}
	}()
	select {
	case <-done:
		t.Log("Stopped client successfully")
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for client to stop")
	}
}

func startClient(ctx context.Context, t *testing.T, addr string) *WSClient {
	t.Helper()

	t.Cleanup(leaktest.Check(t))

	c, err := NewWS(addr, "/websocket")
	require.NoError(t, err)
	require.NoError(t, c.Start(ctx))
	return c
}

func call(ctx context.Context, t *testing.T, method string, c *WSClient) {
	t.Helper()

	err := c.Call(ctx, method, make(map[string]interface{}))
	if ctx.Err() == nil {
		require.NoError(t, err)
	}
}

func handleResponses(ctx context.Context, t *testing.T, c *WSClient) {
	t.Helper()

	for {
		select {
		case resp := <-c.ResponsesCh:
			if resp.Error != nil {
				t.Errorf("unexpected error: %v", resp.Error)
				return
			}
			if resp.Result != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
