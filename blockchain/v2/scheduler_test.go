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
	blockSize  int64  = 10
)

func TestScheduleInit(t *testing.T) {
	sc := newSchedule(initHeight)

	assert.Equal(t, sc.getStateAtHeight(initHeight), blockStateNew)
	assert.Equal(t, sc.getStateAtHeight(initHeight+1), blockStateProcessed)
	assert.Equal(t, sc.getStateAtHeight(initHeight-1), blockStateUnknown)
}

func TestAddPeer(t *testing.T) {
	sc := newSchedule(initHeight)

	assert.Nil(t, sc.addPeer(peerID))
	assert.Nil(t, sc.addPeer(peerIDTwo))
	assert.Equal(t, sc.addPeer(peerID), errDuplicatePeer)
}

func TestTouchPeer(t *testing.T) {
	sc := newSchedule(initHeight)
	now := time.Now()

	assert.Equal(t, errPeerNotFound, sc.touchPeer(peerID, now),
		"Touching an unknown peer should return errPeerNotFound")

	assert.Nil(t, sc.addPeer(peerID),
		"Adding a peer should return no error")
	assert.Nil(t, sc.touchPeer(peerID, now),
		"Touching a peer should return no error")

	threshold := 10 * time.Second
	assert.Equal(t, len(sc.peersSince(threshold, now.Add(9*time.Second))), 0,
		"Expected no peers to have been touched over 9 seconds")
	assert.Equal(t, peerID, sc.peersSince(threshold, now.Add(11*time.Second))[0].peerID,
		"Expected one peer to have been touched over 10 seconds ago")
}

func TestPeerHeight(t *testing.T) {
	sc := newSchedule(initHeight)

	assert.Nil(t, sc.setPeerHeight(peerID, peerHeight))
	for i := initHeight; i <= peerHeight; i++ {
		assert.Equal(t, sc.getStateAtHeight(i), blockStateNew,
			"Expected all blocks to be in blockStateNew")
		assert.Equal(t, len(sc.getPeersAtHeight(i)), 1,
			"Expected the block to be registered to the 1 peer")
		//TODO check peerID
	}
}

func TestHeightFSM(t *testing.T) {
	sc := newSchedule(initHeight)
	now := time.Now()

	assert.Nil(t, sc.addPeer(peerID),
		"Adding a peer should return no error")
	assert.Equal(t, errPeerNotFound, sc.markPending(peerID, peerHeight, now),
		"Expected markingPending on an unknown peer to return errPeerNotFound")
	assert.Nil(t, sc.setPeerHeight(peerID, peerHeight),
		"Expected setPeerHeight to return no error")

	assert.Equal(t, errBadSchedule, sc.markReceived(peerID, peerHeight, blockSize, now.Add(1*time.Second)),
		"Expecting transitioning from blockStateNew to blockStateReceived to fail")
	assert.Equal(t, errBadSchedule, sc.markProcessed(peerHeight),
		"Expecting transitioning from blockStateNew to blockStateReceived to fail")

	assert.Equal(t, blockStateUnknown, sc.getStateAtHeight(peerHeight+10),
		"Expected the maximum height seen + 10 to be in blockStateUnknown")
	assert.Equal(t, errBadSchedule, sc.markPending(peerID, peerHeight+10, now.Add(1*time.Second)),
		"Expected markPending on block in blockStateUnknown height to fail")

	assert.Nil(t, sc.markPending(peerID, initHeight, now.Add(1*time.Second)),
		"Expected markPending on a known height with a known peer to return no error")
	assert.Equal(t, blockStatePending, sc.getStateAtHeight(initHeight),
		"Expected a the markedBlock to be in blockStatePending")

	assert.Nil(t, sc.markReceived(peerID, initHeight, blockSize, now.Add(2*time.Second)),
		"Expected a marking a pending block received to return no error")

	assert.Nil(t, sc.resetBlocks(peerID),
		"Expected resetBlocks to return no error")
	assert.Equal(t, blockStateNew, sc.getStateAtHeight(initHeight),
		"Expected blocks to be in blockStateNew after being reset")

	assert.Nil(t, sc.markPending(peerID, initHeight, now),
		"Expected marking a reset block to pending to return no error")
	assert.Equal(t, blockStatePending, sc.getStateAtHeight(initHeight),
		"Expected block to be in blockStatePending")

	assert.Nil(t, sc.markReceived(peerID, blockSize, initHeight, now.Add(2*time.Second)),
		"Expected marking a pending block as received to return no error")
	assert.Equal(t, blockStateReceived, sc.getStateAtHeight(initHeight))

	assert.Nil(t, sc.markProcessed(initHeight),
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

	assert.Equal(t, initHeight, sc.maxHeight(),
		"Expected max height to be the initialized height")

	assert.Nil(t, sc.addPeer(peerID),
		"Adding a peer should return no error")

	assert.Nil(t, sc.setPeerHeight(peerID, peerHeight),
		"Expected setPeerHeight to return no error")

	assert.Equal(t, peerHeight, sc.maxHeight(),
		"Expected max height to increase to peerHeight")

	assert.Nil(t, sc.markPending(peerID, initHeight, now.Add(1*time.Second)),
		"Expected marking initHeight as pending to return no error")

	assert.Equal(t, initHeight+1, sc.minHeight(),
		"Expected marking initHeight as pending to move minHeight forward")
}

func TestLatePeers(t *testing.T) {
}

func TestSlowPeers(t *testing.T) {
}
