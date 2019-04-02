package snapshot

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/golang/snappy"
	"math"
	"sync"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	cmn "github.com/tendermint/tendermint/libs/common"
	dbm "github.com/tendermint/tendermint/libs/db"
	flow "github.com/tendermint/tendermint/libs/flowrate"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

/*
	State pool is used to hold state chunks this peer requested from peers.
	Peer will request connected peers' syncable state height on state reactor start,
	After a certain timeout, this node will request state keys from different peers that has same
	snapshot.

	On received all expected chunk, recover state related ABCI will be called to save received chunks into state and
	application db. Then state reactor will switch to block reactor to fast sync.
*/

const (
	requestIntervalMS = 2
	minRecvRate       = 7680
)

// TODO: need place in config so that user can tweak it as chunk grows
var peerTimeout = 150 * time.Second

type StatePool struct {
	cmn.BaseService

	mtx          sync.Mutex
	manifest     *abci.Manifest
	manifestHash abci.SHA256Sum
	dummyHash abci.SHA256Sum
	state        *sm.State // tendermint state
	block 		 *types.Block
	seenCommit 	 *types.Commit
	stateDB      dbm.DB
	app       	 proxy.AppConnState

	requesters sync.Map // map[abci.SHA256Sum]*spRequester

	receivedManifests         map[abci.SHA256Sum]*abci.Manifest
	pendingScheduledHashes    chan abci.SHA256Sum // accessed only by makeRequestersRoutine
	triedManifestFinalization int                 // how many times we have tried finalize manifest received

	// peers
	respondedPeers map[abci.SHA256Sum][]p2p.ID // peers that response manifest to us, used to determine whom we will trust and request snapshot chunk from
	trustedPeers   map[p2p.ID]*spPeer          // peers we trusted and want sync from

	// number of pending request snapshots after pool init
	numPending int

	requestsCh chan<- SnapshotRequest
	errorsCh   chan<- peerError
}

func NewStatePool(app proxy.AppConnState, requestsCh chan<- SnapshotRequest, errorsCh chan<- peerError, stateDB dbm.DB) *StatePool {
	sp := &StatePool{
		app: 					app,
		trustedPeers:           make(map[p2p.ID]*spPeer),
		receivedManifests:      make(map[abci.SHA256Sum]*abci.Manifest),
		pendingScheduledHashes: make(chan abci.SHA256Sum, 100),
		respondedPeers:         make(map[abci.SHA256Sum][]p2p.ID),

		requestsCh: requestsCh,
		errorsCh:   errorsCh,
		stateDB:    stateDB,
	}
	const dummyManifestHash = "dummyhash"
	copy(sp.dummyHash[:], dummyManifestHash)
	sp.BaseService = *cmn.NewBaseService(nil, "StatePool", sp)
	return sp
}

func (pool *StatePool) OnStart() error {
	go pool.makeRequestersRoutine()
	return nil
}

func (pool *StatePool) OnStop() {}

// Run spawns requesters as needed.
func (pool *StatePool) makeRequestersRoutine() {
	for {
		select {
		case <-pool.Quit():
			break
		case hash := <-pool.pendingScheduledHashes:
			requester := newSPRequester(pool, hash)
			if err := requester.Start(); err != nil {
				// We missed a way to re-create requester on err for hash here,
				// but as spRequester's start is trivial it should not fail
				requester.Logger.Error("Failed to starting requester, the pending hash would miss and suggest restart process again!", "err", err)
			} else {
				pool.requesters.Store(hash, requester)
			}
		}
	}
}

// Sets the peer's alleged blockchain height.
// Must be called guarded by pool.mtx
func (pool *StatePool) addPeerGuarded(peerID p2p.ID, hash abci.SHA256Sum) {
	if peer := pool.trustedPeers[peerID]; peer != nil {
		pool.Logger.Info("update peer manifest hash from trustedPeers", "peer", peerID)
		peer.manifestHash = hash
	} else {
		pool.Logger.Info("added peer to trusted peers", "peer", peerID)
		peer = newSPPeer(pool, peerID, hash)
		peer.setLogger(pool.Logger.With("peer", peerID))
		pool.trustedPeers[peerID] = peer
	}
}

func (pool *StatePool) removePeer(peerID p2p.ID) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()

	pool.removePeerGuarded(peerID)
}

func (pool *StatePool) removePeerGuarded(peerID p2p.ID) {
	pool.requesters.Range(func(key, value interface{}) bool {
		requester := value.(*spRequester)

		if requester.getPeerID() == peerID {
			requester.redo(peerID)
		}

		return true
	})
	delete(pool.trustedPeers, peerID)
}

// Pick an available peer to fetch snapshot chunk
// If no peers are available, returns nil.
func (pool *StatePool) pickIncrAvailablePeer() (peer *spPeer) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()

	for _, peer := range pool.trustedPeers {
		if peer.didTimeout {
			pool.removePeerGuarded(peer.id)
			continue
		}
		if peer.numPending >= maxInflightRequestPerPeer {
			continue
		}
		peer.incrPending()
		return peer
	}
	return nil
}

func (pool *StatePool) sendRequest(hash abci.SHA256Sum, peerId p2p.ID) {
	if !pool.IsRunning() {
		return
	}
	stateReq := SnapshotRequest{pool.manifest.Height, hash, peerId}
	pool.requestsCh <- stateReq
}

func (pool *StatePool) sendError(err error, peerID p2p.ID) {
	if !pool.IsRunning() {
		return
	}
	pool.errorsCh <- peerError{err, peerID}
}

func (pool *StatePool) addManifest(peerId p2p.ID, msg *bcManifestResponseMessage) error {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()

	if msg.Height == 0 {
		pool.receivedManifests[pool.dummyHash] = nil
		pool.respondedPeers[pool.dummyHash] = append(pool.respondedPeers[pool.dummyHash], peerId)
	} else {
		hash, manifest, err := pool.validateAndUnmarshalManifest(msg)
		if err != nil {
			return err
		}
		if _, ok := pool.receivedManifests[hash]; !ok {
			pool.receivedManifests[hash] = manifest
		}
		pool.respondedPeers[hash] = append(pool.respondedPeers[hash], peerId)

		// if we already finalized manifest and this is a "trusted" peer, we add it to peer list
		if pool.manifestHash == hash {
			pool.addPeerGuarded(peerId, hash)
		}
		pool.Logger.Info("added manifest", "peer", peerId, "manifestHash", fmt.Sprintf("%x", hash))
	}

	return nil
}

func (pool *StatePool) validateAndUnmarshalManifest(msg *bcManifestResponseMessage) (abci.SHA256Sum, *abci.Manifest, error) {
	hash := sha256.Sum256(msg.ManifestPayload)
	if decompressed, err := snappy.Decode(nil, msg.ManifestPayload); err != nil {
		return hash, nil, err
	} else {
		var manifest abci.Manifest
		if err := cdc.UnmarshalBinaryBare(decompressed, &manifest); err == nil {
			if manifest.Version == abci.ManifestVersion {
				return hash, &manifest, nil
			} else {
				return hash, nil, fmt.Errorf("snapshot manifest version mismatch, expected: %d, actual: %d", abci.ManifestVersion, manifest.Version)
			}
		} else {
			return hash, nil, nil
		}
	}
}

func (pool *StatePool) initGuarded(hash abci.SHA256Sum, manifest *abci.Manifest, peers []p2p.ID) error {
	if hash == pool.dummyHash && manifest == nil {
		pool.manifestHash = hash
		return nil
	}

	if err := Manager().WriteManifest(hash, manifest); err != nil {
		return err
	}

	if err := pool.app.StartRecovery(Manager().RestorationManifest); err != nil {
		pool.Logger.Error("failed to start snapshot recovery", "err", err)
		return err
	}

	for _, peer := range peers {
		pool.addPeerGuarded(peer, hash)
	}

	existChunks := make(map[abci.SHA256Sum]abci.SnapshotChunk)

	for _, hash := range manifest.StateHashes {
		pool.loadFromDiskOrSchedule(hash, existChunks)
	}
	for _, hash := range manifest.AppStateHashes {
		pool.loadFromDiskOrSchedule(hash, existChunks)
	}
	for _, hash := range manifest.BlockHashes {
		pool.loadFromDiskOrSchedule(hash, existChunks)
	}

	// not doing this within loadFromDiskOrSchedule because we need maintain correct numPending
	// for example, if last state sync stopped on receive or chunks just not finalize them,
	// we can still signal app correct isComplete flag. Otherwise, every chunk looks like last one
	for hash, chunk := range existChunks {
		pool.numPending--
		pool.processChunkImpl(hash, chunk)
	}

	pool.manifest = manifest
	pool.manifestHash = hash
	pool.Logger.Info("finish init state pool", "height", manifest.Height, "manifestHash", fmt.Sprintf("%x", hash), "numOfPeers", len(peers), "numPending", pool.numPending)

	return nil
}

func (pool *StatePool) loadFromDiskOrSchedule(hash abci.SHA256Sum, existChunks map[abci.SHA256Sum]abci.SnapshotChunk) {
	if compressed, err := Manager().Reader.LoadFromRestoration(hash); err == nil {
		if decompressed, err := snappy.Decode(nil, compressed); err == nil {
			var snapshotChunk abci.SnapshotChunk
			if err := cdc.UnmarshalBinaryBare(decompressed, &snapshotChunk); err == nil {
				existChunks[hash] = snapshotChunk
			} else {
				pool.Logger.Error("error unmarshal a chunk from disk", "hash", fmt.Sprintf("%x", hash), "err", err)
			}
		} else {
			pool.Logger.Error("error decode a chunk from disk", "hash", fmt.Sprintf("%x", hash), "err", err)
		}
	} else {
		pool.pendingScheduledHashes <- hash
	}
	pool.numPending++
}

func (pool *StatePool) tryInit() bool {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()

	if pool.isInitedGuarded() {
		return true
	}

	var majorityManifest *abci.Manifest
	var majorityManifestHash abci.SHA256Sum
	var hasSameNumOfPeers bool
	var numOfPeers int

	pool.triedManifestFinalization += 1

	for hash, peers := range pool.respondedPeers {
		if len(peers) > numOfPeers {
			hasSameNumOfPeers = false
			numOfPeers = len(peers)
			majorityManifest = pool.receivedManifests[hash]
			majorityManifestHash = hash
		} else if len(peers) == numOfPeers {
			hasSameNumOfPeers = true
		}
	}

	if numOfPeers > leastPeersToSync {
		pool.Logger.Info("got enough peers to start state sync", "manifestHash", majorityManifestHash, "numOfPeers", numOfPeers)
		pool.initGuarded(majorityManifestHash, majorityManifest, pool.respondedPeers[majorityManifestHash])
		return true
	}
	if pool.triedManifestFinalization >= maxTriedManifestFinalist && numOfPeers > 1 && !hasSameNumOfPeers {
		pool.Logger.Info("did not collect enough manifest, but will start anyway", "manifestHash", fmt.Sprintf("%x", majorityManifestHash), "numOfPeers", numOfPeers)
		pool.initGuarded(majorityManifestHash, majorityManifest, pool.respondedPeers[majorityManifestHash])
		return true
	}

	pool.Logger.Info("decided not init pool", "peers", len(pool.respondedPeers), "tried", pool.triedManifestFinalization)
	for hash, peers := range pool.respondedPeers {
		pool.Logger.Info(fmt.Sprintf("hash: %x, peers: %v", hash, peers))
	}
	return false
}

func (pool *StatePool) processChunk(peerID p2p.ID, msg *bcChunkResponseMessage, snapshotChunk abci.SnapshotChunk) (err error) {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()

	pool.Logger.Info("processing chunk", "peer", peerID, "resp", msg, "numPending", pool.numPending)

	peer := pool.trustedPeers[peerID]
	if peer != nil {
		peer.decrPending(len(msg.Chunk))
	}

	req, ok := pool.requesters.Load(msg.Hash)
	if !ok || req == nil {
		pool.Logger.Error("peer sent us a chunk we didn't expect", "peer", peerID, "resp", msg, "height", pool.manifest.Height)
		err = errors.New("peer sent us a chunk we didn't expect")
		pool.sendError(err, peerID)
		return
	}

	requester := req.(*spRequester)
	if requester.setGotChunk(peerID) {
		if err = Manager().Writer.Write(msg.Hash, msg.Chunk); err != nil {
			return err
		}
		pool.numPending -= 1
		requester.Stop()
		pool.requesters.Delete(msg.Hash)
	} else {
		pool.Logger.Info("outdated peer", "peer", peerID, "msg", msg)
	}

	err = pool.processChunkImpl(msg.Hash, snapshotChunk)

	if pool.numPending == 0 {
		pool.Logger.Info("received all expected chunks, going to finalize snapshot")
		if err = Manager().Finalize(); err != nil {
			return err
		}
	}
	return err
}

// TODO: this function may block receive goroutine which makes peer timeout
func (pool *StatePool) processChunkImpl(hash abci.SHA256Sum, snapshotChunk abci.SnapshotChunk) (err error){
	numPending := pool.numPending

	switch chunk := snapshotChunk.(type) {
	case *abci.BlockChunk:
		var block types.Block
		var seenCommit types.Commit
		if err = cdc.UnmarshalBinaryBare(chunk.Block, &block); err == nil {
			if err = cdc.UnmarshalBinaryBare(chunk.SeenCommit, &seenCommit); err == nil {
				pool.block = &block
				pool.seenCommit = &seenCommit
				Manager().restoreBlock(pool.block, pool.seenCommit)
				// need signal app level that tendermint collected all expected snapshot chunks
				// TODO: discuss with community whether pass all kind of chunks to app level (and only process app ones there)
				if numPending == 0 {
					err = pool.app.WriteRecoveryChunk(hash, nil, true)
				}
			}
		}
	case *abci.StateChunk:
		var state sm.State
		if err = cdc.UnmarshalBinaryBare(chunk.Statepart, &state); err == nil {
			pool.state = &state
			sm.SaveState(pool.stateDB, *pool.state)
			// need signal app level that tendermint collected all expected snapshot chunks
			// TODO: discuss with community whether pass all kind of chunks to app level (and only process app ones there)
			if numPending == 0 {
				err = pool.app.WriteRecoveryChunk(hash, nil, true)
			}
		}
	case *abci.AppStateChunk:
		return pool.app.WriteRecoveryChunk(hash, chunk, numPending == 0)
	}
	return err
}

func (pool *StatePool) isInitedGuarded() bool {
	return pool.manifest != nil
}

func (pool *StatePool) isComplete() bool {
	pool.mtx.Lock()
	defer pool.mtx.Unlock()

	return pool.isInitedGuarded() && pool.numPending == 0
}

//-------------------------------------

type spPeer struct {
	pool        *StatePool
	id          p2p.ID
	recvMonitor *flow.Monitor

	manifestHash abci.SHA256Sum
	numPending   int32
	timeout      *time.Timer
	didTimeout   bool

	logger log.Logger
}

func newSPPeer(pool *StatePool, peerID p2p.ID, hash abci.SHA256Sum) *spPeer {
	peer := &spPeer{
		pool:         pool,
		id:           peerID,
		manifestHash: hash,
		numPending:   0,
		logger:       log.NewNopLogger(),
	}
	return peer
}

func (peer *spPeer) setLogger(l log.Logger) {
	peer.logger = l
}

func (peer *spPeer) resetMonitor() {
	peer.recvMonitor = flow.New(time.Second, time.Second*types.MonitorWindowInSeconds)
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

func (peer *spPeer) decrPending(recvSize int) {
	peer.numPending--
	if peer.numPending == 0 {
		peer.timeout.Stop()
	} else {
		peer.recvMonitor.Update(recvSize)
		peer.resetTimeout()
	}
}

func (peer *spPeer) onTimeout() {
	peer.pool.mtx.Lock()
	defer peer.pool.mtx.Unlock()

	err := errors.New("state sync peer did not send us anything")
	peer.pool.sendError(err, peer.id)
	peer.logger.Error("SendTimeout", "reason", err, "timeout", peerTimeout)
	peer.didTimeout = true
}

// snapshot requester maintain fetching life span for one chunk
// it should responsible for select peer to send request and error handling and retry
type spRequester struct {
	cmn.BaseService
	pool     *StatePool
	hash     abci.SHA256Sum
	gotChunk bool
	redoCh   chan p2p.ID

	mtx    sync.Mutex
	peerID p2p.ID
}

func newSPRequester(pool *StatePool, hash abci.SHA256Sum) *spRequester {
	spr := &spRequester{
		pool:   pool,
		hash:   hash,
		redoCh: make(chan p2p.ID, 1),

		peerID: "",
	}
	spr.BaseService = *cmn.NewBaseService(nil, "spRequester", spr)
	return spr
}

// NOTE: should not fail (return error), request routine won't retry on requester start failure
func (spr *spRequester) OnStart() error {
	go spr.requestRoutine()
	return nil
}

func (spr *spRequester) getPeerID() p2p.ID {
	spr.mtx.Lock()
	defer spr.mtx.Unlock()
	return spr.peerID
}

// Returns true if the peer matches and chunk doesn't already exist.
func (spr *spRequester) setGotChunk(peerID p2p.ID) bool {
	spr.mtx.Lock()
	defer spr.mtx.Unlock()

	if spr.gotChunk || spr.peerID != peerID {
		return false
	}
	spr.gotChunk = true
	return true
}

// This is called from the requestRoutine, upon redo().
func (spr *spRequester) reset() {
	spr.mtx.Lock()
	defer spr.mtx.Unlock()

	spr.peerID = ""
	spr.gotChunk = false
}

// Tells spRequester to pick another peer and try again.
// NOTE: Nonblocking, and does nothing if another redo
// was already requested.
func (spr *spRequester) redo(peerId p2p.ID) {
	select {
	case spr.redoCh <- peerId:
	default:
	}
}

// Responsible for making more requests as necessary
// Returns only when a block is found (e.g. AddBlock() is called)
func (spr *spRequester) requestRoutine() {
OUTER_LOOP:
	for {
		// Pick a peer to send request to.
		var peer *spPeer
	PICK_PEER_LOOP:
		for {
			if !spr.IsRunning() || !spr.pool.IsRunning() {
				return
			}
			peer = spr.pool.pickIncrAvailablePeer()
			if peer == nil {
				time.Sleep(requestIntervalMS * time.Millisecond)
				continue PICK_PEER_LOOP
			}
			break PICK_PEER_LOOP
		}
		spr.mtx.Lock()
		spr.peerID = peer.id
		spr.mtx.Unlock()

		// Send request and wait.
		spr.pool.sendRequest(spr.hash, peer.id)
	WAIT_LOOP:
		for {
			select {
			case <-spr.pool.Quit():
				spr.Stop()
				return
			case <-spr.Quit():
				return
			case peerID := <-spr.redoCh:
				if peerID == spr.peerID {
					spr.reset()
					continue OUTER_LOOP
				} else {
					continue WAIT_LOOP
				}
			}
		}
	}
}

type SnapshotRequest struct {
	Height int64
	Hash   abci.SHA256Sum
	PeerID p2p.ID
}

func(req SnapshotRequest) String() string {
	return fmt.Sprintf("%d, %x", req.Height, req.Hash)
}
