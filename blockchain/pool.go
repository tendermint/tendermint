package blockchain

import (
	"sync"
	"sync/atomic"
	"time"

	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/types"
)

const (
	maxOutstandingRequestsPerPeer = 10
	inputsChannelCapacity         = 100
	maxTries                      = 3
	requestIntervalMS             = 500
	requestBatchSize              = 50
	maxPendingRequests            = 50
	maxTotalRequests              = 100
	maxRequestsPerPeer            = 20
)

var (
	requestTimeoutSeconds = time.Duration(1)
)

type BlockPool struct {
	// block requests
	requestsMtx sync.Mutex
	requests    map[uint]*bpRequest
	height      uint // the lowest key in requests.
	numPending  int32
	numTotal    int32

	// peers
	peersMtx sync.Mutex
	peers    map[string]*bpPeer

	requestsCh chan<- BlockRequest
	timeoutsCh chan<- string
	repeater   *RepeatTimer

	running int32 // atomic
}

func NewBlockPool(start uint, requestsCh chan<- BlockRequest, timeoutsCh chan<- string) *BlockPool {
	return &BlockPool{
		peers: make(map[string]*bpPeer),

		requests:   make(map[uint]*bpRequest),
		height:     start,
		numPending: 0,
		numTotal:   0,

		requestsCh: requestsCh,
		timeoutsCh: timeoutsCh,
		repeater:   NewRepeatTimer("", requestIntervalMS*time.Millisecond),

		running: 0,
	}
}

func (bp *BlockPool) Start() {
	if atomic.CompareAndSwapInt32(&bp.running, 0, 1) {
		log.Info("Starting BlockPool")
		go bp.run()
	}
}

func (bp *BlockPool) Stop() {
	if atomic.CompareAndSwapInt32(&bp.running, 1, 0) {
		log.Info("Stopping BlockPool")
		bp.repeater.Stop()
	}
}

func (bp *BlockPool) IsRunning() bool {
	return atomic.LoadInt32(&bp.running) == 1
}

// Run spawns requests as needed.
func (bp *BlockPool) run() {
RUN_LOOP:
	for {
		if atomic.LoadInt32(&bp.running) == 0 {
			break RUN_LOOP
		}
		height, numPending, numTotal := bp.GetStatus()
		log.Debug("BlockPool.run", "height", height, "numPending", numPending,
			"numTotal", numTotal)
		if numPending >= maxPendingRequests {
			// sleep for a bit.
			time.Sleep(requestIntervalMS * time.Millisecond)
		} else if numTotal >= maxTotalRequests {
			// sleep for a bit.
			time.Sleep(requestIntervalMS * time.Millisecond)
		} else {
			// request for more blocks.
			height := bp.nextHeight()
			bp.makeRequest(height)
		}
	}
}

func (bp *BlockPool) GetStatus() (uint, int32, int32) {
	bp.requestsMtx.Lock() // Lock
	defer bp.requestsMtx.Unlock()

	return bp.height, bp.numPending, bp.numTotal
}

// We need to see the second block's Validation to validate the first block.
// So we peek two blocks at a time.
func (bp *BlockPool) PeekTwoBlocks() (first *types.Block, second *types.Block) {
	bp.requestsMtx.Lock() // Lock
	defer bp.requestsMtx.Unlock()

	if r := bp.requests[bp.height]; r != nil {
		first = r.block
	}
	if r := bp.requests[bp.height+1]; r != nil {
		second = r.block
	}
	return
}

// Pop the first block at bp.height
// It must have been validated by 'second'.Validation from PeekTwoBlocks().
func (bp *BlockPool) PopRequest() {
	bp.requestsMtx.Lock() // Lock
	defer bp.requestsMtx.Unlock()

	if r := bp.requests[bp.height]; r == nil || r.block == nil {
		panic("PopRequest() requires a valid block")
	}

	delete(bp.requests, bp.height)
	bp.height++
	bp.numTotal--
}

// Invalidates the block at bp.height.
// Remove the peer and request from others.
func (bp *BlockPool) RedoRequest(height uint) {
	bp.requestsMtx.Lock() // Lock
	defer bp.requestsMtx.Unlock()

	request := bp.requests[height]
	if request.block == nil {
		panic("Expected block to be non-nil")
	}
	bp.removePeer(request.peerId)
	request.block = nil
	request.peerId = ""
	bp.numPending++

	go requestRoutine(bp, height)
}

func (bp *BlockPool) hasBlock(height uint) bool {
	bp.requestsMtx.Lock() // Lock
	defer bp.requestsMtx.Unlock()

	request := bp.requests[height]
	return request != nil && request.block != nil
}

func (bp *BlockPool) setPeerForRequest(height uint, peerId string) {
	bp.requestsMtx.Lock() // Lock
	defer bp.requestsMtx.Unlock()

	request := bp.requests[height]
	if request == nil {
		return
	}
	request.peerId = peerId
}

func (bp *BlockPool) AddBlock(block *types.Block, peerId string) {
	bp.requestsMtx.Lock() // Lock
	defer bp.requestsMtx.Unlock()

	request := bp.requests[block.Height]
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
	bp.numPending--
}

func (bp *BlockPool) getPeer(peerId string) *bpPeer {
	bp.peersMtx.Lock() // Lock
	defer bp.peersMtx.Unlock()

	peer := bp.peers[peerId]
	return peer
}

// Sets the peer's blockchain height.
func (bp *BlockPool) SetPeerHeight(peerId string, height uint) {
	bp.peersMtx.Lock() // Lock
	defer bp.peersMtx.Unlock()

	peer := bp.peers[peerId]
	if peer != nil {
		peer.height = height
	} else {
		peer = &bpPeer{
			height:      height,
			id:          peerId,
			numRequests: 0,
		}
		bp.peers[peerId] = peer
	}
}

func (bp *BlockPool) RemovePeer(peerId string) {
	bp.peersMtx.Lock() // Lock
	defer bp.peersMtx.Unlock()

	delete(bp.peers, peerId)
}

// Pick an available peer with at least the given minHeight.
// If no peers are available, returns nil.
func (bp *BlockPool) pickIncrAvailablePeer(minHeight uint) *bpPeer {
	bp.peersMtx.Lock()
	defer bp.peersMtx.Unlock()

	for _, peer := range bp.peers {
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

func (bp *BlockPool) decrPeer(peerId string) {
	bp.peersMtx.Lock()
	defer bp.peersMtx.Unlock()

	peer := bp.peers[peerId]
	if peer == nil {
		return
	}
	peer.numRequests--
}

func (bp *BlockPool) nextHeight() uint {
	bp.requestsMtx.Lock() // Lock
	defer bp.requestsMtx.Unlock()

	return bp.height + uint(bp.numTotal)
}

func (bp *BlockPool) makeRequest(height uint) {
	bp.requestsMtx.Lock() // Lock
	defer bp.requestsMtx.Unlock()

	request := &bpRequest{
		height: height,
		peerId: "",
		block:  nil,
	}
	bp.requests[height] = request

	nextHeight := bp.height + uint(bp.numTotal)
	if nextHeight == height {
		bp.numTotal++
		bp.numPending++
	}

	go requestRoutine(bp, height)
}

func (bp *BlockPool) sendRequest(height uint, peerId string) {
	if atomic.LoadInt32(&bp.running) == 0 {
		return
	}
	bp.requestsCh <- BlockRequest{height, peerId}
}

func (bp *BlockPool) sendTimeout(peerId string) {
	if atomic.LoadInt32(&bp.running) == 0 {
		return
	}
	bp.timeoutsCh <- peerId
}

func (bp *BlockPool) debug() string {
	bp.requestsMtx.Lock() // Lock
	defer bp.requestsMtx.Unlock()

	str := ""
	for h := bp.height; h < bp.height+uint(bp.numTotal); h++ {
		if bp.requests[h] == nil {
			str += Fmt("H(%v):X ", h)
		} else {
			str += Fmt("H(%v):", h)
			str += Fmt("B?(%v) ", bp.requests[h].block != nil)
		}
	}
	return str
}

//-------------------------------------

type bpPeer struct {
	id          string
	height      uint
	numRequests int32
}

type bpRequest struct {
	height uint
	peerId string
	block  *types.Block
}

//-------------------------------------

// Responsible for making more requests as necessary
// Returns when a block is found (e.g. AddBlock() is called)
func requestRoutine(bp *BlockPool, height uint) {
	for {
		var peer *bpPeer = nil
	PICK_LOOP:
		for {
			if !bp.IsRunning() {
				return
			}
			peer = bp.pickIncrAvailablePeer(height)
			if peer == nil {
				time.Sleep(requestIntervalMS * time.Millisecond)
				continue PICK_LOOP
			}
			break PICK_LOOP
		}

		bp.setPeerForRequest(height, peer.id)

		for try := 0; try < maxTries; try++ {
			bp.sendRequest(height, peer.id)
			time.Sleep(requestTimeoutSeconds * time.Second)
			if bp.hasBlock(height) {
				bp.decrPeer(peer.id)
				return
			}
			bpHeight, _, _ := bp.GetStatus()
			if height < bpHeight {
				bp.decrPeer(peer.id)
				return
			}
		}

		bp.RemovePeer(peer.id)
		bp.sendTimeout(peer.id)
	}
}

//-------------------------------------

type BlockRequest struct {
	Height uint
	PeerId string
}
