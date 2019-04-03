package blockchain_new

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
)

var (
	numErrFuncCalls int
	lastErr         error
)

func resetErrors() {
	numErrFuncCalls = 0
	lastErr = nil
}

func errFunc(err error, peerID p2p.ID) {
	_ = peerID
	lastErr = err
	numErrFuncCalls++
}

// check if peer timer is running or not (a running timer can be successfully stopped)
// Note: it does stop the timer!
func checkByStoppingPeerTimer(t *testing.T, peer *bpPeer, running bool) {
	assert.NotPanics(t, func() {
		stopped := peer.timeout.Stop()
		if running {
			assert.True(t, stopped)
		} else {
			assert.False(t, stopped)
		}
	})
}

func TestPeerResetMonitor(t *testing.T) {
	peer := &bpPeer{
		id:      p2p.ID(cmn.RandStr(12)),
		height:  10,
		logger:  log.TestingLogger(),
		errFunc: errFunc,
	}
	peer.resetMonitor()
	assert.NotNil(t, peer.recvMonitor)
}

func TestPeerTimer(t *testing.T) {
	peerTimeout = 2 * time.Millisecond

	peer := &bpPeer{
		id:      p2p.ID(cmn.RandStr(12)),
		height:  10,
		logger:  log.TestingLogger(),
		errFunc: errFunc,
	}
	assert.Nil(t, peer.timeout)

	// initial reset call with peer having a nil timer
	peer.resetTimeout()
	assert.NotNil(t, peer.timeout)
	// make sure timer is running and stop it
	checkByStoppingPeerTimer(t, peer, true)

	// reset with non nil expired timer
	peer.resetTimeout()
	assert.NotNil(t, peer.timeout)
	// make sure timer is running and stop it
	checkByStoppingPeerTimer(t, peer, true)
	resetErrors()

	// reset with running timer (started above)
	time.Sleep(time.Millisecond)
	peer.resetTimeout()
	assert.NotNil(t, peer.timeout)

	// let the timer expire and ...
	time.Sleep(3 * time.Millisecond)
	checkByStoppingPeerTimer(t, peer, false)

	// ... check an error has been sent, error is peerNonResponsive
	assert.Equal(t, 1, numErrFuncCalls)
	assert.Equal(t, lastErr, errNoPeerResponse)
	assert.True(t, peer.didTimeout)
}

func TestIncrPending(t *testing.T) {
	peerTimeout = 2 * time.Millisecond

	peer := &bpPeer{
		id:      p2p.ID(cmn.RandStr(12)),
		height:  10,
		logger:  log.TestingLogger(),
		errFunc: errFunc,
	}

	peer.incrPending()
	assert.NotNil(t, peer.recvMonitor)
	assert.NotNil(t, peer.timeout)
	assert.Equal(t, int32(1), peer.numPending)

	peer.incrPending()
	assert.NotNil(t, peer.recvMonitor)
	assert.NotNil(t, peer.timeout)
	assert.Equal(t, int32(2), peer.numPending)
}

func TestDecrPending(t *testing.T) {
	peerTimeout = 2 * time.Millisecond

	peer := &bpPeer{
		id:      p2p.ID(cmn.RandStr(12)),
		height:  10,
		logger:  log.TestingLogger(),
		errFunc: errFunc,
	}

	// panic if numPending is 0 and try to decrement it
	assert.Panics(t, func() { peer.decrPending(10) })

	// decrement to zero
	peer.incrPending()
	peer.decrPending(10)
	assert.Equal(t, int32(0), peer.numPending)
	// make sure timer is not running
	checkByStoppingPeerTimer(t, peer, false)

	// decrement to non zero
	peer.incrPending()
	peer.incrPending()
	peer.decrPending(10)
	assert.Equal(t, int32(1), peer.numPending)
	// make sure timer is running and stop it
	checkByStoppingPeerTimer(t, peer, true)
}

func TestCanBeRemovedDueToExpiration(t *testing.T) {
	minRecvRate = int64(100) // 100 bytes/sec exponential moving average

	peer := &bpPeer{
		id:      p2p.ID(cmn.RandStr(12)),
		height:  10,
		errFunc: errFunc,
		logger:  log.TestingLogger(),
	}

	peerTimeout = time.Millisecond
	peer.incrPending()
	time.Sleep(2 * time.Millisecond)
	// timer expired, should be able to remove peer
	assert.Equal(t, errNoPeerResponse, peer.isGood())
}

func TestCanBeRemovedDueToLowSpeed(t *testing.T) {
	minRecvRate = int64(100) // 100 bytes/sec exponential moving average

	peer := &bpPeer{
		id:      p2p.ID(cmn.RandStr(12)),
		height:  10,
		errFunc: errFunc,
		logger:  log.TestingLogger(),
	}

	peerTimeout = time.Second
	peerSampleRate = 0
	peerWindowSize = 0

	peer.incrPending()
	peer.numPending = 100

	// monitor starts with a higher rEMA (~ 2*minRecvRate), wait for it to go down
	time.Sleep(900 * time.Millisecond)

	// normal peer - send a bit more than 100 byes/sec, > 10 byes/100msec, check peer is not considered slow
	for i := 0; i < 10; i++ {
		peer.decrPending(11)
		time.Sleep(100 * time.Millisecond)
		require.Nil(t, peer.isGood())
	}

	// slow peer - send a bit less than 10 byes/100msec
	for i := 0; i < 10; i++ {
		peer.decrPending(9)
		time.Sleep(100 * time.Millisecond)
	}
	// check peer is considered slow
	assert.Equal(t, errSlowPeer, peer.isGood())

}

func TestCleanupPeer(t *testing.T) {
	var mtx sync.Mutex
	peer := &bpPeer{
		id:      p2p.ID(cmn.RandStr(12)),
		height:  10,
		logger:  log.TestingLogger(),
		errFunc: errFunc,
	}
	peerTimeout = 2 * time.Millisecond
	assert.Nil(t, peer.timeout)

	// initial reset call with peer having a nil timer
	peer.resetTimeout()
	assert.NotNil(t, peer.timeout)

	mtx.Lock()
	peer.cleanup()
	mtx.Unlock()

	checkByStoppingPeerTimer(t, peer, false)

}
