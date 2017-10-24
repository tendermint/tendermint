package rpcclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tmlibs/log"

	types "github.com/tendermint/tendermint/rpc/lib/types"
)

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
	defer conn.Close()
	for {
		messageType, _, err := conn.ReadMessage()
		if err != nil {
			return
		}

		h.mtx.RLock()
		if h.closeConnAfterRead {
			conn.Close()
		}
		h.mtx.RUnlock()

		res := json.RawMessage(`{}`)
		emptyRespBytes, _ := json.Marshal(types.RPCResponse{Result: &res})
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
	c.conn.Close()

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
			case <-c.ResultsCh:
			case <-c.ErrorsCh:
			case <-c.Quit:
				return
			}
		}
	}()

	// hacky way to abort the connection before write
	c.conn.Close()
	s.Close()

	// results in WS write error
	// provide timeout to avoid blocking
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	c.Call(ctx, "a", make(map[string]interface{}))

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

func startClient(t *testing.T, addr net.Addr) *WSClient {
	c := NewWSClient(addr.String(), "/websocket")
	_, err := c.Start()
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
		case res := <-c.ResultsCh:
			if res != nil {
				wg.Done()
			}
		case err := <-c.ErrorsCh:
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		case <-c.Quit:
			return
		}
	}
}

func TestWSClientReconnectWithJitter(t *testing.T) {
	if testing.Short() {
		t.Skipf("This is a potentially long test")
	}

	n := 8
	maxReconnectAttempts := 3
	// Max wait time is ceil(1+0.999) + ceil(2+0.999) + ceil(4+0.999) + ceil(...) = 2 + 3 + 5 = 10s + ...
	maxSleepTime := time.Second * time.Duration(((1<<uint(maxReconnectAttempts))-1)+maxReconnectAttempts)

	var errNotConnected = errors.New("not connected")
	clientMap := make(map[int]*WSClient)
	buf := new(bytes.Buffer)
	logger := log.NewTMLogger(buf)
	for i := 0; i < n; i++ {
		c := NewWSClient("tcp://foo", "/websocket")
		c.Dialer = func(string, string) (net.Conn, error) {
			return nil, errNotConnected
		}
		c.SetLogger(logger)
		c.maxReconnectAttempts = maxReconnectAttempts
		// Not invoking defer c.Stop() because
		// after all the reconnect attempts have been
		// exhausted, c.Stop is implicitly invoked.
		clientMap[i] = c
		// Trigger the reconnect routine that performs exponential backoff.
		go c.reconnect()
	}

	stopCount := 0
	time.Sleep(maxSleepTime)
	for key, c := range clientMap {
		if !c.IsActive() {
			delete(clientMap, key)
			stopCount += 1
		}
	}
	require.Equal(t, stopCount, n, "expecting all clients to have been stopped")

	// Next we have to examine the logs to ensure that no single time was repeated
	backoffDurRegexp := regexp.MustCompile(`backoff_duration=(.+)`)
	matches := backoffDurRegexp.FindAll(buf.Bytes(), -1)
	seenMap := make(map[string]int)
	for i, match := range matches {
		if origIndex, seen := seenMap[string(match)]; seen {
			t.Errorf("Match #%d (%q) was seen originally at log entry #%d", i, match, origIndex)
		} else {
			seenMap[string(match)] = i
		}
	}
}
