package v2

import (
	"errors"
	"math"
	"math/rand"
	"time"

	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

type height int64

// errors
var (
	// new
	errDuplicatePeer = errors.New("fast sync tried to add a peer twice")
	errPeerNotFound  = errors.New("Peer not found")
	errPeerRemoved   = errors.New("try to remove a removed peer")
	errBadSchedule   = errors.New("Invalid Schedule transition")

	// internal to the package
	errNoErrorFinished        = errors.New("fast sync is finished")
	errInvalidEvent           = errors.New("invalid event in current state")
	errMissingBlock           = errors.New("missing blocks")
	errNilPeerForBlockRequest = errors.New("peer for block request does not exist in the switch")
	errSendQueueFull          = errors.New("block request not made, send-queue is full")
	errPeerTooShort           = errors.New("peer height too low, old peer removed/ new peer not added")
	errSwitchRemovesPeer      = errors.New("switch is removing peer")
	errTimeoutEventWrongState = errors.New("timeout event for a state different than the current one")
	errNoTallerPeer           = errors.New("fast sync timed out on waiting for a peer taller than this node")

	// reported eventually to the switch
	errPeerLowersItsHeight             = errors.New("fast sync peer reports a height lower than previous")            // handle return
	errNoPeerResponseForCurrentHeights = errors.New("fast sync timed out on peer block response for current heights") // handle return
	errNoPeerResponse                  = errors.New("fast sync timed out on peer block response")                     // xx
	errBadDataFromPeer                 = errors.New("fast sync received block from wrong peer or block is bad")       // xx
	errDuplicateBlock                  = errors.New("fast sync received duplicate block from peer")
	errBlockVerificationFailure        = errors.New("fast sync block verification failure")              // xx
	errSlowPeer                        = errors.New("fast sync peer is not sending us data fast enough") // xx

)

type Event interface{}
type schedulerErrorEv struct {
	peerID p2p.ID
	error  error
}

type blockState int

const (
	blockStateUnknown = iota
	blockStateNew
	blockStatePending
	blockStateReceived
	blockStateProcessed
)

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

type scPeer struct {
	peerID      p2p.ID
	state       peerState
	height      int64
	lastTouched time.Time
	lastRate    int64
}

func newScPeer(peerID p2p.ID) *scPeer {
	return &scPeer{
		peerID: peerID,
		state:  peerStateNew,
		height: -1,
	}
}

type schedule struct {
	initHeight int64
	// a list of blocks in which blockState
	blockStates map[int64]blockState

	// a map of peerID to schedule specific peer struct `scPeer`
	peers map[p2p.ID]*scPeer

	// a map of heights to the peer we are waiting for a response from
	pending     map[int64]p2p.ID
	pendingTime map[int64]time.Time

	peerTimeout  uint
	peerMinSpeed uint
}

func newSchedule(initHeight int64) *schedule {
	sc := schedule{
		initHeight: initHeight,
	}

	sc.setStateAtHeight(initHeight, blockStateNew)

	return &sc
}

func (sc *schedule) addPeer(peerID p2p.ID) error {
	if _, ok := sc.peers[peerID]; ok {
		return errDuplicatePeer
	}
	sc.peers[peerID] = newScPeer(peerID)

	return nil
}

func (sc *schedule) touchPeer(peerID p2p.ID, time time.Time) error {
	var peer scPeer
	if peer, ok := sc.peers[peerID]; !ok && peer.state == peerStateRemoved {
		return errPeerNotFound
	}

	peer.lastTouched = time

	return nil
}

func (sc *schedule) removePeer(peerID p2p.ID) error {
	var peer scPeer
	if peer, ok := sc.peers[peerID]; !ok || peer.state == peerStateRemoved {
		return errPeerNotFound
	}

	if peer.state == peerStateRemoved {
		return errPeerRemoved
	}

	for height, pendingPeerID := range sc.pending {
		if peerID == pendingPeerID {
			delete(sc.pending, height)
			sc.blockStates[height] = blockStateNew
		}
	}

	peer.state = peerStateRemoved

	return nil
}

func (sc *schedule) setPeerHeight(peerID p2p.ID, height int64) error {
	var peer scPeer
	if peer, ok := sc.peers[peerID]; !ok || peer.state != peerStateRemoved {
		return errors.New("new peer not found")
	}

	if height < peer.height {
		return errors.New("peer height descending")
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

// XXX: probably needs a better name
func (sc *schedule) peersSince(duration time.Duration, now time.Time) []*scPeer {
	peers := []*scPeer{}
	for _, peer := range sc.peers {
		if now.Sub(peer.lastTouched) > duration {
			peers = append(peers, peer)
		}
	}

	return peers
}

func (sc *schedule) setStateAtHeight(height int64, state blockState) {
	sc.blockStates[height] = state
}

// TODO keep track of when i received this block
func (sc *schedule) markReceived(peerID p2p.ID, height int64, size int64, now time.Time) error {
	var peer scPeer
	if peer, ok := sc.peers[peerID]; !ok || peer.state != peerStateReady {
		return errPeerNotFound
	}

	if sc.getStateAtHeight(height) != blockStatePending {
		// received a block not in pending
		return errBadSchedule
	}

	// check if the block is pending from that peer
	if sc.pending[height] != peerID {
		return errBadSchedule
	}

	var pendingTime time.Time
	if pendingTime, ok := sc.pendingTime[height]; !ok || pendingTime.Sub(now) < 0 {
		return errBadSchedule
	}

	peer.lastRate = size / int64(now.Sub(pendingTime).Seconds())

	sc.setStateAtHeight(height, blockStateReceived)
	delete(sc.pending, height)
	delete(sc.pendingTime, height)

	return nil
}

// todo keep track of when i requested this block
func (sc *schedule) markPending(peerID p2p.ID, height int64, time time.Time) error {
	var peer scPeer
	if peer, ok := sc.peers[peerID]; !ok || peer.state != peerStateReady {
		return errPeerNotFound
	}

	if height > peer.height {
		// tried to request a block from a peer who doesn't have it
		return errBadSchedule
	}

	sc.setStateAtHeight(height, blockStatePending)
	sc.pending[height] = peerID
	// XXX: to make htis more accurate we can introduce a message from
	// the IO rutine which indicates the time the request was put on the wire
	sc.pendingTime[height] = time

	return nil
}

func (sc *schedule) markProcessed(height int64) error {
	if sc.getStateAtHeight(height) != blockStateReceived {
		return errBadSchedule
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
	min := sc.initHeight
	for height, state := range sc.blockStates {
		if state == blockStateNew && height < min {
			min = height
		}
	}

	return min
}

// XXX: THis is not yet used
func (sc *schedule) pendingFrom(peerID p2p.ID) []int64 {
	heights := []int64{}
	for height, pendingPeerID := range sc.pending {
		if pendingPeerID == peerID {
			heights = append(heights, height)
		}
	}
	return heights
}

// XXX: What about pedingTime here?
// XXX: Split up read and write paths here
func (sc *schedule) resetBlocks(peerID p2p.ID) error {
	if _, ok := sc.peers[peerID]; !ok {
		return errPeerNotFound
	}

	// this should use pendingFrom
	for height, pendingPeerID := range sc.pending {
		if pendingPeerID == peerID {
			sc.setStateAtHeight(height, blockStateNew)
		}
	}

	return nil
}

func (sc *schedule) selectPeer(peers []*scPeer) *scPeer {
	// FIXME: properPeerSelector
	s := rand.NewSource(time.Now().Unix())
	r := rand.New(s)

	return peers[r.Intn(len(peers))]
}

func (sc *schedule) prunablePeers(time time.Time, minSpeed int) []p2p.ID {
	// TODO
	return []p2p.ID{}
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

}

func NewScheduler(minHeight int64, targetPending uint32, targetReceived uint32) *Scheduler {
	return &Scheduler{
		sc:             newSchedule(minHeight),
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

func (sdr *Scheduler) handleBlockProcessError(peerID p2p.ID, height int64) Event {
	// remove the peer
	sdr.sc.removePeer(peerID)
	// reSchdule all the blocks we are waiting
	/*
		 XXX: This is wrong as we need to
			foreach block where state != blockStateProcessed
				state => blockStateNew
	*/
	sdr.sc.resetBlocks(peerID)

	return Skip{}
}

func (sdr *Scheduler) handleTimeCheck(peerID p2p.ID, now time.Time) interface{} {
	// prune peers
	// TODO

	// produce new schedule
	events := []scBlockRequestMessage{}
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
				// TODO
			}
			events = append(events, scBlockRequestMessage{peerID: bestPeer.peerID, height: height})
			todo--
		}
	}
	return events
}
