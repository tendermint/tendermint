package blockchain

import (
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"time"

	cmn "github.com/tendermint/tendermint/libs/common"
	flow "github.com/tendermint/tendermint/libs/flowrate"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

/*
	State pool is used to hold state chunks this peer requested from peers.
	Peer will request connected peers' syncable state height on state reactor start,
	After a certain timeout, this node will request state keys evenly from different peers that has same
	syncable state.

	On received all expected chunk, recover state related ABCI will be called to save received chunks into state and
	application db. Then state reactor will switch to block reactor to fast sync.
*/

type StatePool struct {
	cmn.BaseService

	mtx sync.Mutex
	height    int64 // the height in first state status response we received
	numKeys []int64	// numKeys we are expected, in app defined sub store order
	totalKeys int64	// sum of numKeys
	numKeysReceived int64	// numKeys we have received, no need to be atomic, guarded by pool.mtx
	step int64		// how many keys this node should request from peers
	state *sm.State	// tendermint state
	chunks map[int64][][]byte	// startIdx -> [key, value] map
	requests map[p2p.ID]*StateRequest	// requests is used to verify whether we received out-of-date chunk.
										// i.e. in first round we request 3 peers but 1 of which is slow caused us timeout
										// in second round, we request 5 peers (each with less expected chunks) as we discover more peers
										// but in this 2nd round, the 1st round missing chunk comes back
										// we need discard it if that's not the chunk we are expecting from that peer

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
	return nil
}

func (pool *StatePool) OnStop() {}

func (pool *StatePool) AddStateChunk(peerID p2p.ID, msg *bcStateResponseMessage) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()

	pool.Logger.Info("peer sent us a start index", "peer", peerID, "startIndex", msg.StartIdxInc)


	if request, ok := pool.requests[peerID]; ok && request.StartIndex == msg.StartIdxInc && request.EndIndex == msg.EndIdxExc {
		pool.chunks[msg.StartIdxInc] = msg.Chunks

		atomic.AddInt32(&pool.numPending, -1)
		atomic.AddInt64(&pool.numKeysReceived, msg.EndIdxExc - msg.StartIdxInc)
		peer := pool.peers[peerID]
		if peer != nil {
			peer.decrPending()
		}
		delete(pool.requests, peerID)
	} else if ok {
		pool.Logger.Error("peer send us an unexpected index", "peer", peerID, "expected", request.StartIndex)
	} else {
		pool.Logger.Error("peer send us an unexpected index", "peer", peerID)
	}
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

// TODO: enhance, we might can retry
func (pool *StatePool) removePeer(peerID p2p.ID) {
	delete(pool.requests, peerID)
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

	pool.mtx.Lock()
	defer pool.mtx.Unlock()

	for idx, peer := range peers {
		// Send request and wait.
		endIndex := (int64(idx) + 1) * pool.step
		if endIndex > pool.totalKeys {
			endIndex = pool.totalKeys
		}
		stateReq := StateRequest{pool.height, peer.id, int64(idx) * pool.step, endIndex}
		pool.requestsCh <- stateReq
		pool.requests[peer.id] = &stateReq
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

	pool.Logger.Info("Completeness check", "numPending", pool.numPending, "numKeysReceived", pool.numKeysReceived)
	return pool.numKeysReceived == pool.totalKeys
}

func (pool *StatePool) init(msg *bcStateStatusResponseMessage) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()


	pool.height = msg.Height
	pool.numKeys = msg.NumKeys
	pool.totalKeys = 0
	for _, numKey := range pool.numKeys {
		pool.totalKeys += numKey
	}

	pool.Logger.Info("init state pool", "height", msg.Height, "totalKeys", pool.totalKeys)
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
	peer.recvMonitor = flow.New(time.Second, time.Second * types.MonitorWindowInSeconds)
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
