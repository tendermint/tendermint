package p2p_test

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/types"
)

// transportFactory is used to set up transports for tests.
type transportFactory func(t *testing.T) p2p.Transport

// testTransports is a registry of transport factories for withTransports().
var testTransports = map[string]transportFactory{}

// withTransports is a test helper that runs a test against all transports
// registered in testTransports.
func withTransports(t *testing.T, tester func(*testing.T, transportFactory)) {
	t.Helper()
	for name, transportFactory := range testTransports {
		transportFactory := transportFactory
		t.Run(name, func(t *testing.T) {
			t.Cleanup(leaktest.Check(t))
			tester(t, transportFactory)
		})
	}
}

func TestTransport_AcceptClose(t *testing.T) {
	// Just test accept unblock on close, happy path is tested widely elsewhere.
	withTransports(t, func(t *testing.T, makeTransport transportFactory) {
		a := makeTransport(t)

		// In-progress Accept should error on concurrent close.
		errCh := make(chan error, 1)
		go func() {
			time.Sleep(200 * time.Millisecond)
			errCh <- a.Close()
		}()

		_, err := a.Accept()
		require.Error(t, err)
		require.Equal(t, io.EOF, err)
		require.NoError(t, <-errCh)

		// Closed transport should return error immediately.
		_, err = a.Accept()
		require.Error(t, err)
		require.Equal(t, io.EOF, err)
	})
}

func TestTransport_DialEndpoints(t *testing.T) {
	ipTestCases := []struct {
		ip net.IP
		ok bool
	}{
		{net.IPv4zero, true},
		{net.IPv6zero, true},

		{nil, false},
		{net.IPv4bcast, false},
		{net.IPv4allsys, false},
		{[]byte{1, 2, 3}, false},
		{[]byte{1, 2, 3, 4, 5}, false},
	}

	withTransports(t, func(t *testing.T, makeTransport transportFactory) {
		a := makeTransport(t)
		endpoints := a.Endpoints()
		require.NotEmpty(t, endpoints)
		endpoint := endpoints[0]

		// Spawn a goroutine to simply accept any connections until closed.
		go func() {
			for {
				conn, err := a.Accept()
				if err != nil {
					return
				}
				_ = conn.Close()
			}
		}()

		// Dialing self should work.
		conn, err := a.Dial(ctx, endpoint)
		require.NoError(t, err)
		require.NoError(t, conn.Close())

		// Dialing empty endpoint should error.
		_, err = a.Dial(ctx, p2p.Endpoint{})
		require.Error(t, err)

		// Dialing without protocol should error.
		noProtocol := endpoint
		noProtocol.Protocol = ""
		_, err = a.Dial(ctx, noProtocol)
		require.Error(t, err)

		// Dialing with invalid protocol should error.
		fooProtocol := endpoint
		fooProtocol.Protocol = "foo"
		_, err = a.Dial(ctx, fooProtocol)
		require.Error(t, err)

		// Tests for networked endpoints (with IP).
		if len(endpoint.IP) > 0 && endpoint.Protocol != p2p.MemoryProtocol {
			for _, tc := range ipTestCases {
				tc := tc
				t.Run(tc.ip.String(), func(t *testing.T) {
					e := endpoint
					e.IP = tc.ip
					conn, err := a.Dial(ctx, e)
					if tc.ok {
						require.NoError(t, conn.Close())
						require.NoError(t, err)
					} else {
						require.Error(t, err, "endpoint=%s", e)
					}
				})
			}

			// Non-networked endpoints should error.
			noIP := endpoint
			noIP.IP = nil
			noIP.Port = 0
			noIP.Path = "foo"
			_, err := a.Dial(ctx, noIP)
			require.Error(t, err)

		} else {
			// Tests for non-networked endpoints (no IP).
			noPath := endpoint
			noPath.Path = ""
			_, err = a.Dial(ctx, noPath)
			require.Error(t, err)
		}
	})
}

func TestTransport_Dial(t *testing.T) {
	// Most just tests dial failures, happy path is tested widely elsewhere.
	withTransports(t, func(t *testing.T, makeTransport transportFactory) {
		a := makeTransport(t)
		b := makeTransport(t)

		require.NotEmpty(t, a.Endpoints())
		require.NotEmpty(t, b.Endpoints())
		aEndpoint := a.Endpoints()[0]
		bEndpoint := b.Endpoints()[0]

		// Context cancellation should error. We can't test timeouts since we'd
		// need a non-responsive endpoint.
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()
		_, err := a.Dial(cancelCtx, bEndpoint)
		require.Error(t, err)
		require.Equal(t, err, context.Canceled)

		// Unavailable endpoint should error.
		err = b.Close()
		require.NoError(t, err)
		_, err = a.Dial(ctx, bEndpoint)
		require.Error(t, err)

		// Dialing from a closed transport should still work.
		errCh := make(chan error, 1)
		go func() {
			conn, err := a.Accept()
			if err == nil {
				_ = conn.Close()
			}
			errCh <- err
		}()
		conn, err := b.Dial(ctx, aEndpoint)
		require.NoError(t, err)
		require.NoError(t, conn.Close())
		require.NoError(t, <-errCh)
	})
}

func TestTransport_Endpoints(t *testing.T) {
	withTransports(t, func(t *testing.T, makeTransport transportFactory) {
		a := makeTransport(t)
		b := makeTransport(t)

		// Both transports return valid and different endpoints.
		aEndpoints := a.Endpoints()
		bEndpoints := b.Endpoints()
		require.NotEmpty(t, aEndpoints)
		require.NotEmpty(t, bEndpoints)
		require.NotEqual(t, aEndpoints, bEndpoints)
		for _, endpoint := range append(aEndpoints, bEndpoints...) {
			err := endpoint.Validate()
			require.NoError(t, err, "invalid endpoint %q", endpoint)
		}

		// When closed, the transport should no longer return any endpoints.
		err := a.Close()
		require.NoError(t, err)
		require.Empty(t, a.Endpoints())
		require.NotEmpty(t, b.Endpoints())
	})
}

func TestTransport_Protocols(t *testing.T) {
	withTransports(t, func(t *testing.T, makeTransport transportFactory) {
		a := makeTransport(t)
		protocols := a.Protocols()
		endpoints := a.Endpoints()
		require.NotEmpty(t, protocols)
		require.NotEmpty(t, endpoints)

		for _, endpoint := range endpoints {
			require.Contains(t, protocols, endpoint.Protocol)
		}
	})
}

func TestTransport_String(t *testing.T) {
	withTransports(t, func(t *testing.T, makeTransport transportFactory) {
		a := makeTransport(t)
		require.NotEmpty(t, a.String())
	})
}

func TestConnection_Handshake(t *testing.T) {
	withTransports(t, func(t *testing.T, makeTransport transportFactory) {
		a := makeTransport(t)
		b := makeTransport(t)
		ab, ba := dialAccept(t, a, b)

		// A handshake should pass the given keys and NodeInfo.
		aKey := ed25519.GenPrivKey()
		aInfo := types.NodeInfo{
			NodeID: types.NodeIDFromPubKey(aKey.PubKey()),
			ProtocolVersion: types.ProtocolVersion{
				P2P:   1,
				Block: 2,
				App:   3,
			},
			ListenAddr: "listenaddr",
			Network:    "network",
			Version:    "1.2.3",
			Channels:   bytes.HexBytes([]byte{0xf0, 0x0f}),
			Moniker:    "moniker",
			Other: types.NodeInfoOther{
				TxIndex:    "txindex",
				RPCAddress: "rpc.domain.com",
			},
		}
		bKey := ed25519.GenPrivKey()
		bInfo := types.NodeInfo{NodeID: types.NodeIDFromPubKey(bKey.PubKey())}

		errCh := make(chan error, 1)
		go func() {
			// Must use assert due to goroutine.
			peerInfo, peerKey, err := ba.Handshake(ctx, 0, bInfo, bKey)
			if err == nil {
				assert.Equal(t, aInfo, peerInfo)
				assert.Equal(t, aKey.PubKey(), peerKey)
			}
			errCh <- err
		}()

		peerInfo, peerKey, err := ab.Handshake(ctx, 0, aInfo, aKey)
		require.NoError(t, err)
		require.Equal(t, bInfo, peerInfo)
		require.Equal(t, bKey.PubKey(), peerKey)

		require.NoError(t, <-errCh)
	})
}

func TestConnection_HandshakeCancel(t *testing.T) {
	withTransports(t, func(t *testing.T, makeTransport transportFactory) {
		a := makeTransport(t)
		b := makeTransport(t)

		// Handshake should error on context cancellation.
		ab, ba := dialAccept(t, a, b)
		timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
		cancel()
		_, _, err := ab.Handshake(timeoutCtx, 0, types.NodeInfo{}, ed25519.GenPrivKey())
		require.Error(t, err)
		require.Equal(t, context.Canceled, err)
		_ = ab.Close()
		_ = ba.Close()

		// Handshake should error on context timeout.
		ab, ba = dialAccept(t, a, b)
		timeoutCtx, cancel = context.WithTimeout(ctx, 200*time.Millisecond)
		defer cancel()
		_, _, err = ab.Handshake(timeoutCtx, 0, types.NodeInfo{}, ed25519.GenPrivKey())
		require.Error(t, err)
		require.Equal(t, context.DeadlineExceeded, err)
		_ = ab.Close()
		_ = ba.Close()
	})
}

func TestConnection_FlushClose(t *testing.T) {
	withTransports(t, func(t *testing.T, makeTransport transportFactory) {
		a := makeTransport(t)
		b := makeTransport(t)
		ab, _ := dialAcceptHandshake(t, a, b)

		// FIXME: FlushClose should be removed (and replaced by separate Flush
		// and Close calls if necessary). We can't reliably test it, so we just
		// make sure it closes both ends and that it's idempotent.
		err := ab.FlushClose()
		require.NoError(t, err)

		_, _, err = ab.ReceiveMessage()
		require.Error(t, err)
		require.Equal(t, io.EOF, err)

		_, err = ab.SendMessage(chID, []byte("closed"))
		require.Error(t, err)
		require.Equal(t, io.EOF, err)

		err = ab.FlushClose()
		require.NoError(t, err)
	})
}

func TestConnection_LocalRemoteEndpoint(t *testing.T) {
	withTransports(t, func(t *testing.T, makeTransport transportFactory) {
		a := makeTransport(t)
		b := makeTransport(t)
		ab, ba := dialAcceptHandshake(t, a, b)

		// Local and remote connection endpoints correspond to each other.
		require.NotEmpty(t, ab.LocalEndpoint())
		require.NotEmpty(t, ba.LocalEndpoint())
		require.Equal(t, ab.LocalEndpoint(), ba.RemoteEndpoint())
		require.Equal(t, ab.RemoteEndpoint(), ba.LocalEndpoint())
	})
}

func TestConnection_SendReceive(t *testing.T) {
	withTransports(t, func(t *testing.T, makeTransport transportFactory) {
		a := makeTransport(t)
		b := makeTransport(t)
		ab, ba := dialAcceptHandshake(t, a, b)

		// Can send and receive a to b.
		ok, err := ab.SendMessage(chID, []byte("foo"))
		require.NoError(t, err)
		require.True(t, ok)

		ch, msg, err := ba.ReceiveMessage()
		require.NoError(t, err)
		require.Equal(t, []byte("foo"), msg)
		require.Equal(t, chID, ch)

		// Can send and receive b to a.
		_, err = ba.SendMessage(chID, []byte("bar"))
		require.NoError(t, err)

		_, msg, err = ab.ReceiveMessage()
		require.NoError(t, err)
		require.Equal(t, []byte("bar"), msg)

		// TrySendMessage also works.
		ok, err = ba.TrySendMessage(chID, []byte("try"))
		require.NoError(t, err)
		require.True(t, ok)

		ch, msg, err = ab.ReceiveMessage()
		require.NoError(t, err)
		require.Equal(t, []byte("try"), msg)
		require.Equal(t, chID, ch)

		// Connections should still be active after closing the transports.
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
		require.Error(t, err)
		require.Equal(t, io.EOF, err)
		_, err = ab.TrySendMessage(chID, []byte("closed try"))
		require.Error(t, err)
		require.Equal(t, io.EOF, err)
		_, err = ab.SendMessage(chID, []byte("closed"))
		require.Error(t, err)
		require.Equal(t, io.EOF, err)

		_, _, err = ba.ReceiveMessage()
		require.Error(t, err)
		require.Equal(t, io.EOF, err)
		_, err = ba.TrySendMessage(chID, []byte("closed try"))
		require.Error(t, err)
		require.Equal(t, io.EOF, err)
		_, err = ba.SendMessage(chID, []byte("closed"))
		require.Error(t, err)
		require.Equal(t, io.EOF, err)
	})
}

func TestConnection_Status(t *testing.T) {
	withTransports(t, func(t *testing.T, makeTransport transportFactory) {
		a := makeTransport(t)
		b := makeTransport(t)
		ab, _ := dialAcceptHandshake(t, a, b)

		// FIXME: This isn't implemented in all transports, so for now we just
		// check that it doesn't panic, which isn't really much of a test.
		ab.Status()
	})
}

func TestConnection_String(t *testing.T) {
	withTransports(t, func(t *testing.T, makeTransport transportFactory) {
		a := makeTransport(t)
		b := makeTransport(t)
		ab, _ := dialAccept(t, a, b)
		require.NotEmpty(t, ab.String())
	})
}

func TestEndpoint_NodeAddress(t *testing.T) {
	var (
		ip4    = []byte{1, 2, 3, 4}
		ip4in6 = net.IPv4(1, 2, 3, 4)
		ip6    = []byte{0xb1, 0x0c, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01}
		id     = types.NodeID("00112233445566778899aabbccddeeff00112233")
	)

	testcases := []struct {
		endpoint p2p.Endpoint
		expect   p2p.NodeAddress
	}{
		// Valid endpoints.
		{
			p2p.Endpoint{Protocol: "tcp", IP: ip4, Port: 8080, Path: "path"},
			p2p.NodeAddress{Protocol: "tcp", Hostname: "1.2.3.4", Port: 8080, Path: "path"},
		},
		{
			p2p.Endpoint{Protocol: "tcp", IP: ip4in6, Port: 8080, Path: "path"},
			p2p.NodeAddress{Protocol: "tcp", Hostname: "1.2.3.4", Port: 8080, Path: "path"},
		},
		{
			p2p.Endpoint{Protocol: "tcp", IP: ip6, Port: 8080, Path: "path"},
			p2p.NodeAddress{Protocol: "tcp", Hostname: "b10c::1", Port: 8080, Path: "path"},
		},
		{
			p2p.Endpoint{Protocol: "memory", Path: "foo"},
			p2p.NodeAddress{Protocol: "memory", Path: "foo"},
		},
		{
			p2p.Endpoint{Protocol: "memory", Path: string(id)},
			p2p.NodeAddress{Protocol: "memory", Path: string(id)},
		},

		// Partial (invalid) endpoints.
		{p2p.Endpoint{}, p2p.NodeAddress{}},
		{p2p.Endpoint{Protocol: "tcp"}, p2p.NodeAddress{Protocol: "tcp"}},
		{p2p.Endpoint{IP: net.IPv4(1, 2, 3, 4)}, p2p.NodeAddress{Hostname: "1.2.3.4"}},
		{p2p.Endpoint{Port: 8080}, p2p.NodeAddress{}},
		{p2p.Endpoint{Path: "path"}, p2p.NodeAddress{Path: "path"}},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.endpoint.String(), func(t *testing.T) {
			// Without NodeID.
			expect := tc.expect
			require.Equal(t, expect, tc.endpoint.NodeAddress(""))

			// With NodeID.
			expect.NodeID = id
			require.Equal(t, expect, tc.endpoint.NodeAddress(expect.NodeID))
		})
	}
}

func TestEndpoint_String(t *testing.T) {
	var (
		ip4    = []byte{1, 2, 3, 4}
		ip4in6 = net.IPv4(1, 2, 3, 4)
		ip6    = []byte{0xb1, 0x0c, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01}
		nodeID = types.NodeID("00112233445566778899aabbccddeeff00112233")
	)

	testcases := []struct {
		endpoint p2p.Endpoint
		expect   string
	}{
		// Non-networked endpoints.
		{p2p.Endpoint{Protocol: "memory", Path: string(nodeID)}, "memory:" + string(nodeID)},
		{p2p.Endpoint{Protocol: "file", Path: "foo"}, "file:///foo"},
		{p2p.Endpoint{Protocol: "file", Path: "👋"}, "file:///%F0%9F%91%8B"},

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
		{p2p.Endpoint{Protocol: "tcp"}, false},
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
		conn, err := b.Accept()
		errCh <- err
		acceptCh <- conn
	}()

	dialConn, err := a.Dial(ctx, endpoints[0])
	require.NoError(t, err)

	acceptConn := <-acceptCh
	require.NoError(t, <-errCh)

	t.Cleanup(func() {
		_ = dialConn.Close()
		_ = acceptConn.Close()
	})

	return dialConn, acceptConn
}

// dialAcceptHandshake is a helper that dials and handshakes b from a and
// returns both sides of the connection.
func dialAcceptHandshake(t *testing.T, a, b p2p.Transport) (p2p.Connection, p2p.Connection) {
	t.Helper()

	ab, ba := dialAccept(t, a, b)

	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		privKey := ed25519.GenPrivKey()
		nodeInfo := types.NodeInfo{NodeID: types.NodeIDFromPubKey(privKey.PubKey())}
		_, _, err := ba.Handshake(ctx, 0, nodeInfo, privKey)
		errCh <- err
	}()

	privKey := ed25519.GenPrivKey()
	nodeInfo := types.NodeInfo{NodeID: types.NodeIDFromPubKey(privKey.PubKey())}
	_, _, err := ab.Handshake(ctx, 0, nodeInfo, privKey)
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
