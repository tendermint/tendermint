package p2p

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEndpoint_PeerAddress(t *testing.T) {
	var (
		ip4    = []byte{1, 2, 3, 4}
		ip4in6 = net.IPv4(1, 2, 3, 4)
		ip6    = []byte{0xb1, 0x0c, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01}
	)

	testcases := []struct {
		endpoint Endpoint
		expect   PeerAddress
	}{
		// Valid endpoints.
		{
			Endpoint{Protocol: "tcp", IP: ip4, Port: 8080, Path: "path"},
			PeerAddress{Protocol: "tcp", Hostname: "1.2.3.4", Port: 8080, Path: "path"},
		},
		{
			Endpoint{Protocol: "tcp", IP: ip4in6, Port: 8080, Path: "path"},
			PeerAddress{Protocol: "tcp", Hostname: "1.2.3.4", Port: 8080, Path: "path"},
		},
		{
			Endpoint{Protocol: "tcp", IP: ip6, Port: 8080, Path: "path"},
			PeerAddress{Protocol: "tcp", Hostname: "b10c::1", Port: 8080, Path: "path"},
		},
		{
			Endpoint{Protocol: "memory", Path: "foo"},
			PeerAddress{Protocol: "memory", Path: "foo"},
		},

		// Partial (invalid) endpoints.
		{Endpoint{}, PeerAddress{}},
		{Endpoint{Protocol: "tcp"}, PeerAddress{Protocol: "tcp"}},
		{Endpoint{IP: net.IPv4(1, 2, 3, 4)}, PeerAddress{Hostname: "1.2.3.4"}},
		{Endpoint{Port: 8080}, PeerAddress{}},
		{Endpoint{Path: "path"}, PeerAddress{Path: "path"}},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.endpoint.String(), func(t *testing.T) {
			// Without NodeID.
			expect := tc.expect
			require.Equal(t, expect, tc.endpoint.PeerAddress(""))

			// With NodeID.
			expect.NodeID = NodeID("b10c")
			require.Equal(t, expect, tc.endpoint.PeerAddress(expect.NodeID))
		})
	}
}

func TestEndpoint_String(t *testing.T) {
	var (
		ip4    = []byte{1, 2, 3, 4}
		ip4in6 = net.IPv4(1, 2, 3, 4)
		ip6    = []byte{0xb1, 0x0c, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01}
	)

	testcases := []struct {
		endpoint Endpoint
		expect   string
	}{
		// Non-networked endpoints.
		{Endpoint{Protocol: "memory", Path: "foo"}, "memory:foo"},
		{Endpoint{Protocol: "memory", Path: "👋"}, "memory:👋"},

		// IPv4 endpoints.
		{Endpoint{Protocol: "tcp", IP: ip4}, "tcp://1.2.3.4"},
		{Endpoint{Protocol: "tcp", IP: ip4in6}, "tcp://1.2.3.4"},
		{Endpoint{Protocol: "tcp", IP: ip4, Port: 8080}, "tcp://1.2.3.4:8080"},
		{Endpoint{Protocol: "tcp", IP: ip4, Port: 8080, Path: "/path"}, "tcp://1.2.3.4:8080/path"},
		{Endpoint{Protocol: "tcp", IP: ip4, Path: "path/👋"}, "tcp://1.2.3.4/path/%F0%9F%91%8B"},

		// IPv6 endpoints.
		{Endpoint{Protocol: "tcp", IP: ip6}, "tcp://b10c::1"},
		{Endpoint{Protocol: "tcp", IP: ip6, Port: 8080}, "tcp://[b10c::1]:8080"},
		{Endpoint{Protocol: "tcp", IP: ip6, Port: 8080, Path: "/path"}, "tcp://[b10c::1]:8080/path"},
		{Endpoint{Protocol: "tcp", IP: ip6, Path: "path/👋"}, "tcp://b10c::1/path/%F0%9F%91%8B"},

		// Partial (invalid) endpoints.
		{Endpoint{}, ""},
		{Endpoint{Protocol: "tcp"}, "tcp:"},
		{Endpoint{IP: []byte{1, 2, 3, 4}}, "1.2.3.4"},
		{Endpoint{IP: []byte{0xb1, 0x0c, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01}}, "b10c::1"},
		{Endpoint{Port: 8080}, ""},
		{Endpoint{Path: "foo"}, "/foo"},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.expect, func(t *testing.T) {
			require.Equal(t, tc.expect, tc.endpoint.String())
		})
	}
}

func TestEndpoint_Validate(t *testing.T) {
	var (
		ip4    = []byte{1, 2, 3, 4}
		ip4in6 = net.IPv4(1, 2, 3, 4)
		ip6    = []byte{0xb1, 0x0c, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01}
	)

	testcases := []struct {
		endpoint    Endpoint
		expectValid bool
	}{
		// Valid endpoints.
		{Endpoint{Protocol: "tcp", IP: ip4}, true},
		{Endpoint{Protocol: "tcp", IP: ip4in6}, true},
		{Endpoint{Protocol: "tcp", IP: ip6}, true},
		{Endpoint{Protocol: "tcp", IP: ip4, Port: 8008}, true},
		{Endpoint{Protocol: "tcp", IP: ip4, Port: 8080, Path: "path"}, true},
		{Endpoint{Protocol: "memory", Path: "path"}, true},

		// Invalid endpoints.
		{Endpoint{}, false},
		{Endpoint{IP: ip4}, false},
		{Endpoint{Protocol: "tcp", IP: []byte{1, 2, 3}}, false},
		{Endpoint{Protocol: "tcp", Port: 8080, Path: "path"}, false},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.endpoint.String(), func(t *testing.T) {
			err := tc.endpoint.Validate()
			if tc.expectValid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
