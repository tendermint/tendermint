package rpcserver

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/log"
)

func TestMaxOpenConnections(t *testing.T) {
	const max = 5 // max simultaneous connections

	// Start the server.
	var open int32
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if n := atomic.AddInt32(&open, 1); n > int32(max) {
			t.Errorf("%d open connections, want <= %d", n, max)
		}
		defer atomic.AddInt32(&open, -1)
		time.Sleep(10 * time.Millisecond)
		fmt.Fprint(w, "some body")
	})
	l, err := Listen("tcp://127.0.0.1:0", Config{MaxOpenConnections: max})
	require.NoError(t, err)
	defer l.Close()
	go StartHTTPServer(l, mux, log.TestingLogger())

	// Make N GET calls to the server.
	attempts := max * 2
	var wg sync.WaitGroup
	var failed int32
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := http.Client{Timeout: 3 * time.Second}
			r, err := c.Get("http://" + l.Addr().String())
			if err != nil {
				t.Log(err)
				atomic.AddInt32(&failed, 1)
				return
			}
			defer r.Body.Close()
			io.Copy(ioutil.Discard, r.Body)
		}()
	}
	wg.Wait()

	// We expect some Gets to fail as the server's accept queue is filled,
	// but most should succeed.
	if int(failed) >= attempts/2 {
		t.Errorf("%d requests failed within %d attempts", failed, attempts)
	}
}

func TestStartHTTPAndTLSServer(t *testing.T) {
	// set up fixtures
	listenerAddr := "tcp://0.0.0.0:0"
	listener, err := Listen(listenerAddr, Config{MaxOpenConnections: 1})
	require.NoError(t, err)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {})

	// test failure
	err = StartHTTPAndTLSServer(listener, mux, "", "", log.TestingLogger())
	require.IsType(t, (*os.PathError)(nil), err)

	// TODO: test that starting the server can actually work
}
