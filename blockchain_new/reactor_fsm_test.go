package blockchain_new

import (
	"github.com/stretchr/testify/assert"
	cmn "github.com/tendermint/tendermint/libs/common"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"

	"testing"
	"time"
)

var (
	failSendStatusRequest bool
	failSendBlockRequest  bool
	numStatusRequests     int
	numBlockRequests      int
)

type lastBlockRequestT struct {
	peerID p2p.ID
	height int64
}

var lastBlockRequest lastBlockRequestT

type lastPeerErrorT struct {
	peerID p2p.ID
	err    error
}

var lastPeerError lastPeerErrorT

var stateTimerStarts map[string]int

func resetTestValues() {
	stateTimerStarts = make(map[string]int)
	failSendBlockRequest = false
	failSendStatusRequest = false
	numStatusRequests = 0
	numBlockRequests = 0
	lastBlockRequest.peerID = ""
	lastBlockRequest.height = 0
	lastPeerError.peerID = ""
	lastPeerError.err = nil
}

type fsmStepTestValues struct {
	currentState  string
	event         bReactorEvent
	expectedState string

	// input
	failStatusReq       bool
	shouldSendStatusReq bool

	failBlockReq      bool
	blockReqIncreased bool

	data bReactorEventData

	expectedLastBlockReq *lastBlockRequestT
}

// WIP
func TestFSMTransitionSequences(t *testing.T) {
	maxRequestBatchSize = 2
	fsmTransitionSequenceTests := [][]fsmStepTestValues{
		{
			{currentState: "unknown", event: startFSMEv, shouldSendStatusReq: true,
				expectedState: "waitForPeer"},
			{currentState: "waitForPeer", event: statusResponseEv,
				data:              bReactorEventData{peerId: "P1", height: 10},
				blockReqIncreased: true,
				expectedState:     "waitForBlock"},
		},
	}

	for _, tt := range fsmTransitionSequenceTests {
		// Create and start the FSM
		testBcR := &testReactor{logger: log.TestingLogger()}
		blockDB := dbm.NewMemDB()
		store := NewBlockStore(blockDB)
		fsm := NewFSM(store, testBcR)
		fsm.setLogger(log.TestingLogger())
		resetTestValues()

		// always start from unknown
		fsm.resetStateTimer(unknown)
		assert.Equal(t, 1, stateTimerStarts[unknown.name])

		for _, step := range tt {
			assert.Equal(t, step.currentState, fsm.state.name)
			failSendStatusRequest = step.failStatusReq
			failSendBlockRequest = step.failBlockReq

			oldNumStatusRequests := numStatusRequests
			oldNumBlockRequests := numBlockRequests

			_ = sendEventToFSM(fsm, step.event, step.data)
			if step.shouldSendStatusReq {
				assert.Equal(t, oldNumStatusRequests+1, numStatusRequests)
			} else {
				assert.Equal(t, oldNumStatusRequests, numStatusRequests)
			}

			if step.blockReqIncreased {
				assert.Equal(t, oldNumBlockRequests+maxRequestBatchSize, numBlockRequests)
			} else {
				assert.Equal(t, oldNumBlockRequests, numBlockRequests)
			}

			assert.Equal(t, step.expectedState, fsm.state.name)
		}
	}
}

func TestReactorFSMBasic(t *testing.T) {
	maxRequestBatchSize = 2

	resetTestValues()
	// Create and start the FSM
	testBcR := &testReactor{logger: log.TestingLogger()}
	blockDB := dbm.NewMemDB()
	store := NewBlockStore(blockDB)
	fsm := NewFSM(store, testBcR)
	fsm.setLogger(log.TestingLogger())

	if err := fsm.handle(&bReactorMessageData{event: startFSMEv}); err != nil {
	}

	// Check that FSM sends a status request message
	assert.Equal(t, 1, numStatusRequests)
	assert.Equal(t, waitForPeer.name, fsm.state.name)

	// Send a status response message to FSM
	peerID := p2p.ID(cmn.RandStr(12))
	sendStatusResponse2(fsm, peerID, 10)

	// Check that FSM sends a block request message and...
	assert.Equal(t, maxRequestBatchSize, numBlockRequests)
	// ... the block request has the expected height
	assert.Equal(t, int64(maxRequestBatchSize), lastBlockRequest.height)
	assert.Equal(t, waitForBlock.name, fsm.state.name)
}

func TestReactorFSMPeerTimeout(t *testing.T) {
	maxRequestBatchSize = 2
	resetTestValues()
	peerTimeout = 20 * time.Millisecond
	// Create and start the FSM
	testBcR := &testReactor{logger: log.TestingLogger()}
	blockDB := dbm.NewMemDB()
	store := NewBlockStore(blockDB)
	fsm := NewFSM(store, testBcR)
	fsm.setLogger(log.TestingLogger())
	fsm.start()

	// Check that FSM sends a status request message
	time.Sleep(time.Millisecond)
	assert.Equal(t, 1, numStatusRequests)

	// Send a status response message to FSM
	peerID := p2p.ID(cmn.RandStr(12))
	sendStatusResponse(fsm, peerID, 10)
	time.Sleep(5 * time.Millisecond)

	// Check that FSM sends a block request message and...
	assert.Equal(t, maxRequestBatchSize, numBlockRequests)
	// ... the block request has the expected height and peer
	assert.Equal(t, int64(maxRequestBatchSize), lastBlockRequest.height)
	assert.Equal(t, peerID, lastBlockRequest.peerID)

	// let FSM timeout on the block response message
	time.Sleep(100 * time.Millisecond)

}

// reactor for FSM testing
type testReactor struct {
	logger log.Logger
	fsm    *bReactorFSM
	testCh chan bReactorMessageData
}

func sendEventToFSM(fsm *bReactorFSM, ev bReactorEvent, data bReactorEventData) error {
	return fsm.handle(&bReactorMessageData{event: ev, data: data})
}

// ----------------------------------------
// implementation for the test reactor APIs

func (testR *testReactor) sendPeerError(err error, peerID p2p.ID) {
	testR.logger.Info("Reactor received sendPeerError call from FSM", "peer", peerID, "err", err)
	lastPeerError.peerID = peerID
	lastPeerError.err = err
}

func (testR *testReactor) sendStatusRequest() error {
	testR.logger.Info("Reactor received sendStatusRequest call from FSM")
	numStatusRequests++
	if failSendStatusRequest {
		return errSendQueueFull
	}
	return nil
}

func (testR *testReactor) sendBlockRequest(peerID p2p.ID, height int64) error {
	testR.logger.Info("Reactor received sendBlockRequest call from FSM", "peer", peerID, "height", height)
	numBlockRequests++
	lastBlockRequest.peerID = peerID
	lastBlockRequest.height = height
	return nil
}

func (testR *testReactor) resetStateTimer(name string, timer *time.Timer, timeout time.Duration, f func()) {
	testR.logger.Info("Reactor received resetStateTimer call from FSM", "state", name, "timeout", timeout)
	if _, ok := stateTimerStarts[name]; !ok {
		stateTimerStarts[name] = 1
	} else {
		stateTimerStarts[name]++
	}
}

func (testR *testReactor) processBlocks(first *types.Block, second *types.Block) error {
	testR.logger.Info("Reactor received processBlocks call from FSM", "first", first.Height, "second", second.Height)
	return nil
}

func (testR *testReactor) switchToConsensus() {
	testR.logger.Info("Reactor received switchToConsensus call from FSM")

}

// ----------------------------------------

// -------------------------------------------------------
// helper functions for tests to simulate different events
func sendStatusResponse(fsm *bReactorFSM, peerID p2p.ID, height int64) {
	msgBytes := makeStatusResponseMessage(height)
	msgData := bReactorMessageData{
		event: statusResponseEv,
		data: bReactorEventData{
			peerId: peerID,
			height: height,
			length: len(msgBytes),
		},
	}

	sendMessageToFSM(fsm, msgData)
}

func sendStatusResponse2(fsm *bReactorFSM, peerID p2p.ID, height int64) {
	msgBytes := makeStatusResponseMessage(height)
	msgData := &bReactorMessageData{
		event: statusResponseEv,
		data: bReactorEventData{
			peerId: peerID,
			height: height,
			length: len(msgBytes),
		},
	}
	_ = fsm.handle(msgData)
}

func sendStateTimeout(fsm *bReactorFSM, name string) {
	msgData := &bReactorMessageData{
		event: stateTimeoutEv,
		data: bReactorEventData{
			stateName: name,
		},
	}
	_ = fsm.handle(msgData)
}

// -------------------------------------------------------

// ----------------------------------------------------
// helper functions to make blockchain reactor messages
func makeStatusResponseMessage(height int64) []byte {
	msgBytes := cdc.MustMarshalBinaryBare(&bcStatusResponseMessage{height})
	return msgBytes
}

// ----------------------------------------------------
