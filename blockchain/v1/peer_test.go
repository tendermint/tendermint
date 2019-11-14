package v1

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

func TestPeerMonitor(t *testing.T) {
	peer := NewBpPeer(
		p2p.ID(cmn.RandStr(12)), 10,
		func(err error, _ p2p.ID) {},
		nil)
	peer.SetLogger(log.TestingLogger())
	peer.startMonitor()
	assert.NotNil(t, peer.recvMonitor)
	peer.stopMonitor()
	assert.Nil(t, peer.recvMonitor)
}

func TestPeerResetBlockResponseTimer(t *testing.T) {
	var (
		numErrFuncCalls int        // number of calls to the errFunc
		lastErr         error      // last generated error
		peerTestMtx     sync.Mutex // modifications of ^^ variables are also done from timer handler goroutine
	)
	params := &BpPeerParams{timeout: 2 * time.Millisecond}

	peer := NewBpPeer(
		p2p.ID(cmn.RandStr(12)), 10,
		func(err error, _ p2p.ID) {
			peerTestMtx.Lock()
			defer peerTestMtx.Unlock()
			lastErr = err
			numErrFuncCalls++
		},
		params)

	peer.SetLogger(log.TestingLogger())
	checkByStoppingPeerTimer(t, peer, false)

	// initial reset call with peer having a nil timer
	peer.resetBlockResponseTimer()
	assert.NotNil(t, peer.blockResponseTimer)
	// make sure timer is running and stop it
	checkByStoppingPeerTimer(t, peer, true)

	// reset with running timer
	peer.resetBlockResponseTimer()
	time.Sleep(time.Millisecond)
	peer.resetBlockResponseTimer()
	assert.NotNil(t, peer.blockResponseTimer)

	// let the timer expire and ...
	time.Sleep(3 * time.Millisecond)
	// ... check timer is not running
	checkByStoppingPeerTimer(t, peer, false)

	peerTestMtx.Lock()
	// ... check errNoPeerResponse has been sent
	assert.Equal(t, 1, numErrFuncCalls)
	assert.Equal(t, lastErr, errNoPeerResponse)
	peerTestMtx.Unlock()
}

func TestPeerRequestSent(t *testing.T) {
	params := &BpPeerParams{timeout: 2 * time.Millisecond}

	peer := NewBpPeer(
		p2p.ID(cmn.RandStr(12)), 10,
		func(err error, _ p2p.ID) {},
		params)

	peer.SetLogger(log.TestingLogger())

	peer.RequestSent(1)
	assert.NotNil(t, peer.recvMonitor)
	assert.NotNil(t, peer.blockResponseTimer)
	assert.Equal(t, 1, peer.NumPendingBlockRequests)

	peer.RequestSent(1)
	assert.NotNil(t, peer.recvMonitor)
	assert.NotNil(t, peer.blockResponseTimer)
	assert.Equal(t, 2, peer.NumPendingBlockRequests)
}

func TestPeerGetAndRemoveBlock(t *testing.T) {
	peer := NewBpPeer(
		p2p.ID(cmn.RandStr(12)), 100,
		func(err error, _ p2p.ID) {},
		nil)

	// Change peer height
	peer.Height = int64(10)
	assert.Equal(t, int64(10), peer.Height)

	// request some blocks and receive few of them
	for i := 1; i <= 10; i++ {
		peer.RequestSent(int64(i))
		if i > 5 {
			// only receive blocks 1..5
			continue
		}
		_ = peer.AddBlock(makeSmallBlock(i), 10)
	}

	tests := []struct {
		name         string
		height       int64
		wantErr      error
		blockPresent bool
	}{
		{"no request", 100, errMissingBlock, false},
		{"no block", 6, errMissingBlock, false},
		{"block 1 present", 1, nil, true},
		{"block max present", 5, nil, true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// try to get the block
			b, err := peer.BlockAtHeight(tt.height)
			assert.Equal(t, tt.wantErr, err)
			assert.Equal(t, tt.blockPresent, b != nil)

			// remove the block
			peer.RemoveBlock(tt.height)
			_, err = peer.BlockAtHeight(tt.height)
			assert.Equal(t, errMissingBlock, err)
		})
	}
}

func TestPeerAddBlock(t *testing.T) {
	peer := NewBpPeer(
		p2p.ID(cmn.RandStr(12)), 100,
		func(err error, _ p2p.ID) {},
		nil)

	// request some blocks, receive one
	for i := 1; i <= 10; i++ {
		peer.RequestSent(int64(i))
		if i == 5 {
			// receive block 5
			_ = peer.AddBlock(makeSmallBlock(i), 10)
		}
	}

	tests := []struct {
		name         string
		height       int64
		wantErr      error
		blockPresent bool
	}{
		{"no request", 50, errMissingBlock, false},
		{"duplicate block", 5, errDuplicateBlock, true},
		{"block 1 successfully received", 1, nil, true},
		{"block max successfully received", 10, nil, true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// try to get the block
			err := peer.AddBlock(makeSmallBlock(int(tt.height)), 10)
			assert.Equal(t, tt.wantErr, err)
			_, err = peer.BlockAtHeight(tt.height)
			assert.Equal(t, tt.blockPresent, err == nil)
		})
	}
}

func TestPeerOnErrFuncCalledDueToExpiration(t *testing.T) {

	params := &BpPeerParams{timeout: 2 * time.Millisecond}
	var (
		numErrFuncCalls int        // number of calls to the onErr function
		lastErr         error      // last generated error
		peerTestMtx     sync.Mutex // modifications of ^^ variables are also done from timer handler goroutine
	)

	peer := NewBpPeer(
		p2p.ID(cmn.RandStr(12)), 10,
		func(err error, _ p2p.ID) {
			peerTestMtx.Lock()
			defer peerTestMtx.Unlock()
			lastErr = err
			numErrFuncCalls++
		},
		params)

	peer.SetLogger(log.TestingLogger())

	peer.RequestSent(1)
	time.Sleep(4 * time.Millisecond)
	// timer should have expired by now, check that the on error function was called
	peerTestMtx.Lock()
	assert.Equal(t, 1, numErrFuncCalls)
	assert.Equal(t, errNoPeerResponse, lastErr)
	peerTestMtx.Unlock()
}

func TestPeerCheckRate(t *testing.T) {
	params := &BpPeerParams{
		timeout:     time.Second,
		minRecvRate: int64(100), // 100 bytes/sec exponential moving average
	}
	peer := NewBpPeer(
		p2p.ID(cmn.RandStr(12)), 10,
		func(err error, _ p2p.ID) {},
		params)
	peer.SetLogger(log.TestingLogger())

	require.Nil(t, peer.CheckRate())

	for i := 0; i < 40; i++ {
		peer.RequestSent(int64(i))
	}

	// monitor starts with a higher rEMA (~ 2*minRecvRate), wait for it to go down
	time.Sleep(900 * time.Millisecond)

	// normal peer - send a bit more than 100 bytes/sec, > 10 bytes/100msec, check peer is not considered slow
	for i := 0; i < 10; i++ {
		_ = peer.AddBlock(makeSmallBlock(i), 11)
		time.Sleep(100 * time.Millisecond)
		require.Nil(t, peer.CheckRate())
	}

	// slow peer - send a bit less than 10 bytes/100msec
	for i := 10; i < 20; i++ {
		_ = peer.AddBlock(makeSmallBlock(i), 9)
		time.Sleep(100 * time.Millisecond)
	}
	// check peer is considered slow
	assert.Equal(t, errSlowPeer, peer.CheckRate())
}

func TestPeerCleanup(t *testing.T) {
	params := &BpPeerParams{timeout: 2 * time.Millisecond}

	peer := NewBpPeer(
		p2p.ID(cmn.RandStr(12)), 10,
		func(err error, _ p2p.ID) {},
		params)
	peer.SetLogger(log.TestingLogger())

	assert.Nil(t, peer.blockResponseTimer)
	peer.RequestSent(1)
	assert.NotNil(t, peer.blockResponseTimer)

	peer.Cleanup()
	checkByStoppingPeerTimer(t, peer, false)
}

// Check if peer timer is running or not (a running timer can be successfully stopped).
// Note: stops the timer.
func checkByStoppingPeerTimer(t *testing.T, peer *BpPeer, running bool) {
	assert.NotPanics(t, func() {
		stopped := peer.stopBlockResponseTimer()
		if running {
			assert.True(t, stopped)
		} else {
			assert.False(t, stopped)
		}
	})
}

func makeSmallBlock(height int) *types.Block {
	return types.MakeBlock(int64(height), []types.Tx{types.Tx("foo")}, nil, nil)
}
