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

type blockData struct {
	block  *types.Block
	peerId p2p.ID
}

func (bd *blockData) String() string {
	if bd == nil {
		return fmt.Sprintf("blockData nil")
	}
	if bd.block == nil {
		return fmt.Sprintf("block: nil peer: %v", bd.peerId)
	}
	return fmt.Sprintf("block: %v peer: %v", bd.block.Height, bd.peerId)

}

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

	blocks map[int64]*blockData
	height int64 // processing height

	peers         map[p2p.ID]*bpPeer
	maxPeerHeight int64

	store *BlockStore

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
				if err := fsm.setPeerHeight(data.peerId, data.height); err != nil {
					if len(fsm.peers) == 0 {
						return waitForPeer, err
					}
				}

				// send first block requests
				err := fsm.sendRequestBatch()
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
				err := fsm.setPeerHeight(data.peerId, data.height)
				return waitForBlock, err

			case blockResponseEv:
				// add block to fsm.blocks
				fsm.logger.Debug("blockResponseEv", "H", data.block.Height)
				err := fsm.addBlock(data.peerId, data.block, data.length)
				if err != nil {
					// unsolicited, from different peer, already have it..
					fsm.removePeer(data.peerId, err)
					// ignore block
					return waitForBlock, err
				}

				fsm.sendSignalToProcessBlock()

				if fsm.state.timer != nil {
					fsm.state.timer.Stop()
				}
				return waitForBlock, nil

			case tryProcessBlockEv:
				fsm.logger.Debug("FSM blocks", "blocks", fsm.blocks, "fsm_height", fsm.height)
				if err := fsm.processBlock(); err != nil {
					if err == errMissingBlocks {
						// continue so we ask for more blocks
					}
					if err == errBlockVerificationFailure {
						// remove peers that sent us those blocks, blocks will also be removed
						first := fsm.blocks[fsm.height].peerId
						fsm.removePeer(first, err)
						second := fsm.blocks[fsm.height+1].peerId
						fsm.removePeer(second, err)
					}
				} else {
					delete(fsm.blocks, fsm.height)
					fsm.height++
					fsm.processSignalActive = false
					fsm.removeShortPeers()

					// processed block, check if we are done
					if fsm.height >= fsm.maxPeerHeight {
						// TODO should we wait for more status responses in case a high peer is slow?
						fsm.bcr.switchToConsensus()
						return finished, nil
					}
				}
				// get other block(s)
				err := fsm.sendRequestBatch()
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
				fsm.removePeer(data.peerId, data.err)
				err := fsm.sendRequestBatch()
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

func NewFSM(store *BlockStore, bcr sendMessage) *bReactorFSM {
	messageCh := make(chan bReactorMessageData, maxTotalMessages)

	return &bReactorFSM{
		state: unknown,
		store: store,

		blocks: make(map[int64]*blockData),

		peers:     make(map[p2p.ID]*bpPeer),
		height:    store.Height() + 1,
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

// WIP
// TODO - pace the requests to peers
func (fsm *bReactorFSM) sendRequestBatch() error {
	// remove slow and timed out peers
	for _, peer := range fsm.peers {
		if err := peer.isGood(); err != nil {
			fsm.logger.Info("Removing bad peer", "peer", peer.id, "err", err)
			fsm.removePeer(peer.id, err)
		}
	}

	var err error
	// make requests
	for i := 0; i < maxRequestBatchSize; i++ {
		// request height
		height := fsm.height + int64(i)
		if height > fsm.maxPeerHeight {
			fsm.logger.Debug("Will not send request for", "height", height)
			return err
		}
		req := fsm.blocks[height]
		if req == nil {
			// make new request
			err = fsm.sendRequest(height)
			if err != nil {
				// couldn't find a good peer or couldn't communicate with it
			}
		}
	}

	return nil
}

func (fsm *bReactorFSM) sendRequest(height int64) error {
	// make requests
	// TODO - sort peers in order of goodness
	fsm.logger.Debug("try to send request for", "height", height)
	for _, peer := range fsm.peers {
		// Send Block Request message to peer
		if peer.height < height {
			continue
		}
		fsm.logger.Debug("Try to send request to peer", "peer", peer.id, "height", height)
		err := fsm.bcr.sendBlockRequest(peer.id, height)
		if err == errSendQueueFull {
			fsm.logger.Error("cannot send request, queue full", "peer", peer.id, "height", height)
			continue
		}
		if err == errNilPeerForBlockRequest {
			// this peer does not exist in the switch, delete locally
			fsm.logger.Error("peer doesn't exist in the switch", "peer", peer.id)
			fsm.deletePeer(peer.id)
			continue
		}

		fsm.logger.Debug("Sent request to peer", "peer", peer.id, "height", height)

		// reserve space for block
		fsm.blocks[height] = &blockData{peerId: peer.id, block: nil}
		fsm.peers[peer.id].incrPending()
		return nil
	}

	return errNoPeerFoundForRequest
}

// Sets the peer's blockchain height.
func (fsm *bReactorFSM) setPeerHeight(peerID p2p.ID, height int64) error {

	peer := fsm.peers[peerID]

	if height < fsm.height {
		fsm.logger.Info("Peer height too small", "peer", peerID, "height", height, "fsm_height", fsm.height)

		// Don't add or update a peer that is not useful.
		if peer != nil {
			fsm.logger.Info("remove short peer", "peer", peerID, "height", height, "fsm_height", fsm.height)
			fsm.removePeer(peerID, errPeerTooShort)
		}
		return errPeerTooShort
	}

	if peer == nil {
		peer = newBPPeer(peerID, height, fsm.processPeerError)
		peer.setLogger(fsm.logger.With("peer", peerID))
		fsm.peers[peerID] = peer
	} else {
		// remove any requests made for heights in (height, peer.height]
		for blockHeight, bData := range fsm.blocks {
			if bData.peerId == peerID && blockHeight > height {
				delete(fsm.blocks, blockHeight)
			}
		}
	}

	peer.height = height
	if height > fsm.maxPeerHeight {
		fsm.maxPeerHeight = height
	}
	return nil
}

func (fsm *bReactorFSM) getMaxPeerHeight() int64 {
	return fsm.maxPeerHeight
}

// called from:
// - the switch from its go routing
// - when peer times out from the timer go routine.
// Send message to FSM
func (fsm *bReactorFSM) processPeerError(err error, peerID p2p.ID) {
	msgData := bReactorMessageData{
		event: peerErrEv,
		data: bReactorEventData{
			err:    err,
			peerId: peerID,
		},
	}
	sendMessageToFSM(fsm, msgData)
}

// called by the switch on peer error
func (fsm *bReactorFSM) RemovePeer(peerID p2p.ID) {
	fsm.logger.Info("Switch removes peer", "peer", peerID, "fsm_height", fsm.height)
	fsm.processPeerError(errSwitchPeerErr, peerID)
}

// called every time FSM advances its height
func (fsm *bReactorFSM) removeShortPeers() {
	for _, peer := range fsm.peers {
		if peer.height < fsm.height {
			fsm.logger.Info("removeShortPeers", "peer", peer.id)
			fsm.removePeer(peer.id, nil)
		}
	}
}

// removes any blocks and requests associated with the peer, deletes the peer and informs the switch if needed.
func (fsm *bReactorFSM) removePeer(peerID p2p.ID, err error) {
	fsm.logger.Debug("removePeer", "peer", peerID, "err", err)
	// remove all data for blocks waiting for the peer or not processed yet
	for h, bData := range fsm.blocks {
		if bData.peerId == peerID {
			if h == fsm.height {
				fsm.processSignalActive = false
			}
			delete(fsm.blocks, h)
		}
	}

	// delete peer
	fsm.deletePeer(peerID)

	// recompute maxPeerHeight
	fsm.maxPeerHeight = 0
	for _, peer := range fsm.peers {
		if peer.height > fsm.maxPeerHeight {
			fsm.maxPeerHeight = peer.height
		}
	}

	// send error to switch if not coming from it
	if err != nil && err != errSwitchPeerErr {
		fsm.bcr.sendPeerError(err, peerID)
	}
}

// stops the peer timer and deletes the peer
func (fsm *bReactorFSM) deletePeer(peerID p2p.ID) {
	if p, exist := fsm.peers[peerID]; exist && p.timeout != nil {
		p.timeout.Stop()
	}
	delete(fsm.peers, peerID)
}

// Validates that the block comes from the peer it was expected from and stores it in the 'blocks' map.
func (fsm *bReactorFSM) addBlock(peerID p2p.ID, block *types.Block, blockSize int) error {

	blockData := fsm.blocks[block.Height]

	if blockData == nil {
		fsm.logger.Error("peer sent us a block we didn't expect", "peer", peerID, "curHeight", fsm.height, "blockHeight", block.Height)
		return errBadDataFromPeer
	}

	if blockData.peerId != peerID {
		fsm.logger.Error("invalid peer", "peer", peerID, "blockHeight", block.Height)
		return errBadDataFromPeer
	}
	if blockData.block != nil {
		fsm.logger.Error("already have a block for height", "height", block.Height)
		return errBadDataFromPeer
	}

	fsm.blocks[block.Height].block = block
	peer := fsm.peers[peerID]
	if peer != nil {
		peer.decrPending(blockSize)
	}
	return nil
}

func (fsm *bReactorFSM) sendSignalToProcessBlock() {
	first := fsm.blocks[fsm.height]
	second := fsm.blocks[fsm.height+1]
	if first == nil || first.block == nil || second == nil || second.block == nil {
		// We need both to sync the first block.
		fsm.logger.Debug("No need to send signal to process, blocks missing")
		return
	}

	fsm.logger.Debug("Send signal to process", "first", fsm.height, "second", fsm.height+1)
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
func (fsm *bReactorFSM) processBlock() error {
	first := fsm.blocks[fsm.height]
	second := fsm.blocks[fsm.height+1]
	if first == nil || first.block == nil || second == nil || second.block == nil {
		// We need both to sync the first block.
		fsm.logger.Debug("process blocks doesn't have the blocks", "first", first, "second", second)
		return errMissingBlocks
	}
	fsm.logger.Debug("process blocks", "first", first.block.Height, "second", second.block.Height)
	fsm.logger.Debug("FSM blocks", "blocks", fsm.blocks)

	if err := fsm.bcr.processBlocks(first.block, second.block); err != nil {
		fsm.logger.Error("process blocks returned error", "err", err, "first", first.block.Height, "second", second.block.Height)
		fsm.logger.Error("FSM blocks", "blocks", fsm.blocks)
		return err
	}
	return nil
}

func (fsm *bReactorFSM) IsFinished() bool {
	return fsm.state == finished
}
