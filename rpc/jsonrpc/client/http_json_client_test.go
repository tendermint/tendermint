package client

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPClientMakeHTTPDialer(t *testing.T) {
	remote := []string{"https://foo-bar.com:80", "http://foo-bar.net:80", "https://user:pass@foo-bar.net:80"}

	for _, f := range remote {
		u, err := newParsedURL(f)
		require.NoError(t, err)
		dialFn, err := makeHTTPDialer(f)
		require.Nil(t, err)

		addr, err := dialFn(u.Scheme, u.GetHostWithPath())
		require.NoError(t, err)
		require.NotNil(t, addr)
	}

}
