package blockchain

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
type bpPeerParams struct {
	peerTimeout    time.Duration
	minRecvRate    int64
	peerSampleRate time.Duration
	peerWindowSize time.Duration
}

type bpPeer struct {
	logger log.Logger
	id     p2p.ID

	height                  int64                  // the peer reported height
	numPendingBlockRequests int32                  // number of requests still waiting for block responses
	blocks                  map[int64]*types.Block // blocks received or expected to be received from this peer
	blockResponseTimer      *time.Timer
	recvMonitor             *flow.Monitor
	parameters              *bpPeerParams // parameters for timer and monitor

	errFunc func(onErr error, peerID p2p.ID) // function to call on error

}

func NewBPPeer(
	peerID p2p.ID, height int64, errFunc func(err error, peerID p2p.ID), parameters *bpPeerParams) *bpPeer {

	if parameters == nil {
		parameters = bpPeerDefaultParams()
	}
	return &bpPeer{
		id:         peerID,
		height:     height,
		blocks:     make(map[int64]*types.Block, maxRequestsPerPeer),
		logger:     log.NewNopLogger(),
		errFunc:    errFunc,
		parameters: parameters,
	}
}

func bpPeerDefaultParams() *bpPeerParams {
	return &bpPeerParams{
		// Timeout for a peer to respond to a block request.
		peerTimeout: 15 * time.Second,

		// Minimum recv rate to ensure we're receiving blocks from a peer fast
		// enough. If a peer is not sending data at at least that rate, we
		// consider them to have timedout and we disconnect.
		//
		// Assuming a DSL connection (not a good choice) 128 Kbps (upload) ~ 15 KB/s,
		// sending data across atlantic ~ 7.5 KB/s.
		minRecvRate: int64(7680),

		// Monitor parameters
		peerSampleRate: time.Second,
		peerWindowSize: 40 * time.Second,
	}
}

func (peer *bpPeer) ID() p2p.ID {
	return peer.id
}

func (peer *bpPeer) SetLogger(l log.Logger) {
	peer.logger = l
}

func (peer *bpPeer) GetHeight() int64 {
	return peer.height
}

func (peer *bpPeer) SetHeight(height int64) {
	peer.height = height
}

func (peer *bpPeer) GetNumPendingBlockRequests() int32 {
	return peer.numPendingBlockRequests
}

func (peer *bpPeer) GetBlockAtHeight(height int64) (*types.Block, error) {
	block, ok := peer.blocks[height]
	if !ok {
		return nil, errMissingRequest
	}
	if block == nil {
		return nil, errMissingBlock
	}
	return peer.blocks[height], nil
}

func (peer *bpPeer) SetBlockAtHeight(height int64, block *types.Block) {
	peer.blocks[height] = block
}

func (peer *bpPeer) RemoveBlockAtHeight(height int64) {
	if _, ok := peer.blocks[height]; !ok {
		return
	}
	delete(peer.blocks, height)
}

func (peer *bpPeer) resetMonitor() {
	peer.recvMonitor = flow.New(peer.parameters.peerSampleRate, peer.parameters.peerWindowSize)
	initialValue := float64(peer.parameters.minRecvRate) * math.E
	peer.recvMonitor.SetREMA(initialValue)
}

func (peer *bpPeer) resetTimeout() {
	if peer.blockResponseTimer == nil {
		peer.blockResponseTimer = time.AfterFunc(peer.parameters.peerTimeout, peer.onTimeout)
	} else {
		peer.blockResponseTimer.Reset(peer.parameters.peerTimeout)
	}
}

func (peer *bpPeer) IncrPending() {
	if peer.numPendingBlockRequests == 0 {
		peer.resetMonitor()
		peer.resetTimeout()
	}
	peer.numPendingBlockRequests++
}

func (peer *bpPeer) StopBlockResponseTimer() bool {
	if peer.blockResponseTimer == nil {
		return false
	}
	return peer.blockResponseTimer.Stop()
}

func (peer *bpPeer) DecrPending(recvSize int) {
	if peer.GetNumPendingBlockRequests() == 0 {
		panic("cannot decrement, peer does not have pending requests")
	}

	peer.numPendingBlockRequests--
	if peer.GetNumPendingBlockRequests() == 0 {
		_ = peer.StopBlockResponseTimer()
	} else {
		peer.recvMonitor.Update(recvSize)
		peer.resetTimeout()
	}
}

func (peer *bpPeer) onTimeout() {
	peer.errFunc(errNoPeerResponse, peer.id)
}

func (peer *bpPeer) IsGood() error {
	if peer.numPendingBlockRequests == 0 {
		return nil
	}
	curRate := peer.recvMonitor.Status().CurRate
	// curRate can be 0 on start
	if curRate != 0 && curRate < peer.parameters.minRecvRate {
		err := errSlowPeer
		peer.logger.Error("SendTimeout", "peer", peer,
			"reason", err,
			"curRate", fmt.Sprintf("%d KB/s", curRate/1024),
			"minRate", fmt.Sprintf("%d KB/s", peer.parameters.minRecvRate/1024))
		return err
	}
	return nil
}

func (peer *bpPeer) Cleanup() {
	_ = peer.StopBlockResponseTimer()
}

func (peer *bpPeer) String() string {
	return fmt.Sprintf("peer: %v height: %v pending: %v", peer.id, peer.height, peer.numPendingBlockRequests)
}
