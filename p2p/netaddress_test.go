package p2p

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNetAddress(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	tcpAddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:8080")
	require.Nil(err)
	addr := NewNetAddress("", tcpAddr)

	assert.Equal("127.0.0.1:8080", addr.String())

	assert.NotPanics(func() {
		NewNetAddress("", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8000})
	}, "Calling NewNetAddress with UDPAddr should not panic in testing")
}

func TestNewNetAddressString(t *testing.T) {
	testCases := []struct {
		addr     string
		expected string
		correct  bool
	}{
		{"127.0.0.1:8080", "127.0.0.1:8080", true},
		{"tcp://127.0.0.1:8080", "127.0.0.1:8080", true},
		{"udp://127.0.0.1:8080", "127.0.0.1:8080", true},
		// {"udp//127.0.0.1:8080", "", false},
		// {"127.0.0:8080", false},
		{"notahost", "", false},
		{"127.0.0.1:notapath", "", false},
		// {"notahost:8080", "", false},
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
	addrs, errs := NewNetAddressStrings([]string{"127.0.0.1:8080", "127.0.0.2:8080"})
	assert.Len(t, errs, 0)
	assert.Equal(t, 2, len(addrs))
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

func TestDial(t *testing.T) {
	addr, err := NewNetAddressString("notresolvable:8800")

	assert.NoError(t, err)

	conn, err := addr.Dial()

	assert.Nil(t, conn)
	assert.Error(t, err, IPNotSetError)
}

func TestDialTimeout(t *testing.T) {
	addr, err := NewNetAddressString("notresolvable:8800")

	assert.NoError(t, err)

	conn, err := addr.DialTimeout(1 * time.Second)

	assert.Nil(t, conn)
	assert.Error(t, err, IPNotSetError)
}
