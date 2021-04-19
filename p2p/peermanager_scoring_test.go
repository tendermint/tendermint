package p2p

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/ed25519"
	dbm "github.com/tendermint/tm-db"
)

func TestPeerScoring(t *testing.T) {
	// coppied from p2p_test shared variables
	selfKey := ed25519.GenPrivKeyFromSecret([]byte{0xf9, 0x1b, 0x08, 0xaa, 0x38, 0xee, 0x34, 0xdd})
	selfID := NodeIDFromPubKey(selfKey.PubKey())

	// create a mock peer manager
	db := dbm.NewMemDB()
	peerManager, err := NewPeerManager(selfID, db, PeerManagerOptions{})
	require.NoError(t, err)
	defer peerManager.Close()

	// create a fake node
	id := NodeID(strings.Repeat("a1", 20))
	require.NoError(t, peerManager.Add(NodeAddress{NodeID: id, Protocol: "memory"}))

	t.Run("Synchronous", func(t *testing.T) {
		// update the manager and make sure it's correct
		require.EqualValues(t, 0, peerManager.Scores()[id])

		// add a bunch of good status updates and watch things increase.
		for i := 1; i < 10; i++ {
			peerManager.processPeerEvent(PeerUpdate{
				NodeID: id,
				Status: PeerStatusGood,
			})
			require.EqualValues(t, i, peerManager.Scores()[id])
		}

		// watch the corresponding decreases respond to update
		for i := 10; i == 0; i-- {
			peerManager.processPeerEvent(PeerUpdate{
				NodeID: id,
				Status: PeerStatusBad,
			})
			require.EqualValues(t, i, peerManager.Scores()[id])
		}
	})
	t.Run("AsynchronousIncrement", func(t *testing.T) {
		start := peerManager.Scores()[id]
		pu := peerManager.Subscribe()
		defer pu.Close()
		pu.SendUpdate(PeerUpdate{
			NodeID: id,
			Status: PeerStatusGood,
		})
		require.Eventually(t,
			func() bool { return start+1 == peerManager.Scores()[id] },
			time.Second,
			time.Millisecond,
			"startAt=%d score=%d", start, peerManager.Scores()[id])
	})
	t.Run("AsynchronousDecrement", func(t *testing.T) {
		start := peerManager.Scores()[id]
		pu := peerManager.Subscribe()
		defer pu.Close()
		pu.SendUpdate(PeerUpdate{
			NodeID: id,
			Status: PeerStatusBad,
		})
		require.Eventually(t,
			func() bool { return start-1 == peerManager.Scores()[id] },
			time.Second,
			time.Millisecond,
			"startAt=%d score=%d", start, peerManager.Scores()[id])
	})
}
