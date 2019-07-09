package v2

import (
	"errors"
	"math"
	"math/rand"
	"time"

	"github.com/tendermint/tendermint/p2p"
)

type height int64

type blockState int

const (
	blockStateNew = iota
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

	// a map of which blocks are available from which peers
	blockPeers map[height]map[p2p.ID]scPeer

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
	return &schedule{
		minHeight:      minHeight,
		targetPending:  targetPending,
		targetReceived: targetReceived,
	}
}

func (sc *schedule) addPeer(peerID p2p.ID) error {
	if ok, _ := sc.peers[peerID]; ok {
		return errors.New("duplicate peer")
	}
	peers[peerID] = newScPeer(peerID)
}

func (sc *schedule) touchPeer(peerID p2p.ID, time time.Time) error {
	if ok, peer := sc.peers[peerID]; !ok {
		return errors.New("peer not found")
	}

	peer.lastTouch = time
}

func (sc *schedule) removePeer(peerID p2p.ID) error {
	if ok, peer := sc.peers[peerID]; !ok {
		return errors.New("peer not found")
	}

	if peer.state == peerStateRemoved {
		return errors.New("try to remove a removed peer")
	}

	// cleanup blockStates & pending
	for height, pendingPeerID := range sc.pending {
		if peerID == pendingPeerID {
			delete(sc.pending[height])
			sc.blockStates[height] = blockStateNew
		}
	}

	peer.state = peerStateRemoved

	return nil
}

func (sc *schedule) setPeerHeight(peerID p2p.ID, height int64) {
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

		sc.blockPeers[i][peerID] = peer
	}
}

func (sc *schedule) markReceived(peerID p2p.ID, height int64, size int64) error {
	if ok, peer := sc.peers[peerID]; !ok || peer.state != peerStateReady {
		return errors.New("received a block from an unknown peer")
	}

	if blockState[height] != blockStatePending {
		return errors.New("received an unrequested block")
	}

	// TODO: download speed

	// check if the block is pending from that peer
	if sc.pending[height] != peerID {
		return errors.New("Didn't request this block from this peer")
	}

	blockState[i] = blockStateReceived
	delete(sc.pending[height])

	return nil
}

func (sc *schedule) markProcessed(height int64) error {
	if sc.blockState[height] != received {
		return errors.New("Block was processed without being received")
	}

	sc.blockState[height] = blockStateProcessed
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
	min := 0
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
		return errors.New("peer not found")
	}

	for _, height := range sc.pending[peerID] {
		sc.blockState[height] = blockStateNew
	}
}

func (sc *schedule) selectPeer(peers []scPeer) scPeer {
	// TODO
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

func (sc *schedule) popSchedule(maxRequest int) []scBlockRequestMessage {
	// We only want to schedule requests such that we have less than sc.targetPending and sc.targetReceived
	// This ensures we don't saturate the network or flood the processor with unprocessed blocks
	pendingBlock := sc.numBlockInState(blockStatePending)
	receivedBlocks := sc.numBlockInState(blockStateReceived)
	todo := math.Min(sc.targetPending-pendingBlocks, sc.targetReceived-receivedBlocks)
	events := []scBlockRequestMessage{}
	for height := sc.minHeight(); height < sc.maxHeight(); height++ {
		if todo == 0 {
			break
		}
		if blockStates[height] == blockStateNew {
			peer = sc.selectPeer(blockPeers[i])
			sc.blockStates[height] = blockStatePending
			sc.pending[height] = peer
			events = append(events, scBlockRequestMessage{peerID: peer.peerID, height: i})
			todo--
		}
	}
	return events
}

type Scheduler struct {
	sc schedule
}

//  What should the scheduler_test look like?

/*
func addPeer(peerID) {
	schedule.addPeer(peerID)
}

func handleStatusResponse(peerID, height, time) {
	schedule.touchPeer(peerID, time)
	schedule.setPeerHeight(peerID, height)
}

func handleBlockResponseMessage(peerID, height, block, time) {
	schedule.touchPeer(peerID, time)
	schedule.markReceived(peerID, height, size(block))
}

func handleNoBlockResponseMessage(peerID, height, time) {
	schedule.touchPeer(peerID, time)
    // reschedule that block, punish peer...
}

func handleTimeCheckEv(time) {
	// clean peer list

    events = []
	for peerID := range schedule.peersTouchedSince(time) {
		pending = schedule.pendingFrom(peerID)
		schedule.setPeerState(peerID, timedout)
		schedule.resetBlocks(pending)
		events = append(events, peerTimeout{peerID})
    }

	events = append(events, schedule.getSchedule())

    return events
}
*/
