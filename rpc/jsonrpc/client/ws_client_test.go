package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	tmsync "github.com/tendermint/tendermint/internal/libs/sync"
	"github.com/tendermint/tendermint/libs/log"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

var wsCallTimeout = 5 * time.Second

type myHandler struct {
	closeConnAfterRead bool
	mtx                tmsync.RWMutex
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func (h *myHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	for {
		messageType, in, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var req rpctypes.RPCRequest
		err = json.Unmarshal(in, &req)
		if err != nil {
			panic(err)
		}

		h.mtx.RLock()
		if h.closeConnAfterRead {
			if err := conn.Close(); err != nil {
				panic(err)
			}
		}
		h.mtx.RUnlock()

		res := json.RawMessage(`{}`)
		emptyRespBytes, _ := json.Marshal(rpctypes.RPCResponse{Result: res, ID: req.ID})
		if err := conn.WriteMessage(messageType, emptyRespBytes); err != nil {
			return
		}
	}
}

func TestWSClientReconnectsAfterReadFailure(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	// start server
	h := &myHandler{}
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
	call(t, "a", c)

	// expect to reconnect almost immediately
	time.Sleep(10 * time.Millisecond)
	h.mtx.Lock()
	h.closeConnAfterRead = false
	h.mtx.Unlock()

	// should succeed
	call(t, "b", c)
}

func TestWSClientReconnectsAfterWriteFailure(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	// start server
	h := &myHandler{}
	s := httptest.NewServer(h)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := startClient(ctx, t, "//"+s.Listener.Addr().String())

	go handleResponses(ctx, t, c)

	// hacky way to abort the connection before write
	if err := c.conn.Close(); err != nil {
		t.Error(err)
	}

	// results in WS write error, the client should resend on reconnect
	call(t, "a", c)

	// expect to reconnect almost immediately
	time.Sleep(10 * time.Millisecond)

	// should succeed
	call(t, "b", c)
}

func TestWSClientReconnectFailure(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	// start server
	h := &myHandler{}
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
		call(t, "b", c)
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

	timeout := 2 * time.Second
	s := httptest.NewServer(&myHandler{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := startClient(ctx, t, "//"+s.Listener.Addr().String())
	c.Call(context.Background(), "a", make(map[string]interface{})) // nolint:errcheck // ignore for tests
	// Let the readRoutine get around to blocking
	time.Sleep(time.Second)
	passCh := make(chan struct{})
	go func() {
		// Unless we have a non-blocking write to ResponsesCh from readRoutine
		// this blocks forever ont the waitgroup
		err := c.Stop()
		require.NoError(t, err)

		select {
		case <-ctx.Done():
		case passCh <- struct{}{}:
		}
	}()
	select {
	case <-passCh:
		// Pass
	case <-time.After(timeout):
		t.Fatalf("WSClient did failed to stop within %v seconds - is one of the read/write routines blocking?",
			timeout.Seconds())
	}
}

func startClient(ctx context.Context, t *testing.T, addr string) *WSClient {
	c, err := NewWS(addr, "/websocket")
	require.Nil(t, err)
	err = c.Start(ctx)
	require.Nil(t, err)
	c.Logger = log.TestingLogger()
	return c
}

func call(t *testing.T, method string, c *WSClient) {
	err := c.Call(context.Background(), method, make(map[string]interface{}))
	require.NoError(t, err)
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
