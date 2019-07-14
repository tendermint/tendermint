package v2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	targetPending  = 10
	targetReceived = 10
)

func TestScheduleInit(t *testing.T) {
	sc := newSchedule(5, targetPending, targetReceived)

	assert.Equal(t, sc.getStateAtHeight(5), blockStateNew)
	assert.Equal(t, sc.getStateAtHeight(4), blockStateProcessed)
	assert.Equal(t, sc.getStateAtHeight(6), blockStateUnknown)
}

func TestAddPeer(t *testing.T) {
	sc := newSchedule(5, targetPending, targetReceived)
	peerID := "1"
	peerIDTwo := "2"

	assert.EqualNil(t, sc.addPeer(peerID))
	assert.EqualNil(t, sc.addPeer(peerIDTwo))
	assert.Equal(t, sc.addPeer(peerID), errDuplicatePeer)
}

func TestTouchPeer(t *testing.T) {
	sc := newSchedule(5, targetPending, targetReceived)

	peerID := "1"
	peerIDTwo := "2"
	now := time.Now()

	assert.Equal(t, errPeerNotFound, sc.touchPeer(peerID, time.Now()),
		"Touching an unknown peer should return errPeerNotFound")

	assert.EqualNil(t, sc.addPeer(peerID),
		"Adding a peer should return no error")
	assert.EqualNil(t, sc.touchPeer(peerID, now),
		"Touching a peer should return no error")

	threshold := 10 * time.Second
	assert.Equal(t, peerID, len(sc.peersSince(threshold, now.Add(9*time.Second))), 0,
		"Expected no peers to have been touched over 9 seconds")
	assert.Equal(t, peerID, sc.peersSince(threshold, now.Add(11*time.Second))[0].peerID, peerID,
		"Expected one peer to have been touched over 10 seconds ago")
}

func TestPeerHeight(t *testing.T) {
	startHeight := 10
	peerHeight := 20
	peerID := "1"
	sc := newSchedule(startHeight, targetPending, targetReceived)

	assert.EqualNil(t, sc.setPeerHeight(peerID, peerHeight))
	for i := startHeight; i <= peerHeight; i++ {
		assert.Equal(t, sc.getStateAtHeight(i), blockStateNew,
			"Expected all blocks to be in blockStateNew")
		assert.Equal(t, sc.getPeersAtHeight(i)[0].peerID, peerID,
			"Expected the block to be registered to the correct peer")
	}
}

func TestHeightFSM(t *testing.T) {
	startHeight := 10
	peerHeight := 20
	peerID := "1"
	sc := newSchedule(startHeight, targetPending, targetReceived)

	assert.EqualNil(t, sc.addPeer(peerID),
		"Adding a peer should return no error")
	assert.Equal(t, errPeerNotFound, sc.markPending(peerID, peerHeight),
		"Expected markingPending on an unknown peer to return errPeerNotFound")
	assert.EqualNil(t, sc.setPeerHeight(peerID, peerHeight),
		"Expected setPeerHeight to return no error")

	assert.Equal(t, errBadSchedule, sc.markReceived(peerID, peerHeight),
		"Expecting transitioning from blockStateNew to blockStateReceived to fail")
	assert.Equal(t, errBadSchedule, sc.markProcessed(peerID, peerHeight),
		"Expecting transitioning from blockStateNew to blockStateReceived to fail")

	assert.Equal(t, blockStateUnknown, sc.getStateAtHeight(peerHeight+10),
		"Expected the maximum height seen + 10 to be in blockStateUnknown")
	assert.Equal(t, errBadSchedule, sc.markPending(peerID, peerHeight+10),
		"Expected markPending on block in blockStateUnknown height to fail")

	assert.EqualNil(t, sc.markPending(peerID, startHeight),
		"Expected markPending on a known height with a known peer to return no error")
	assert.Equal(t, blockStatePending, sc.getStateAtHeight(startHeight),
		"Expected a the markedBlock to be in blockStatePending")

	assert.EqualNil(t, sc.markReceived(peerID, startHeight),
		"Expected a marking a pending block received to return no error")

	assert.EqualNil(t, sc.resetBlocks(peerID),
		"Expected resetBlocks to return no error")
	assert.Equal(t, blockStateNew, sc.getStateAtHeight(startHeight),
		"Expected blocks to be in blockStateNew after being reset")

	assert.EqualNil(t, sc.markPending(peerID, startHeight),
		"Expected marking a reset block to pending to return no error")
	assert.Equal(t, blockStatePending, sc.getStateAtHeight(startHeight),
		"Expected block to be in blockStatePending")

	assert.EqualNil(t, sc.markReceived(peerID, startHeight),
		"Expected marking a pending block as received to return no error")
	assert.Equal(t, blockStateReceived, sc.getStateAtHeight(startHeight))

	assert.EqualNil(t, sc.markProcessed(peerID, startHeight),
		"Expected marking a block as processed to success")
	assert.Equal(t, blockStateProcessed, sc.getStateAtHeight(startHeight),
		"Expected the block to in blockStateProcessed")
}

func TestMinMaxHeight(t *testing.T) {
	startHeight := 10
	peerHeight := 20
	peerID := "1"
	sc := newSchedule(startHeight, targetPending, targetReceived)

	assert.Equal(t, startHeight, sc.minHeight(),
		"Expected min height to be the initialized height")

	assert.Equal(t, startHeight, sc.maxHeight(),
		"Expected max height to be the initialized height")

	assert.Equal(t, startHeight, sc.maxHeight(),
		"Expected max height to be the initialized height")

	assert.EqualNil(t, sc.addPeer(peerID),
		"Adding a peer should return no error")

	assert.EqualNil(t, sc.setPeerHeight(peerID, peerHeight),
		"Expected setPeerHeight to return no error")

	assert.Equal(t, peerHeight, sc.maxHeight(),
		"Expected max height to increase to peerHeight")

	assert.EqualNil(t, sc.markPending(startHeight),
		"Expected marking startHeight as pending to return no error")

	assert.Equal(t, startHeight+1, sc.minHeight(),
		"Expected marking startHeight as pending to move minHeight forward")
}
