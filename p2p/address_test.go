package p2p_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/p2p"
)

func TestNewNodeID(t *testing.T) {
	// Most tests are in TestNodeID_Validate, this just checks that it's validated.
	testcases := []struct {
		input  string
		expect p2p.NodeID
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
			id, err := p2p.NewNodeID(tc.input)
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
	nodeID := p2p.NodeIDFromPubKey(privKey.PubKey())
	require.Equal(t, p2p.NodeID("045f5600654182cfeaccfe6cb19f0642e8a59898"), nodeID)
}

func TestNodeID_Bytes(t *testing.T) {
	testcases := []struct {
		nodeID p2p.NodeID
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
		nodeID p2p.NodeID
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
	id := p2p.NodeID(user)

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
			"TCP://" + user + "@hostname.DOMAIN:8080/Path/%F0%9F%91%8B#Anchor",
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
			p2p.NodeAddress{Protocol: "memory", NodeID: id, Path: user},
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

func TestNodeAddress_Validate(t *testing.T) {
	id := p2p.NodeID("00112233445566778899aabbccddeeff00112233")
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
