package v2

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

// Events

// XXX: The handle API would be much simpler if it return a single event, an
// Event, which embeds a terminationEvent if it wants to terminate the routine.

// Input events into the scheduler:
// ticker event for cleaning peers
type tryPrunePeer struct {
	priorityHigh
	time time.Time
}

// ticker event for scheduling block requests
type trySchedule struct {
	priorityHigh
	time time.Time
}

// blockResponse message received from a peer
type bcBlockResponse struct {
	priorityNormal
	time   time.Time
	peerID p2p.ID
	height int64
	size   int64
	block  *types.Block
}

// statusResponse message received from a peer
type bcStatusResponse struct {
	priorityNormal
	time   time.Time
	peerID p2p.ID
	height int64
}

// new peer is connected
type addNewPeer struct {
	priorityNormal
	peerID p2p.ID
}

// Output events issued by the scheduler:
// all blocks have been processed
type scFinishedEv struct {
	priorityNormal
}

// send a blockRequest message
type scBlockRequest struct {
	priorityNormal
	peerID p2p.ID
	height int64
}

// a block has been received and validated by the scheduler
type scBlockReceived struct {
	priorityNormal
	peerID p2p.ID
	block  *types.Block
}

// scheduler detected a peer error
type scPeerError struct {
	priorityHigh
	peerID p2p.ID
	reason error
}

// scheduler removed a set of peers (timed out or slow peer)
type scPeersPruned struct {
	priorityHigh
	peers []p2p.ID
}

// XXX: make this fatal?
// scheduler encountered a fatal error
type scSchedulerFail struct {
	priorityHigh
	reason error
}

type blockState int

const (
	blockStateUnknown   blockState = iota + 1 // no known peer has this block
	blockStateNew                             // indicates that a peer has reported having this block
	blockStatePending                         // indicates that this block has been requested from a peer
	blockStateReceived                        // indicates that this block has been received by a peer
	blockStateProcessed                       // indicates that this block has been applied
)

func (e blockState) String() string {
	switch e {
	case blockStateUnknown:
		return "Unknown"
	case blockStateNew:
		return "New"
	case blockStatePending:
		return "Pending"
	case blockStateReceived:
		return "Received"
	case blockStateProcessed:
		return "Processed"
	default:
		return fmt.Sprintf("invalid blockState: %d", e)
	}
}

type peerState int

const (
	peerStateNew = iota + 1
	peerStateReady
	peerStateRemoved
)

func (e peerState) String() string {
	switch e {
	case peerStateNew:
		return "New"
	case peerStateReady:
		return "Ready"
	case peerStateRemoved:
		return "Removed"
	default:
		panic(fmt.Sprintf("unknown peerState: %d", e))
	}
}

type scPeer struct {
	peerID p2p.ID

	// initialized as New when peer is added, updated to Ready when statusUpdate is received,
	// updated to Removed when peer is removed
	state peerState

	height      int64 // updated when statusResponse is received
	lastTouched time.Time
	lastRate    int64 // last receive rate in bytes
}

func (p scPeer) String() string {
	return fmt.Sprintf("{state %v, height %d, lastTouched %v, lastRate %d, id %v}",
		p.state, p.height, p.lastTouched, p.lastRate, p.peerID)
}

func newScPeer(peerID p2p.ID) *scPeer {
	return &scPeer{
		peerID: peerID,
		state:  peerStateNew,
		height: -1,
	}
}

// The scheduler keep track of the state of each block and each peer. The
// scheduler will attempt to schedule new block requests with `trySchedule`
// events and remove slow peers with `tryPrune` events.
type scheduler struct {
	initHeight int64

	// next block that needs to be processed. All blocks with smaller height are
	// in Processed state.
	height int64

	// a map of peerID to scheduler specific peer struct `scPeer` used to keep
	// track of peer specific state
	peers       map[p2p.ID]*scPeer
	peerTimeout time.Duration
	minRecvRate int64 // minimum receive rate from peer otherwise prune

	// the maximum number of blocks that should be New, Received or Pending at any point
	// in time. This is used to enforce a limit on the blockStates map.
	targetPending int
	// a list of blocks to be scheduled (New), Pending or Received. Its length should be
	// smaller than targetPending.
	blockStates map[int64]blockState

	// a map of heights to the peer we are waiting a response from
	pendingBlocks map[int64]p2p.ID

	// the time at which a block was put in blockStatePending
	pendingTime map[int64]time.Time

	// a map of heights to the peers that put the block in blockStateReceived
	receivedBlocks map[int64]p2p.ID
}

func (sc scheduler) String() string {
	return fmt.Sprintf("ih: %d, bst: %v, peers: %v, pblks: %v, ptm %v, rblks: %v",
		sc.initHeight, sc.blockStates, sc.peers, sc.pendingBlocks, sc.pendingTime, sc.receivedBlocks)
}

func newScheduler(initHeight int64) *scheduler {
	sc := scheduler{
		initHeight:     initHeight,
		height:         initHeight + 1,
		blockStates:    make(map[int64]blockState),
		peers:          make(map[p2p.ID]*scPeer),
		pendingBlocks:  make(map[int64]p2p.ID),
		pendingTime:    make(map[int64]time.Time),
		receivedBlocks: make(map[int64]p2p.ID),
	}

	return &sc
}

func (sc *scheduler) addPeer(peerID p2p.ID) error {
	if _, ok := sc.peers[peerID]; ok {
		// In the future we should be able to add a previously removed peer
		return fmt.Errorf("cannot add duplicate peer %s", peerID)
	}
	sc.peers[peerID] = newScPeer(peerID)
	return nil
}

func (sc *scheduler) touchPeer(peerID p2p.ID, time time.Time) error {
	peer, ok := sc.peers[peerID]
	if !ok {
		return fmt.Errorf("couldn't find peer %s", peerID)
	}

	if peer.state != peerStateReady {
		return fmt.Errorf("tried to touch peer in state %s, must be Ready", peer.state)
	}

	peer.lastTouched = time

	return nil
}

func (sc *scheduler) removePeer(peerID p2p.ID) error {
	peer, ok := sc.peers[peerID]
	if !ok {
		return fmt.Errorf("couldn't find peer %s", peerID)
	}

	if peer.state == peerStateRemoved {
		return fmt.Errorf("tried to remove peer %s in peerStateRemoved", peerID)
	}

	for height, pendingPeerID := range sc.pendingBlocks {
		if pendingPeerID == peerID {
			sc.setStateAtHeight(height, blockStateNew)
			delete(sc.pendingTime, height)
			delete(sc.pendingBlocks, height)
		}
	}

	for height, rcvPeerID := range sc.receivedBlocks {
		if rcvPeerID == peerID {
			sc.setStateAtHeight(height, blockStateNew)
			delete(sc.receivedBlocks, height)
		}
	}

	// remove the blocks from blockStates if the peer removal causes the max peer height to be lower.
	peer.state = peerStateRemoved
	maxPeerHeight := int64(0)
	for _, otherPeer := range sc.peers {
		if otherPeer.state != peerStateReady {
			continue
		}
		if otherPeer.peerID != peer.peerID && otherPeer.height > maxPeerHeight {
			maxPeerHeight = otherPeer.height
		}
	}
	for h := range sc.blockStates {
		if h > maxPeerHeight {
			delete(sc.blockStates, h)
		}
	}

	return nil
}

// check if the blockPool is running low and add new blocks in New state to be requested.
// This function is called when there is an increase in the maximum peer height or when
// blocks are processed.
func (sc *scheduler) addNewBlocks() {
	if len(sc.blockStates) >= sc.targetPending {
		return
	}

	for i := sc.height; i < int64(sc.targetPending)+sc.height; i++ {
		if i > sc.maxHeight() {
			break
		}
		if sc.getStateAtHeight(i) == blockStateUnknown {
			sc.setStateAtHeight(i, blockStateNew)
		}
	}
}

func (sc *scheduler) setPeerHeight(peerID p2p.ID, height int64) error {
	peer, ok := sc.peers[peerID]
	if !ok {
		return fmt.Errorf("cannot find peer %s", peerID)
	}

	if peer.state == peerStateRemoved {
		return fmt.Errorf("cannot set peer height for a peer in peerStateRemoved")
	}

	if height < peer.height {
		return fmt.Errorf("cannot move peer height lower. from %d to %d", peer.height, height)
	}

	peer.height = height
	peer.state = peerStateReady

	sc.addNewBlocks()
	return nil
}

func (sc *scheduler) getStateAtHeight(height int64) blockState {
	if height <= sc.initHeight {
		return blockStateProcessed
	} else if state, ok := sc.blockStates[height]; ok {
		return state
	} else {
		return blockStateUnknown
	}
}

func (sc *scheduler) getPeersAtHeightOrAbove(height int64) []p2p.ID {
	peers := make([]p2p.ID, 0)
	for _, peer := range sc.peers {
		if peer.state != peerStateReady {
			continue
		}
		if peer.height >= height {
			peers = append(peers, peer.peerID)
		}
	}
	return peers
}

func (sc *scheduler) peersInactiveSince(duration time.Duration, now time.Time) []p2p.ID {
	peers := []p2p.ID{}
	for _, peer := range sc.peers {
		if peer.state != peerStateReady {
			continue
		}
		if now.Sub(peer.lastTouched) > duration {
			peers = append(peers, peer.peerID)
		}
	}

	// Ensure the order is deterministic for testing
	sort.Sort(PeerByID(peers))
	return peers
}

// will return peers who's lastRate i slower than minSpeed denominated in bytes
func (sc *scheduler) peersSlowerThan(minSpeed int64) []p2p.ID {
	peers := []p2p.ID{}
	for peerID, peer := range sc.peers {
		if peer.state != peerStateReady {
			continue
		}
		if peer.lastRate < minSpeed {
			peers = append(peers, peerID)
		}
	}

	// Ensure the order is deterministic for testing
	sort.Sort(PeerByID(peers))
	return peers
}

func (sc *scheduler) prunablePeers(peerTimout time.Duration, minRecvRate int64, now time.Time) []p2p.ID {
	prunable := []p2p.ID{}
	for peerID, peer := range sc.peers {
		if peer.state != peerStateReady {
			continue
		}
		if now.Sub(peer.lastTouched) > peerTimout || peer.lastRate < minRecvRate {
			prunable = append(prunable, peerID)
		}
	}
	// Tests for handleTryPrunePeer() may fail without sort due to range non-determinism
	sort.Sort(PeerByID(prunable))
	return prunable
}

func (sc *scheduler) setStateAtHeight(height int64, state blockState) {
	sc.blockStates[height] = state
}

func (sc *scheduler) markReceived(peerID p2p.ID, height int64, size int64, now time.Time) error {
	peer, ok := sc.peers[peerID]
	if !ok {
		return fmt.Errorf("couldn't find peer %s", peerID)
	}

	if peer.state == peerStateRemoved {
		return fmt.Errorf("cannot receive blocks from removed peer %s", peerID)
	}

	if state := sc.getStateAtHeight(height); state != blockStatePending || sc.pendingBlocks[height] != peerID {
		return fmt.Errorf("received block %d from peer %s without being requested", height, peerID)
	}

	pendingTime, ok := sc.pendingTime[height]
	if !ok || now.Sub(pendingTime) <= 0 {
		return fmt.Errorf("clock error: block %d received at %s but requested at %s",
			height, pendingTime, now)
	}

	peer.lastRate = size / now.Sub(pendingTime).Nanoseconds()

	sc.setStateAtHeight(height, blockStateReceived)
	delete(sc.pendingBlocks, height)
	delete(sc.pendingTime, height)

	sc.receivedBlocks[height] = peerID

	return nil
}

func (sc *scheduler) markPending(peerID p2p.ID, height int64, time time.Time) error {
	state := sc.getStateAtHeight(height)
	if state != blockStateNew {
		return fmt.Errorf("block %d should be in blockStateNew but is %s", height, state)
	}

	peer, ok := sc.peers[peerID]
	if !ok {
		return fmt.Errorf("cannot find peer %s", peerID)
	}

	if peer.state != peerStateReady {
		return fmt.Errorf("cannot schedule %d from %s in %s", height, peerID, peer.state)
	}

	if height > peer.height {
		return fmt.Errorf("cannot request height %d from peer %s that is at height %d",
			height, peerID, peer.height)
	}

	sc.setStateAtHeight(height, blockStatePending)
	sc.pendingBlocks[height] = peerID
	// XXX: to make this more accurate we can introduce a message from
	// the IO routine which indicates the time the request was put on the wire
	sc.pendingTime[height] = time

	return nil
}

func (sc *scheduler) markProcessed(height int64) error {
	state := sc.getStateAtHeight(height)
	if state != blockStateReceived {
		return fmt.Errorf("cannot mark height %d received from block state %s", height, state)
	}

	sc.height++
	delete(sc.receivedBlocks, height)
	delete(sc.blockStates, height)
	sc.addNewBlocks()

	return nil
}

func (sc *scheduler) allBlocksProcessed() bool {
	return sc.height >= sc.maxHeight()
}

// returns max peer height or the last processed block, i.e. sc.height
func (sc *scheduler) maxHeight() int64 {
	max := sc.height - 1
	for _, peer := range sc.peers {
		if peer.state != peerStateReady {
			continue
		}
		if peer.height > max {
			max = peer.height
		}
	}
	return max
}

// lowest block in sc.blockStates with state == blockStateNew or -1 if no new blocks
func (sc *scheduler) nextHeightToSchedule() int64 {
	var min int64 = math.MaxInt64
	for height, state := range sc.blockStates {
		if state == blockStateNew && height < min {
			min = height
		}
	}
	if min == math.MaxInt64 {
		min = -1
	}
	return min
}

func (sc *scheduler) pendingFrom(peerID p2p.ID) []int64 {
	var heights []int64
	for height, pendingPeerID := range sc.pendingBlocks {
		if pendingPeerID == peerID {
			heights = append(heights, height)
		}
	}
	return heights
}

func (sc *scheduler) selectPeer(height int64) (p2p.ID, error) {
	peers := sc.getPeersAtHeightOrAbove(height)
	if len(peers) == 0 {
		return "", fmt.Errorf("cannot find peer for height %d", height)
	}

	// create a map from number of pending requests to a list
	// of peers having that number of pending requests.
	pendingFrom := make(map[int][]p2p.ID)
	for _, peerID := range peers {
		numPending := len(sc.pendingFrom(peerID))
		pendingFrom[numPending] = append(pendingFrom[numPending], peerID)
	}

	// find the set of peers with minimum number of pending requests.
	minPending := math.MaxInt64
	for mp := range pendingFrom {
		if mp < minPending {
			minPending = mp
		}
	}

	sort.Sort(PeerByID(pendingFrom[minPending]))
	return pendingFrom[minPending][0], nil
}

// PeerByID is a list of peers sorted by peerID.
type PeerByID []p2p.ID

func (peers PeerByID) Len() int {
	return len(peers)
}
func (peers PeerByID) Less(i, j int) bool {
	return bytes.Compare([]byte(peers[i]), []byte(peers[j])) == -1
}

func (peers PeerByID) Swap(i, j int) {
	it := peers[i]
	peers[i] = peers[j]
	peers[j] = it
}

// Handlers

// This handler gets the block, performs some validation and then passes it on to the processor.
func (sc *scheduler) handleBlockResponse(event bcBlockResponse) (Event, error) {
	err := sc.touchPeer(event.peerID, event.time)
	if err != nil {
		return scPeerError{peerID: event.peerID, reason: err}, nil
	}

	err = sc.markReceived(event.peerID, event.block.Height, event.size, event.time)
	if err != nil {
		return scPeerError{peerID: event.peerID, reason: err}, nil
	}

	return scBlockReceived{peerID: event.peerID, block: event.block}, nil
}

func (sc *scheduler) handleBlockProcessed(event pcBlockProcessed) (Event, error) {
	if event.height != sc.height {
		panic(fmt.Sprintf("processed height %d but expected height %d", event.height, sc.height))
	}
	err := sc.markProcessed(event.height)
	if err != nil {
		// It is possible that a peer error or timeout is handled after the processor
		// has processed the block but before the scheduler received this event,
		// so when pcBlockProcessed event is received the block had been requested again
		return scSchedulerFail{reason: err}, nil
	}

	if sc.allBlocksProcessed() {
		return scFinishedEv{}, nil
	}

	return noOp, nil
}

// Handles an error from the processor. The processor had already cleaned the blocks from
// the peers included in this event. Just attempt to remove the peers.
func (sc *scheduler) handleBlockProcessError(event pcBlockVerificationFailure) (Event, error) {
	if len(sc.peers) == 0 {
		return noOp, nil
	}
	// The peers may have been just removed due to errors, low speed or timeouts.
	_ = sc.removePeer(event.firstPeerID)
	if event.firstPeerID != event.secondPeerID {
		_ = sc.removePeer(event.secondPeerID)
	}

	if sc.allBlocksProcessed() {
		return scFinishedEv{}, nil
	}

	return noOp, nil
}

func (sc *scheduler) handleAddNewPeer(event addNewPeer) (Event, error) {
	err := sc.addPeer(event.peerID)
	if err != nil {
		return scSchedulerFail{reason: err}, nil
	}
	return noOp, nil
}

// XXX: unify types peerError
func (sc *scheduler) handlePeerError(event peerError) (Event, error) {
	err := sc.removePeer(event.peerID)
	if err != nil {
		// XXX - It is possible that the removePeer fails here for legitimate reasons
		// for example if a peer timeout or error was handled just before this.
		return scSchedulerFail{reason: err}, nil
	}
	if sc.allBlocksProcessed() {
		return scFinishedEv{}, nil
	}
	return noOp, nil
}

func (sc *scheduler) handleTryPrunePeer(event tryPrunePeer) (Event, error) {
	prunablePeers := sc.prunablePeers(sc.peerTimeout, sc.minRecvRate, event.time)
	if len(prunablePeers) == 0 {
		return noOp, nil
	}
	for _, peerID := range prunablePeers {
		err := sc.removePeer(peerID)
		if err != nil {
			// Should never happen as prunablePeers() returns only existing peers in Ready state.
			panic("scheduler data corruption")
		}
	}

	// If all blocks are processed we should finish even some peers were pruned.
	if sc.allBlocksProcessed() {
		return scFinishedEv{}, nil
	}

	return scPeersPruned{peers: prunablePeers}, nil

}

// TODO - Schedule multiple block requests
func (sc *scheduler) handleTrySchedule(event trySchedule) (Event, error) {

	nextHeight := sc.nextHeightToSchedule()
	if nextHeight == -1 {
		return noOp, nil
	}

	bestPeerID, err := sc.selectPeer(nextHeight)
	if err != nil {
		return scSchedulerFail{reason: err}, nil
	}
	if err := sc.markPending(bestPeerID, nextHeight, event.time); err != nil {
		return scSchedulerFail{reason: err}, nil // XXX: peerError might be more appropriate
	}
	return scBlockRequest{peerID: bestPeerID, height: nextHeight}, nil

}

func (sc *scheduler) handleStatusResponse(event bcStatusResponse) (Event, error) {
	err := sc.setPeerHeight(event.peerID, event.height)
	if err != nil {
		return scPeerError{peerID: event.peerID, reason: err}, nil
	}
	return noOp, nil
}

func (sc *scheduler) handle(event Event) (Event, error) {
	switch event := event.(type) {
	case bcStatusResponse:
		nextEvent, err := sc.handleStatusResponse(event)
		return nextEvent, err
	case bcBlockResponse:
		nextEvent, err := sc.handleBlockResponse(event)
		return nextEvent, err
	case trySchedule:
		nextEvent, err := sc.handleTrySchedule(event)
		return nextEvent, err
	case addNewPeer:
		nextEvent, err := sc.handleAddNewPeer(event)
		return nextEvent, err
	case tryPrunePeer:
		nextEvent, err := sc.handleTryPrunePeer(event)
		return nextEvent, err
	case peerError:
		nextEvent, err := sc.handlePeerError(event)
		return nextEvent, err
	case pcBlockProcessed:
		nextEvent, err := sc.handleBlockProcessed(event)
		return nextEvent, err
	case pcBlockVerificationFailure:
		nextEvent, err := sc.handleBlockProcessError(event)
		return nextEvent, err
	default:
		return scSchedulerFail{reason: fmt.Errorf("unknown event %v", event)}, nil
	}
	//return noOp, nil
}
