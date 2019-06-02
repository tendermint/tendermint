package blockchain

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

// check if peer timer is running or not (a running timer can be successfully stopped)
// Note: it does stop the timer!
func checkByStoppingPeerTimer(t *testing.T, peer *bpPeer, running bool) {
	assert.NotPanics(t, func() {
		stopped := peer.StopBlockResponseTimer()
		if running {
			assert.True(t, stopped)
		} else {
			assert.False(t, stopped)
		}
	})
}

func TestPeerResetMonitor(t *testing.T) {
	peer := NewBPPeer(
		p2p.ID(cmn.RandStr(12)), 10,
		func(err error, _ p2p.ID) {},
		nil)
	peer.SetLogger(log.TestingLogger())
	peer.resetMonitor()
	assert.NotNil(t, peer.recvMonitor)
}

func TestPeerTimer(t *testing.T) {
	var (
		numErrFuncCalls int        // number of calls to the errFunc
		lastErr         error      // last generated error
		peerTestMtx     sync.Mutex // modifications of ^^ variables are also done from timer handler goroutine
	)
	params := &bpPeerParams{peerTimeout: 2 * time.Millisecond}

	peer := NewBPPeer(
		p2p.ID(cmn.RandStr(12)), 10,
		func(err error, _ p2p.ID) {
			peerTestMtx.Lock()
			defer peerTestMtx.Unlock()
			lastErr = err
			numErrFuncCalls++
		},
		params)

	peer.SetLogger(log.TestingLogger())
	assert.Nil(t, peer.blockResponseTimer)

	// initial reset call with peer having a nil timer
	peer.resetTimeout()
	assert.NotNil(t, peer.blockResponseTimer)
	// make sure timer is running and stop it
	checkByStoppingPeerTimer(t, peer, true)

	// reset with non nil expired timer
	peer.resetTimeout()
	assert.NotNil(t, peer.blockResponseTimer)
	// make sure timer is running and stop it
	checkByStoppingPeerTimer(t, peer, true)

	// reset with running timer (started above)
	time.Sleep(time.Millisecond)
	peer.resetTimeout()
	assert.NotNil(t, peer.blockResponseTimer)

	// let the timer expire and ...
	time.Sleep(3 * time.Millisecond)
	checkByStoppingPeerTimer(t, peer, false)

	peerTestMtx.Lock()
	// ... check an error has been sent, error is peerNonResponsive
	assert.Equal(t, 1, numErrFuncCalls)
	assert.Equal(t, lastErr, errNoPeerResponse)
	peerTestMtx.Unlock()
}

func TestPeerIncrPending(t *testing.T) {
	params := &bpPeerParams{peerTimeout: 2 * time.Millisecond}

	peer := NewBPPeer(
		p2p.ID(cmn.RandStr(12)), 10,
		func(err error, _ p2p.ID) {},
		params)

	peer.SetLogger(log.TestingLogger())

	peer.IncrPending()
	assert.NotNil(t, peer.recvMonitor)
	assert.NotNil(t, peer.blockResponseTimer)
	assert.Equal(t, int32(1), peer.GetNumPendingBlockRequests())

	peer.IncrPending()
	assert.NotNil(t, peer.recvMonitor)
	assert.NotNil(t, peer.blockResponseTimer)
	assert.Equal(t, int32(2), peer.GetNumPendingBlockRequests())
}

func TestPeerDecrPending(t *testing.T) {
	params := &bpPeerParams{peerTimeout: 2 * time.Millisecond}

	peer := NewBPPeer(
		p2p.ID(cmn.RandStr(12)), 10,
		func(err error, _ p2p.ID) {},
		params)

	peer.SetLogger(log.TestingLogger())

	// panic if numPendingBlockRequests is 0 and try to decrement it
	assert.Panics(t, func() { peer.DecrPending(10) })

	// decrement to zero
	peer.IncrPending()
	peer.DecrPending(10)
	assert.Equal(t, int32(0), peer.GetNumPendingBlockRequests())
	// make sure timer is not running
	checkByStoppingPeerTimer(t, peer, false)

	// decrement to non zero
	peer.IncrPending()
	peer.IncrPending()
	peer.DecrPending(10)
	assert.Equal(t, int32(1), peer.GetNumPendingBlockRequests())
	// make sure timer is running and stop it
	checkByStoppingPeerTimer(t, peer, true)
}

func TestPeerCanBeRemovedDueToExpiration(t *testing.T) {

	params := &bpPeerParams{peerTimeout: 2 * time.Millisecond}
	var (
		numErrFuncCalls int        // number of calls to the errFunc
		lastErr         error      // last generated error
		peerTestMtx     sync.Mutex // modifications of ^^ variables are also done from timer handler goroutine
	)

	peer := NewBPPeer(
		p2p.ID(cmn.RandStr(12)), 10,
		func(err error, _ p2p.ID) {
			peerTestMtx.Lock()
			defer peerTestMtx.Unlock()
			lastErr = err
			numErrFuncCalls++
		},
		params)

	peer.SetLogger(log.TestingLogger())

	peer.IncrPending()
	time.Sleep(3 * time.Millisecond)
	// timer expired, should be able to remove peer
	peerTestMtx.Lock()
	assert.Equal(t, 1, numErrFuncCalls)
	assert.Equal(t, errNoPeerResponse, lastErr)
	peerTestMtx.Unlock()
}

func TestPeerCanBeRemovedDueToLowSpeed(t *testing.T) {
	params := &bpPeerParams{
		peerTimeout: time.Second,
		minRecvRate: int64(100), // 100 bytes/sec exponential moving average
	}
	peer := NewBPPeer(
		p2p.ID(cmn.RandStr(12)), 10,
		func(err error, _ p2p.ID) {},
		params)
	peer.SetLogger(log.TestingLogger())

	peer.IncrPending()
	peer.numPendingBlockRequests = 100

	// monitor starts with a higher rEMA (~ 2*minRecvRate), wait for it to go down
	time.Sleep(900 * time.Millisecond)

	// normal peer - send a bit more than 100 bytes/sec, > 10 bytes/100msec, check peer is not considered slow
	for i := 0; i < 10; i++ {
		peer.DecrPending(11)
		time.Sleep(100 * time.Millisecond)
		require.Nil(t, peer.IsGood())
	}

	// slow peer - send a bit less than 10 bytes/100msec
	for i := 0; i < 10; i++ {
		peer.DecrPending(9)
		time.Sleep(100 * time.Millisecond)
	}
	// check peer is considered slow
	assert.Equal(t, errSlowPeer, peer.IsGood())

}

func TestPeerCleanup(t *testing.T) {
	params := &bpPeerParams{peerTimeout: 2 * time.Millisecond}

	peer := NewBPPeer(
		p2p.ID(cmn.RandStr(12)), 10,
		func(err error, _ p2p.ID) {},
		params)
	peer.SetLogger(log.TestingLogger())

	assert.Nil(t, peer.blockResponseTimer)

	// initial reset call with peer having a nil timer
	peer.resetTimeout()
	assert.NotNil(t, peer.blockResponseTimer)

	peer.Cleanup()
	checkByStoppingPeerTimer(t, peer, false)
}
