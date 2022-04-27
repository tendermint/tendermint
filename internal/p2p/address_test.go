package p2p_test

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/types"
)

func TestNewNodeID(t *testing.T) {
	// Most tests are in TestNodeID_Validate, this just checks that it's validated.
	testcases := []struct {
		input  string
		expect types.NodeID
		ok     bool
	}{
		{"", "", false},
		{"foo", "", false},
		{"00112233445566778899aabbccddeeff00112233", "00112233445566778899aabbccddeeff00112233", true},
		{"00112233445566778899AABBCCDDEEFF00112233", "00112233445566778899aabbccddeeff00112233", true},
		{"00112233445566778899aabbccddeeff0011223", "", false},
		{"00112233445566778899aabbccddeeff0011223g", "", false},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			id, err := types.NewNodeID(tc.input)
			if !tc.ok {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, id, tc.expect)
			}
		})
	}
}

func TestNewNodeIDFromPubKey(t *testing.T) {
	privKey := ed25519.GenPrivKeyFromSecret([]byte("foo"))
	nodeID := types.NodeIDFromPubKey(privKey.PubKey())
	require.Equal(t, types.NodeID("045f5600654182cfeaccfe6cb19f0642e8a59898"), nodeID)
	require.NoError(t, nodeID.Validate())
}

func TestNodeID_Bytes(t *testing.T) {
	testcases := []struct {
		nodeID types.NodeID
		expect []byte
		ok     bool
	}{
		{"", []byte{}, true},
		{"01f0", []byte{0x01, 0xf0}, true},
		{"01F0", []byte{0x01, 0xf0}, true},
		{"01F", nil, false},
		{"01g0", nil, false},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(string(tc.nodeID), func(t *testing.T) {
			bz, err := tc.nodeID.Bytes()
			if tc.ok {
				require.NoError(t, err)
				require.Equal(t, tc.expect, bz)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestNodeID_Validate(t *testing.T) {
	testcases := []struct {
		nodeID types.NodeID
		ok     bool
	}{
		{"", false},
		{"00", false},
		{"00112233445566778899aabbccddeeff00112233", true},
		{"00112233445566778899aabbccddeeff001122334", false},
		{"00112233445566778899aabbccddeeffgg001122", false},
		{"00112233445566778899AABBCCDDEEFF00112233", false},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(string(tc.nodeID), func(t *testing.T) {
			err := tc.nodeID.Validate()
			if tc.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestParseNodeAddress(t *testing.T) {
	user := "00112233445566778899aabbccddeeff00112233"
	id := types.NodeID(user)

	testcases := []struct {
		url    string
		expect p2p.NodeAddress
		ok     bool
	}{
		// Valid addresses.
		{
			"mconn://" + user + "@127.0.0.1:26657/some/path?foo=bar",
			p2p.NodeAddress{Protocol: "mconn", NodeID: id, Hostname: "127.0.0.1", Port: 26657, Path: "/some/path?foo=bar"},
			true,
		},
		{
			"TCP://" + strings.ToUpper(user) + "@hostname.DOMAIN:8080/Path/%f0%9f%91%8B#Anchor",
			p2p.NodeAddress{Protocol: "tcp", NodeID: id, Hostname: "hostname.domain", Port: 8080, Path: "/Path/👋#Anchor"},
			true,
		},
		{
			user + "@127.0.0.1",
			p2p.NodeAddress{Protocol: "mconn", NodeID: id, Hostname: "127.0.0.1"},
			true,
		},
		{
			user + "@hostname.domain",
			p2p.NodeAddress{Protocol: "mconn", NodeID: id, Hostname: "hostname.domain"},
			true,
		},
		{
			user + "@hostname.domain:80",
			p2p.NodeAddress{Protocol: "mconn", NodeID: id, Hostname: "hostname.domain", Port: 80},
			true,
		},
		{
			user + "@%F0%9F%91%8B",
			p2p.NodeAddress{Protocol: "mconn", NodeID: id, Hostname: "👋"},
			true,
		},
		{
			user + "@%F0%9F%91%8B:80/path",
			p2p.NodeAddress{Protocol: "mconn", NodeID: id, Hostname: "👋", Port: 80, Path: "/path"},
			true,
		},
		{
			user + "@127.0.0.1:26657",
			p2p.NodeAddress{Protocol: "mconn", NodeID: id, Hostname: "127.0.0.1", Port: 26657},
			true,
		},
		{
			user + "@127.0.0.1:26657/path",
			p2p.NodeAddress{Protocol: "mconn", NodeID: id, Hostname: "127.0.0.1", Port: 26657, Path: "/path"},
			true,
		},
		{
			user + "@0.0.0.0:0",
			p2p.NodeAddress{Protocol: "mconn", NodeID: id, Hostname: "0.0.0.0", Port: 0},
			true,
		},
		{
			"memory:" + user,
			p2p.NodeAddress{Protocol: "memory", NodeID: id},
			true,
		},
		{
			user + "@[1::]",
			p2p.NodeAddress{Protocol: "mconn", NodeID: id, Hostname: "1::"},
			true,
		},
		{
			"mconn://" + user + "@[fd80:b10c::2]:26657",
			p2p.NodeAddress{Protocol: "mconn", NodeID: id, Hostname: "fd80:b10c::2", Port: 26657},
			true,
		},

		// Invalid addresses.
		{"", p2p.NodeAddress{}, false},
		{"127.0.0.1", p2p.NodeAddress{}, false},
		{"hostname", p2p.NodeAddress{}, false},
		{"scheme:", p2p.NodeAddress{}, false},
		{"memory:foo", p2p.NodeAddress{}, false},
		{user + "@%F%F0", p2p.NodeAddress{}, false},
		{"//" + user + "@127.0.0.1", p2p.NodeAddress{}, false},
		{"://" + user + "@127.0.0.1", p2p.NodeAddress{}, false},
		{"mconn://foo@127.0.0.1", p2p.NodeAddress{}, false},
		{"mconn://" + user + "@127.0.0.1:65536", p2p.NodeAddress{}, false},
		{"mconn://" + user + "@:80", p2p.NodeAddress{}, false},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.url, func(t *testing.T) {
			address, err := p2p.ParseNodeAddress(tc.url)
			if !tc.ok {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expect, address)
			}
		})
	}
}

func TestNodeAddress_Resolve(t *testing.T) {
	id := types.NodeID("00112233445566778899aabbccddeeff00112233")

	bctx, bcancel := context.WithCancel(context.Background())
	defer bcancel()

	testcases := []struct {
		address p2p.NodeAddress
		expect  *p2p.Endpoint
		ok      bool
	}{
		// Valid networked addresses (with hostname).
		{
			p2p.NodeAddress{Protocol: "tcp", Hostname: "127.0.0.1", Port: 80, Path: "/path"},
			&p2p.Endpoint{Protocol: "tcp", IP: net.IPv4(127, 0, 0, 1), Port: 80, Path: "/path"},
			true,
		},
		{
			p2p.NodeAddress{Protocol: "tcp", Hostname: "localhost", Port: 80, Path: "/path"},
			&p2p.Endpoint{Protocol: "tcp", IP: net.IPv4(127, 0, 0, 1), Port: 80, Path: "/path"},
			true,
		},
		{
			p2p.NodeAddress{Protocol: "tcp", Hostname: "localhost", Port: 80, Path: "/path"},
			&p2p.Endpoint{Protocol: "tcp", IP: net.IPv6loopback, Port: 80, Path: "/path"},
			true,
		},
		{
			p2p.NodeAddress{Protocol: "tcp", Hostname: "127.0.0.1"},
			&p2p.Endpoint{Protocol: "tcp", IP: net.IPv4(127, 0, 0, 1)},
			true,
		},
		{
			p2p.NodeAddress{Protocol: "tcp", Hostname: "::1"},
			&p2p.Endpoint{Protocol: "tcp", IP: net.IPv6loopback},
			true,
		},
		{
			p2p.NodeAddress{Protocol: "tcp", Hostname: "8.8.8.8"},
			&p2p.Endpoint{Protocol: "tcp", IP: net.IPv4(8, 8, 8, 8)},
			true,
		},
		{
			p2p.NodeAddress{Protocol: "tcp", Hostname: "2001:0db8::ff00:0042:8329"},
			&p2p.Endpoint{Protocol: "tcp", IP: []byte{
				0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0x00, 0x00, 0x42, 0x83, 0x29}},
			true,
		},
		{
			p2p.NodeAddress{Protocol: "tcp", Hostname: "some.missing.host.tendermint.com"},
			&p2p.Endpoint{},
			false,
		},

		// Valid non-networked addresses.
		{
			p2p.NodeAddress{Protocol: "memory", NodeID: id},
			&p2p.Endpoint{Protocol: "memory", Path: string(id)},
			true,
		},
		{
			p2p.NodeAddress{Protocol: "memory", NodeID: id, Path: string(id)},
			&p2p.Endpoint{Protocol: "memory", Path: string(id)},
			true,
		},

		// Invalid addresses.
		{p2p.NodeAddress{}, &p2p.Endpoint{}, false},
		{p2p.NodeAddress{Hostname: "127.0.0.1"}, &p2p.Endpoint{}, false},
		{p2p.NodeAddress{Protocol: "tcp", Hostname: "127.0.0.1:80"}, &p2p.Endpoint{}, false},
		{p2p.NodeAddress{Protocol: "memory"}, &p2p.Endpoint{}, false},
		{p2p.NodeAddress{Protocol: "memory", Path: string(id)}, &p2p.Endpoint{}, false},
		{p2p.NodeAddress{Protocol: "tcp", Hostname: "💥"}, &p2p.Endpoint{}, false},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.address.String(), func(t *testing.T) {
			ctx, cancel := context.WithCancel(bctx)
			defer cancel()

			endpoints, err := tc.address.Resolve(ctx)
			if !tc.ok {
				require.Error(t, err)
				return
			}
			require.Contains(t, endpoints, tc.expect)
		})
	}
}

func TestNodeAddress_String(t *testing.T) {
	id := types.NodeID("00112233445566778899aabbccddeeff00112233")
	user := string(id)
	testcases := []struct {
		address p2p.NodeAddress
		expect  string
	}{
		// Valid networked addresses (with hostname).
		{
			p2p.NodeAddress{Protocol: "tcp", NodeID: id, Hostname: "host", Port: 80, Path: "/path/sub?foo=bar&x=y#anchor"},
			"tcp://" + user + "@host:80/path/sub%3Ffoo=bar&x=y%23anchor",
		},
		{
			p2p.NodeAddress{Protocol: "tcp", NodeID: id, Hostname: "host.domain"},
			"tcp://" + user + "@host.domain",
		},
		{
			p2p.NodeAddress{NodeID: id, Hostname: "host", Port: 80, Path: "foo/bar"},
			user + "@host:80/foo/bar",
		},

		// Valid non-networked addresses (without hostname).
		{
			p2p.NodeAddress{Protocol: "memory", NodeID: id, Path: string(id)},
			"memory:" + user,
		},
		{
			p2p.NodeAddress{Protocol: "memory", NodeID: id},
			"memory:" + user,
		},

		// Addresses with weird contents, which are technically fine (not harmful).
		{
			p2p.NodeAddress{Protocol: "💬", NodeID: "👨", Hostname: "💻", Port: 80, Path: "🛣"},
			"💬://%F0%9F%91%A8@%F0%9F%92%BB:80/%F0%9F%9B%A3",
		},

		// Partial (invalid) addresses.
		{p2p.NodeAddress{}, ""},
		{p2p.NodeAddress{NodeID: id}, user + "@"},
		{p2p.NodeAddress{Protocol: "tcp"}, "tcp:"},
		{p2p.NodeAddress{Hostname: "host"}, "host"},
		{p2p.NodeAddress{Port: 80}, ""},
		{p2p.NodeAddress{Path: "path"}, "/path"},
		{p2p.NodeAddress{NodeID: id, Port: 80}, user + "@"},
		{p2p.NodeAddress{Protocol: "tcp", Hostname: "host"}, "tcp://host"},
		{
			p2p.NodeAddress{Protocol: "memory", NodeID: id, Path: "path"},
			"memory://00112233445566778899aabbccddeeff00112233@/path",
		},
		{
			p2p.NodeAddress{Protocol: "memory", NodeID: id, Port: 80},
			"memory:00112233445566778899aabbccddeeff00112233",
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.address.String(), func(t *testing.T) {
			require.Equal(t, tc.expect, tc.address.String())
		})
	}
}

func TestNodeAddress_Validate(t *testing.T) {
	id := types.NodeID("00112233445566778899aabbccddeeff00112233")
	testcases := []struct {
		address p2p.NodeAddress
		ok      bool
	}{
		// Valid addresses.
		{p2p.NodeAddress{Protocol: "mconn", NodeID: id, Hostname: "host", Port: 80, Path: "/path"}, true},
		{p2p.NodeAddress{Protocol: "mconn", NodeID: id, Hostname: "host"}, true},
		{p2p.NodeAddress{Protocol: "mconn", NodeID: id, Path: "path"}, true},
		{p2p.NodeAddress{Protocol: "mconn", NodeID: id, Hostname: "👋", Path: "👋"}, true},

		// Invalid addresses.
		{p2p.NodeAddress{}, false},
		{p2p.NodeAddress{NodeID: "foo", Hostname: "host"}, false},
		{p2p.NodeAddress{Protocol: "mconn", NodeID: id}, true},
		{p2p.NodeAddress{Protocol: "mconn", NodeID: "foo", Hostname: "host"}, false},
		{p2p.NodeAddress{Protocol: "mconn", NodeID: id, Port: 80, Path: "path"}, false},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.address.String(), func(t *testing.T) {
			err := tc.address.Validate()
			if tc.ok {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
