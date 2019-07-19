package v2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/p2p"
)

const (
	initHeight int64  = 5
	peerID     p2p.ID = "1"
	peerIDTwo  p2p.ID = "2"
	peerHeight int64  = 20
	blockSize  int64  = 1024
)

func TestScheduleInit(t *testing.T) {
	sc := newSchedule(initHeight)

	assert.Equal(t, blockStateNew, sc.getStateAtHeight(initHeight))
	assert.Equal(t, blockStateProcessed, sc.getStateAtHeight(initHeight-1))
	assert.Equal(t, blockStateUnknown, sc.getStateAtHeight(initHeight+1))
}

func TestAddPeer(t *testing.T) {
	sc := newSchedule(initHeight)

	assert.Nil(t, sc.addPeer(peerID))
	assert.Nil(t, sc.addPeer(peerIDTwo))
	assert.Error(t, sc.addPeer(peerID))
}

func TestTouchPeer(t *testing.T) {
	sc := newSchedule(initHeight)
	now := time.Now()

	assert.Error(t, sc.touchPeer(peerID, now),
		"Touching an unknown peer should return errPeerNotFound")

	assert.Nil(t, sc.addPeer(peerID),
		"Adding a peer should return no error")
	assert.Nil(t, sc.touchPeer(peerID, now),
		"Touching a peer should return no error")

	// the peer should
	threshold := 10 * time.Second
	assert.Equal(t, 0, len(sc.peersInactiveSince(threshold, now.Add(9*time.Second))),
		"Expected no peers to have been touched over 9 seconds")
	assert.Containsf(t, sc.peersInactiveSince(threshold, now.Add(11*time.Second)), peerID,
		"Expected one %s to have been touched over 10 seconds ago", peerID)
}

func TestPeerHeight(t *testing.T) {
	sc := newSchedule(initHeight)

	assert.NoError(t, sc.addPeer(peerID),
		"Adding a peer should return no error")
	assert.NoError(t, sc.setPeerHeight(peerID, peerHeight))
	for i := initHeight; i <= peerHeight; i++ {
		assert.Equal(t, sc.getStateAtHeight(i), blockStateNew,
			"Expected all blocks to be in blockStateNew")
		peerIDs := []p2p.ID{}
		for _, peer := range sc.getPeersAtHeight(i) {
			peerIDs = append(peerIDs, peer.peerID)
		}

		assert.Containsf(t, peerIDs, peerID,
			"Expected %s to have block %d", peerID, i)
	}
}

// TODO Split this into transitions
// TODO: Use formatting to describe test failures

func TestHeightFSM(t *testing.T) {
	sc := newSchedule(initHeight)
	now := time.Now()

	assert.Nil(t, sc.addPeer(peerID),
		"Adding a peer should return no error")

	assert.Nil(t, sc.addPeer(peerIDTwo),
		"Adding a peer should return no error")
	assert.Error(t, sc.markPending(peerID, peerHeight, now),
		"Expected markingPending on an unknown peer to return an error")

	assert.Nil(t, sc.setPeerHeight(peerID, peerHeight),
		"Expected setPeerHeight to return no error")
	assert.Nil(t, sc.setPeerHeight(peerIDTwo, peerHeight),
		"Expected setPeerHeight to return no error")

	assert.Error(t, sc.markReceived(peerID, peerHeight, blockSize, now.Add(1*time.Second)),
		"Expecting transitioning from blockStateNew to blockStateReceived to fail")
	assert.Error(t, sc.markProcessed(peerHeight),
		"Expecting transitioning from blockStateNew to blockStateReceived to fail")

	assert.Equal(t, blockStateUnknown, sc.getStateAtHeight(peerHeight+10),
		"Expected the maximum height seen + 10 to be in blockStateUnknown")

	assert.Error(t, sc.markPending(peerID, peerHeight+10, now.Add(1*time.Second)),
		"Expected markPending on block in blockStateUnknown height to fail")
	assert.Nil(t, sc.markPending(peerID, initHeight, now.Add(1*time.Second)),
		"Expected markPending on a known height with a known peer to return no error")
	assert.Equal(t, blockStatePending, sc.getStateAtHeight(initHeight),
		"Expected a the markedBlock to be in blockStatePending")

	assert.Nil(t, sc.markReceived(peerID, initHeight, blockSize, now.Add(2*time.Second)),
		"Expected marking markReceived on a pending block to return no error")

	assert.NoError(t, sc.removePeer(peerID),
		"Expected resetBlocks to return no error")
	assert.Equal(t, blockStateNew, sc.getStateAtHeight(initHeight),
		"Expected blocks to be in blockStateNew after being reset")

	assert.NoError(t, sc.markPending(peerIDTwo, initHeight, now),
		"Expected marking a reset block to pending to return no error")
	assert.Equal(t, blockStatePending, sc.getStateAtHeight(initHeight),
		"Expected block to be in blockStatePending")

	assert.NoError(t, sc.markReceived(peerIDTwo, initHeight, blockSize, now.Add(2*time.Second)),
		"Expected marking a pending block as received to return no error")
	assert.Equal(t, blockStateReceived, sc.getStateAtHeight(initHeight))

	assert.NoError(t, sc.markProcessed(initHeight),
		"Expected marking a block as processed to success")
	assert.Equal(t, blockStateProcessed, sc.getStateAtHeight(initHeight),
		"Expected the block to in blockStateProcessed")
}

func TestMinMaxHeight(t *testing.T) {
	sc := newSchedule(initHeight)
	now := time.Now()

	assert.Equal(t, initHeight, sc.minHeight(),
		"Expected min height to be the initialized height")

	assert.Equal(t, initHeight, sc.maxHeight(),
		"Expected max height to be the initialized height")

	assert.NoError(t, sc.addPeer(peerID),
		"Adding a peer should return no error")

	assert.NoError(t, sc.setPeerHeight(peerID, peerHeight),
		"Expected setPeerHeight to return no error")

	assert.Equal(t, peerHeight, sc.maxHeight(),
		"Expected max height to increase to peerHeight")

	assert.Nil(t, sc.markPending(peerID, initHeight, now.Add(1*time.Second)),
		"Expected marking initHeight as pending to return no error")

	assert.Equal(t, initHeight+1, sc.minHeight(),
		"Expected marking initHeight as pending to move minHeight forward")
}

func TestPeersSlowerThan(t *testing.T) {
	sc := newSchedule(initHeight)
	now := time.Now()
	receivedAt := now.Add(1 * time.Second)

	assert.NoError(t, sc.addPeer(peerID),
		"Adding a peer should return no error")

	assert.NoError(t, sc.setPeerHeight(peerID, peerHeight),
		"Expected setPeerHeight to return no error")

	assert.NoError(t, sc.markPending(peerID, peerHeight, now),
		"Expected markingPending on to return no error")

	assert.NoError(t, sc.markReceived(peerID, peerHeight, blockSize, receivedAt),
		"Expected markingPending on to return no error")

	assert.Empty(t, sc.peersSlowerThan(blockSize-1),
		"expected no peers to be slower than blockSize-1 bytes/sec")

	assert.Containsf(t, sc.peersSlowerThan(blockSize+1), peerID,
		"expected %s to be slower than blockSize+1 bytes/sec", peerID)
}
