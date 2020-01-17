package v1

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

// Blockchain Reactor State
type bcReactorFSMState struct {
	name string

	// called when transitioning out of current state
	handle func(*BcReactorFSM, bReactorEvent, bReactorEventData) (next *bcReactorFSMState, err error)
	// called when entering the state
	enter func(fsm *BcReactorFSM)

	// timeout to ensure FSM is not stuck in a state forever
	// the timer is owned and run by the fsm instance
	timeout time.Duration
}

func (s *bcReactorFSMState) String() string {
	return s.name
}

// BcReactorFSM is the datastructure for the Blockchain Reactor State Machine
type BcReactorFSM struct {
	logger log.Logger
	mtx    sync.Mutex

	startTime time.Time

	state      *bcReactorFSMState
	stateTimer *time.Timer
	pool       *BlockPool

	// interface used to call the Blockchain reactor to send StatusRequest, BlockRequest, reporting errors, etc.
	toBcR bcReactor
}

// NewFSM creates a new reactor FSM.
func NewFSM(height int64, toBcR bcReactor) *BcReactorFSM {
	return &BcReactorFSM{
		state:     unknown,
		startTime: time.Now(),
		pool:      NewBlockPool(height, toBcR),
		toBcR:     toBcR,
	}
}

// bReactorEventData is part of the message sent by the reactor to the FSM and used by the state handlers.
type bReactorEventData struct {
	peerID         p2p.ID
	err            error        // for peer error: timeout, slow; for processed block event if error occurred
	height         int64        // for status response; for processed block event
	block          *types.Block // for block response
	stateName      string       // for state timeout events
	length         int          // for block response event, length of received block, used to detect slow peers
	maxNumRequests int          // for request needed event, maximum number of pending requests
}

// Blockchain Reactor Events (the input to the state machine)
type bReactorEvent uint

const (
	// message type events
	startFSMEv = iota + 1
	statusResponseEv
	blockResponseEv
	processedBlockEv
	makeRequestsEv
	stopFSMEv

	// other events
	peerRemoveEv = iota + 256
	stateTimeoutEv
)

func (msg *bcReactorMessage) String() string {
	var dataStr string

	switch msg.event {
	case startFSMEv:
		dataStr = ""
	case statusResponseEv:
		dataStr = fmt.Sprintf("peer=%v height=%v", msg.data.peerID, msg.data.height)
	case blockResponseEv:
		dataStr = fmt.Sprintf("peer=%v block.height=%v length=%v",
			msg.data.peerID, msg.data.block.Height, msg.data.length)
	case processedBlockEv:
		dataStr = fmt.Sprintf("error=%v", msg.data.err)
	case makeRequestsEv:
		dataStr = ""
	case stopFSMEv:
		dataStr = ""
	case peerRemoveEv:
		dataStr = fmt.Sprintf("peer: %v is being removed by the switch", msg.data.peerID)
	case stateTimeoutEv:
		dataStr = fmt.Sprintf("state=%v", msg.data.stateName)
	default:
		dataStr = fmt.Sprintf("cannot interpret message data")
	}

	return fmt.Sprintf("%v: %v", msg.event, dataStr)
}

func (ev bReactorEvent) String() string {
	switch ev {
	case startFSMEv:
		return "startFSMEv"
	case statusResponseEv:
		return "statusResponseEv"
	case blockResponseEv:
		return "blockResponseEv"
	case processedBlockEv:
		return "processedBlockEv"
	case makeRequestsEv:
		return "makeRequestsEv"
	case stopFSMEv:
		return "stopFSMEv"
	case peerRemoveEv:
		return "peerRemoveEv"
	case stateTimeoutEv:
		return "stateTimeoutEv"
	default:
		return "event unknown"
	}

}

// states
var (
	unknown      *bcReactorFSMState
	waitForPeer  *bcReactorFSMState
	waitForBlock *bcReactorFSMState
	finished     *bcReactorFSMState
)

// timeouts for state timers
const (
	waitForPeerTimeout                 = 3 * time.Second
	waitForBlockAtCurrentHeightTimeout = 10 * time.Second
)

// errors
var (
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
	// handle return
	errPeerLowersItsHeight = errors.New("fast sync peer reports a height lower than previous")
	// handle return
	errNoPeerResponseForCurrentHeights = errors.New("fast sync timed out on peer block response for current heights")
	errNoPeerResponse                  = errors.New("fast sync timed out on peer block response")               // xx
	errBadDataFromPeer                 = errors.New("fast sync received block from wrong peer or block is bad") // xx
	errDuplicateBlock                  = errors.New("fast sync received duplicate block from peer")
	errBlockVerificationFailure        = errors.New("fast sync block verification failure")              // xx
	errSlowPeer                        = errors.New("fast sync peer is not sending us data fast enough") // xx

)

func init() {
	unknown = &bcReactorFSMState{
		name: "unknown",
		handle: func(fsm *BcReactorFSM, ev bReactorEvent, data bReactorEventData) (*bcReactorFSMState, error) {
			switch ev {
			case startFSMEv:
				// Broadcast Status message. Currently doesn't return non-nil error.
				fsm.toBcR.sendStatusRequest()
				return waitForPeer, nil

			case stopFSMEv:
				return finished, errNoErrorFinished

			default:
				return unknown, errInvalidEvent
			}
		},
	}

	waitForPeer = &bcReactorFSMState{
		name:    "waitForPeer",
		timeout: waitForPeerTimeout,
		enter: func(fsm *BcReactorFSM) {
			// Stop when leaving the state.
			fsm.resetStateTimer()
		},
		handle: func(fsm *BcReactorFSM, ev bReactorEvent, data bReactorEventData) (*bcReactorFSMState, error) {
			switch ev {
			case stateTimeoutEv:
				if data.stateName != "waitForPeer" {
					fsm.logger.Error("received a state timeout event for different state",
						"state", data.stateName)
					return waitForPeer, errTimeoutEventWrongState
				}
				// There was no statusResponse received from any peer.
				// Should we send status request again?
				return finished, errNoTallerPeer

			case statusResponseEv:
				if err := fsm.pool.UpdatePeer(data.peerID, data.height); err != nil {
					if fsm.pool.NumPeers() == 0 {
						return waitForPeer, err
					}
				}
				if fsm.stateTimer != nil {
					fsm.stateTimer.Stop()
				}
				return waitForBlock, nil

			case stopFSMEv:
				if fsm.stateTimer != nil {
					fsm.stateTimer.Stop()
				}
				return finished, errNoErrorFinished

			default:
				return waitForPeer, errInvalidEvent
			}
		},
	}

	waitForBlock = &bcReactorFSMState{
		name:    "waitForBlock",
		timeout: waitForBlockAtCurrentHeightTimeout,
		enter: func(fsm *BcReactorFSM) {
			// Stop when leaving the state.
			fsm.resetStateTimer()
		},
		handle: func(fsm *BcReactorFSM, ev bReactorEvent, data bReactorEventData) (*bcReactorFSMState, error) {
			switch ev {

			case statusResponseEv:
				err := fsm.pool.UpdatePeer(data.peerID, data.height)
				if fsm.pool.NumPeers() == 0 {
					return waitForPeer, err
				}
				if fsm.pool.ReachedMaxHeight() {
					return finished, err
				}
				return waitForBlock, err

			case blockResponseEv:
				fsm.logger.Debug("blockResponseEv", "H", data.block.Height)
				err := fsm.pool.AddBlock(data.peerID, data.block, data.length)
				if err != nil {
					// A block was received that was unsolicited, from unexpected peer, or that we already have it.
					// Ignore block, remove peer and send error to switch.
					fsm.pool.RemovePeer(data.peerID, err)
					fsm.toBcR.sendPeerError(err, data.peerID)
				}
				if fsm.pool.NumPeers() == 0 {
					return waitForPeer, err
				}
				return waitForBlock, err

			case processedBlockEv:
				if data.err != nil {
					first, second, _ := fsm.pool.FirstTwoBlocksAndPeers()
					fsm.logger.Error("error processing block", "err", data.err,
						"first", first.block.Height, "second", second.block.Height)
					fsm.logger.Error("send peer error for", "peer", first.peer.ID)
					fsm.toBcR.sendPeerError(data.err, first.peer.ID)
					fsm.logger.Error("send peer error for", "peer", second.peer.ID)
					fsm.toBcR.sendPeerError(data.err, second.peer.ID)
					// Remove the first two blocks. This will also remove the peers
					fsm.pool.InvalidateFirstTwoBlocks(data.err)
				} else {
					fsm.pool.ProcessedCurrentHeightBlock()
					// Since we advanced one block reset the state timer
					fsm.resetStateTimer()
				}

				// Both cases above may result in achieving maximum height.
				if fsm.pool.ReachedMaxHeight() {
					return finished, nil
				}

				return waitForBlock, data.err

			case peerRemoveEv:
				// This event is sent by the switch to remove disconnected and errored peers.
				fsm.pool.RemovePeer(data.peerID, data.err)
				if fsm.pool.NumPeers() == 0 {
					return waitForPeer, nil
				}
				if fsm.pool.ReachedMaxHeight() {
					return finished, nil
				}
				return waitForBlock, nil

			case makeRequestsEv:
				fsm.makeNextRequests(data.maxNumRequests)
				return waitForBlock, nil

			case stateTimeoutEv:
				if data.stateName != "waitForBlock" {
					fsm.logger.Error("received a state timeout event for different state",
						"state", data.stateName)
					return waitForBlock, errTimeoutEventWrongState
				}
				// We haven't received the block at current height or height+1. Remove peer.
				fsm.pool.RemovePeerAtCurrentHeights(errNoPeerResponseForCurrentHeights)
				fsm.resetStateTimer()
				if fsm.pool.NumPeers() == 0 {
					return waitForPeer, errNoPeerResponseForCurrentHeights
				}
				if fsm.pool.ReachedMaxHeight() {
					return finished, nil
				}
				return waitForBlock, errNoPeerResponseForCurrentHeights

			case stopFSMEv:
				if fsm.stateTimer != nil {
					fsm.stateTimer.Stop()
				}
				return finished, errNoErrorFinished

			default:
				return waitForBlock, errInvalidEvent
			}
		},
	}

	finished = &bcReactorFSMState{
		name: "finished",
		enter: func(fsm *BcReactorFSM) {
			fsm.logger.Info("Time to switch to consensus reactor!", "height", fsm.pool.Height)
			fsm.toBcR.switchToConsensus()
			fsm.cleanup()
		},
		handle: func(fsm *BcReactorFSM, ev bReactorEvent, data bReactorEventData) (*bcReactorFSMState, error) {
			return finished, nil
		},
	}
}

// Interface used by FSM for sending Block and Status requests,
// informing of peer errors and state timeouts
// Implemented by BlockchainReactor and tests
type bcReactor interface {
	sendStatusRequest()
	sendBlockRequest(peerID p2p.ID, height int64) error
	sendPeerError(err error, peerID p2p.ID)
	resetStateTimer(name string, timer **time.Timer, timeout time.Duration)
	switchToConsensus()
}

// SetLogger sets the FSM logger.
func (fsm *BcReactorFSM) SetLogger(l log.Logger) {
	fsm.logger = l
	fsm.pool.SetLogger(l)
}

// Start starts the FSM.
func (fsm *BcReactorFSM) Start() {
	_ = fsm.Handle(&bcReactorMessage{event: startFSMEv})
}

// Handle processes messages and events sent to the FSM.
func (fsm *BcReactorFSM) Handle(msg *bcReactorMessage) error {
	fsm.mtx.Lock()
	defer fsm.mtx.Unlock()
	fsm.logger.Debug("FSM received", "event", msg, "state", fsm.state)

	if fsm.state == nil {
		fsm.state = unknown
	}
	next, err := fsm.state.handle(fsm, msg.event, msg.data)
	if err != nil {
		fsm.logger.Error("FSM event handler returned", "err", err,
			"state", fsm.state, "event", msg.event)
	}

	oldState := fsm.state.name
	fsm.transition(next)
	if oldState != fsm.state.name {
		fsm.logger.Info("FSM changed state", "new_state", fsm.state)
	}
	return err
}

func (fsm *BcReactorFSM) transition(next *bcReactorFSMState) {
	if next == nil {
		return
	}
	if fsm.state != next {
		fsm.state = next
		if next.enter != nil {
			next.enter(fsm)
		}
	}
}

// Called when entering an FSM state in order to detect lack of progress in the state machine.
// Note the use of the 'bcr' interface to facilitate testing without timer expiring.
func (fsm *BcReactorFSM) resetStateTimer() {
	fsm.toBcR.resetStateTimer(fsm.state.name, &fsm.stateTimer, fsm.state.timeout)
}

func (fsm *BcReactorFSM) isCaughtUp() bool {
	return fsm.state == finished
}

func (fsm *BcReactorFSM) makeNextRequests(maxNumRequests int) {
	fsm.pool.MakeNextRequests(maxNumRequests)
}

func (fsm *BcReactorFSM) cleanup() {
	fsm.pool.Cleanup()
}

// NeedsBlocks checks if more block requests are required.
func (fsm *BcReactorFSM) NeedsBlocks() bool {
	fsm.mtx.Lock()
	defer fsm.mtx.Unlock()
	return fsm.state.name == "waitForBlock" && fsm.pool.NeedsBlocks()
}

// FirstTwoBlocks returns the two blocks at pool height and height+1
func (fsm *BcReactorFSM) FirstTwoBlocks() (first, second *types.Block, err error) {
	fsm.mtx.Lock()
	defer fsm.mtx.Unlock()
	firstBP, secondBP, err := fsm.pool.FirstTwoBlocksAndPeers()
	if err == nil {
		first = firstBP.block
		second = secondBP.block
	}
	return
}

// Status returns the pool's height and the maximum peer height.
func (fsm *BcReactorFSM) Status() (height, maxPeerHeight int64) {
	fsm.mtx.Lock()
	defer fsm.mtx.Unlock()
	return fsm.pool.Height, fsm.pool.MaxPeerHeight
}
