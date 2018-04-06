package p2p

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNetAddress(t *testing.T) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:8080")
	require.Nil(t, err)
	addr := NewNetAddress("", tcpAddr)

	assert.Equal(t, "127.0.0.1:8080", addr.String())

	assert.NotPanics(t, func() {
		NewNetAddress("", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8000})
	}, "Calling NewNetAddress with UDPAddr should not panic in testing")
}

func TestNewNetAddressStringWithOptionalID(t *testing.T) {
	testCases := []struct {
		addr     string
		expected string
		correct  bool
	}{
		{"127.0.0.1:8080", "127.0.0.1:8080", true},
		{"tcp://127.0.0.1:8080", "127.0.0.1:8080", true},
		{"udp://127.0.0.1:8080", "127.0.0.1:8080", true},
		{"udp//127.0.0.1:8080", "", false},
		// {"127.0.0:8080", false},
		{"notahost", "", false},
		{"127.0.0.1:notapath", "", false},
		{"notahost:8080", "", false},
		{"8082", "", false},
		{"127.0.0:8080000", "", false},

		{"deadbeef@127.0.0.1:8080", "", false},
		{"this-isnot-hex@127.0.0.1:8080", "", false},
		{"xxxxbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080", "", false},
		{"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080", true},

		{"tcp://deadbeef@127.0.0.1:8080", "", false},
		{"tcp://this-isnot-hex@127.0.0.1:8080", "", false},
		{"tcp://xxxxbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080", "", false},
		{"tcp://deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080", true},

		{"tcp://@127.0.0.1:8080", "", false},
		{"tcp://@", "", false},
		{"", "", false},
		{"@", "", false},
		{" @", "", false},
		{" @ ", "", false},
	}

	for _, tc := range testCases {
		addr, err := NewNetAddressStringWithOptionalID(tc.addr)
		if tc.correct {
			if assert.Nil(t, err, tc.addr) {
				assert.Equal(t, tc.expected, addr.String())
			}
		} else {
			assert.NotNil(t, err, tc.addr)
		}
	}
}

func TestNewNetAddressString(t *testing.T) {
	testCases := []struct {
		addr     string
		expected string
		correct  bool
	}{
		{"127.0.0.1:8080", "127.0.0.1:8080", false},
		{"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080", true},
	}

	for _, tc := range testCases {
		addr, err := NewNetAddressString(tc.addr)
		if tc.correct {
			if assert.Nil(t, err, tc.addr) {
				assert.Equal(t, tc.expected, addr.String())
			}
		} else {
			assert.NotNil(t, err, tc.addr)
		}
	}
}

func TestNewNetAddressStrings(t *testing.T) {
	addrs, errs := NewNetAddressStrings([]string{
		"127.0.0.1:8080",
		"deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080",
		"deadbeefdeadbeefdeadbeefdeadbeefdeadbeed@127.0.0.2:8080"})
	assert.Len(t, errs, 1)
	assert.Equal(t, 2, len(addrs))
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
		{"127.0.0.1:8080", true, true, false},
		{"ya.ru:80", true, false, true},
	}

	for _, tc := range testCases {
		addr, err := NewNetAddressStringWithOptionalID(tc.addr)
		require.Nil(t, err)

		assert.Equal(t, tc.valid, addr.Valid())
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
		{"127.0.0.1:8080", "127.0.0.1:8081", 0},
		{"ya.ru:80", "127.0.0.1:8080", 1},
	}

	for _, tc := range testCases {
		addr, err := NewNetAddressStringWithOptionalID(tc.addr)
		require.Nil(t, err)

		other, err := NewNetAddressStringWithOptionalID(tc.other)
		require.Nil(t, err)

		assert.Equal(t, tc.reachability, addr.ReachabilityTo(other))
	}
}
