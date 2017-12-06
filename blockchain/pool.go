package blockchain

import (
	"math"
	"sync"
	"time"

	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tmlibs/common"
	flow "github.com/tendermint/tmlibs/flowrate"
	"github.com/tendermint/tmlibs/log"
)

/*

eg, L = latency = 0.1s
	P = num peers = 10
	FN = num full nodes
	BS = 1kB block size
	CB = 1 Mbit/s = 128 kB/s
	CB/P = 12.8 kB
	B/S = CB/P/BS = 12.8 blocks/s

	12.8 * 0.1 = 1.28 blocks on conn

*/

const (
	requestIntervalMS         = 100
	maxTotalRequesters        = 1000
	maxPendingRequests        = maxTotalRequesters
	maxPendingRequestsPerPeer = 50
	minRecvRate               = 10240 // 10Kb/s
)

var peerTimeoutSeconds = time.Duration(15) // not const so we can override with tests

/*
	Peers self report their heights when we join the block pool.
	Starting from our latest pool.height, we request blocks
	in sequence from peers that reported higher heights than ours.
	Every so often we ask peers what height they're on so we can keep going.

	Requests are continuously made for blocks of higher heights until
	the limit is reached. If most of the requests have no available peers, and we
	are not at peer limits, we can probably switch to consensus reactor
*/

type BlockPool struct {
	cmn.BaseService
	startTime time.Time

	mtx sync.Mutex
	// block requests
	requesters map[int64]*bpRequester
	height     int64 // the lowest key in requesters.
	numPending int32 // number of requests pending assignment or block response
	// peers
	peers         map[string]*bpPeer
	maxPeerHeight int64

	requestsCh chan<- BlockRequest
	timeoutsCh chan<- string
}

func NewBlockPool(start int64, requestsCh chan<- BlockRequest, timeoutsCh chan<- string) *BlockPool {
	bp := &BlockPool{
		peers: make(map[string]*bpPeer),

		requesters: make(map[int64]*bpRequester),
		height:     start,
		numPending: 0,

		requestsCh: requestsCh,
		timeoutsCh: timeoutsCh,
	}
	bp.BaseService = *cmn.NewBaseService(nil, "BlockPool", bp)
	return bp
}

func (pool *BlockPool) OnStart() error {
	go pool.makeRequestersRoutine()
	pool.startTime = time.Now()
	return nil
}

func (pool *BlockPool) OnStop() {}

// Run spawns requesters as needed.
func (pool *BlockPool) makeRequestersRoutine() {

	for {
		if !pool.IsRunning() {
			break
		}

		_, numPending, lenRequesters := pool.GetStatus()
		if numPending >= maxPendingRequests {
			// sleep for a bit.
			time.Sleep(requestIntervalMS * time.Millisecond)
			// check for timed out peers
			pool.removeTimedoutPeers()
		} else if lenRequesters >= maxTotalRequesters {
			// sleep for a bit.
			time.Sleep(requestIntervalMS * time.Millisecond)
			// check for timed out peers
			pool.removeTimedoutPeers()
		} else {
			// request for more blocks.
			pool.makeNextRequester()
		}
	}
}

func (pool *BlockPool) removeTimedoutPeers() {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()

	for _, peer := range pool.peers {
		if !peer.didTimeout && peer.numPending > 0 {
			curRate := peer.recvMonitor.Status().CurRate
			// XXX remove curRate != 0
			if curRate != 0 && curRate < minRecvRate {
				pool.sendTimeout(peer.id)
				pool.Logger.Error("SendTimeout", "peer", peer.id, "reason", "curRate too low")
				peer.didTimeout = true
			}
		}
		if peer.didTimeout {
			pool.removePeer(peer.id)
		}
	}
}

func (pool *BlockPool) GetStatus() (height int64, numPending int32, lenRequesters int) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()

	return pool.height, pool.numPending, len(pool.requesters)
}

// TODO: relax conditions, prevent abuse.
func (pool *BlockPool) IsCaughtUp() bool {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()

	// Need at least 1 peer to be considered caught up.
	if len(pool.peers) == 0 {
		pool.Logger.Debug("Blockpool has no peers")
		return false
	}

	// some conditions to determine if we're caught up
	receivedBlockOrTimedOut := (pool.height > 0 || time.Since(pool.startTime) > 5*time.Second)
	ourChainIsLongestAmongPeers := pool.maxPeerHeight == 0 || pool.height >= pool.maxPeerHeight
	isCaughtUp := receivedBlockOrTimedOut && ourChainIsLongestAmongPeers
	return isCaughtUp
}

// We need to see the second block's Commit to validate the first block.
// So we peek two blocks at a time.
// The caller will verify the commit.
func (pool *BlockPool) PeekTwoBlocks() (first *types.Block, second *types.Block) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()

	if r := pool.requesters[pool.height]; r != nil {
		first = r.getBlock()
	}
	if r := pool.requesters[pool.height+1]; r != nil {
		second = r.getBlock()
	}
	return
}

// Pop the first block at pool.height
// It must have been validated by 'second'.Commit from PeekTwoBlocks().
func (pool *BlockPool) PopRequest() {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()

	if r := pool.requesters[pool.height]; r != nil {
		/*  The block can disappear at any time, due to removePeer().
		if r := pool.requesters[pool.height]; r == nil || r.block == nil {
			PanicSanity("PopRequest() requires a valid block")
		}
		*/
		r.Stop()
		delete(pool.requesters, pool.height)
		pool.height++
	} else {
		cmn.PanicSanity(cmn.Fmt("Expected requester to pop, got nothing at height %v", pool.height))
	}
}

// Invalidates the block at pool.height,
// Remove the peer and redo request from others.
func (pool *BlockPool) RedoRequest(height int64) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()

	request := pool.requesters[height]

	if request.block == nil {
		cmn.PanicSanity("Expected block to be non-nil")
	}
	// RemovePeer will redo all requesters associated with this peer.
	// TODO: record this malfeasance
	pool.removePeer(request.peerID)
}

// TODO: ensure that blocks come in order for each peer.
func (pool *BlockPool) AddBlock(peerID string, block *types.Block, blockSize int) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()

	requester := pool.requesters[block.Height]
	if requester == nil {
		// a block we didn't expect.
		// TODO:if height is too far ahead, punish peer
		return
	}

	if requester.setBlock(block, peerID) {
		pool.numPending--
		peer := pool.peers[peerID]
		if peer != nil {
			peer.decrPending(blockSize)
		}
	} else {
		// Bad peer?
	}
}

// MaxPeerHeight returns the highest height reported by a peer.
func (pool *BlockPool) MaxPeerHeight() int64 {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()
	return pool.maxPeerHeight
}

// Sets the peer's alleged blockchain height.
func (pool *BlockPool) SetPeerHeight(peerID string, height int64) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()

	peer := pool.peers[peerID]
	if peer != nil {
		peer.height = height
	} else {
		peer = newBPPeer(pool, peerID, height)
		peer.setLogger(pool.Logger.With("peer", peerID))
		pool.peers[peerID] = peer
	}

	if height > pool.maxPeerHeight {
		pool.maxPeerHeight = height
	}
}

func (pool *BlockPool) RemovePeer(peerID string) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()

	pool.removePeer(peerID)
}

func (pool *BlockPool) removePeer(peerID string) {
	for _, requester := range pool.requesters {
		if requester.getPeerID() == peerID {
			if requester.getBlock() != nil {
				pool.numPending++
			}
			go requester.redo() // pick another peer and ...
		}
	}
	delete(pool.peers, peerID)
}

// Pick an available peer with at least the given minHeight.
// If no peers are available, returns nil.
func (pool *BlockPool) pickIncrAvailablePeer(minHeight int64) *bpPeer {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()

	for _, peer := range pool.peers {
		if peer.didTimeout {
			pool.removePeer(peer.id)
			continue
		}
		if peer.numPending >= maxPendingRequestsPerPeer {
			continue
		}
		if peer.height < minHeight {
			continue
		}
		peer.incrPending()
		return peer
	}
	return nil
}

func (pool *BlockPool) makeNextRequester() {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()

	nextHeight := pool.height + pool.requestersLen()
	request := newBPRequester(pool, nextHeight)
	// request.SetLogger(pool.Logger.With("height", nextHeight))

	pool.requesters[nextHeight] = request
	pool.numPending++

	err := request.Start()
	if err != nil {
		request.Logger.Error("Error starting request", "err", err)
	}
}

func (pool *BlockPool) requestersLen() int64 {
	return int64(len(pool.requesters))
}

func (pool *BlockPool) sendRequest(height int64, peerID string) {
	if !pool.IsRunning() {
		return
	}
	pool.requestsCh <- BlockRequest{height, peerID}
}

func (pool *BlockPool) sendTimeout(peerID string) {
	if !pool.IsRunning() {
		return
	}
	pool.timeoutsCh <- peerID
}

// unused by tendermint; left for debugging purposes
func (pool *BlockPool) debug() string {
	pool.mtx.Lock() // Lock
	defer pool.mtx.Unlock()

	str := ""
	nextHeight := pool.height + pool.requestersLen()
	for h := pool.height; h < nextHeight; h++ {
		if pool.requesters[h] == nil {
			str += cmn.Fmt("H(%v):X ", h)
		} else {
			str += cmn.Fmt("H(%v):", h)
			str += cmn.Fmt("B?(%v) ", pool.requesters[h].block != nil)
		}
	}
	return str
}

//-------------------------------------

type bpPeer struct {
	pool        *BlockPool
	id          string
	recvMonitor *flow.Monitor

	height     int64
	numPending int32
	timeout    *time.Timer
	didTimeout bool

	logger log.Logger
}

func newBPPeer(pool *BlockPool, peerID string, height int64) *bpPeer {
	peer := &bpPeer{
		pool:       pool,
		id:         peerID,
		height:     height,
		numPending: 0,
		logger:     log.NewNopLogger(),
	}
	return peer
}

func (peer *bpPeer) setLogger(l log.Logger) {
	peer.logger = l
}

func (peer *bpPeer) resetMonitor() {
	peer.recvMonitor = flow.New(time.Second, time.Second*40)
	initialValue := float64(minRecvRate) * math.E
	peer.recvMonitor.SetREMA(initialValue)
}

func (peer *bpPeer) resetTimeout() {
	if peer.timeout == nil {
		peer.timeout = time.AfterFunc(time.Second*peerTimeoutSeconds, peer.onTimeout)
	} else {
		peer.timeout.Reset(time.Second * peerTimeoutSeconds)
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
	peer.numPending--
	if peer.numPending == 0 {
		peer.timeout.Stop()
	} else {
		peer.recvMonitor.Update(recvSize)
		peer.resetTimeout()
	}
}

func (peer *bpPeer) onTimeout() {
	peer.pool.mtx.Lock()
	defer peer.pool.mtx.Unlock()

	peer.pool.sendTimeout(peer.id)
	peer.logger.Error("SendTimeout", "reason", "onTimeout")
	peer.didTimeout = true
}

//-------------------------------------

type bpRequester struct {
	cmn.BaseService
	pool       *BlockPool
	height     int64
	gotBlockCh chan struct{}
	redoCh     chan struct{}

	mtx    sync.Mutex
	peerID string
	block  *types.Block
}

func newBPRequester(pool *BlockPool, height int64) *bpRequester {
	bpr := &bpRequester{
		pool:       pool,
		height:     height,
		gotBlockCh: make(chan struct{}),
		redoCh:     make(chan struct{}),

		peerID: "",
		block:  nil,
	}
	bpr.BaseService = *cmn.NewBaseService(nil, "bpRequester", bpr)
	return bpr
}

func (bpr *bpRequester) OnStart() error {
	go bpr.requestRoutine()
	return nil
}

// Returns true if the peer matches
func (bpr *bpRequester) setBlock(block *types.Block, peerID string) bool {
	bpr.mtx.Lock()
	if bpr.block != nil || bpr.peerID != peerID {
		bpr.mtx.Unlock()
		return false
	}
	bpr.block = block
	bpr.mtx.Unlock()

	bpr.gotBlockCh <- struct{}{}
	return true
}

func (bpr *bpRequester) getBlock() *types.Block {
	bpr.mtx.Lock()
	defer bpr.mtx.Unlock()
	return bpr.block
}

func (bpr *bpRequester) getPeerID() string {
	bpr.mtx.Lock()
	defer bpr.mtx.Unlock()
	return bpr.peerID
}

func (bpr *bpRequester) reset() {
	bpr.mtx.Lock()
	bpr.peerID = ""
	bpr.block = nil
	bpr.mtx.Unlock()
}

// Tells bpRequester to pick another peer and try again.
// NOTE: blocking
func (bpr *bpRequester) redo() {
	bpr.redoCh <- struct{}{}
}

// Responsible for making more requests as necessary
// Returns only when a block is found (e.g. AddBlock() is called)
func (bpr *bpRequester) requestRoutine() {
OUTER_LOOP:
	for {
		// Pick a peer to send request to.
		var peer *bpPeer = nil
	PICK_PEER_LOOP:
		for {
			if !bpr.IsRunning() || !bpr.pool.IsRunning() {
				return
			}
			peer = bpr.pool.pickIncrAvailablePeer(bpr.height)
			if peer == nil {
				//log.Info("No peers available", "height", height)
				time.Sleep(requestIntervalMS * time.Millisecond)
				continue PICK_PEER_LOOP
			}
			break PICK_PEER_LOOP
		}
		bpr.mtx.Lock()
		bpr.peerID = peer.id
		bpr.mtx.Unlock()

		// Send request and wait.
		bpr.pool.sendRequest(bpr.height, peer.id)
		select {
		case <-bpr.pool.Quit:
			bpr.Stop()
			return
		case <-bpr.Quit:
			return
		case <-bpr.redoCh:
			bpr.reset()
			continue OUTER_LOOP // When peer is removed
		case <-bpr.gotBlockCh:
			// We got the block, now see if it's good.
			select {
			case <-bpr.pool.Quit:
				bpr.Stop()
				return
			case <-bpr.Quit:
				return
			case <-bpr.redoCh:
				bpr.reset()
				continue OUTER_LOOP
			}
		}
	}
}

//-------------------------------------

type BlockRequest struct {
	Height int64
	PeerID string
}
