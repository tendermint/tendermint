package p2p_test

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/conn"
)

// Transports are mainly tested by common tests in transport_test.go, we
// register a transport factory here to get included in those tests.
func init() {
	testTransports["mconn"] = func(t *testing.T) p2p.Transport {
		transport := p2p.NewMConnTransport(
			log.TestingLogger(),
			conn.DefaultMConnConfig(),
			[]*p2p.ChannelDescriptor{{ID: byte(chID), Priority: 1}},
			p2p.MConnTransportOptions{},
		)
		err := transport.Listen(p2p.Endpoint{
			Protocol: p2p.MConnProtocol,
			IP:       net.IPv4(127, 0, 0, 1),
			Port:     0, // assign a random port
		})
		require.NoError(t, err)

		t.Cleanup(func() {
			require.NoError(t, transport.Close())
		})

		return transport
	}
}

func TestMConnTransport_AcceptBeforeListen(t *testing.T) {
	transport := p2p.NewMConnTransport(
		log.TestingLogger(),
		conn.DefaultMConnConfig(),
		[]*p2p.ChannelDescriptor{{ID: byte(chID), Priority: 1}},
		p2p.MConnTransportOptions{
			MaxAcceptedConnections: 2,
		},
	)
	t.Cleanup(func() {
		_ = transport.Close()
	})

	_, err := transport.Accept()
	require.Error(t, err)
	require.NotEqual(t, io.EOF, err) // io.EOF should be returned after Close()
}

func TestMConnTransport_AcceptMaxAcceptedConnections(t *testing.T) {
	transport := p2p.NewMConnTransport(
		log.TestingLogger(),
		conn.DefaultMConnConfig(),
		[]*p2p.ChannelDescriptor{{ID: byte(chID), Priority: 1}},
		p2p.MConnTransportOptions{
			MaxAcceptedConnections: 2,
		},
	)
	t.Cleanup(func() {
		_ = transport.Close()
	})
	err := transport.Listen(p2p.Endpoint{
		Protocol: p2p.MConnProtocol,
		IP:       net.IPv4(127, 0, 0, 1),
	})
	require.NoError(t, err)
	require.NotEmpty(t, transport.Endpoints())
	endpoint := transport.Endpoints()[0]

	// Start a goroutine to just accept any connections.
	acceptCh := make(chan p2p.Connection, 10)
	go func() {
		for {
			conn, err := transport.Accept()
			if err != nil {
				return
			}
			acceptCh <- conn
		}
	}()

	// The first two connections should be accepted just fine.
	dial1, err := transport.Dial(ctx, endpoint)
	require.NoError(t, err)
	defer dial1.Close()
	accept1 := <-acceptCh
	defer accept1.Close()
	require.Equal(t, dial1.LocalEndpoint(), accept1.RemoteEndpoint())

	dial2, err := transport.Dial(ctx, endpoint)
	require.NoError(t, err)
	defer dial2.Close()
	accept2 := <-acceptCh
	defer accept2.Close()
	require.Equal(t, dial2.LocalEndpoint(), accept2.RemoteEndpoint())

	// The third connection will be dialed successfully, but the accept should
	// not go through.
	dial3, err := transport.Dial(ctx, endpoint)
	require.NoError(t, err)
	defer dial3.Close()
	select {
	case <-acceptCh:
		require.Fail(t, "unexpected accept")
	case <-time.After(time.Second):
	}

	// However, once either of the other connections are closed, the accept
	// goes through.
	require.NoError(t, accept1.Close())
	accept3 := <-acceptCh
	defer accept3.Close()
	require.Equal(t, dial3.LocalEndpoint(), accept3.RemoteEndpoint())
}

func TestMConnTransport_Listen(t *testing.T) {
	testcases := []struct {
		endpoint p2p.Endpoint
		ok       bool
	}{
		// Valid v4 and v6 addresses, with mconn and tcp protocols.
		{p2p.Endpoint{Protocol: p2p.MConnProtocol, IP: net.IPv4zero}, true},
		{p2p.Endpoint{Protocol: p2p.MConnProtocol, IP: net.IPv4(127, 0, 0, 1)}, true},
		{p2p.Endpoint{Protocol: p2p.MConnProtocol, IP: net.IPv6zero}, true},
		{p2p.Endpoint{Protocol: p2p.MConnProtocol, IP: net.IPv6loopback}, true},
		{p2p.Endpoint{Protocol: p2p.TCPProtocol, IP: net.IPv4zero}, true},

		// Invalid endpoints.
		{p2p.Endpoint{}, false},
		{p2p.Endpoint{Protocol: p2p.MConnProtocol, Path: "foo"}, false},
		{p2p.Endpoint{Protocol: p2p.MConnProtocol, IP: net.IPv4zero, Path: "foo"}, false},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.endpoint.String(), func(t *testing.T) {
			t.Cleanup(leaktest.Check(t))

			transport := p2p.NewMConnTransport(
				log.TestingLogger(),
				conn.DefaultMConnConfig(),
				[]*p2p.ChannelDescriptor{{ID: byte(chID), Priority: 1}},
				p2p.MConnTransportOptions{},
			)
			t.Cleanup(func() {
				_ = transport.Close()
			})

			// Transport should not listen on any endpoints yet.
			require.Empty(t, transport.Endpoints())

			// Start listening, and check any expected errors.
			err := transport.Listen(tc.endpoint)
			if !tc.ok {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Start a goroutine to just accept any connections.
			go func() {
				for {
					conn, err := transport.Accept()
					if err != nil {
						return
					}
					defer func() {
						_ = conn.Close()
					}()
				}
			}()

			// Check the endpoint.
			endpoints := transport.Endpoints()
			require.Len(t, endpoints, 1)
			endpoint := endpoints[0]

			require.Equal(t, p2p.MConnProtocol, endpoint.Protocol)
			if tc.endpoint.IP.IsUnspecified() {
				require.True(t, endpoint.IP.IsUnspecified(),
					"expected unspecified IP, got %v", endpoint.IP)
			} else {
				require.True(t, tc.endpoint.IP.Equal(endpoint.IP),
					"expected %v, got %v", tc.endpoint.IP, endpoint.IP)
			}
			require.NotZero(t, endpoint.Port)
			require.Empty(t, endpoint.Path)

			// Dialing the endpoint should work.
			conn, err := transport.Dial(ctx, endpoint)
			require.NoError(t, err)
			require.NoError(t, conn.Close())

			// Trying to listen again should error.
			err = transport.Listen(tc.endpoint)
			require.Error(t, err)
		})
	}
}
