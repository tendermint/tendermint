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
	blockStateUnknown = iota,
		blockStateNew,
		blockStatePending,
		blockStateReceived,
		blockStateProcessed
)

type scBlockRequestMessage struct {
	peerID p2p.ID
	height int64
}

type peerState int

const (
	peerStateNew = iota
	peerStateReady,
	peerStateRemoved
)

type scPeer struct {
	peerID      p2p.ID
	state       scPeerState
	lastTouched time.Time
	monitor     flow.Monitor
}

func newScPeer(peerID p2p.ID) *scPeer {
	return &scPeer{
		peerID: peerID,
		state:  peerStateNew,
	}
}

type schedule struct {
	minHeight int64
	// a list of blocks in which blockState
	blockStates map[height]blockState

	// a map of peerID to schedule specific peer struct `scPeer`
	peers map[p2p.ID]scPeer

	// a map of heights to the peer we are waiting for a response from
	pending map[height]p2p.ID

	targetPending  uint // the number of blocks we want in blockStatePending
	targetReceived uint // the number of blocks we want in blockStateReceived

	peerTimeout  uint
	peerMinSpeed uint
}

func newSchedule(minHeight int64, targetPending uint, targetReceived uint) *schedule {
	sc := schedule{
		minHeight:      minHeight,
		targetPending:  targetPending,
		targetReceived: targetReceived,
	}

	sc.setStateAtHeight(minHeight, blockStateNew)

	return &sc
}

func (sc *schedule) addPeer(peerID p2p.ID) error {
	if ok, _ := sc.peers[peerID]; ok {
		return errDuplicatePeer
	}
	peers[peerID] = newScPeer(peerID)
}

func (sc *schedule) touchPeer(peerID p2p.ID, time time.Time) error {
	if ok, peer := sc.peers[peerID]; !ok && peer.state == peerStateRemoved {
		return errPeerNotFound
	}

	peer.lastTouched = time
}

func (sc *schedule) removePeer(peerID p2p.ID) error {
	if ok, peer := sc.peers[peerID]; !ok {
		return errPeerNotFound
	}

	if peer.state == peerStateRemoved {
		return errPeerRemoved
	}

	for height, pendingPeerID := range sc.pending {
		if peerID == pendingPeerID {
			delete(sc.pending[height])
			sc.blockStates[height] = blockStateNew
		}
	}

	peer.state = peerStateRemoved

	return nil
}

func (sc *schedule) setPeerHeight(peerID p2p.ID, height int64) error {
	if ok, peer := sc.peers[peerID]; !ok || peer.state != peerStateRemoved {
		return errors.New("new peer not found")
	}

	if height < peer.height {
		return errors.New("peer height descending")
	}

	peer.height = height
	peer.state = peerStateReady
	for i := sc.minHeight(); i <= height; i++ {
		if !sc.blockState[i] {
			sc.blockState[i] = blockStateNew
		}
	}

	return nil
}

func (sc *schedule) getStateAtHeight(height int64) blockState {
	if height < sc.minHeight {
		return blockStateProcessed
	} else if ok, state := sc.blockState[height]; ok {
		return state
	} else {
		return blockStateUnknown
	}
}

func (sc *schedule) getPeersAtHeight(height int64) []scPeer {
	peers := []scPeer{}
	for perrID, peer := range sc.peers {
		if peer.height >= height {
			peers = append(peers, peer)
		}
	}

	return peers
}

// XXX: probably needs a better name
func (sc *schedule) peersSince(duration time.Duration, now time.Time) []scPeer {
	peers := []scPeer{}
	for id, peer := range sc.peers {
		if now-peer.lastTouched > duration {
			peers = append(peers, peer)
		}
	}

	return peers
}

func (sc *schedule) setStateAtHeight(height int64, state blockState) {
	sc.blockStates[height] = state
}

func (sc *schedule) markReceived(peerID p2p.ID, height int64, size int64) error {
	if ok, peer := sc.peers[peerID]; !ok || peer.state != peerStateReady {
		return errPeerNotFound
	}

	if sc.getStateAtHeight(height) != blockStatePending {
		// received a block not in pending
		return errBadSchedule
	}

	// TODO: download speed

	// check if the block is pending from that peer
	if sc.pending[height] != peerID {
		return errBadSchedule
	}

	sc.setStateAtHeight(height, blockStateReceived)
	delete(sc.pending[height])

	return nil
}

func (sc *schedule) markPending(peerID p2p.ID, height int64, size int64) error {
	if ok, peer := sc.peers[peerID]; !ok || peer.state != peerStateReady {
		return errPeerNotFound
	}

	if height > peer.height {
		// tried to request a block from a peer who doesn't have it
		return errBadSchedule
	}

	sc.setStateAtHeight(height, blockStatePending)
	sc.pending[height] = peer

	return nil
}

func (sc *schedule) markProcessed(height int64) error {
	if sc.getStateAtHeight(height) != blockStateReceived {
		return errBadSchedule
	}

	sc.setStateAtHeight(height, blockStateProcessed)
}

func (sc *schedule) allBlocksProcessed() bool {
	for height, state := range sc.blockStates {
		if state != blockStateProcessed {
			return false
		}
	}
	return true
}

// heighter block | state == blockStateNew
func (sc *schedule) maxHeight() int64 {
	max := 0
	for height, state := range sc.blockStates {
		if state == blockStateNew && height > max {
			max = height
		}
	}

	return max
}

// lowest block | state == blockStateNew
func (sc *schedule) minHeight() int64 {
	min := sc.minHeight
	for height, state := range sc.blockStates {
		if state == blockStateNew && height < min {
			min = height
		}
	}

	return min
}

func (sc *schedule) pendingFrom(peerID p2p.ID) {
	return sc.pending[peerID]
}

func (sc *schedule) resetBlocks(peerID p2p.ID) error {
	if ok, peer := sc.peers[peerID]; !ok {
		return errPeerNotFound
	}

	for _, height := range sc.pending[peerID] {
		sc.setStateAtHeight(height, blockStateNew)
	}
}

func (sc *schedule) selectPeer(peers []scPeer) scPeer {
	// FIXME: properPeerSelector
	r.Intn(len(reasons))
	s := rand.NewSource(time.Now().Unix())
	r := rand.New(s)

	return peers[r.Intn(len(peers))]
}

func (sc *schedule) prunablePeers(time time.Time, minSpeed int) []p2p.ID {
	// TODO
	return []p2p.ID{}
}

func (sc *schedule) numBlockInState(targetState blockState) uint32 {
	num := 0
	for height, state := range sc.blockState {
		if state == targetState {
			num++
		}
	}
	return num
}

type Scheduler struct {
	sc schedule
}

func NewScheduler(minHeight int64, targetPending uint, targetReceived uint) *Scheduler {
	return &Scheduler{
		sc: newSchedule(minHeight, targetPending, targetReceived),
	}
}

func (sdr *Scheduler) handleAddPeer(peerID p2p.ID) []Event {
	err := sdr.sc.addPeer(peerID)
	if err != nil {
		[]Event{schedulerErrorEv{peerID, err}}
	}

	return []Event{}
}

func (sdr *Scheduler) handleRemovePeer(peerID p2p.ID) []Events {
	err := sdr.sc.removePeer(peerID)
	if err != nil {
		[]Event{schedulerErrorEv{peerID, err}}
	}

	return []Event{}
}

func (sdr *Scheduler) handleStatusResponse(peerID p2p.ID) []Event {
	err := sdr.sc.touchPeer(peerID, time)
	if err != nil {
		return []Event{schedulerErrorEv{peerID, err}}
	}

	err = sdr.sc.setPeerHeight(peerID, height)
	if err != nil {
		return []Event{schedulerErrorEv{peerID, err}}
	}

	return []Event{}
}

type bcBlockResponseEv struct {
	peerID  p2p.ID
	height  int64
	block   *types.Block
	msgSize int64
}

func (sdr *Scheduler) handleBlockResponse(peerID p2p.ID, msg *bcBlockResponse) []Event {
	err := scr.sc.touchPeer(peerID, time)
	if err != nil {
		return []Event{schedulerErrorEv{peerID, err}}
	}

	err = sdr.sc.markReceived(peerID, height, msg)
	if err != nil {
		return []Event{schedulerErrorEv{peerID, err}}
	}

	return []Event{scBlockReceivedEv{peerID}}
}

type scFinishedEv struct{}

func (sdr *Scheduler) handleBlockProcessed(peerID p2p.ID, height int64) []Events {
	err = sdr.sc.markProcessed(peerID, height)
	if err != nil {
		return []Event{schedulerErrorEv{peerID, err}}
	}

	if sdr.sc.allBlocksProcessed() {
		return []Event{scFinishedEv{}}
	}

	return []Event{}
}

func (sdr *Scheduler) handleBlockProcessError(peerID p2p.ID, height int64) []Event {
	// remove the peer
	sc.removePeer(peerID)
	// reSchdule all the blocks we are waiting
	/*
		 XXX: This is wrong as we need to
			foreach block where state != blockStateProcessed
				state => blockStateNew
	*/
	sc.resetBlocks(peerID)

	return []Event{}
}

func (sdr *Scheduler) handleTimeCheck(peerID p2p.ID) []Events {
	// prune peers
	// TODO

	// produce new schedule
	events := []scBlockRequestMessage{}
	pendingBlock := sdr.sc.numBlockInState(blockStatePending)
	receivedBlocks := sdr.sc.numBlockInState(blockStateReceived)
	todo := math.Min(sdr.targetPending-pendingBlocks, sdr.targetReceived-receivedBlocks)
	for height := sdr.sc.minHeight(); height <= sdr.sc.maxHeight(); height++ {
		if todo == 0 {
			break
		}
		if sdr.sc.getStateAt(height) == blockStateNew {
			allPeers := sdr.sc.getPeersAtHeight(height)
			bestPeer := sdr.sc.selectPeer(allPeers)
			err := sc.markPending(peerID, height)
			if err != nil {
				// TODO
			}
			events = append(events, scBlockRequestMessage{peerID: bestPeer.peerID, height: height})
			todo--
		}
	}
	return events
}
