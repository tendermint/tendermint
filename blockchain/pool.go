package blockchain

import (
	"sync"
	"time"

	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/types"
)

const (
	maxTries              = 3
	inputsChannelCapacity = 200
	requestIntervalMS     = 500
	maxPendingRequests    = 200
	maxTotalRequests      = 300
	maxRequestsPerPeer    = 300
)

var (
	requestTimeoutSeconds = time.Duration(3)
)

/*
	Peers self report their heights when we join the block pool.
	Starting from our latest pool.height, we request blocks
	in sequence from peers that reported higher heights than ours.
	Every so often we ask peers what height they're on so we can keep going.

	Requests are continuously made for blocks of heigher heights until
	the limits. If most of the requests have no available peers, and we
	are not at peer limits, we can probably switch to consensus reactor
*/

type BlockPool struct {
	BaseService

	// block requests
	requestsMtx   sync.Mutex
	requests      map[int]*bpRequest
	height        int   // the lowest key in requests.
	numUnassigned int32 // number of requests not yet assigned to a peer
	numPending    int32 // number of requests pending assignment or block response

	// peers
	peersMtx sync.Mutex
	peers    map[string]*bpPeer

	requestsCh chan<- BlockRequest
	timeoutsCh chan<- string
	repeater   *RepeatTimer
}

func NewBlockPool(start int, requestsCh chan<- BlockRequest, timeoutsCh chan<- string) *BlockPool {
	bp := &BlockPool{
		peers: make(map[string]*bpPeer),

		requests:      make(map[int]*bpRequest),
		height:        start,
		numUnassigned: 0,
		numPending:    0,

		requestsCh: requestsCh,
		timeoutsCh: timeoutsCh,
		repeater:   nil,
	}
	bp.BaseService = *NewBaseService(log, "BlockPool", bp)
	return bp
}

func (pool *BlockPool) OnStart() error {
	pool.BaseService.OnStart()
	pool.repeater = NewRepeatTimer("", requestIntervalMS*time.Millisecond)
	go pool.run()
	return nil
}

func (pool *BlockPool) OnStop() {
	pool.BaseService.OnStop()
	pool.repeater.Stop()
}

// Run spawns requests as needed.
func (pool *BlockPool) run() {
RUN_LOOP:
	for {
		if !pool.IsRunning() {
			break RUN_LOOP
		}
		_, numPending, _ := pool.GetStatus()
		if numPending >= maxPendingRequests {
			// sleep for a bit.
			time.Sleep(requestIntervalMS * time.Millisecond)
		} else if len(pool.requests) >= maxTotalRequests {
			// sleep for a bit.
			time.Sleep(requestIntervalMS * time.Millisecond)
		} else {
			// request for more blocks.
			pool.makeNextRequest()
		}
	}
}

func (pool *BlockPool) GetStatus() (height int, numPending int32, numUnssigned int32) {
	pool.requestsMtx.Lock() // Lock
	defer pool.requestsMtx.Unlock()

	return pool.height, pool.numPending, pool.numUnassigned
}

// We need to see the second block's Validation to validate the first block.
// So we peek two blocks at a time.
func (pool *BlockPool) PeekTwoBlocks() (first *types.Block, second *types.Block) {
	pool.requestsMtx.Lock() // Lock
	defer pool.requestsMtx.Unlock()

	if r := pool.requests[pool.height]; r != nil {
		first = r.block
	}
	if r := pool.requests[pool.height+1]; r != nil {
		second = r.block
	}
	return
}

// Pop the first block at pool.height
// It must have been validated by 'second'.Validation from PeekTwoBlocks().
func (pool *BlockPool) PopRequest() {
	pool.requestsMtx.Lock() // Lock
	defer pool.requestsMtx.Unlock()

	if r := pool.requests[pool.height]; r == nil || r.block == nil {
		PanicSanity("PopRequest() requires a valid block")
	}

	delete(pool.requests, pool.height)
	pool.height++
}

// Invalidates the block at pool.height.
// Remove the peer and request from others.
func (pool *BlockPool) RedoRequest(height int) {
	pool.requestsMtx.Lock() // Lock
	defer pool.requestsMtx.Unlock()

	request := pool.requests[height]
	if request.block == nil {
		PanicSanity("Expected block to be non-nil")
	}
	// TODO: record this malfeasance
	// maybe punish peer on switch (an invalid block!)
	pool.RemovePeer(request.peerId) // Lock on peersMtx.
	request.block = nil
	request.peerId = ""
	pool.numPending++
	pool.numUnassigned++

	go requestRoutine(pool, height)
}

func (pool *BlockPool) hasBlock(height int) bool {
	pool.requestsMtx.Lock() // Lock
	defer pool.requestsMtx.Unlock()

	request := pool.requests[height]
	return request != nil && request.block != nil
}

func (pool *BlockPool) setPeerForRequest(height int, peerId string) {
	pool.requestsMtx.Lock() // Lock
	defer pool.requestsMtx.Unlock()

	request := pool.requests[height]
	if request == nil {
		return
	}
	pool.numUnassigned--
	request.peerId = peerId
}

func (pool *BlockPool) removePeerForRequest(height int, peerId string) {
	pool.requestsMtx.Lock() // Lock
	defer pool.requestsMtx.Unlock()

	request := pool.requests[height]
	if request == nil {
		return
	}
	pool.numUnassigned++
	request.peerId = ""
}

func (pool *BlockPool) AddBlock(block *types.Block, peerId string) {
	pool.requestsMtx.Lock() // Lock
	defer pool.requestsMtx.Unlock()

	request := pool.requests[block.Height]
	if request == nil {
		return
	}
	if request.peerId != peerId {
		return
	}
	if request.block != nil {
		return
	}
	request.block = block
	pool.numPending--
}

func (pool *BlockPool) getPeer(peerId string) *bpPeer {
	pool.peersMtx.Lock() // Lock
	defer pool.peersMtx.Unlock()

	peer := pool.peers[peerId]
	return peer
}

// Sets the peer's alleged blockchain height.
func (pool *BlockPool) SetPeerHeight(peerId string, height int) {
	pool.peersMtx.Lock() // Lock
	defer pool.peersMtx.Unlock()

	peer := pool.peers[peerId]
	if peer != nil {
		peer.height = height
	} else {
		peer = &bpPeer{
			height:      height,
			id:          peerId,
			numRequests: 0,
		}
		pool.peers[peerId] = peer
	}
}

func (pool *BlockPool) RemovePeer(peerId string) {
	pool.peersMtx.Lock() // Lock
	defer pool.peersMtx.Unlock()

	delete(pool.peers, peerId)
}

// Pick an available peer with at least the given minHeight.
// If no peers are available, returns nil.
func (pool *BlockPool) pickIncrAvailablePeer(minHeight int) *bpPeer {
	pool.peersMtx.Lock()
	defer pool.peersMtx.Unlock()

	for _, peer := range pool.peers {
		if peer.numRequests >= maxRequestsPerPeer {
			continue
		}
		if peer.height < minHeight {
			continue
		}
		peer.numRequests++
		return peer
	}
	return nil
}

func (pool *BlockPool) decrPeer(peerId string) {
	pool.peersMtx.Lock()
	defer pool.peersMtx.Unlock()

	peer := pool.peers[peerId]
	if peer == nil {
		return
	}
	peer.numRequests--
}

func (pool *BlockPool) makeNextRequest() {
	pool.requestsMtx.Lock() // Lock
	defer pool.requestsMtx.Unlock()

	nextHeight := pool.height + len(pool.requests)
	request := &bpRequest{
		height: nextHeight,
		peerId: "",
		block:  nil,
	}

	pool.requests[nextHeight] = request
	pool.numUnassigned++
	pool.numPending++

	go requestRoutine(pool, nextHeight)
}

func (pool *BlockPool) sendRequest(height int, peerId string) {
	if !pool.IsRunning() {
		return
	}
	pool.requestsCh <- BlockRequest{height, peerId}
}

func (pool *BlockPool) sendTimeout(peerId string) {
	if !pool.IsRunning() {
		return
	}
	pool.timeoutsCh <- peerId
}

func (pool *BlockPool) debug() string {
	pool.requestsMtx.Lock() // Lock
	defer pool.requestsMtx.Unlock()

	str := ""
	for h := pool.height; h < pool.height+len(pool.requests); h++ {
		if pool.requests[h] == nil {
			str += Fmt("H(%v):X ", h)
		} else {
			str += Fmt("H(%v):", h)
			str += Fmt("B?(%v) ", pool.requests[h].block != nil)
		}
	}
	return str
}

//-------------------------------------

type bpPeer struct {
	id          string
	height      int
	numRequests int32
}

type bpRequest struct {
	height int
	peerId string
	block  *types.Block
}

//-------------------------------------

// Responsible for making more requests as necessary
// Returns only when a block is found (e.g. AddBlock() is called)
func requestRoutine(pool *BlockPool, height int) {
	for {
		var peer *bpPeer = nil
	PICK_LOOP:
		for {
			if !pool.IsRunning() {
				log.Info("BlockPool not running. Stopping requestRoutine", "height", height)
				return
			}
			peer = pool.pickIncrAvailablePeer(height)
			if peer == nil {
				//log.Info("No peers available", "height", height)
				time.Sleep(requestIntervalMS * time.Millisecond)
				continue PICK_LOOP
			}
			break PICK_LOOP
		}

		// set the peer, decrement numUnassigned
		pool.setPeerForRequest(height, peer.id)

		for try := 0; try < maxTries; try++ {
			pool.sendRequest(height, peer.id)
			time.Sleep(requestTimeoutSeconds * time.Second)
			// if successful the block is either in the pool,
			if pool.hasBlock(height) {
				pool.decrPeer(peer.id)
				return
			}
			// or already processed and we've moved past it
			bpHeight, _, _ := pool.GetStatus()
			if height < bpHeight {
				pool.decrPeer(peer.id)
				return
			}
		}

		// unset the peer, increment numUnassigned
		pool.removePeerForRequest(height, peer.id)

		// this peer failed us, try again
		pool.RemovePeer(peer.id)
		pool.sendTimeout(peer.id)
	}
}

//-------------------------------------

type BlockRequest struct {
	Height int
	PeerId string
}
