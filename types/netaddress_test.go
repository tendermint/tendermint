package types

import (
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNetAddress_String(t *testing.T) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:8080")
	require.Nil(t, err)

	netAddr := NewNetAddress("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", tcpAddr)

	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = netAddr.String()
		}()
	}

	wg.Wait()

	s := netAddr.String()
	require.Equal(t, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080", s)
}

func TestNewNetAddress(t *testing.T) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:8080")
	require.Nil(t, err)

	assert.Panics(t, func() {
		NewNetAddress("", tcpAddr)
	})

	addr := NewNetAddress("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", tcpAddr)
	assert.Equal(t, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080", addr.String())

	assert.NotPanics(t, func() {
		NewNetAddress("", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8000})
	}, "Calling NewNetAddress with UDPAddr should not panic in testing")
}

func TestNewNetAddressIPPort(t *testing.T) {
	addr := NewNetAddressIPPort(net.ParseIP("127.0.0.1"), 8080)
	assert.Equal(t, "127.0.0.1:8080", addr.String())
}

func TestNetAddressProperties(t *testing.T) {
	// TODO add more test cases
	testCases := []struct {
		addr     string
		valid    bool
		local    bool
		routable bool
	}{
		{"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080", true, true, false},
		{"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@ya.ru:80", true, false, true},
	}

	for _, tc := range testCases {
		addr, err := ParseAddressString(tc.addr)
		require.Nil(t, err)

		err = addr.Valid()
		if tc.valid {
			assert.NoError(t, err)
		} else {
			assert.Error(t, err)
		}
		assert.Equal(t, tc.local, addr.Local())
		assert.Equal(t, tc.routable, addr.Routable())
	}
}

func TestNetAddressReachabilityTo(t *testing.T) {
	// TODO add more test cases
	testCases := []struct {
		addr         string
		other        string
		reachability int
	}{
		{
			"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080",
			"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8081",
			0,
		},
		{"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@ya.ru:80", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080", 1},
	}

	for _, tc := range testCases {
		addr, err := ParseAddressString(tc.addr)
		require.Nil(t, err)

		other, err := ParseAddressString(tc.other)
		require.Nil(t, err)

		assert.Equal(t, tc.reachability, addr.ReachabilityTo(other))
	}
}
