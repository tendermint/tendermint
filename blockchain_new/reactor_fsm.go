package blockchain_new

import (
	"errors"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

var (
	// should be >= 2
	maxRequestBatchSize = 40
)

// Blockchain Reactor State
type bReactorFSMState struct {
	name string
	// called when transitioning out of current state
	handle func(*bReactorFSM, bReactorEvent, bReactorEventData) (next *bReactorFSMState, err error)
	// called when entering the state
	enter func(fsm *bReactorFSM)

	// timer to ensure FSM is not stuck in a state forever
	timer   *time.Timer
	timeout time.Duration
}

func (s *bReactorFSMState) String() string {
	return s.name
}

// Blockchain Reactor State Machine
type bReactorFSM struct {
	logger    log.Logger
	startTime time.Time

	state *bReactorFSMState

	pool *blockPool

	// channel to receive messages
	messageCh           chan bReactorMessageData
	processSignalActive bool

	// interface used to send StatusRequest, BlockRequest, errors
	bcr sendMessage
}

// bReactorEventData is part of the message sent by the reactor to the FSM and used by the state handlers
type bReactorEventData struct {
	peerId    p2p.ID
	err       error        // for peer error: timeout, slow,
	height    int64        // for status response
	block     *types.Block // for block response
	stateName string       // for state timeout events
	length    int          // for block response to detect slow peers

}

// bReactorMessageData structure is used by the reactor when sending messages to the FSM.
type bReactorMessageData struct {
	event bReactorEvent
	data  bReactorEventData
}

func (msg *bReactorMessageData) String() string {
	var dataStr string

	switch msg.event {
	case startFSMEv:
		dataStr = ""
	case statusResponseEv:
		dataStr = fmt.Sprintf("peer: %v height: %v", msg.data.peerId, msg.data.height)
	case blockResponseEv:
		dataStr = fmt.Sprintf("peer: %v block.height: %v lenght: %v", msg.data.peerId, msg.data.block.Height, msg.data.length)
	case tryProcessBlockEv:
		dataStr = ""
	case stopFSMEv:
		dataStr = ""
	case peerErrEv:
		dataStr = fmt.Sprintf("peer: %v err: %v", msg.data.peerId, msg.data.err)
	case stateTimeoutEv:
		dataStr = fmt.Sprintf("state: %v", msg.data.stateName)
	default:
		dataStr = fmt.Sprintf("cannot interpret message data")
		return "event unknown"
	}

	return fmt.Sprintf("event: %v %v", msg.event, dataStr)
}

// Blockchain Reactor Events (the input to the state machine).
type bReactorEvent uint

const (
	// message type events
	startFSMEv = iota + 1
	statusResponseEv
	blockResponseEv
	tryProcessBlockEv
	stopFSMEv

	// other events
	peerErrEv = iota + 256
	stateTimeoutEv
)

func (ev bReactorEvent) String() string {
	switch ev {
	case startFSMEv:
		return "startFSMEv"
	case statusResponseEv:
		return "statusResponseEv"
	case blockResponseEv:
		return "blockResponseEv"
	case tryProcessBlockEv:
		return "tryProcessBlockEv"
	case stopFSMEv:
		return "stopFSMEv"
	case peerErrEv:
		return "peerErrEv"
	case stateTimeoutEv:
		return "stateTimeoutEv"
	default:
		return "event unknown"
	}

}

// states
var (
	unknown      *bReactorFSMState
	waitForPeer  *bReactorFSMState
	waitForBlock *bReactorFSMState
	finished     *bReactorFSMState
)

// state timers
var (
	waitForPeerTimeout  = 20 * time.Second
	waitForBlockTimeout = 30 * time.Second // > peerTimeout which is 15 sec
)

// errors
var (
	// errors
	errInvalidEvent             = errors.New("invalid event in current state")
	errNoErrorFinished          = errors.New("FSM is finished")
	errNoPeerResponse           = errors.New("FSM timed out on peer response")
	errNoPeerFoundForRequest    = errors.New("cannot use peer")
	errBadDataFromPeer          = errors.New("received from wrong peer or bad block")
	errMissingBlocks            = errors.New("missing blocks")
	errBlockVerificationFailure = errors.New("block verification failure, redo")
	errNilPeerForBlockRequest   = errors.New("nil peer for block request")
	errSendQueueFull            = errors.New("block request not made, send-queue is full")
	errPeerTooShort             = errors.New("peer height too low, peer was either not added " +
		"or removed after status update")
	errSwitchPeerErr = errors.New("switch detected peer error")
	errSlowPeer      = errors.New("peer is not sending us data fast enough")
)

func init() {
	unknown = &bReactorFSMState{
		name: "unknown",
		handle: func(fsm *bReactorFSM, ev bReactorEvent, data bReactorEventData) (*bReactorFSMState, error) {
			switch ev {
			case startFSMEv:
				// Broadcast Status message. Currently doesn't return non-nil error.
				_ = fsm.bcr.sendStatusRequest()
				if fsm.state.timer != nil {
					fsm.state.timer.Stop()
				}
				return waitForPeer, nil

			case stopFSMEv:
				// cleanup
				return finished, errNoErrorFinished

			default:
				return unknown, errInvalidEvent
			}
		},
	}

	waitForPeer = &bReactorFSMState{
		name:    "waitForPeer",
		timeout: waitForPeerTimeout,
		enter: func(fsm *bReactorFSM) {
			// stop when leaving the state
			fsm.resetStateTimer(waitForPeer)
		},
		handle: func(fsm *bReactorFSM, ev bReactorEvent, data bReactorEventData) (*bReactorFSMState, error) {
			switch ev {
			case stateTimeoutEv:
				// no statusResponse received from any peer
				// Should we send status request again?
				if fsm.state.timer != nil {
					fsm.state.timer.Stop()
				}
				return finished, errNoPeerResponse

			case statusResponseEv:
				// update peer
				if err := fsm.pool.updatePeer(data.peerId, data.height, fsm.processPeerError); err != nil {
					if len(fsm.pool.peers) == 0 {
						return waitForPeer, err
					}
				}

				// send first block requests
				err := fsm.pool.sendRequestBatch(fsm.bcr.sendBlockRequest)
				if err != nil {
					// wait for more peers or state timeout
					return waitForPeer, err
				}
				if fsm.state.timer != nil {
					fsm.state.timer.Stop()
				}
				return waitForBlock, nil

			case stopFSMEv:
				// cleanup
				return finished, errNoErrorFinished

			default:
				return waitForPeer, errInvalidEvent
			}
		},
	}

	waitForBlock = &bReactorFSMState{
		name:    "waitForBlock",
		timeout: waitForBlockTimeout,
		enter: func(fsm *bReactorFSM) {
			// stop when leaving the state or receiving a block
			fsm.resetStateTimer(waitForBlock)
		},
		handle: func(fsm *bReactorFSM, ev bReactorEvent, data bReactorEventData) (*bReactorFSMState, error) {
			switch ev {
			case stateTimeoutEv:
				// no blockResponse
				// Should we send status request again? Switch to consensus?
				// Note that any unresponsive peers have been already removed by their timer expiry handler.
				if fsm.state.timer != nil {
					fsm.state.timer.Stop()
				}
				return finished, errNoPeerResponse

			case statusResponseEv:
				err := fsm.pool.updatePeer(data.peerId, data.height, fsm.processPeerError)
				return waitForBlock, err

			case blockResponseEv:
				fsm.logger.Debug("blockResponseEv", "H", data.block.Height)
				err := fsm.pool.addBlock(data.peerId, data.block, data.length)
				if err != nil {
					// unsolicited, from different peer, already have it..
					fsm.pool.removePeer(data.peerId, err)
					// ignore block
					// send error to switch
					if err != nil {
						fsm.bcr.sendPeerError(err, data.peerId)
					}
					fsm.processSignalActive = false
					return waitForBlock, err
				}

				fsm.sendSignalToProcessBlock()

				if fsm.state.timer != nil {
					fsm.state.timer.Stop()
				}
				return waitForBlock, nil

			case tryProcessBlockEv:
				fsm.logger.Debug("FSM blocks", "blocks", fsm.pool.blocks, "fsm_height", fsm.pool.height)
				// process block, it detects errors and deals with them
				fsm.processBlock()

				// processed block, check if we are done
				if fsm.pool.reachedMaxHeight() {
					// TODO should we wait for more status responses in case a high peer is slow?
					fsm.bcr.switchToConsensus()
					return finished, nil
				}

				// get other block(s)
				err := fsm.pool.sendRequestBatch(fsm.bcr.sendBlockRequest)
				if err != nil {
					// TBD on what to do here...
					// wait for more peers or state timeout
				}
				fsm.sendSignalToProcessBlock()

				if fsm.state.timer != nil {
					fsm.state.timer.Stop()
				}
				return waitForBlock, err

			case peerErrEv:
				// This event is sent by:
				// - the switch to report disconnected and errored peers
				// - when the FSM pool peer times out
				fsm.pool.removePeer(data.peerId, data.err)
				if data.err != nil && data.err != errSwitchPeerErr {
					// not sent by the switch so report it
					fsm.bcr.sendPeerError(data.err, data.peerId)
				}
				err := fsm.pool.sendRequestBatch(fsm.bcr.sendBlockRequest)
				if err != nil {
					// TBD on what to do here...
					// wait for more peers or state timeout
				}
				if fsm.state.timer != nil {
					fsm.state.timer.Stop()
				}
				return waitForBlock, err

			case stopFSMEv:
				// cleanup
				return finished, errNoErrorFinished

			default:
				return waitForBlock, errInvalidEvent
			}
		},
	}

	finished = &bReactorFSMState{
		name: "finished",
		enter: func(fsm *bReactorFSM) {
			// cleanup
		},
		handle: func(fsm *bReactorFSM, ev bReactorEvent, data bReactorEventData) (*bReactorFSMState, error) {
			return nil, nil
		},
	}

}

func NewFSM(height int64, bcr sendMessage) *bReactorFSM {
	messageCh := make(chan bReactorMessageData, maxTotalMessages)

	return &bReactorFSM{
		state:     unknown,
		pool:      newBlockPool(height),
		bcr:       bcr,
		messageCh: messageCh,
	}
}

func sendMessageToFSM(fsm *bReactorFSM, msg bReactorMessageData) {
	fsm.logger.Debug("send message to FSM", "msg", msg.String())
	fsm.messageCh <- msg
}

func (fsm *bReactorFSM) setLogger(l log.Logger) {
	fsm.logger = l
	fsm.pool.setLogger(l)
}

// starts the FSM go routine
func (fsm *bReactorFSM) start() {
	go fsm.startRoutine()
	fsm.startTime = time.Now()
}

// stops the FSM go routine
func (fsm *bReactorFSM) stop() {
	msg := bReactorMessageData{
		event: stopFSMEv,
	}
	sendMessageToFSM(fsm, msg)
}

// start the FSM
func (fsm *bReactorFSM) startRoutine() {

	_ = fsm.handle(&bReactorMessageData{event: startFSMEv})

forLoop:
	for {
		select {
		case msg := <-fsm.messageCh:
			fsm.logger.Debug("FSM Received message", "msg", msg.String())
			_ = fsm.handle(&msg)
			if msg.event == stopFSMEv {
				break forLoop
			}
			// TODO - stop also on some errors returned by handle

		default:
		}
	}
}

// handle processes messages and events sent to the FSM.
func (fsm *bReactorFSM) handle(msg *bReactorMessageData) error {
	fsm.logger.Debug("FSM received event", "event", msg.event, "state", fsm.state.name)

	if fsm.state == nil {
		fsm.state = unknown
	}
	next, err := fsm.state.handle(fsm, msg.event, msg.data)
	if err != nil {
		fsm.logger.Error("FSM event handler returned", "err", err, "state", fsm.state.name, "event", msg.event)
	}

	oldState := fsm.state.name
	fsm.transition(next)
	if oldState != fsm.state.name {
		fsm.logger.Info("FSM changed state", "old_state", oldState, "event", msg.event, "new_state", fsm.state.name)
	}
	return err
}

func (fsm *bReactorFSM) transition(next *bReactorFSMState) {
	if next == nil {
		return
	}
	fsm.logger.Debug("changes state: ", "old", fsm.state.name, "new", next.name)

	if fsm.state != next {
		fsm.state = next
		if next.enter != nil {
			next.enter(fsm)
		}
	}
}

// Interface for sending Block and Status requests
// Implemented by BlockchainReactor and tests
type sendMessage interface {
	sendStatusRequest() error
	sendBlockRequest(peerID p2p.ID, height int64) error
	sendPeerError(err error, peerID p2p.ID)
	processBlocks(first *types.Block, second *types.Block) error
	resetStateTimer(name string, timer *time.Timer, timeout time.Duration, f func())
	switchToConsensus()
}

// FSM state timeout handler
func (fsm *bReactorFSM) sendStateTimeoutEvent(stateName string) {
	// Check that the timeout is for the state we are currently in to prevent wrong transitions.
	if stateName == fsm.state.name {
		msg := bReactorMessageData{
			event: stateTimeoutEv,
			data: bReactorEventData{
				stateName: stateName,
			},
		}
		sendMessageToFSM(fsm, msg)
	}
}

// This is called when entering an FSM state in order to detect lack of progress in the state machine.
// Note the use of the 'bcr' interface to facilitate testing without timer running.
func (fsm *bReactorFSM) resetStateTimer(state *bReactorFSMState) {
	fsm.bcr.resetStateTimer(state.name, state.timer, state.timeout, func() {
		fsm.sendStateTimeoutEvent(state.name)
	})
}

// called by the switch on peer error
func (fsm *bReactorFSM) RemovePeer(peerID p2p.ID) {
	fsm.logger.Info("Switch removes peer", "peer", peerID, "fsm_height", fsm.pool.height)
	fsm.processPeerError(errSwitchPeerErr, peerID)
}

func (fsm *bReactorFSM) sendSignalToProcessBlock() {
	_, _, err := fsm.pool.getNextTwoBlocks()
	if err != nil {
		fsm.logger.Debug("No need to send signal, blocks missing")
		return
	}

	fsm.logger.Debug("Send signal to process blocks")
	if fsm.processSignalActive {
		fsm.logger.Debug("..already sent")
		return
	}

	msgData := bReactorMessageData{
		event: tryProcessBlockEv,
		data:  bReactorEventData{},
	}
	sendMessageToFSM(fsm, msgData)
	fsm.processSignalActive = true
}

// Processes block at height H = fsm.height. Expects both H and H+1 to be available
func (fsm *bReactorFSM) processBlock() {
	first, second, err := fsm.pool.getNextTwoBlocks()
	if err != nil {
		fsm.processSignalActive = false
		return
	}
	fsm.logger.Debug("process blocks", "first", first.block.Height, "second", second.block.Height)
	fsm.logger.Debug("FSM blocks", "blocks", fsm.pool.blocks)

	if err = fsm.bcr.processBlocks(first.block, second.block); err != nil {
		fsm.logger.Error("process blocks returned error", "err", err, "first", first.block.Height, "second", second.block.Height)
		fsm.logger.Error("FSM blocks", "blocks", fsm.pool.blocks)
		fsm.pool.invalidateFirstTwoBlocks(err)
		fsm.bcr.sendPeerError(err, first.peerId)
		fsm.bcr.sendPeerError(err, second.peerId)

	} else {
		fsm.pool.processedCurrentHeightBlock()
	}

	fsm.processSignalActive = false
}

func (fsm *bReactorFSM) IsFinished() bool {
	return fsm.state == finished
}
