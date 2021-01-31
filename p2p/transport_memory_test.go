package p2p_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
)

// Most tests are done via the test suite in transport_test.go, we register a
// transport factory here to get tested by the suite.
func init() {
	var (
		network *p2p.MemoryNetwork // shared by transports in the same test
		i       byte               // incrementing transport ID
	)

	testTransports["memory"] = func(t *testing.T) p2p.Transport {
		if network == nil {
			network = p2p.NewMemoryNetwork(log.TestingLogger())
		}
		nodeID, err := p2p.NewNodeID(hex.EncodeToString(bytes.Repeat([]byte{i<<4 + i}, 20)))
		require.NoError(t, err)
		transport := network.CreateTransport(nodeID)
		i++

		t.Cleanup(func() {
			require.NoError(t, transport.Close())
			network, i = nil, 0 // set up new memory network per test
		})

		return transport
	}
}
