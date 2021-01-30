package p2p_test

import (
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"

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
	ctx                = context.Background()          // convenience context
	chID               = p2p.ChannelID(1)              // channel ID for use in tests
	transportFactories = map[string]transportFactory{} // registry for testTransports
)

// testTransports is a test helper that runs a test against all transports
// registered in transportFactories.
func testTransports(t *testing.T, tester func(*testing.T, transportFactory)) {
	t.Helper()
	for name, transportFactory := range transportFactories {
		transportFactory := transportFactory
		t.Run(name, func(t *testing.T) {
			t.Cleanup(leaktest.Check(t))
			tester(t, transportFactory)
		})
	}
}

func TestTransport_AcceptClose(t *testing.T) {
	// Just tests unblock on close, since happy path is tested widely elsewhere.
	testTransports(t, func(t *testing.T, makeTransport transportFactory) {
		a := makeTransport(t)

		errCh := make(chan error, 1)
		go func() {
			time.Sleep(200 * time.Millisecond)
			errCh <- a.Close()
		}()

		_, err := a.Accept(ctx)
		require.Error(t, err)
		require.Equal(t, io.EOF, err)
		require.NoError(t, <-errCh)

		_, err = a.Accept(ctx)
		require.Error(t, err)
		require.Equal(t, io.EOF, err)
	})
}

func TestTransport_DialFailure(t *testing.T) {
	// Just test dial failures, since happy path is tested widely elsewhere.
	testTransports(t, func(t *testing.T, makeTransport transportFactory) {
		a := makeTransport(t)
		b := makeTransport(t)

		endpoints := b.Endpoints()
		require.NotEmpty(t, endpoints)
		endpoint := endpoints[0]

		// Context cancellation should error.
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()
		_, err := a.Dial(cancelCtx, endpoint)
		require.Error(t, err)

		// Unavailable endpoint should error.
		err = b.Close()
		require.NoError(t, err)
		_, err = a.Dial(ctx, endpoint)
		require.Error(t, err)
	})
}

func TestTransport_Handshake(t *testing.T) {
	testTransports(t, func(t *testing.T, makeTransport transportFactory) {
		a := makeTransport(t)
		b := makeTransport(t)
		ab, ba := dialAccept(t, a, b)

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
			// Must use assert due to goroutine.
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

func TestTransport_HandshakeCancel(t *testing.T) {
	testTransports(t, func(t *testing.T, makeTransport transportFactory) {
		a := makeTransport(t)
		b := makeTransport(t)

		// Test handshake context cancel.
		// FIXME: This doesn't actually work, since we rely on socket deadlines.
		/*ab, _ := dialAccept(t, a, b)
		timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
		cancel()
		_, _, err := ab.Handshake(timeoutCtx, p2p.NodeInfo{}, ed25519.GenPrivKey())
		require.Error(t, err)
		require.Equal(t, context.Canceled, err)
		_ = ab.Close()*/

		// Test handshake context timeout.
		ab, _ := dialAccept(t, a, b)
		timeoutCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
		defer cancel()
		_, _, err := ab.Handshake(timeoutCtx, p2p.NodeInfo{}, ed25519.GenPrivKey())
		require.Error(t, err)
		require.Equal(t, context.DeadlineExceeded, err)
		_ = ab.Close()
	})
}

func TestTransport_Endpoints(t *testing.T) {
	testTransports(t, func(t *testing.T, makeTransport transportFactory) {
		a := makeTransport(t)
		b := makeTransport(t)

		require.NotEmpty(t, a.Endpoints())
		require.NotEmpty(t, a.Endpoints()[0])
		require.NotEmpty(t, b.Endpoints())
		require.NotEmpty(t, b.Endpoints()[0])

		ab, ba := dialAcceptHandshake(t, a, b)

		require.NotEmpty(t, ab.LocalEndpoint())
		require.NotEmpty(t, ba.LocalEndpoint())
		require.Equal(t, ab.LocalEndpoint(), ba.RemoteEndpoint())
		require.Equal(t, ab.RemoteEndpoint(), ba.LocalEndpoint())

		err := a.Close()
		require.NoError(t, err)
	})
}

func TestTransport_SendReceive(t *testing.T) {
	testTransports(t, func(t *testing.T, makeTransport transportFactory) {
		a := makeTransport(t)
		b := makeTransport(t)
		ab, ba := dialAcceptHandshake(t, a, b)

		// a to b
		ok, err := ab.SendMessage(chID, []byte("foo"))
		require.NoError(t, err)
		require.True(t, ok)

		ch, msg, err := ba.ReceiveMessage()
		require.NoError(t, err)
		require.Equal(t, []byte("foo"), msg)
		require.Equal(t, chID, ch)

		// b to a
		_, err = ba.SendMessage(chID, []byte("bar"))
		require.NoError(t, err)

		_, msg, err = ab.ReceiveMessage()
		require.NoError(t, err)
		require.Equal(t, []byte("bar"), msg)

		// Close the transports. The connections should still be active.
		err = a.Close()
		require.NoError(t, err)
		err = b.Close()
		require.NoError(t, err)

		_, err = ab.SendMessage(chID, []byte("still here"))
		require.NoError(t, err)
		ch, msg, err = ba.ReceiveMessage()
		require.NoError(t, err)
		require.Equal(t, chID, ch)
		require.Equal(t, []byte("still here"), msg)

		// Close one side of the connection. Both sides should then error
		// with io.EOF when trying to send or receive.
		err = ba.Close()
		require.NoError(t, err)

		_, _, err = ab.ReceiveMessage()
		require.Equal(t, io.EOF, err)
		_, err = ab.SendMessage(chID, []byte("closed")) // FIXME check err
		require.Equal(t, io.EOF, err)

		_, _, err = ba.ReceiveMessage()
		require.Equal(t, io.EOF, err)
		_, err = ba.SendMessage(chID, []byte("closed")) // FIXME check err
		require.Equal(t, io.EOF, err)
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

// dialAccept is a helper that dials b from a and returns both sides of the
// connection.
func dialAccept(t *testing.T, a, b p2p.Transport) (p2p.Connection, p2p.Connection) {
	t.Helper()

	endpoints := b.Endpoints()
	require.NotEmpty(t, endpoints, "peer not listening on any endpoints")

	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

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

	t.Cleanup(func() {
		require.NoError(t, dialConn.Close())
		require.NoError(t, acceptConn.Close())
	})

	return dialConn, acceptConn
}

// dialAcceptHandshake is a helper that dials and handshakes b from a and returns
// both sides of the connection.
func dialAcceptHandshake(t *testing.T, a, b p2p.Transport) (p2p.Connection, p2p.Connection) {
	t.Helper()

	ab, ba := dialAccept(t, a, b)

	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		privKey := ed25519.GenPrivKey()
		nodeInfo := p2p.NodeInfo{NodeID: p2p.NodeIDFromPubKey(privKey.PubKey())}
		_, _, err := ba.Handshake(ctx, nodeInfo, privKey)
		errCh <- err
	}()

	privKey := ed25519.GenPrivKey()
	nodeInfo := p2p.NodeInfo{NodeID: p2p.NodeIDFromPubKey(privKey.PubKey())}
	_, _, err := ab.Handshake(ctx, nodeInfo, privKey)
	require.NoError(t, err)

	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-timer.C:
		require.Fail(t, "handshake timed out")
	}

	return ab, ba
}
