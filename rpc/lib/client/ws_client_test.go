package rpcclient

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"

	types "github.com/tendermint/tendermint/rpc/lib/types"
)

var wsCallTimeout = 5 * time.Second

type myHandler struct {
	closeConnAfterRead bool
	mtx                sync.RWMutex
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
	defer conn.Close() // nolint: errcheck
	for {
		messageType, _, err := conn.ReadMessage()
		if err != nil {
			return
		}

		h.mtx.RLock()
		if h.closeConnAfterRead {
			if err := conn.Close(); err != nil {
				panic(err)
			}
		}
		h.mtx.RUnlock()

		res := json.RawMessage(`{}`)
		emptyRespBytes, _ := json.Marshal(types.RPCResponse{Result: res})
		if err := conn.WriteMessage(messageType, emptyRespBytes); err != nil {
			return
		}
	}
}

func TestWSClientReconnectsAfterReadFailure(t *testing.T) {
	var wg sync.WaitGroup

	// start server
	h := &myHandler{}
	s := httptest.NewServer(h)
	defer s.Close()

	c := startClient(t, s.Listener.Addr())
	defer c.Stop()

	wg.Add(1)
	go callWgDoneOnResult(t, c, &wg)

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

	wg.Wait()
}

func TestWSClientReconnectsAfterWriteFailure(t *testing.T) {
	var wg sync.WaitGroup

	// start server
	h := &myHandler{}
	s := httptest.NewServer(h)

	c := startClient(t, s.Listener.Addr())
	defer c.Stop()

	wg.Add(2)
	go callWgDoneOnResult(t, c, &wg)

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

	wg.Wait()
}

func TestWSClientReconnectFailure(t *testing.T) {
	// start server
	h := &myHandler{}
	s := httptest.NewServer(h)

	c := startClient(t, s.Listener.Addr())
	defer c.Stop()

	go func() {
		for {
			select {
			case <-c.ResponsesCh:
			case <-c.Quit():
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
	ctx, cancel := context.WithTimeout(context.Background(), wsCallTimeout)
	defer cancel()
	if err := c.Call(ctx, "a", make(map[string]interface{})); err != nil {
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
	timeout := 2 * time.Second
	s := httptest.NewServer(&myHandler{})
	c := startClient(t, s.Listener.Addr())
	c.Call(context.Background(), "a", make(map[string]interface{}))
	// Let the readRoutine get around to blocking
	time.Sleep(time.Second)
	passCh := make(chan struct{})
	go func() {
		// Unless we have a non-blocking write to ResponsesCh from readRoutine
		// this blocks forever ont the waitgroup
		c.Stop()
		passCh <- struct{}{}
	}()
	select {
	case <-passCh:
		// Pass
	case <-time.After(timeout):
		t.Fatalf("WSClient did failed to stop within %v seconds - is one of the read/write routines blocking?",
			timeout.Seconds())
	}
}

func startClient(t *testing.T, addr net.Addr) *WSClient {
	c := NewWSClient(addr.String(), "/websocket")
	err := c.Start()
	require.Nil(t, err)
	c.SetLogger(log.TestingLogger())
	return c
}

func call(t *testing.T, method string, c *WSClient) {
	err := c.Call(context.Background(), method, make(map[string]interface{}))
	require.NoError(t, err)
}

func callWgDoneOnResult(t *testing.T, c *WSClient, wg *sync.WaitGroup) {
	for {
		select {
		case resp := <-c.ResponsesCh:
			if resp.Error != nil {
				t.Fatalf("unexpected error: %v", resp.Error)
			}
			if resp.Result != nil {
				wg.Done()
			}
		case <-c.Quit():
			return
		}
	}
}
