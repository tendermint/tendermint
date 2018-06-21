package blockchain

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	cmn "github.com/tendermint/tendermint/libs/common"
	flow "github.com/tendermint/tendermint/libs/flowrate"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
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

	// Minimum recv rate to ensure we're receiving blocks from a peer fast
	// enough. If a peer is not sending us data at at least that rate, we
	// consider them to have timedout and we disconnect.
	//
	// Assuming a DSL connection (not a good choice) 128 Kbps (upload) ~ 15 KB/s,
	// sending data across atlantic ~ 7.5 KB/s.
	minRecvRate = 7680

	// Maximum difference between current and new block's height.
	maxDiffBetweenCurrentAndReceivedBlockHeight = 100
)

var peerTimeout = 15 * time.Second // not const so we can override with tests

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
	// peers
	peers         map[p2p.ID]*bpPeer
	maxPeerHeight int64

	// atomic
	numPending int32 // number of requests pending assignment or block response

	requestsCh chan<- BlockRequest
	errorsCh   chan<- peerError
}

func NewBlockPool(start int64, requestsCh chan<- BlockRequest, errorsCh chan<- peerError) *BlockPool {
	bp := &BlockPool{
		peers: make(map[p2p.ID]*bpPeer),

		requesters: make(map[int64]*bpRequester),
		height:     start,
		numPending: 0,

		requestsCh: requestsCh,
		errorsCh:   errorsCh,
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
			// curRate can be 0 on start
			if curRate != 0 && curRate < minRecvRate {
				err := errors.New("peer is not sending us data fast enough")
				pool.sendError(err, peer.id)
				pool.Logger.Error("SendTimeout", "peer", peer.id,
					"reason", err,
					"curRate", fmt.Sprintf("%d KB/s", curRate/1024),
					"minRate", fmt.Sprintf("%d KB/s", minRecvRate/1024))
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

	return pool.height, atomic.LoadInt32(&pool.numPending), len(pool.requesters)
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
		panic(fmt.Sprintf("Expected requester to pop, got nothing at height %v", pool.height))
	}
}

// Invalidates the block at pool.height,
// Remove the peer and redo request from others.
// Returns the ID of the removed peer.
func (pool *BlockPool) RedoRequest(height int64) p2p.ID {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()

	request := pool.requesters[height]

	if request.block == nil {
		panic("Expected block to be non-nil")
	}

	// RemovePeer will redo all requesters associated with this peer.
	pool.removePeer(request.peerID)
	return request.peerID
}

// TODO: ensure that blocks come in order for each peer.
func (pool *BlockPool) AddBlock(peerID p2p.ID, block *types.Block, blockSize int) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()

	requester := pool.requesters[block.Height]
	if requester == nil {
		pool.Logger.Info("peer sent us a block we didn't expect", "peer", peerID, "curHeight", pool.height, "blockHeight", block.Height)
		diff := pool.height - block.Height
		if diff < 0 {
			diff *= -1
		}
		if diff > maxDiffBetweenCurrentAndReceivedBlockHeight {
			pool.sendError(errors.New("peer sent us a block we didn't expect with a height too far ahead/behind"), peerID)
		}
		return
	}

	if requester.setBlock(block, peerID) {
		atomic.AddInt32(&pool.numPending, -1)
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
func (pool *BlockPool) SetPeerHeight(peerID p2p.ID, height int64) {
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

func (pool *BlockPool) RemovePeer(peerID p2p.ID) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()

	pool.removePeer(peerID)
}

func (pool *BlockPool) removePeer(peerID p2p.ID) {
	for _, requester := range pool.requesters {
		if requester.getPeerID() == peerID {
			requester.redo()
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
	atomic.AddInt32(&pool.numPending, 1)

	err := request.Start()
	if err != nil {
		request.Logger.Error("Error starting request", "err", err)
	}
}

func (pool *BlockPool) requestersLen() int64 {
	return int64(len(pool.requesters))
}

func (pool *BlockPool) sendRequest(height int64, peerID p2p.ID) {
	if !pool.IsRunning() {
		return
	}
	pool.requestsCh <- BlockRequest{height, peerID}
}

func (pool *BlockPool) sendError(err error, peerID p2p.ID) {
	if !pool.IsRunning() {
		return
	}
	pool.errorsCh <- peerError{err, peerID}
}

// unused by tendermint; left for debugging purposes
func (pool *BlockPool) debug() string {
	pool.mtx.Lock()
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
	id          p2p.ID
	recvMonitor *flow.Monitor

	height     int64
	numPending int32
	timeout    *time.Timer
	didTimeout bool

	logger log.Logger
}

func newBPPeer(pool *BlockPool, peerID p2p.ID, height int64) *bpPeer {
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

	err := errors.New("peer did not send us anything")
	peer.pool.sendError(err, peer.id)
	peer.logger.Error("SendTimeout", "reason", err, "timeout", peerTimeout)
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
	peerID p2p.ID
	block  *types.Block
}

func newBPRequester(pool *BlockPool, height int64) *bpRequester {
	bpr := &bpRequester{
		pool:       pool,
		height:     height,
		gotBlockCh: make(chan struct{}, 1),
		redoCh:     make(chan struct{}, 1),

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

// Returns true if the peer matches and block doesn't already exist.
func (bpr *bpRequester) setBlock(block *types.Block, peerID p2p.ID) bool {
	bpr.mtx.Lock()
	if bpr.block != nil || bpr.peerID != peerID {
		bpr.mtx.Unlock()
		return false
	}
	bpr.block = block
	bpr.mtx.Unlock()

	select {
	case bpr.gotBlockCh <- struct{}{}:
	default:
	}
	return true
}

func (bpr *bpRequester) getBlock() *types.Block {
	bpr.mtx.Lock()
	defer bpr.mtx.Unlock()
	return bpr.block
}

func (bpr *bpRequester) getPeerID() p2p.ID {
	bpr.mtx.Lock()
	defer bpr.mtx.Unlock()
	return bpr.peerID
}

// This is called from the requestRoutine, upon redo().
func (bpr *bpRequester) reset() {
	bpr.mtx.Lock()
	defer bpr.mtx.Unlock()

	if bpr.block != nil {
		atomic.AddInt32(&bpr.pool.numPending, 1)
	}

	bpr.peerID = ""
	bpr.block = nil
}

// Tells bpRequester to pick another peer and try again.
// NOTE: Nonblocking, and does nothing if another redo
// was already requested.
func (bpr *bpRequester) redo() {
	select {
	case bpr.redoCh <- struct{}{}:
	default:
	}
}

// Responsible for making more requests as necessary
// Returns only when a block is found (e.g. AddBlock() is called)
func (bpr *bpRequester) requestRoutine() {
OUTER_LOOP:
	for {
		// Pick a peer to send request to.
		var peer *bpPeer
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
	WAIT_LOOP:
		for {
			select {
			case <-bpr.pool.Quit():
				bpr.Stop()
				return
			case <-bpr.Quit():
				return
			case <-bpr.redoCh:
				bpr.reset()
				continue OUTER_LOOP
			case <-bpr.gotBlockCh:
				// We got a block!
				// Continue the for-loop and wait til Quit.
				continue WAIT_LOOP
			}
		}
	}
}

//-------------------------------------

type BlockRequest struct {
	Height int64
	PeerID p2p.ID
}
