package mempool

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/types"
)

func TestMempoolIDsBasic(t *testing.T) {
	ids := NewMempoolIDs()

	peerID, err := types.NewNodeID("0011223344556677889900112233445566778899")
	require.NoError(t, err)
	require.EqualValues(t, 0, ids.GetForPeer(peerID))

	ids.ReserveForPeer(peerID)
	require.EqualValues(t, 1, ids.GetForPeer(peerID))

	ids.Reclaim(peerID)
	require.EqualValues(t, 0, ids.GetForPeer(peerID))

	ids.ReserveForPeer(peerID)
	require.EqualValues(t, 1, ids.GetForPeer(peerID))
}

func TestMempoolIDsPeerDupReserve(t *testing.T) {
	ids := NewMempoolIDs()

	peerID, err := types.NewNodeID("0011223344556677889900112233445566778899")
	require.NoError(t, err)
	require.EqualValues(t, 0, ids.GetForPeer(peerID))

	ids.ReserveForPeer(peerID)
	require.EqualValues(t, 1, ids.GetForPeer(peerID))

	ids.ReserveForPeer(peerID)
	require.EqualValues(t, 1, ids.GetForPeer(peerID))
}

func TestMempoolIDs2Peers(t *testing.T) {
	ids := NewMempoolIDs()

	peer1ID, _ := types.NewNodeID("0011223344556677889900112233445566778899")
	require.EqualValues(t, 0, ids.GetForPeer(peer1ID))

	ids.ReserveForPeer(peer1ID)
	require.EqualValues(t, 1, ids.GetForPeer(peer1ID))

	ids.Reclaim(peer1ID)
	require.EqualValues(t, 0, ids.GetForPeer(peer1ID))

	peer2ID, _ := types.NewNodeID("1011223344556677889900112233445566778899")

	ids.ReserveForPeer(peer2ID)
	require.EqualValues(t, 1, ids.GetForPeer(peer2ID))

	ids.ReserveForPeer(peer1ID)
	require.EqualValues(t, 2, ids.GetForPeer(peer1ID))
}

func TestMempoolIDsNextExistID(t *testing.T) {
	ids := NewMempoolIDs()

	peer1ID, _ := types.NewNodeID("0011223344556677889900112233445566778899")
	ids.ReserveForPeer(peer1ID)
	require.EqualValues(t, 1, ids.GetForPeer(peer1ID))

	peer2ID, _ := types.NewNodeID("1011223344556677889900112233445566778899")
	ids.ReserveForPeer(peer2ID)
	require.EqualValues(t, 2, ids.GetForPeer(peer2ID))

	peer3ID, _ := types.NewNodeID("2011223344556677889900112233445566778899")
	ids.ReserveForPeer(peer3ID)
	require.EqualValues(t, 3, ids.GetForPeer(peer3ID))

	ids.Reclaim(peer1ID)
	require.EqualValues(t, 0, ids.GetForPeer(peer1ID))

	ids.Reclaim(peer3ID)
	require.EqualValues(t, 0, ids.GetForPeer(peer3ID))

	ids.ReserveForPeer(peer1ID)
	require.EqualValues(t, 1, ids.GetForPeer(peer1ID))

	ids.ReserveForPeer(peer3ID)
	require.EqualValues(t, 3, ids.GetForPeer(peer3ID))
}
