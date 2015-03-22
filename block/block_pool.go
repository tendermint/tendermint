package block

import (
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
	height     uint // the lowest key in blockInfos.
	started    int32
	stopped    int32
	numPending int32
	numTotal   int32
	quit       chan struct{}

	eventsCh   chan interface{}
	requestsCh chan<- BlockRequest // output of new requests to make.
	timeoutsCh chan<- string       // output of peers that timed out.
	blocksCh   chan<- *Block       // output of ordered blocks.
	repeater   *RepeatTimer        // for requesting more bocks.
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
			// make more requests if necessary.
			for i := 0; i < requestBatchSize; i++ {
				if atomic.LoadInt32(&bp.numPending) < maxPendingRequests &&
					atomic.LoadInt32(&bp.numTotal) < maxTotalRequests {
					atomic.AddInt32(&bp.numPending, 1)
					atomic.AddInt32(&bp.numTotal, 1)
					requestHeight := bp.height + uint(bp.numTotal)
					blockInfo := bpNewBlockInfo(requestHeight)
					bp.blockInfos[requestHeight] = blockInfo
				} else {
					break
				}
			}
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
			if blockInfo.block == nil {
				// peer is the first to give it to us.
				blockInfo.block = event.block
				blockInfo.blockBy = peer.id
				atomic.AddInt32(&bp.numPending, -1)
				if event.block.Height == bp.height {
					// push block to blocksCh.
					atomic.AddInt32(&bp.numTotal, -1)
					delete(peer.requests, bp.height)
					delete(blockInfo.requests, peer.id)
					go func() { bp.blocksCh <- event.block }()
				}
			}
		}
	case bpPeerStatus:
		// we have updated (or new) status from peer,
		// request blocks if possible.
		bp.requestBlocksFromPeer(event.peerId, event.height)
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
				event := BlockRequest{event.height, peer.id}
				go func() { bp.requestsCh <- event }()
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

func (bp *BlockPool) requestBlocksFromPeer(peerId string, height uint) {
	peer := bp.peers[peerId]
	if peer == nil {
		bp.peers[peerId] = bpNewPeer(peerId, height)
	}
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
