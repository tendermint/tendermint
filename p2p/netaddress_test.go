package p2p

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNetAddress(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	tcpAddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:8080")
	require.Nil(err)
	addr := NewNetAddress(tcpAddr)

	assert.Equal("127.0.0.1:8080", addr.String())

	assert.NotPanics(func() {
		NewNetAddress(&net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8000})
	}, "Calling NewNetAddress with UDPAddr should not panic in testing")
}

func TestNewNetAddressString(t *testing.T) {
	assert := assert.New(t)

	tests := []struct {
		addr    string
		correct bool
	}{
		{"127.0.0.1:8080", true},
		// {"127.0.0:8080", false},
		{"a", false},
		{"127.0.0.1:a", false},
		{"a:8080", false},
		{"8082", false},
		{"127.0.0:8080000", false},
	}

	for _, t := range tests {
		addr, err := NewNetAddressString(t.addr)
		if t.correct {
			if assert.Nil(err, t.addr) {
				assert.Equal(t.addr, addr.String())
			}
		} else {
			assert.NotNil(err, t.addr)
		}
	}
}

func TestNewNetAddressStrings(t *testing.T) {
	assert, require := assert.New(t), require.New(t)
	addrs, err := NewNetAddressStrings([]string{"127.0.0.1:8080", "127.0.0.2:8080"})
	require.Nil(err)

	assert.Equal(2, len(addrs))
}

func TestNewNetAddressIPPort(t *testing.T) {
	assert := assert.New(t)
	addr := NewNetAddressIPPort(net.ParseIP("127.0.0.1"), 8080)

	assert.Equal("127.0.0.1:8080", addr.String())
}

func TestNetAddressProperties(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// TODO add more test cases
	tests := []struct {
		addr     string
		valid    bool
		local    bool
		routable bool
	}{
		{"127.0.0.1:8080", true, true, false},
		{"ya.ru:80", true, false, true},
	}

	for _, t := range tests {
		addr, err := NewNetAddressString(t.addr)
		require.Nil(err)

		assert.Equal(t.valid, addr.Valid())
		assert.Equal(t.local, addr.Local())
		assert.Equal(t.routable, addr.Routable())
	}
}

func TestNetAddressReachabilityTo(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// TODO add more test cases
	tests := []struct {
		addr         string
		other        string
		reachability int
	}{
		{"127.0.0.1:8080", "127.0.0.1:8081", 0},
		{"ya.ru:80", "127.0.0.1:8080", 1},
	}

	for _, t := range tests {
		addr, err := NewNetAddressString(t.addr)
		require.Nil(err)

		other, err := NewNetAddressString(t.other)
		require.Nil(err)

		assert.Equal(t.reachability, addr.ReachabilityTo(other))
	}
}
