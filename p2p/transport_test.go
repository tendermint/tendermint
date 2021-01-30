package p2p_test

import (
	"context"
	"net"
	"sync"
	"testing"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/p2p"
)

// transportFactory is used to set up transports for tests.
type transportFactory func(t *testing.T) p2p.Transport

var (
	// ctx is a convenience context, to avoid creating it in tests.
	ctx = context.Background()

	// transportFactories contains registered transports for testTransports().
	transportFactories = map[string]transportFactory{}
)

// testTransports is a test helper that runs a test against all
// transports registered in transportFactories.
func testTransports(t *testing.T, tester func(*testing.T, transportFactory)) {
	t.Helper()
	for name, transportFactory := range transportFactories {
		transportFactory := transportFactory
		t.Run(name, func(t *testing.T) {
			defer leaktest.Check(t)
			tester(t, transportFactory)
		})
	}
}

// dialAccept is a helper that dials b from a and returns both sides of the
// connection.
func dialAccept(t *testing.T, a, b p2p.Transport) (p2p.Connection, p2p.Connection) {
	t.Helper()

	endpoints := b.Endpoints()
	require.NotEmpty(t, endpoints, "peer not listening on any endpoints")

	acceptCh := make(chan p2p.Connection, 1)
	errCh := make(chan error, 1)
	go func() {
		conn, err := b.Accept(ctx)
		errCh <- err
		acceptCh <- conn
	}()

	dialConn, err := a.Dial(ctx, endpoints[0])
	require.NoError(t, err)

	acceptConn := <-acceptCh
	require.NoError(t, <-errCh)

	return dialConn, acceptConn
}

func TestTransport_DialAcceptHandshake(t *testing.T) {
	testTransports(t, func(t *testing.T, newTransport transportFactory) {
		a := newTransport(t)
		b := newTransport(t)

		ab, ba := dialAccept(t, a, b)

		// Both ends should have corresponding endpoints.
		require.NotEmpty(t, ab.LocalEndpoint())
		require.NotEmpty(t, ba.LocalEndpoint())
		require.Equal(t, ab.LocalEndpoint(), ba.RemoteEndpoint())
		require.Equal(t, ab.RemoteEndpoint(), ba.LocalEndpoint())

		// We should be able to handshake.
		aKey := ed25519.GenPrivKey()
		aInfo := p2p.NodeInfo{
			NodeID:          p2p.NodeIDFromPubKey(aKey.PubKey()),
			ProtocolVersion: p2p.NewProtocolVersion(1, 2, 3),
			ListenAddr:      "listenaddr",
			Network:         "network",
			Version:         "1.2.3",
			Channels:        bytes.HexBytes([]byte{0xf0, 0x0f}),
			Moniker:         "moniker",
			Other: p2p.NodeInfoOther{
				TxIndex:    "txindex",
				RPCAddress: "rpc.domain.com",
			},
		}
		bKey := ed25519.GenPrivKey()
		bInfo := p2p.NodeInfo{NodeID: p2p.NodeIDFromPubKey(bKey.PubKey())}

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			// must use assert due to goroutine
			peerInfo, peerKey, err := ba.Handshake(ctx, bInfo, bKey)
			if assert.NoError(t, err) {
				assert.Equal(t, aInfo, peerInfo)
				assert.Equal(t, aKey.PubKey(), peerKey)
			}
		}()

		peerInfo, peerKey, err := ab.Handshake(ctx, aInfo, aKey)
		require.NoError(t, err)
		require.Equal(t, bInfo, peerInfo)
		require.Equal(t, bKey.PubKey(), peerKey)

		wg.Wait()
	})
}

func TestEndpoint_PeerAddress(t *testing.T) {
	var (
		ip4    = []byte{1, 2, 3, 4}
		ip4in6 = net.IPv4(1, 2, 3, 4)
		ip6    = []byte{0xb1, 0x0c, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01}
	)

	testcases := []struct {
		endpoint p2p.Endpoint
		expect   p2p.PeerAddress
	}{
		// Valid endpoints.
		{
			p2p.Endpoint{Protocol: "tcp", IP: ip4, Port: 8080, Path: "path"},
			p2p.PeerAddress{Protocol: "tcp", Hostname: "1.2.3.4", Port: 8080, Path: "path"},
		},
		{
			p2p.Endpoint{Protocol: "tcp", IP: ip4in6, Port: 8080, Path: "path"},
			p2p.PeerAddress{Protocol: "tcp", Hostname: "1.2.3.4", Port: 8080, Path: "path"},
		},
		{
			p2p.Endpoint{Protocol: "tcp", IP: ip6, Port: 8080, Path: "path"},
			p2p.PeerAddress{Protocol: "tcp", Hostname: "b10c::1", Port: 8080, Path: "path"},
		},
		{
			p2p.Endpoint{Protocol: "memory", Path: "foo"},
			p2p.PeerAddress{Protocol: "memory", Path: "foo"},
		},

		// Partial (invalid) endpoints.
		{p2p.Endpoint{}, p2p.PeerAddress{}},
		{p2p.Endpoint{Protocol: "tcp"}, p2p.PeerAddress{Protocol: "tcp"}},
		{p2p.Endpoint{IP: net.IPv4(1, 2, 3, 4)}, p2p.PeerAddress{Hostname: "1.2.3.4"}},
		{p2p.Endpoint{Port: 8080}, p2p.PeerAddress{}},
		{p2p.Endpoint{Path: "path"}, p2p.PeerAddress{Path: "path"}},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.endpoint.String(), func(t *testing.T) {
			// Without NodeID.
			expect := tc.expect
			require.Equal(t, expect, tc.endpoint.PeerAddress(""))

			// With NodeID.
			expect.NodeID = p2p.NodeID("b10c")
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
		endpoint p2p.Endpoint
		expect   string
	}{
		// Non-networked endpoints.
		{p2p.Endpoint{Protocol: "memory", Path: "foo"}, "memory:foo"},
		{p2p.Endpoint{Protocol: "memory", Path: "👋"}, "memory:👋"},

		// IPv4 endpoints.
		{p2p.Endpoint{Protocol: "tcp", IP: ip4}, "tcp://1.2.3.4"},
		{p2p.Endpoint{Protocol: "tcp", IP: ip4in6}, "tcp://1.2.3.4"},
		{p2p.Endpoint{Protocol: "tcp", IP: ip4, Port: 8080}, "tcp://1.2.3.4:8080"},
		{p2p.Endpoint{Protocol: "tcp", IP: ip4, Port: 8080, Path: "/path"}, "tcp://1.2.3.4:8080/path"},
		{p2p.Endpoint{Protocol: "tcp", IP: ip4, Path: "path/👋"}, "tcp://1.2.3.4/path/%F0%9F%91%8B"},

		// IPv6 endpoints.
		{p2p.Endpoint{Protocol: "tcp", IP: ip6}, "tcp://b10c::1"},
		{p2p.Endpoint{Protocol: "tcp", IP: ip6, Port: 8080}, "tcp://[b10c::1]:8080"},
		{p2p.Endpoint{Protocol: "tcp", IP: ip6, Port: 8080, Path: "/path"}, "tcp://[b10c::1]:8080/path"},
		{p2p.Endpoint{Protocol: "tcp", IP: ip6, Path: "path/👋"}, "tcp://b10c::1/path/%F0%9F%91%8B"},

		// Partial (invalid) endpoints.
		{p2p.Endpoint{}, ""},
		{p2p.Endpoint{Protocol: "tcp"}, "tcp:"},
		{p2p.Endpoint{IP: []byte{1, 2, 3, 4}}, "1.2.3.4"},
		{p2p.Endpoint{IP: []byte{0xb1, 0x0c, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01}}, "b10c::1"},
		{p2p.Endpoint{Port: 8080}, ""},
		{p2p.Endpoint{Path: "foo"}, "/foo"},
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
		endpoint    p2p.Endpoint
		expectValid bool
	}{
		// Valid endpoints.
		{p2p.Endpoint{Protocol: "tcp", IP: ip4}, true},
		{p2p.Endpoint{Protocol: "tcp", IP: ip4in6}, true},
		{p2p.Endpoint{Protocol: "tcp", IP: ip6}, true},
		{p2p.Endpoint{Protocol: "tcp", IP: ip4, Port: 8008}, true},
		{p2p.Endpoint{Protocol: "tcp", IP: ip4, Port: 8080, Path: "path"}, true},
		{p2p.Endpoint{Protocol: "memory", Path: "path"}, true},

		// Invalid endpoints.
		{p2p.Endpoint{}, false},
		{p2p.Endpoint{IP: ip4}, false},
		{p2p.Endpoint{Protocol: "tcp", IP: []byte{1, 2, 3}}, false},
		{p2p.Endpoint{Protocol: "tcp", Port: 8080, Path: "path"}, false},
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
