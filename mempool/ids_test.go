package mempool

// import (
// 	"testing"

// 	"github.com/stretchr/testify/require"
// 	"github.com/tendermint/tendermint/types"
// )

// func TestMempoolIDsBasic(t *testing.T) {
// 	ids := NewMempoolIDs()

// 	peerID, err := types.NewNodeID("0011223344556677889900112233445566778899")
// 	require.NoError(t, err)

// 	ids.ReserveForPeer(peerID)
// 	require.EqualValues(t, 1, ids.GetForPeer(peerID))
// 	ids.Reclaim(peerID)

// 	ids.ReserveForPeer(peerID)
// 	require.EqualValues(t, 2, ids.GetForPeer(peerID))
// 	ids.Reclaim(peerID)
// }
