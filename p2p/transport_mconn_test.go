package p2p_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/conn"
)

// Most tests are done via the test suite in transport_test.go, we register a
// transport factory here to get tested by the suite.
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
