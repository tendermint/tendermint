package v2

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

type height int64

type Event interface{}
type schedulerErrorEv struct {
	peerID p2p.ID
	error  error
}

type blockState int

const (
	blockStateUnknown blockState = iota
	blockStateNew
	blockStatePending
	blockStateReceived
	blockStateProcessed
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
		return fmt.Sprintf("unknown blockState: %d", e)
	}
}

type scBlockRequestMessage struct {
	peerID p2p.ID
	height int64
}

type peerState int

const (
	peerStateNew = iota
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
		return fmt.Sprintf("unknown peerState: %d", e)
	}
}

type scPeer struct {
	peerID      p2p.ID
	state       peerState
	height      int64
	lastTouched time.Time
	lastRate    int64
}

func newScPeer(peerID p2p.ID) *scPeer {
	return &scPeer{
		peerID:      peerID,
		state:       peerStateNew,
		height:      -1,
		lastTouched: time.Time{},
	}
}

type schedule struct {
	initHeight int64
	// a list of blocks in which blockState
	blockStates map[int64]blockState

	// a map of peerID to schedule specific peer struct `scPeer`
	peers map[p2p.ID]*scPeer

	// a map of heights to the peer we are waiting for a response from
	pendingBlocks map[int64]p2p.ID
	pendingTime   map[int64]time.Time

	receivedBlocks map[int64]p2p.ID

	peerTimeout  uint
	peerMinSpeed uint
}

func newSchedule(initHeight int64) *schedule {
	sc := schedule{
		initHeight:     initHeight,
		blockStates:    make(map[int64]blockState),
		peers:          make(map[p2p.ID]*scPeer),
		pendingBlocks:  make(map[int64]p2p.ID),
		pendingTime:    make(map[int64]time.Time),
		receivedBlocks: make(map[int64]p2p.ID),
	}

	sc.setStateAtHeight(initHeight, blockStateNew)

	return &sc
}

func (sc *schedule) addPeer(peerID p2p.ID) error {
	if _, ok := sc.peers[peerID]; ok {
		return fmt.Errorf("Cannot add duplicate peer %s", peerID)
	}
	sc.peers[peerID] = newScPeer(peerID)
	return nil
}

func (sc *schedule) touchPeer(peerID p2p.ID, time time.Time) error {
	peer, ok := sc.peers[peerID]
	if !ok {
		return fmt.Errorf("Couldn't find peer %s", peerID)
	}

	if peer.state == peerStateRemoved {
		return fmt.Errorf("Tried to touch peer in peerStateRemoved")
	}

	peer.lastTouched = time

	return nil
}

func (sc *schedule) removePeer(peerID p2p.ID) error {
	if _, ok := sc.peers[peerID]; !ok {
		return fmt.Errorf("Can't find peer %s", peerID)
	}

	peer, ok := sc.peers[peerID]
	if !ok {
		return fmt.Errorf("Couldn't find peer %s", peerID)
	}

	if peer.state == peerStateRemoved {
		return fmt.Errorf("Tried to remove peer %s in peerStateRemoved", peerID)
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

	peer.state = peerStateRemoved

	return nil
}

func (sc *schedule) setPeerHeight(peerID p2p.ID, height int64) error {
	peer, ok := sc.peers[peerID]
	if !ok {
		return fmt.Errorf("Can't find peer %s", peerID)
	}

	if peer.state == peerStateRemoved {
		return fmt.Errorf("Cannot set peer height for a peer in peerStateRemoved")
	}

	if height < peer.height {
		return fmt.Errorf("Cannot move peer height lower. from %d to %d", peer.height, height)
	}

	peer.height = height
	peer.state = peerStateReady
	for i := sc.minHeight(); i <= height; i++ {
		if sc.getStateAtHeight(i) == blockStateUnknown {
			sc.setStateAtHeight(i, blockStateNew)
		}
	}

	return nil
}

func (sc *schedule) getStateAtHeight(height int64) blockState {
	if height < sc.initHeight {
		return blockStateProcessed
	} else if state, ok := sc.blockStates[height]; ok {
		return state
	} else {
		return blockStateUnknown
	}
}

func (sc *schedule) getPeersAtHeight(height int64) []*scPeer {
	peers := []*scPeer{}
	for _, peer := range sc.peers {
		if peer.height >= height {
			peers = append(peers, peer)
		}
	}

	return peers
}

func (sc *schedule) peersInactiveSince(duration time.Duration, now time.Time) []p2p.ID {
	peers := []p2p.ID{}
	for _, peer := range sc.peers {
		if now.Sub(peer.lastTouched) > duration {
			peers = append(peers, peer.peerID)
		}
	}

	return peers
}

func (sc *schedule) peersSlowerThan(minSpeed int64) []p2p.ID {
	peers := []p2p.ID{}
	for _, peer := range sc.peers {
		if peer.lastRate < minSpeed {
			peers = append(peers, peer.peerID)
		}
	}

	return peers
}

func (sc *schedule) setStateAtHeight(height int64, state blockState) {
	sc.blockStates[height] = state
}

func (sc *schedule) markReceived(peerID p2p.ID, height int64, size int64, now time.Time) error {
	peer, ok := sc.peers[peerID]
	if !ok {
		return fmt.Errorf("Can't find peer %s", peerID)
	}

	if peer.state == peerStateRemoved {
		return fmt.Errorf("Cannot receive blocks from removed peer %s", peerID)
	}

	if state := sc.getStateAtHeight(height); state != blockStatePending || sc.pendingBlocks[height] != peerID {
		return fmt.Errorf("Received block %d from peer %s without being requested", height, peerID)
	}

	pendingTime, ok := sc.pendingTime[height]
	if !ok || now.Sub(pendingTime) <= 0 {
		return fmt.Errorf("Clock error. Block %d received at %s but requested at %s",
			height, pendingTime, now)
	}

	peer.lastRate = size / int64(now.Sub(pendingTime).Seconds())

	sc.setStateAtHeight(height, blockStateReceived)
	delete(sc.pendingBlocks, height)
	delete(sc.pendingTime, height)

	sc.receivedBlocks[height] = peerID

	return nil
}

// todo keep track of when i requested this block
func (sc *schedule) markPending(peerID p2p.ID, height int64, time time.Time) error {
	peer, ok := sc.peers[peerID]
	if !ok {
		return fmt.Errorf("Can't find peer %s", peerID)
	}

	if peer.state != peerStateReady {
		return fmt.Errorf("Cannot schedule %d from %s in %s", height, peerID, peer.state)
	}

	if height > peer.height {
		return fmt.Errorf("Cannot request height %d from peer %s who is at height %d",
			height, peerID, peer.height)
	}

	sc.setStateAtHeight(height, blockStatePending)
	sc.pendingBlocks[height] = peerID
	// XXX: to make this more accurate we can introduce a message from
	// the IO routine which indicates the time the request was put on the wire
	sc.pendingTime[height] = time

	return nil
}

func (sc *schedule) markProcessed(height int64) error {
	state := sc.getStateAtHeight(height)
	if state != blockStateReceived {
		return fmt.Errorf("Can't mark height %d received from block state %s", height, state)
	}

	sc.setStateAtHeight(height, blockStateProcessed)

	return nil
}

func (sc *schedule) allBlocksProcessed() bool {
	for _, state := range sc.blockStates {
		if state != blockStateProcessed {
			return false
		}
	}
	return true
}

// heighter block | state == blockStateNew
func (sc *schedule) maxHeight() int64 {
	var max int64 = 0
	for height, state := range sc.blockStates {
		if state == blockStateNew && height > max {
			max = height
		}
	}

	return max
}

// lowest block | state == blockStateNew
func (sc *schedule) minHeight() int64 {
	var min int64 = math.MaxInt64
	for height, state := range sc.blockStates {
		if state == blockStateNew && height < min {
			min = height
		}
	}

	return min
}

func (sc *schedule) pendingFrom(peerID p2p.ID) []int64 {
	heights := []int64{}
	for height, pendingPeerID := range sc.pendingBlocks {
		if pendingPeerID == peerID {
			heights = append(heights, height)
		}
	}
	return heights
}

func (sc *schedule) selectPeer(peers []*scPeer) *scPeer {
	// FIXME: properPeerSelector
	s := rand.NewSource(time.Now().Unix())
	r := rand.New(s)

	return peers[r.Intn(len(peers))]
}

// XXX: this duplicates the logic of peersInactiveSince and peersSlowerThan
func (sc *schedule) prunablePeers(peerTimout time.Duration, minRecvRate int64, now time.Time) []p2p.ID {
	prunable := []p2p.ID{}
	for peerID, peer := range sc.peers {
		if now.Sub(peer.lastTouched) > peerTimout || peer.lastRate < minRecvRate {
			prunable = append(prunable, peerID)
		}
	}

	return prunable
}

func (sc *schedule) numBlockInState(targetState blockState) uint32 {
	var num uint32 = 0
	for _, state := range sc.blockStates {
		if state == targetState {
			num++
		}
	}
	return num
}

type Scheduler struct {
	sc             *schedule
	targetPending  uint32 // the number of blocks we want in blockStatePending
	targetReceived uint32 // the number of blocks we want in blockStateReceived
	minRecvRate    int64
	peerTimeout    time.Duration
}

func NewScheduler(minHeight int64, minRecvRate int64, peerTimeout time.Duration,
	targetPending uint32, targetReceived uint32) *Scheduler {
	return &Scheduler{
		sc:             newSchedule(minHeight),
		minRecvRate:    minRecvRate,
		peerTimeout:    peerTimeout,
		targetPending:  targetPending,
		targetReceived: targetReceived,
	}
}

type Skip struct {
}

func (sdr *Scheduler) handleAddPeer(peerID p2p.ID) Event {
	err := sdr.sc.addPeer(peerID)
	if err != nil {
		return schedulerErrorEv{peerID, err}
	}

	return Skip{}
}

type Events []Event

func (sdr *Scheduler) handleRemovePeer(peerID p2p.ID) Event {
	err := sdr.sc.removePeer(peerID)
	if err != nil {
		return schedulerErrorEv{peerID, err}
	}

	return Skip{}
}

func (sdr *Scheduler) handleStatusResponse(peerID p2p.ID, height int64, now time.Time) Event {
	err := sdr.sc.touchPeer(peerID, now)
	if err != nil {
		return schedulerErrorEv{peerID, err}
	}

	err = sdr.sc.setPeerHeight(peerID, height)
	if err != nil {
		return schedulerErrorEv{peerID, err}
	}

	return Skip{}
}

type bcBlockResponseEv struct {
	peerID  p2p.ID
	height  int64
	block   *types.Block
	msgSize int64
}

type scBlockReceivedEv struct {
	peerID p2p.ID
}

func (sdr *Scheduler) handleBlockResponse(peerID p2p.ID, msg *bcBlockResponseEv, now time.Time) Event {
	err := sdr.sc.touchPeer(peerID, now)
	if err != nil {
		return schedulerErrorEv{peerID, err}
	}

	err = sdr.sc.markReceived(peerID, msg.height, msg.msgSize, now)
	if err != nil {
		return schedulerErrorEv{peerID, err}
	}

	return scBlockReceivedEv{peerID}
}

type scFinishedEv struct{}

func (sdr *Scheduler) handleBlockProcessed(peerID p2p.ID, height int64) Event {
	err := sdr.sc.markProcessed(height)
	if err != nil {
		return schedulerErrorEv{peerID, err}
	}

	if sdr.sc.allBlocksProcessed() {
		return scFinishedEv{}
	}

	return Skip{}
}

type scPrunePeerEv struct {
	peerID p2p.ID
	reason error
}

func (sdr *Scheduler) handleBlockProcessError(peerID p2p.ID, height int64) Event {
	err := sdr.sc.removePeer(peerID)
	if err != nil {
		return scPrunePeerEv{peerID: peerID, reason: fmt.Errorf("Failed to process block %d", height)}
	}

	return Skip{}
}

type scSchedulerFailure struct {
	peerID p2p.ID
	time   time.Time
	reason error
}

func (sdr *Scheduler) handleTimeCheck(now time.Time) Events {
	prunablePeers := sdr.sc.prunablePeers(sdr.peerTimeout, sdr.minRecvRate, now)

	events := []Event{}
	for _, peerID := range prunablePeers {
		err := sdr.sc.removePeer(peerID)
		if err != nil {
			events = append(events, scPrunePeerEv{peerID: peerID, reason: err})
		}
	}

	pendingBlocks := sdr.sc.numBlockInState(blockStatePending)
	receivedBlocks := sdr.sc.numBlockInState(blockStateReceived)
	todo := math.Min(float64(sdr.targetPending-pendingBlocks), float64(sdr.targetReceived-receivedBlocks))
	for height := sdr.sc.minHeight(); height <= sdr.sc.maxHeight(); height++ {
		if todo == 0 {
			break
		}
		if sdr.sc.getStateAtHeight(height) == blockStateNew {
			allPeers := sdr.sc.getPeersAtHeight(height)
			bestPeer := sdr.sc.selectPeer(allPeers)
			err := sdr.sc.markPending(peerID, height, now)
			if err != nil {
				// this should be fatal
				events = append(events, scSchedulerFailure{peerID: peerID, time: now, reason: err})
				return events
			}
			events = append(events, scBlockRequestMessage{peerID: bestPeer.peerID, height: height})
			todo--
		}
	}
	return events
}
