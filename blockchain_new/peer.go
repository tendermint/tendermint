package blockchain_new

import (
	"fmt"
	"math"
	"time"

	flow "github.com/tendermint/tendermint/libs/flowrate"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

//--------
// Peer
var (
	peerTimeout = 15 * time.Second // not const so we can override with tests

	// Minimum recv rate to ensure we're receiving blocks from a peer fast
	// enough. If a peer is not sending us data at at least that rate, we
	// consider them to have timedout and we disconnect.
	//
	// Assuming a DSL connection (not a good choice) 128 Kbps (upload) ~ 15 KB/s,
	// sending data across atlantic ~ 7.5 KB/s.
	minRecvRate = int64(7680)

	// Monitor parameters
	peerSampleRate = time.Second
	peerWindowSize = 40 * peerSampleRate
)

type bpPeer struct {
	id          p2p.ID
	recvMonitor *flow.Monitor

	height     int64                  // the peer reported height
	numPending int32                  // number of requests pending assignment or block response
	blocks     map[int64]*types.Block // blocks received or waiting to be received from this peer
	timeout    *time.Timer
	didTimeout bool

	logger  log.Logger
	errFunc func(err error, peerID p2p.ID) // function to call on error
}

func newBPPeer(
	peerID p2p.ID, height int64, errFunc func(err error, peerID p2p.ID)) *bpPeer {
	peer := &bpPeer{
		id:         peerID,
		height:     height,
		blocks:     make(map[int64]*types.Block, maxRequestsPerPeer),
		numPending: 0,
		logger:     log.NewNopLogger(),
		errFunc:    errFunc,
	}
	return peer
}

func (peer *bpPeer) setLogger(l log.Logger) {
	peer.logger = l
}

func (peer *bpPeer) resetMonitor() {
	peer.recvMonitor = flow.New(peerSampleRate, peerWindowSize)
	initialValue := float64(minRecvRate) * math.E
	peer.recvMonitor.SetREMA(initialValue)
}

func (peer *bpPeer) resetTimeout() {
	if peer.timeout == nil {
		peer.timeout = time.AfterFunc(peerTimeout, peer.onTimeout)
	} else {
		peer.timeout.Reset(peerTimeout)
	}
}

func (peer *bpPeer) incrPending() {
	if peer.numPending == 0 {
		peer.resetMonitor()
		peer.resetTimeout()
	}
	peer.numPending++
}

func (peer *bpPeer) decrPending(recvSize int) {
	if peer.numPending == 0 {
		panic("cannot decrement, peer does not have pending requests")
	}

	peer.numPending--
	if peer.numPending == 0 {
		peer.timeout.Stop()
	} else {
		peer.recvMonitor.Update(recvSize)
		peer.resetTimeout()
	}
}

func (peer *bpPeer) onTimeout() {
	peer.errFunc(errNoPeerResponse, peer.id)
	peer.logger.Error("SendTimeout", "reason", errNoPeerResponse, "timeout", peerTimeout)
	peer.didTimeout = true
}

func (peer *bpPeer) isGood() error {
	if peer.didTimeout {
		return errNoPeerResponse
	}

	if !peer.didTimeout && peer.numPending > 0 {
		curRate := peer.recvMonitor.Status().CurRate
		// curRate can be 0 on start
		if curRate != 0 && curRate < minRecvRate {
			err := errSlowPeer
			peer.logger.Error("SendTimeout", "peer", peer.id,
				"reason", err,
				"curRate", fmt.Sprintf("%d KB/s", curRate/1024),
				"minRate", fmt.Sprintf("%d KB/s", minRecvRate/1024))
			// consider the peer timedout
			peer.didTimeout = true
			return errSlowPeer
		}
	}

	return nil
}

func (peer *bpPeer) cleanup() {
	if peer.timeout != nil {
		peer.timeout.Stop()
	}
}

func (peer *bpPeer) String() string {
	return fmt.Sprintf("peer: %v height: %v pending: %v", peer.id, peer.height, peer.numPending)
}
