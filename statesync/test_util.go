package statesync

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/p2p"
	p2pmocks "github.com/tendermint/tendermint/p2p/mocks"
)

func tryUntil(t *testing.T, tryFn func() bool, tick, timeout time.Duration) {
	t.Helper()

	ticker := time.Tick(tick)

	for {
		select {
		case <-ticker:
			if tryFn() {
				return
			}
		case <-time.After(timeout):
			t.FailNow()
		}
	}
}

// Sets up a simple peer mock with an ID
func simplePeer(t *testing.T, id string) (*p2pmocks.Peer, p2p.PeerID) {
	t.Helper()

	peer := &p2pmocks.Peer{}
	peer.On("ID").Return(p2p.ID(id))

	pID, err := p2p.PeerIDFromString(string(peer.ID()))
	require.NoError(t, err)

	return peer, pID
}
