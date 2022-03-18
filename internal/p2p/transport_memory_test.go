package p2p_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

// Transports are mainly tested by common tests in transport_test.go, we
// register a transport factory here to get included in those tests.
func init() {
	var network *p2p.MemoryNetwork // shared by transports in the same test

	testTransports["memory"] = func(t *testing.T) p2p.Transport {
		if network == nil {
			network = p2p.NewMemoryNetwork(log.NewNopLogger(), 1)
		}
		i := byte(network.Size())
		nodeID, err := types.NewNodeID(hex.EncodeToString(bytes.Repeat([]byte{i<<4 + i}, 20)))
		require.NoError(t, err)
		transport := network.CreateTransport(nodeID)

		t.Cleanup(func() {
			require.NoError(t, transport.Close())
			network = nil // set up a new memory network for the next test
		})

		return transport
	}
}
