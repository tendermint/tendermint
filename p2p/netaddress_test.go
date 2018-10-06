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
		name     string
		addr     string
		expected string
		correct  bool
	}{
		{"no node id, no protocol", "127.0.0.1:8080", "127.0.0.1:8080", true},
		{"no node id, tcp input", "tcp://127.0.0.1:8080", "127.0.0.1:8080", true},
		{"no node id, udp input", "udp://127.0.0.1:8080", "127.0.0.1:8080", true},
		{"malformed udp input", "udp//127.0.0.1:8080", "", false},
		// {"127.0.0:8080", false},
		{"invalid host", "notahost", "", false},
		{"invalid port", "127.0.0.1:notapath", "", false},
		{"invalid host w/ port", "notahost:8080", "", false},
		{"just a port", "8082", "", false},
		{"non-existent port", "127.0.0:8080000", "", false},

		{"too short nodeId", "deadbeef@127.0.0.1:8080", "", false},
		{"too short, not hex nodeId", "this-isnot-hex@127.0.0.1:8080", "", false},
		{"not hex nodeId", "xxxxbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080", "", false},
		{"correct nodeId", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080", true},

		{"too short nodeId w/tcp", "tcp://deadbeef@127.0.0.1:8080", "", false},
		{"too short notHex nodeId w/tcp", "tcp://this-isnot-hex@127.0.0.1:8080", "", false},
		{"notHex nodeId w/tcp", "tcp://xxxxbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080", "", false},
		{"correct nodeId w/tcp", "tcp://deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef@127.0.0.1:8080", true},

		{"no node id when expected", "tcp://@127.0.0.1:8080", "", false},
		{"no node id or IP", "tcp://@", "", false},
		{"tcp no host, w/ port", "tcp://:26656", "", false},
		{"empty", "", "", false},
		{"node id delimiter 1", "@", "", false},
		{"node id delimiter 2", " @", "", false},
		{"node id delimiter 3", " @ ", "", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			addr, err := NewNetAddressStringWithOptionalID(tc.addr)
			if tc.correct {
				if assert.Nil(t, err, tc.addr) {
					assert.Equal(t, tc.expected, addr.String())
				}
			} else {
				assert.NotNil(t, err, tc.addr)
			}
		})
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
