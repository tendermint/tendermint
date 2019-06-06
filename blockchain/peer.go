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
	timeout     time.Duration
	minRecvRate int64
	sampleRate  time.Duration
	windowSize  time.Duration
}

type bpPeer struct {
	logger log.Logger
	ID     p2p.ID

	Height                  int64                  // the peer reported height
	NumPendingBlockRequests int                    // number of requests still waiting for block responses
	blocks                  map[int64]*types.Block // blocks received or expected to be received from this peer
	blockResponseTimer      *time.Timer
	recvMonitor             *flow.Monitor
	params                  *bpPeerParams // parameters for timer and monitor

	onErr func(err error, peerID p2p.ID) // function to call on error

}

// NewBPPeer creates a new peer.
func NewBPPeer(
	peerID p2p.ID, height int64, onErr func(err error, peerID p2p.ID), params *bpPeerParams) *bpPeer {

	if params == nil {
		params = bpPeerDefaultParams()
	}
	return &bpPeer{
		ID:     peerID,
		Height: height,
		blocks: make(map[int64]*types.Block, maxRequestsPerPeer),
		logger: log.NewNopLogger(),
		onErr:  onErr,
		params: params,
	}
}

// String returns a string representation of a peer.
func (peer *bpPeer) String() string {
	return fmt.Sprintf("peer: %v height: %v pending: %v", peer.ID, peer.Height, peer.NumPendingBlockRequests)
}

// SetLogger sets the logger of the peer.
func (peer *bpPeer) SetLogger(l log.Logger) {
	peer.logger = l
}

// Cleanup performs cleanup of the peer, removes blocks, requests, stops timers and monitors.
func (peer *bpPeer) Cleanup() {
	if peer.blockResponseTimer != nil {
		peer.blockResponseTimer.Stop()
	}
	if peer.NumPendingBlockRequests != 0 {
		peer.logger.Info("peer with pending requests is being cleaned")
	}
	if len(peer.blocks)-peer.NumPendingBlockRequests != 0 {
		peer.logger.Info("peer with pending blocks is being cleaned")
	}
	for h := range peer.blocks {
		delete(peer.blocks, h)
	}
	peer.NumPendingBlockRequests = 0
	peer.recvMonitor = nil
}

// BlockAtHeight returns the block at a given height if available and a non-nil error otherwise.
func (peer *bpPeer) BlockAtHeight(height int64) (*types.Block, error) {
	block, ok := peer.blocks[height]
	if !ok {
		return nil, errMissingRequest
	}
	if block == nil {
		return nil, errMissingBlock
	}
	return peer.blocks[height], nil
}

// AddBlock adds a block at peer level. Block must be non-nil and recvSize a positive integer
// The peer must have a pending request for this block.
func (peer *bpPeer) AddBlock(block *types.Block, recvSize int) error {
	if block == nil || recvSize < 0 {
		panic("bad parameters")
	}
	existingBlock, ok := peer.blocks[block.Height]
	if !ok {
		peer.logger.Error("unsolicited block", "blockHeight", block.Height, "peer", peer.ID)
		return errMissingRequest
	}
	if existingBlock != nil {
		peer.logger.Error("already have a block for height", "height", block.Height)
		return errDuplicateBlock
	}
	if peer.NumPendingBlockRequests == 0 {
		panic("peer does not have pending requests")
	}
	peer.blocks[block.Height] = block
	peer.NumPendingBlockRequests--
	if peer.NumPendingBlockRequests == 0 {
		peer.stopMonitor()
		peer.stopBlockResponseTimer()
	} else {
		peer.recvMonitor.Update(recvSize)
		peer.resetBlockResponseTimer()
	}
	return nil
}

// RemoveBlock removes the block of given height
func (peer *bpPeer) RemoveBlock(height int64) {
	delete(peer.blocks, height)
}

// RequestSent records that a request was sent, and starts the peer timer and monitor if needed.
func (peer *bpPeer) RequestSent(height int64) {
	peer.blocks[height] = nil

	if peer.NumPendingBlockRequests == 0 {
		peer.startMonitor()
		peer.resetBlockResponseTimer()
	}
	peer.NumPendingBlockRequests++
}

// CheckRate verifies that the response rate of the peer is acceptable (higher than the minimum allowed).
func (peer *bpPeer) CheckRate() error {
	if peer.NumPendingBlockRequests == 0 {
		return nil
	}
	curRate := peer.recvMonitor.Status().CurRate
	// curRate can be 0 on start
	if curRate != 0 && curRate < peer.params.minRecvRate {
		err := errSlowPeer
		peer.logger.Error("SendTimeout", "peer", peer,
			"reason", err,
			"curRate", fmt.Sprintf("%d KB/s", curRate/1024),
			"minRate", fmt.Sprintf("%d KB/s", peer.params.minRecvRate/1024))
		return err
	}
	return nil
}

func (peer *bpPeer) onTimeout() {
	peer.onErr(errNoPeerResponse, peer.ID)
}

func (peer *bpPeer) stopMonitor() {
	peer.recvMonitor.Done()
	peer.recvMonitor = nil
}

func (peer *bpPeer) startMonitor() {
	peer.recvMonitor = flow.New(peer.params.sampleRate, peer.params.windowSize)
	initialValue := float64(peer.params.minRecvRate) * math.E
	peer.recvMonitor.SetREMA(initialValue)
}

func (peer *bpPeer) resetBlockResponseTimer() {
	if peer.blockResponseTimer == nil {
		peer.blockResponseTimer = time.AfterFunc(peer.params.timeout, peer.onTimeout)
	} else {
		peer.blockResponseTimer.Reset(peer.params.timeout)
	}
}

func (peer *bpPeer) stopBlockResponseTimer() bool {
	if peer.blockResponseTimer == nil {
		return false
	}
	return peer.blockResponseTimer.Stop()
}

func bpPeerDefaultParams() *bpPeerParams {
	return &bpPeerParams{
		// Timeout for a peer to respond to a block request.
		timeout: 15 * time.Second,

		// Minimum recv rate to ensure we're receiving blocks from a peer fast
		// enough. If a peer is not sending data at at least that rate, we
		// consider them to have timedout and we disconnect.
		//
		// Assuming a DSL connection (not a good choice) 128 Kbps (upload) ~ 15 KB/s,
		// sending data across atlantic ~ 7.5 KB/s.
		minRecvRate: int64(7680),

		// Monitor parameters
		sampleRate: time.Second,
		windowSize: 40 * time.Second,
	}
}
