package client

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPClientMakeHTTPDialer(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Hi!\n"))
	})
	ts := httptest.NewServer(handler)
	defer ts.Close()

	tsTLS := httptest.NewTLSServer(handler)
	defer tsTLS.Close()
	// This silences a TLS handshake error, caused by the dialer just immediately
	// disconnecting, which we can just ignore.
	tsTLS.Config.ErrorLog = log.New(ioutil.Discard, "", 0)

	for _, testURL := range []string{ts.URL, tsTLS.URL} {
		u, err := newParsedURL(testURL)
		require.NoError(t, err)
		dialFn, err := makeHTTPDialer(testURL)
		require.Nil(t, err)

		addr, err := dialFn(u.Scheme, u.GetHostWithPath())
		require.NoError(t, err)
		require.NotNil(t, addr)
	}
}
