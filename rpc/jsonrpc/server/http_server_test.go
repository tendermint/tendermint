package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/log"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
)

type sampleResult struct {
	Value string `json:"value"`
}

func TestMaxOpenConnections(t *testing.T) {
	const max = 5 // max simultaneous connections

	t.Cleanup(leaktest.Check(t))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

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
	config := DefaultConfig()
	l, err := Listen("tcp://127.0.0.1:0", max)
	require.NoError(t, err)
	defer l.Close()

	go Serve(ctx, l, mux, logger, config) //nolint:errcheck // ignore for tests

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
				atomic.AddInt32(&failed, 1)
				return
			}
			defer r.Body.Close()
		}()
	}
	wg.Wait()

	// We expect some Gets to fail as the server's accept queue is filled,
	// but most should succeed.
	if int(failed) >= attempts/2 {
		t.Errorf("%d requests failed within %d attempts", failed, attempts)
	}
}

func TestServeTLS(t *testing.T) {
	t.Cleanup(leaktest.Check(t))

	ln, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer ln.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "some body")
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	chErr := make(chan error, 1)
	go func() {
		select {
		case chErr <- ServeTLS(ctx, ln, mux, "test.crt", "test.key", logger, DefaultConfig()):
		case <-ctx.Done():
		}
	}()

	select {
	case err := <-chErr:
		require.NoError(t, err)
	case <-time.After(100 * time.Millisecond):
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	c := &http.Client{Transport: tr}
	res, err := c.Get("https://" + ln.Addr().String())
	require.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, http.StatusOK, res.StatusCode)

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	assert.Equal(t, []byte("some body"), body)
}

func TestWriteRPCResponse(t *testing.T) {
	req := rpctypes.NewRequest(-1)

	// one argument
	w := httptest.NewRecorder()
	logger := log.NewNopLogger()
	writeRPCResponse(w, logger, req.MakeResponse(&sampleResult{"hello"}))
	resp := w.Result()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, resp.Body.Close())
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	assert.Equal(t, "", resp.Header.Get("Cache-control"))
	assert.Equal(t, `{"jsonrpc":"2.0","id":-1,"result":{"value":"hello"}}`, string(body))

	// multiple arguments
	w = httptest.NewRecorder()
	writeRPCResponse(w, logger,
		req.MakeResponse(&sampleResult{"hello"}),
		req.MakeResponse(&sampleResult{"world"}),
	)
	resp = w.Result()
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, resp.Body.Close())
	require.NoError(t, err)

	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	assert.Equal(t, `[{"jsonrpc":"2.0","id":-1,"result":{"value":"hello"}},`+
		`{"jsonrpc":"2.0","id":-1,"result":{"value":"world"}}]`, string(body))
}

func TestWriteHTTPResponse(t *testing.T) {
	w := httptest.NewRecorder()
	logger := log.NewNopLogger()
	req := rpctypes.NewRequest(-1)
	writeHTTPResponse(w, logger, req.MakeErrorf(rpctypes.CodeInternalError, "foo"))
	resp := w.Result()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, resp.Body.Close())
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	assert.Equal(t, `{"code":-32603,"message":"Internal error","data":"foo"}`, string(body))
}
