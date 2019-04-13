package blockchain_new

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/types"
)

// reactor for FSM testing
type testReactor struct {
	logger log.Logger
	fsm    *bReactorFSM
}

func sendEventToFSM(fsm *bReactorFSM, ev bReactorEvent, data bReactorEventData) error {
	return fsm.handle(&bReactorMessageData{event: ev, data: data})
}

var (
	failSendStatusRequest bool
	failSendBlockRequest  bool
	numStatusRequests     int
	numBlockRequests      int32
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
	startingHeight int64
	currentState   string
	event          bReactorEvent
	data           bReactorEventData
	errWanted      error
	expectedState  string

	failStatusReq       bool
	shouldSendStatusReq bool

	failBlockReq      bool
	blockReqIncreased bool
	blocksAdded       []int64

	expectedLastBlockReq *lastBlockRequestT
}

var (
	unknown_StartFSMEv_WantWaitForPeer = fsmStepTestValues{
		currentState: "unknown", event: startFSMEv,
		expectedState:       "waitForPeer",
		shouldSendStatusReq: true}
	waitForBlock_ProcessedBlockEv_WantWaitForBlock = fsmStepTestValues{
		currentState: "waitForBlock", event: processedBlockEv,
		data:          bReactorEventData{err: nil},
		expectedState: "waitForBlock",
	}
	waitForBlock_ProcessedBlockEv_Err_WantWaitForBlock = fsmStepTestValues{
		currentState: "waitForBlock", event: processedBlockEv,
		data:          bReactorEventData{err: errBlockVerificationFailure},
		expectedState: "waitForBlock",
		errWanted:     errBlockVerificationFailure,
	}
	waitForBlock_ProcessedBlockEv_WantFinished = fsmStepTestValues{
		currentState: "waitForBlock", event: processedBlockEv,
		data:          bReactorEventData{err: nil},
		expectedState: "finished",
	}
)

func waitForBlock_MakeRequestEv_WantWaitForBlock(maxPendingRequests int32) fsmStepTestValues {
	return fsmStepTestValues{
		currentState: "waitForBlock", event: makeRequestsEv,
		data:              bReactorEventData{maxNumRequests: maxPendingRequests},
		expectedState:     "waitForBlock",
		blockReqIncreased: true,
	}
}

func waitForPeer_StatusEv_WantWaitForBlock(peerID p2p.ID, height int64) fsmStepTestValues {
	return fsmStepTestValues{
		currentState: "waitForPeer", event: statusResponseEv,
		data:          bReactorEventData{peerId: peerID, height: height},
		expectedState: "waitForBlock"}
}

func waitForBlock_StatusEv_WantWaitForBlock(peerID p2p.ID, height int64) fsmStepTestValues {
	return fsmStepTestValues{
		currentState: "waitForBlock", event: statusResponseEv,
		data:          bReactorEventData{peerId: peerID, height: height},
		expectedState: "waitForBlock"}
}

func waitForBlock_BlockRespEv_WantWaitForBlock(peerID p2p.ID, height int64, prevBlocks []int64) fsmStepTestValues {
	txs := []types.Tx{types.Tx("foo"), types.Tx("bar")}

	return fsmStepTestValues{
		currentState: "waitForBlock", event: blockResponseEv,
		data: bReactorEventData{
			peerId: peerID,
			height: height,
			block:  types.MakeBlock(int64(height), txs, nil, nil)},
		expectedState: "waitForBlock",
		blocksAdded:   append(prevBlocks, height),
	}
}

func newTestReactor(height int64) *testReactor {
	testBcR := &testReactor{logger: log.TestingLogger()}
	testBcR.fsm = NewFSM(height, testBcR)
	testBcR.fsm.setLogger(testBcR.logger)
	return testBcR
}

func fixBlockResponseEvStep(step *fsmStepTestValues, testBcR *testReactor) {
	// There is no good way to know to which peer a block request was sent.
	// So before we simulate a block response we cheat and look where it is expected from.
	if step.event == blockResponseEv {
		height := step.data.height
		peerID, ok := testBcR.fsm.pool.blocks[height]
		if ok {
			step.data.peerId = peerID
		}
	}
}

func TestFSMBasic(t *testing.T) {
	tests := []struct {
		name               string
		startingHeight     int64
		maxRequestsPerPeer int32
		steps              []fsmStepTestValues
	}{
		{
			name:               "one block, one peer",
			startingHeight:     1,
			maxRequestsPerPeer: 2,
			steps: []fsmStepTestValues{
				// startFSMEv
				unknown_StartFSMEv_WantWaitForPeer,
				// statusResponseEv
				waitForPeer_StatusEv_WantWaitForBlock("P1", 2),
				// makeRequestEv
				waitForBlock_MakeRequestEv_WantWaitForBlock(maxNumPendingRequests),
				// blockResponseEv for height 1
				waitForBlock_BlockRespEv_WantWaitForBlock("P1", 1, []int64{}),
				// blockResponseEv for height 2
				waitForBlock_BlockRespEv_WantWaitForBlock("P1", 2, []int64{1}),
				// processedBlockEv
				waitForBlock_ProcessedBlockEv_WantFinished,
			},
		},
		{
			name:               "multi block, multi peer",
			startingHeight:     1,
			maxRequestsPerPeer: 2,
			steps: []fsmStepTestValues{
				// startFSMEv
				unknown_StartFSMEv_WantWaitForPeer,
				// statusResponseEv from P1
				waitForPeer_StatusEv_WantWaitForBlock("P1", 4),
				// statusResponseEv from P2
				waitForBlock_StatusEv_WantWaitForBlock("P2", 4),
				// makeRequestEv
				waitForBlock_MakeRequestEv_WantWaitForBlock(maxNumPendingRequests),
				// blockResponseEv for height 1
				waitForBlock_BlockRespEv_WantWaitForBlock("P1", 1, []int64{}),
				// blockResponseEv for height 2
				waitForBlock_BlockRespEv_WantWaitForBlock("P1", 2, []int64{1}),
				// blockResponseEv for height 3
				waitForBlock_BlockRespEv_WantWaitForBlock("P2", 3, []int64{1, 2}),
				// blockResponseEv for height 4
				waitForBlock_BlockRespEv_WantWaitForBlock("P2", 4, []int64{1, 2, 3}),
				// processedBlockEv
				waitForBlock_ProcessedBlockEv_WantWaitForBlock,
				waitForBlock_ProcessedBlockEv_WantWaitForBlock,
				waitForBlock_ProcessedBlockEv_WantFinished,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test reactor
			testBcR := newTestReactor(tt.startingHeight)
			resetTestValues()

			if tt.maxRequestsPerPeer != 0 {
				maxRequestsPerPeer = tt.maxRequestsPerPeer
			}

			for _, step := range tt.steps {
				assert.Equal(t, step.currentState, testBcR.fsm.state.name)
				failSendStatusRequest = step.failStatusReq
				failSendBlockRequest = step.failBlockReq

				oldNumStatusRequests := numStatusRequests
				oldNumBlockRequests := numBlockRequests

				fixBlockResponseEvStep(&step, testBcR)
				fsmErr := sendEventToFSM(testBcR.fsm, step.event, step.data)
				assert.Equal(t, step.errWanted, fsmErr)

				if step.shouldSendStatusReq {
					assert.Equal(t, oldNumStatusRequests+1, numStatusRequests)
				} else {
					assert.Equal(t, oldNumStatusRequests, numStatusRequests)
				}

				if step.blockReqIncreased {
					assert.True(t, oldNumBlockRequests < numBlockRequests)
				} else {
					assert.Equal(t, oldNumBlockRequests, numBlockRequests)
				}

				for _, height := range step.blocksAdded {
					_, err := testBcR.fsm.pool.getBlockAndPeerAtHeight(height)
					assert.Nil(t, err)
				}
				assert.Equal(t, step.expectedState, testBcR.fsm.state.name)
			}

		})
	}
}

func TestFSMBlockVerificationFailure(t *testing.T) {
	tests := []struct {
		name               string
		startingHeight     int64
		maxRequestsPerPeer int32
		steps              []fsmStepTestValues
	}{
		{
			name:               "block verification failure",
			startingHeight:     1,
			maxRequestsPerPeer: 2,
			steps: []fsmStepTestValues{
				// startFSMEv
				unknown_StartFSMEv_WantWaitForPeer,
				// statusResponseEv from P1
				waitForPeer_StatusEv_WantWaitForBlock("P1", 3),
				// statusResponseEv from P2
				waitForBlock_StatusEv_WantWaitForBlock("P2", 3),
				// makeRequestEv
				waitForBlock_MakeRequestEv_WantWaitForBlock(maxNumPendingRequests),
				// blockResponseEv for height 1
				waitForBlock_BlockRespEv_WantWaitForBlock("P1", 1, []int64{}),
				// blockResponseEv for height 2
				waitForBlock_BlockRespEv_WantWaitForBlock("P1", 2, []int64{1}),
				// blockResponseEv for height 3
				waitForBlock_BlockRespEv_WantWaitForBlock("P2", 3, []int64{1, 2}),

				// processedBlockEv with Error
				waitForBlock_ProcessedBlockEv_Err_WantWaitForBlock,

				// makeRequestEv
				waitForBlock_MakeRequestEv_WantWaitForBlock(maxNumPendingRequests),
				// blockResponseEv for height 1
				waitForBlock_BlockRespEv_WantWaitForBlock("P2", 1, []int64{3}),
				// blockResponseEv for height 2
				waitForBlock_BlockRespEv_WantWaitForBlock("P2", 2, []int64{1, 3}),

				waitForBlock_ProcessedBlockEv_WantWaitForBlock,
				waitForBlock_ProcessedBlockEv_WantFinished,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test reactor
			testBcR := newTestReactor(tt.startingHeight)
			resetTestValues()

			if tt.maxRequestsPerPeer != 0 {
				maxRequestsPerPeer = tt.maxRequestsPerPeer
			}

			for _, step := range tt.steps {
				assert.Equal(t, step.currentState, testBcR.fsm.state.name)
				failSendStatusRequest = step.failStatusReq
				failSendBlockRequest = step.failBlockReq

				oldNumStatusRequests := numStatusRequests
				oldNumBlockRequests := numBlockRequests

				var heightBefore int64
				if step.event == processedBlockEv && step.data.err == errBlockVerificationFailure {
					heightBefore = testBcR.fsm.pool.height
				}

				fixBlockResponseEvStep(&step, testBcR)

				fsmErr := sendEventToFSM(testBcR.fsm, step.event, step.data)
				assert.Equal(t, step.errWanted, fsmErr)

				if step.shouldSendStatusReq {
					assert.Equal(t, oldNumStatusRequests+1, numStatusRequests)
				} else {
					assert.Equal(t, oldNumStatusRequests, numStatusRequests)
				}

				if step.blockReqIncreased {
					assert.True(t, oldNumBlockRequests < numBlockRequests)
				} else {
					assert.Equal(t, oldNumBlockRequests, numBlockRequests)
				}

				for _, height := range step.blocksAdded {
					_, err := testBcR.fsm.pool.getBlockAndPeerAtHeight(height)
					assert.Nil(t, err)
				}

				if step.event == processedBlockEv && step.data.err == errBlockVerificationFailure {
					heightAfter := testBcR.fsm.pool.height
					assert.Equal(t, heightBefore, heightAfter)
					firstAfter, err1 := testBcR.fsm.pool.getBlockAndPeerAtHeight(testBcR.fsm.pool.height)
					secondAfter, err2 := testBcR.fsm.pool.getBlockAndPeerAtHeight(testBcR.fsm.pool.height + 1)
					assert.NotNil(t, err1)
					assert.NotNil(t, err2)
					assert.Nil(t, firstAfter.block)
					assert.Nil(t, firstAfter.peer)
					assert.Nil(t, secondAfter.block)
					assert.Nil(t, secondAfter.peer)
				}
				assert.Equal(t, step.expectedState, testBcR.fsm.state.name)
			}
		})
	}
}

const (
	maxStartingHeightTest       = 100
	maxRequestsPerPeerTest      = 40
	maxTotalPendingRequestsTest = 600
	maxNumPeersTest             = 1000
	maxNumBlocksInChainTest     = 100000
)

type testFields struct {
	name               string
	startingHeight     int64
	maxRequestsPerPeer int32
	maxPendingRequests int32
	steps              []fsmStepTestValues
}

func makeCorrectTransitionSequence(startingHeight int64, numBlocks int64, numPeers int, randomPeerHeights bool,
	maxRequestsPerPeer int32, maxPendingRequests int32) testFields {

	// Generate numPeers peers with random or numBlocks heights according to the randomPeerHeights flag
	peerHeights := make([]int64, numPeers)
	for i := 0; i < numPeers; i++ {
		if i == 0 {
			peerHeights[0] = numBlocks
			continue
		}
		if randomPeerHeights {
			peerHeights[i] = int64(cmn.MaxInt(cmn.RandIntn(int(numBlocks)), int(startingHeight)+1))
		} else {
			peerHeights[i] = numBlocks
		}
	}

	// Approximate the slice capacity to save time for appends.
	testSteps := make([]fsmStepTestValues, 0, 3*numBlocks+int64(numPeers))

	testName := fmt.Sprintf("%v-blocks %v-startingHeight %v-peers %v-maxRequestsPerPeer %v-maxNumPendingRequests",
		numBlocks, startingHeight, numPeers, maxRequestsPerPeer, maxPendingRequests)

	// Add startFSMEv step.
	testSteps = append(testSteps, unknown_StartFSMEv_WantWaitForPeer)

	// For each peer, add statusResponseEv step.
	for i := 0; i < numPeers; i++ {
		peerName := fmt.Sprintf("P%d", i)
		if i == 0 {
			testSteps = append(testSteps, waitForPeer_StatusEv_WantWaitForBlock(p2p.ID(peerName), peerHeights[i]))
		} else {
			testSteps = append(testSteps, waitForBlock_StatusEv_WantWaitForBlock(p2p.ID(peerName), peerHeights[i]))
		}
	}

	height := startingHeight
	numBlocksReceived := 0
	prevBlocks := make([]int64, 0, maxPendingRequests)

forLoop:
	for i := 0; i < int(numBlocks); i++ {

		// Add the makeRequestEv step periodically
		if i%int(maxRequestsPerPeer) == 0 {
			testSteps = append(testSteps, waitForBlock_MakeRequestEv_WantWaitForBlock(maxPendingRequests))
		}

		// Add the blockRespEv step
		testSteps = append(testSteps, waitForBlock_BlockRespEv_WantWaitForBlock("P0", height, prevBlocks))
		prevBlocks = append(prevBlocks, height)
		height++
		numBlocksReceived++

		// Add the processedBlockEv step periodically
		if numBlocksReceived >= int(maxRequestsPerPeer) || height >= numBlocks {
			for j := int(height) - numBlocksReceived; j < int(height); j++ {
				if j >= int(numBlocks)-1 {
					// This is the last block that is processed, we should be in "finished" state.
					testSteps = append(testSteps, waitForBlock_ProcessedBlockEv_WantFinished)
					break forLoop
				}
				testSteps = append(testSteps, waitForBlock_ProcessedBlockEv_WantWaitForBlock)
			}
			numBlocksReceived = 0
			prevBlocks = make([]int64, 0, maxPendingRequests)
		}
	}

	return testFields{
		name:               testName,
		startingHeight:     startingHeight,
		maxRequestsPerPeer: maxRequestsPerPeer,
		steps:              testSteps,
	}
}

func makeCorrectTransitionSequenceWithRandomParameters() testFields {
	// generate a starting height for fast sync
	startingHeight := int64(cmn.RandIntn(maxStartingHeightTest) + 1)

	// generate the number of requests per peer
	maxRequestsPerPeer := int32(cmn.RandIntn(maxRequestsPerPeerTest) + 1)

	// generate the maximum number of total pending requests
	maxPendingRequests := int32(cmn.RandIntn(maxTotalPendingRequestsTest) + 1)

	// generate the number of blocks to be synced
	numBlocks := int64(cmn.RandIntn(maxNumBlocksInChainTest)) + startingHeight

	// generate a number of peers and their heights
	numPeers := cmn.RandIntn(maxNumPeersTest) + 1

	return makeCorrectTransitionSequence(startingHeight, numBlocks, numPeers, true, maxRequestsPerPeer, maxPendingRequests)
}

func TestFSMCorrectTransitionSequences(t *testing.T) {

	tests := []testFields{
		makeCorrectTransitionSequence(1, 100, 10, true, 10, 40),
		makeCorrectTransitionSequenceWithRandomParameters(),
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test reactor
			testBcR := newTestReactor(tt.startingHeight)
			resetTestValues()

			if tt.maxRequestsPerPeer != 0 {
				maxRequestsPerPeer = tt.maxRequestsPerPeer
			}

			for _, step := range tt.steps {
				assert.Equal(t, step.currentState, testBcR.fsm.state.name)
				failSendStatusRequest = step.failStatusReq
				failSendBlockRequest = step.failBlockReq

				oldNumStatusRequests := numStatusRequests
				fixBlockResponseEvStep(&step, testBcR)

				fsmErr := sendEventToFSM(testBcR.fsm, step.event, step.data)
				assert.Equal(t, step.errWanted, fsmErr)

				if step.shouldSendStatusReq {
					assert.Equal(t, oldNumStatusRequests+1, numStatusRequests)
				} else {
					assert.Equal(t, oldNumStatusRequests, numStatusRequests)
				}

				for _, height := range step.blocksAdded {
					_, err := testBcR.fsm.pool.getBlockAndPeerAtHeight(height)
					assert.Nil(t, err)
				}
				assert.Equal(t, step.expectedState, testBcR.fsm.state.name)
			}

		})
	}
}

func TestReactorFSMPeerTimeout(t *testing.T) {
	maxRequestsPerPeer = 2
	resetTestValues()
	peerTimeout = 20 * time.Millisecond
	// Create and start the FSM
	testBcR := newTestReactor(1)
	fsm := testBcR.fsm
	fsm.start()

	// Check that FSM sends a status request message
	time.Sleep(time.Millisecond)
	assert.Equal(t, 1, numStatusRequests)

	// Send a status response message to FSM
	peerID := p2p.ID(cmn.RandStr(12))
	sendStatusResponse(fsm, peerID, 10)
	time.Sleep(5 * time.Millisecond)

	if err := fsm.handle(&bReactorMessageData{
		event: makeRequestsEv,
		data:  bReactorEventData{maxNumRequests: maxNumPendingRequests}}); err != nil {
	}
	// Check that FSM sends a block request message and...
	assert.Equal(t, maxRequestsPerPeer, numBlockRequests)
	// ... the block request has the expected height and peer
	assert.Equal(t, maxRequestsPerPeer, int32(len(fsm.pool.peers[peerID].blocks)))
	assert.Equal(t, peerID, lastBlockRequest.peerID)

	// let FSM timeout on the block response message
	time.Sleep(100 * time.Millisecond)

}

// ----------------------------------------
// implementation for the test reactor APIs

func (testR *testReactor) sendPeerError(err error, peerID p2p.ID) {
	testR.logger.Info("Reactor received sendPeerError call from FSM", "peer", peerID, "err", err)
	lastPeerError.peerID = peerID
	lastPeerError.err = err
}

func (testR *testReactor) sendStatusRequest() {
	testR.logger.Info("Reactor received sendStatusRequest call from FSM")
	numStatusRequests++
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

	_ = sendMessageToFSMSync(fsm, msgData)
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
