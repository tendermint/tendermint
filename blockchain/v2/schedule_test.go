package v2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/p2p"
)

func TestScheduleInit(t *testing.T) {
	var (
		initHeight int64 = 5
		sc               = newSchedule(initHeight)
	)

	assert.Equal(t, blockStateNew, sc.getStateAtHeight(initHeight))
	assert.Equal(t, blockStateProcessed, sc.getStateAtHeight(initHeight-1))
	assert.Equal(t, blockStateUnknown, sc.getStateAtHeight(initHeight+1))
}

func TestAddPeer(t *testing.T) {
	var (
		initHeight int64  = 5
		peerID     p2p.ID = "1"
		peerIDTwo  p2p.ID = "2"
		sc                = newSchedule(initHeight)
	)

	assert.Nil(t, sc.addPeer(peerID))
	assert.Nil(t, sc.addPeer(peerIDTwo))
	assert.Error(t, sc.addPeer(peerID))
}

func TestTouchPeer(t *testing.T) {
	var (
		initHeight int64  = 5
		peerID     p2p.ID = "1"
		sc                = newSchedule(initHeight)
		now               = time.Now()
	)

	assert.Error(t, sc.touchPeer(peerID, now),
		"Touching an unknown peer should return errPeerNotFound")

	assert.Nil(t, sc.addPeer(peerID),
		"Adding a peer should return no error")
	assert.Nil(t, sc.touchPeer(peerID, now),
		"Touching a peer should return no error")

	threshold := 10 * time.Second
	assert.Empty(t, sc.peersInactiveSince(threshold, now.Add(9*time.Second)),
		"Expected no peers to have been touched over 9 seconds")
	assert.Containsf(t, sc.peersInactiveSince(threshold, now.Add(11*time.Second)), peerID,
		"Expected one %s to have been touched over 10 seconds ago", peerID)
}

func TestPeerHeight(t *testing.T) {
	var (
		initHeight int64  = 5
		peerID     p2p.ID = "1"
		peerHeight int64  = 20
		sc                = newSchedule(initHeight)
	)

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

func TestTransitionPending(t *testing.T) {
	var (
		initHeight int64  = 5
		peerID     p2p.ID = "1"
		peerIDTwo  p2p.ID = "2"
		peerHeight int64  = 20
		sc                = newSchedule(initHeight)
		now               = time.Now()
	)

	assert.NoError(t, sc.addPeer(peerID),
		"Adding a peer should return no error")
	assert.Nil(t, sc.addPeer(peerIDTwo),
		"Adding a peer should return no error")

	assert.Error(t, sc.markPending(peerID, peerHeight, now),
		"Expected scheduling a block from a peer in peerStateNew to fail")

	assert.NoError(t, sc.setPeerHeight(peerID, peerHeight),
		"Expected setPeerHeight to return no error")
	assert.NoError(t, sc.setPeerHeight(peerIDTwo, peerHeight),
		"Expected setPeerHeight to return no error")

	assert.NoError(t, sc.markPending(peerID, peerHeight, now),
		"Expected markingPending new block to succeed")
	assert.Error(t, sc.markPending(peerIDTwo, peerHeight, now),
		"Expected markingPending by a second peer to fail")

	assert.Equal(t, blockStatePending, sc.getStateAtHeight(peerHeight),
		"Expected the block to to be in blockStatePending")

	assert.NoError(t, sc.removePeer(peerID),
		"Expected removePeer to return no error")

	assert.Equal(t, blockStateNew, sc.getStateAtHeight(peerHeight),
		"Expected the block to to be in blockStateNew")

	assert.Error(t, sc.markPending(peerID, peerHeight, now),
		"Expected markingPending removed peer to fail")

	assert.NoError(t, sc.markPending(peerIDTwo, peerHeight, now),
		"Expected markingPending on a ready peer to succeed")

	assert.Equal(t, blockStatePending, sc.getStateAtHeight(peerHeight),
		"Expected the block to to be in blockStatePending")
}

func TestTransitionReceived(t *testing.T) {
	var (
		initHeight int64  = 5
		peerID     p2p.ID = "1"
		peerIDTwo  p2p.ID = "2"
		peerHeight int64  = 20
		blockSize  int64  = 1024
		sc                = newSchedule(initHeight)
		now               = time.Now()
		receivedAt        = now.Add(1 * time.Second)
	)

	assert.NoError(t, sc.addPeer(peerID),
		"Expected adding peer %s to succeed", peerID)
	assert.NoError(t, sc.addPeer(peerIDTwo),
		"Expected adding peer %s to succeed", peerIDTwo)
	assert.NoError(t, sc.setPeerHeight(peerID, peerHeight),
		"Expected setPeerHeight to return no error")
	assert.NoErrorf(t, sc.setPeerHeight(peerIDTwo, peerHeight),
		"Expected setPeerHeight on %s to %d to succeed", peerIDTwo, peerHeight)
	assert.NoError(t, sc.markPending(peerID, initHeight, now),
		"Expected markingPending new block to succeed")

	assert.Error(t, sc.markReceived(peerIDTwo, initHeight, blockSize, receivedAt),
		"Expected marking markReceived from a non requesting peer to fail")

	assert.NoError(t, sc.markReceived(peerID, initHeight, blockSize, receivedAt),
		"Expected marking markReceived on a pending block to succeed")

	assert.Error(t, sc.markReceived(peerID, initHeight, blockSize, receivedAt),
		"Expected marking markReceived on received block to fail")

	assert.Equalf(t, blockStateReceived, sc.getStateAtHeight(initHeight),
		"Expected block %d to be blockHeightReceived", initHeight)

	assert.NoErrorf(t, sc.removePeer(peerID),
		"Expected removePeer removing %s to succeed", peerID)

	assert.Equalf(t, blockStateNew, sc.getStateAtHeight(initHeight),
		"Expected block %d to be blockStateNew", initHeight)

	assert.NoErrorf(t, sc.markPending(peerIDTwo, initHeight, now),
		"Expected markingPending  %d from %s to succeed", initHeight, peerIDTwo)
	assert.NoErrorf(t, sc.markReceived(peerIDTwo, initHeight, blockSize, receivedAt),
		"Expected marking markReceived %d from %s to succeed", initHeight, peerIDTwo)
	assert.Equalf(t, blockStateReceived, sc.getStateAtHeight(initHeight),
		"Expected block %d to be blockStateReceived", initHeight)
}

func TestTransitionProcessed(t *testing.T) {
	var (
		initHeight int64  = 5
		peerID     p2p.ID = "1"
		peerHeight int64  = 20
		blockSize  int64  = 1024
		sc                = newSchedule(initHeight)
		now               = time.Now()
		receivedAt        = now.Add(1 * time.Second)
	)

	assert.NoError(t, sc.addPeer(peerID),
		"Expected adding peer %s to succeed", peerID)
	assert.NoErrorf(t, sc.setPeerHeight(peerID, peerHeight),
		"Expected setPeerHeight on %s to %d to succeed", peerID, peerHeight)
	assert.NoError(t, sc.markPending(peerID, initHeight, now),
		"Expected markingPending new block to succeed")
	assert.NoError(t, sc.markReceived(peerID, initHeight, blockSize, receivedAt),
		"Expected marking markReceived on a pending block to succeed")

	assert.Error(t, sc.markProcessed(initHeight+1),
		"Expected marking %d as processed to fail", initHeight+1)
	assert.NoError(t, sc.markProcessed(initHeight),
		"Expected marking %d as processed to succeed", initHeight)

	assert.Equalf(t, blockStateProcessed, sc.getStateAtHeight(initHeight),
		"Expected block %d to be blockStateProcessed", initHeight)

	assert.NoError(t, sc.removePeer(peerID),
		"Expected removing peer %s to succeed", peerID)

	assert.Equalf(t, blockStateProcessed, sc.getStateAtHeight(initHeight),
		"Expected block %d to be blockStateProcessed", initHeight)
}

func TestMinMaxHeight(t *testing.T) {
	var (
		initHeight int64  = 5
		peerID     p2p.ID = "1"
		peerHeight int64  = 20
		sc                = newSchedule(initHeight)
		now               = time.Now()
	)

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
	var (
		initHeight int64  = 5
		peerID     p2p.ID = "1"
		peerHeight int64  = 20
		blockSize  int64  = 1024
		sc                = newSchedule(initHeight)
		now               = time.Now()
		receivedAt        = now.Add(1 * time.Second)
	)

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
