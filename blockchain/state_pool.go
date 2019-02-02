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
	sm "github.com/tendermint/tendermint/state"
)

/*
	XXX: This file is copied from blockchain/pool.go
*/

/*
	Peers self report their heights when we join the block pool.
	Starting from our latest pool.height, we request blocks
	in sequence from peers that reported higher heights than ours.
	Every so often we ask peers what height they're on so we can keep going.

	Requests are continuously made for blocks of higher heights until
	the limit is reached. If most of the requests have no available peers, and we
	are not at peer limits, we can probably switch to consensus reactor
*/


// TODO: revisit for potential mutlithread issue
type StatePool struct {
	cmn.BaseService
	startTime time.Time

	mtx sync.Mutex
	// block requests
	height    int64 // the height in first state status response we received
	numKeys []int64	// numKeys we are expected
	totalKeys int64
	numKeysReceived int64	// numKeys we have received, no need to be atomic, guarded by pool.mtx
	step int64
	state *sm.State
	chunks map[int64][][]byte	// startIdx -> [key, value] map, TODO: sync.Map
	requests map[p2p.ID]*StateRequest	// TODO: sync.Map

	// peers
	peers         map[p2p.ID]*spPeer

	// atomic
	numPending int32 // number of requests pending assignment or state response

	requestsCh chan<- StateRequest
	errorsCh   chan<- peerError
}

func NewStatePool(requestsCh chan<- StateRequest, errorsCh chan<- peerError) *StatePool {
	sp := &StatePool{
		peers: make(map[p2p.ID]*spPeer),
		chunks: make(map[int64][][]byte),
		requests: make(map[p2p.ID]*StateRequest),

		requestsCh: requestsCh,
		errorsCh:   errorsCh,
	}
	sp.BaseService = *cmn.NewBaseService(nil, "StatePool", sp)
	return sp
}

func (pool *StatePool) OnStart() error {
	pool.startTime = time.Now()
	return nil
}

func (pool *StatePool) OnStop() {}

func (pool *StatePool) removeTimedoutPeers() {
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

func (pool *StatePool) AddStateChunk(peerID p2p.ID, msg *bcStateResponseMessage) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()

	pool.Logger.Info("peer sent us a start index we didn't expect", "peer", peerID, "startIndex", msg.StartIdxInc)

	pool.chunks[msg.StartIdxInc] = msg.Chunks

	atomic.AddInt32(&pool.numPending, -1)
	atomic.AddInt64(&pool.numKeysReceived, msg.EndIdxExc - msg.StartIdxInc)
	peer := pool.peers[peerID]
	if peer != nil {
		peer.decrPending()
	}
	delete(pool.requests, peerID)
}

// Sets the peer's alleged blockchain height.
func (pool *StatePool) SetPeerHeight(peerID p2p.ID, height int64) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()

	peer := pool.peers[peerID]
	if peer != nil {
		peer.height = height
	} else {
		peer = newSPPeer(pool, peerID, height)
		peer.setLogger(pool.Logger.With("peer", peerID))
		pool.peers[peerID] = peer
	}
}

func (pool *StatePool) RemovePeer(peerID p2p.ID) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()

	pool.removePeer(peerID)
}

// MUST BE REVISITED, WE MIGHT CAN RETRY
func (pool *StatePool) removePeer(peerID p2p.ID) {
	if _, ok := pool.requests[peerID]; ok {
		delete(pool.requests, peerID)
	}
	delete(pool.peers, peerID)
}

// Pick an available peer with at least the given minHeight.
// If no peers are available, returns nil.
func (pool *StatePool) pickAvailablePeers() (peers []*spPeer) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()

	peers = make([]*spPeer, 0, len(pool.peers))

	for _, peer := range pool.peers {
		if peer.didTimeout {
			pool.removePeer(peer.id)
			continue
		}
		if peer.numPending >= 1 {
			continue
		}
		if math.Abs(float64(peer.height) - float64(pool.height)) > peerStateHeightThreshold {
			continue
		}
		peer.incrPending()
		peers = append(peers, peer)
	}
	return peers
}

func (pool *StatePool) sendRequest() {
	if !pool.IsRunning() {
		pool.Logger.Error("send request on a stopped pool")
		return
	}
	var peers []*spPeer
	peers = pool.pickAvailablePeers()
	if len(peers) == 0 {
		pool.Logger.Info("No peers available", "height", pool.height)
	}

	pool.step = int64(math.Ceil(float64(pool.totalKeys) / float64(len(peers))))
	for idx, peer := range peers {
		// Send request and wait.
		endIndex := (int64(idx) + 1) * pool.step
		if endIndex > pool.totalKeys {
			endIndex = pool.totalKeys
		}
		pool.requestsCh <- StateRequest{pool.height, peer.id, int64(idx) * pool.step, endIndex}
		atomic.AddInt32(&pool.numPending, 1)
	}
}

func (pool *StatePool) sendError(err error, peerID p2p.ID) {
	if !pool.IsRunning() {
		return
	}
	pool.errorsCh <- peerError{err, peerID}
}

func (pool *StatePool) isComplete() bool {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()
	return atomic.LoadInt32(&pool.numPending) == 0 && pool.numKeysReceived == pool.totalKeys
}

func (pool *StatePool) reset() {
	pool.mtx.Lock()
	pool.mtx.Unlock()

	if pool.isComplete() {
		// we might already complete, possible routine:
		// 1. poolRoutine find us timeout the ticker of retry
		// 2. we received last pieces of state from peers
		// 3. poolRoutine call reset

		// Deliberately do nothing here, pool should has been stopped
	} else {
		pool.height = 0
		pool.numKeys = make([]int64, 0)
		pool.totalKeys = 0
		pool.numKeysReceived = 0
		atomic.StoreInt32(&pool.numPending, 0)
		pool.chunks = make(map[int64][][]byte)
		pool.step = 0
		pool.state = nil
		pool.requests = make(map[p2p.ID]*StateRequest)
	}
}

//-------------------------------------

type spPeer struct {
	pool        *StatePool
	id          p2p.ID
	recvMonitor *flow.Monitor

	height     int64
	numPending int32
	timeout    *time.Timer
	didTimeout bool

	logger log.Logger
}

func newSPPeer(pool *StatePool, peerID p2p.ID, height int64) *spPeer {
	peer := &spPeer{
		pool:       pool,
		id:         peerID,
		height:     height,
		numPending: 0,
		logger:     log.NewNopLogger(),
	}
	return peer
}

func (peer *spPeer) setLogger(l log.Logger) {
	peer.logger = l
}

func (peer *spPeer) resetMonitor() {
	peer.recvMonitor = flow.New(time.Second, time.Second*40)
	initialValue := float64(minRecvRate) * math.E
	peer.recvMonitor.SetREMA(initialValue)
}

func (peer *spPeer) resetTimeout() {
	if peer.timeout == nil {
		peer.timeout = time.AfterFunc(peerTimeout, peer.onTimeout)
	} else {
		peer.timeout.Reset(peerTimeout)
	}
}

func (peer *spPeer) incrPending() {
	if peer.numPending == 0 {
		peer.resetMonitor()
		peer.resetTimeout()
	}
	peer.numPending++
}

func (peer *spPeer) decrPending() {
	peer.numPending--
	if peer.numPending == 0 {
		peer.timeout.Stop()
	} else {
		peer.resetTimeout()
	}
}

func (peer *spPeer) onTimeout() {
	peer.pool.mtx.Lock()
	defer peer.pool.mtx.Unlock()

	err := errors.New("peer did not send us anything")
	peer.pool.sendError(err, peer.id)
	peer.logger.Error("SendTimeout", "reason", err, "timeout", peerTimeout)
	peer.didTimeout = true
}

type StateRequest struct {
	Height int64
	PeerID p2p.ID
	StartIndex int64
	EndIndex int64
}
