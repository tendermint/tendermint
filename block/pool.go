package block

import (
	"math/rand"
	"sync/atomic"
	"time"

	. "github.com/tendermint/tendermint/common"
)

const (
	maxOutstandingRequestsPerPeer = 10
	eventsChannelCapacity         = 100
	requestTimeoutSeconds         = 10
	maxTries                      = 3
	requestIntervalMS             = 500
	requestBatchSize              = 50
	maxPendingRequests            = 50
	maxTotalRequests              = 100
	maxPeersPerRequest            = 1
)

type BlockRequest struct {
	Height uint
	PeerId string
}

type BlockPool struct {
	peers      map[string]*bpPeer
	blockInfos map[uint]*bpBlockInfo
	height     uint  // the lowest key in blockInfos.
	started    int32 // atomic
	stopped    int32 // atomic
	numPending int32
	numTotal   int32
	eventsCh   chan interface{}    // internal events.
	requestsCh chan<- BlockRequest // output of new requests to make.
	timeoutsCh chan<- string       // output of peers that timed out.
	blocksCh   chan<- *Block       // output of ordered blocks.
	repeater   *RepeatTimer        // for requesting more bocks.
	quit       chan struct{}
}

func NewBlockPool(start uint, timeoutsCh chan<- string, requestsCh chan<- BlockRequest, blocksCh chan<- *Block) *BlockPool {
	return &BlockPool{
		peers:      make(map[string]*bpPeer),
		blockInfos: make(map[uint]*bpBlockInfo),
		height:     start,
		started:    0,
		stopped:    0,
		numPending: 0,
		numTotal:   0,
		quit:       make(chan struct{}),

		eventsCh:   make(chan interface{}, eventsChannelCapacity),
		requestsCh: requestsCh,
		timeoutsCh: timeoutsCh,
		blocksCh:   blocksCh,
		repeater:   NewRepeatTimer("", requestIntervalMS*time.Millisecond),
	}
}

func (bp *BlockPool) Start() {
	if atomic.CompareAndSwapInt32(&bp.started, 0, 1) {
		log.Info("Starting BlockPool")
		go bp.run()
	}
}

func (bp *BlockPool) Stop() {
	if atomic.CompareAndSwapInt32(&bp.stopped, 0, 1) {
		log.Info("Stopping BlockPool")
		close(bp.quit)
		close(bp.eventsCh)
		close(bp.requestsCh)
		close(bp.timeoutsCh)
		close(bp.blocksCh)
		bp.repeater.Stop()
	}
}

// AddBlock should be called when a block is received.
func (bp *BlockPool) AddBlock(block *Block, peerId string) {
	bp.eventsCh <- bpBlockResponse{block, peerId}
}

func (bp *BlockPool) SetPeerStatus(peerId string, height uint) {
	bp.eventsCh <- bpPeerStatus{peerId, height}
}

// Runs in a goroutine and processes messages.
func (bp *BlockPool) run() {
FOR_LOOP:
	for {
		select {
		case msg := <-bp.eventsCh:
			bp.handleEvent(msg)
		case <-bp.repeater.Ch:
			bp.makeMoreBlockInfos()
			bp.requestBlocksFromRandomPeers(10)
		case <-bp.quit:
			break FOR_LOOP
		}
	}
}

func (bp *BlockPool) handleEvent(event_ interface{}) {
	switch event := event_.(type) {
	case bpBlockResponse:
		peer := bp.peers[event.peerId]
		blockInfo := bp.blockInfos[event.block.Height]
		if blockInfo == nil {
			// block was unwanted.
			if peer != nil {
				peer.bad++
			}
		} else {
			// block was wanted.
			if peer != nil {
				peer.good++
			}
			delete(peer.requests, event.block.Height)
			if blockInfo.block == nil {
				// peer is the first to give it to us.
				blockInfo.block = event.block
				blockInfo.blockBy = peer.id
				bp.numPending--
				if event.block.Height == bp.height {
					go bp.pushBlocksFromStart()
				}
			}
		}
	case bpPeerStatus:
		// we have updated (or new) status from peer,
		// request blocks if possible.
		peer := bp.peers[event.peerId]
		if peer == nil {
			peer = bpNewPeer(event.peerId, event.height)
			bp.peers[peer.id] = peer
		}
		bp.requestBlocksFromPeer(peer)
	case bpRequestTimeout:
		peer := bp.peers[event.peerId]
		request := peer.requests[event.height]
		if request == nil || request.block != nil {
			// a request for event.height might have timed out for peer.
			// but not necessarily, the timeout is unconditional.
		} else {
			peer.bad++
			if request.tries < maxTries {
				// try again, start timer again.
				request.start(bp.eventsCh)
				msg := BlockRequest{event.height, peer.id}
				go func() { bp.requestsCh <- msg }()
			} else {
				// delete the request.
				if peer != nil {
					delete(peer.requests, event.height)
				}
				blockInfo := bp.blockInfos[event.height]
				if blockInfo != nil {
					delete(blockInfo.requests, peer.id)
				}
				go func() { bp.timeoutsCh <- peer.id }()
			}
		}
	}
}

func (bp *BlockPool) requestBlocksFromRandomPeers(maxPeers int) {
	chosen := bp.pickAvailablePeers(maxPeers)
	log.Debug("requestBlocksFromRandomPeers", "chosen", len(chosen))
	for _, peer := range chosen {
		bp.requestBlocksFromPeer(peer)
	}
}

func (bp *BlockPool) requestBlocksFromPeer(peer *bpPeer) {
	// If peer is available and can provide something...
	for height := bp.height; peer.available(); height++ {
		blockInfo := bp.blockInfos[height]
		if blockInfo == nil {
			// We're out of range.
			return
		}
		needsMorePeers := blockInfo.needsMorePeers()
		alreadyAskedPeer := blockInfo.requests[peer.id] != nil
		if needsMorePeers && !alreadyAskedPeer {
			// Create a new request and start the timer.
			request := &bpBlockRequest{
				height: height,
				peer:   peer,
			}
			blockInfo.requests[peer.id] = request
			peer.requests[height] = request
			request.start(bp.eventsCh)
			msg := BlockRequest{height, peer.id}
			go func() { bp.requestsCh <- msg }()
		}
	}
}

func (bp *BlockPool) makeMoreBlockInfos() {
	// make more requests if necessary.
	for i := 0; i < requestBatchSize; i++ {
		if bp.numPending < maxPendingRequests && bp.numTotal < maxTotalRequests {
			// Make a request for the next block height
			requestHeight := bp.height + uint(bp.numTotal)
			log.Debug("New blockInfo", "height", requestHeight)
			blockInfo := bpNewBlockInfo(requestHeight)
			bp.blockInfos[requestHeight] = blockInfo
			bp.numPending++
			bp.numTotal++
		} else {
			break
		}
	}
}

func (bp *BlockPool) pickAvailablePeers(choose int) []*bpPeer {
	available := []*bpPeer{}
	for _, peer := range bp.peers {
		if peer.available() {
			available = append(available, peer)
		}
	}
	perm := rand.Perm(MinInt(choose, len(available)))
	chosen := make([]*bpPeer, len(perm))
	for i, idx := range perm {
		chosen[i] = available[idx]
	}
	return chosen
}

func (bp *BlockPool) pushBlocksFromStart() {
	for height := bp.height; ; height++ {
		// push block to blocksCh.
		blockInfo := bp.blockInfos[height]
		if blockInfo == nil || blockInfo.block == nil {
			break
		}
		bp.numTotal--
		bp.height++
		delete(bp.blockInfos, height)
		go func() { bp.blocksCh <- blockInfo.block }()
	}
}

//-----------------------------------------------------------------------------

type bpBlockInfo struct {
	height   uint
	requests map[string]*bpBlockRequest
	block    *Block // first block received
	blockBy  string // peerId of source
}

func bpNewBlockInfo(height uint) *bpBlockInfo {
	return &bpBlockInfo{
		height:   height,
		requests: make(map[string]*bpBlockRequest),
	}
}

func (blockInfo *bpBlockInfo) needsMorePeers() bool {
	return len(blockInfo.requests) < maxPeersPerRequest
}

//-------------------------------------

type bpBlockRequest struct {
	peer   *bpPeer
	height uint
	block  *Block
	tries  int
}

// bump tries++ and set timeout.
// NOTE: the timer is unconditional.
func (request bpBlockRequest) start(eventsCh chan<- interface{}) {
	request.tries++
	time.AfterFunc(requestTimeoutSeconds*time.Second, func() {
		eventsCh <- bpRequestTimeout{
			peerId: request.peer.id,
			height: request.height,
		}
	})
}

//-------------------------------------

type bpPeer struct {
	id       string
	height   uint
	requests map[uint]*bpBlockRequest
	// Count good/bad events from peer.
	good uint
	bad  uint
}

func bpNewPeer(peerId string, height uint) *bpPeer {
	return &bpPeer{
		id:       peerId,
		height:   height,
		requests: make(map[uint]*bpBlockRequest),
	}
}

func (peer *bpPeer) available() bool {
	return len(peer.requests) < maxOutstandingRequestsPerPeer
}

//-------------------------------------
// bp.eventsCh messages

type bpBlockResponse struct {
	block  *Block
	peerId string
}

type bpPeerStatus struct {
	peerId string
	height uint // blockchain tip of peer
}

type bpRequestTimeout struct {
	peerId string
	height uint
}
