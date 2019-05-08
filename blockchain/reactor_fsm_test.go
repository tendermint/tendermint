package blockchain

import (
	"fmt"
	"sync"
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
	return fsm.handle(&bcReactorMessage{event: ev, data: data})
}

var (
	testMutex         sync.Mutex
	numStatusRequests int
	numBlockRequests  int32
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
	data          bReactorEventData
	errWanted     error
	expectedState string

	shouldSendStatusReq bool
	blockReqIncreased   bool
	blocksAdded         []int64
	peersNotInPool      []p2p.ID
}

// --------------------------------------------
// helper function to make tests easier to read
func makeStepStopFSMEv(current, expected string) fsmStepTestValues {
	return fsmStepTestValues{
		currentState:  current,
		expectedState: expected,
		event:         stopFSMEv,
		errWanted:     errNoErrorFinished}
}

func makeStepUnknownFSMEv(current string) fsmStepTestValues {
	return fsmStepTestValues{
		currentState:  current,
		event:         1234,
		expectedState: current,
		errWanted:     errInvalidEvent}
}

func makeStepStartFSMEv() fsmStepTestValues {
	return fsmStepTestValues{
		currentState:        "unknown",
		event:               startFSMEv,
		expectedState:       "waitForPeer",
		shouldSendStatusReq: true}
}

func makeStepStateTimeoutEv(current, expected string, timedoutState string, errWanted error) fsmStepTestValues {
	return fsmStepTestValues{
		currentState: current,
		event:        stateTimeoutEv,
		data: bReactorEventData{
			stateName: timedoutState,
		},
		expectedState: expected,
		errWanted:     errWanted,
	}
}

func makeStepProcessedBlockEv(current, expected string, reactorError error) fsmStepTestValues {
	return fsmStepTestValues{
		currentState:  current,
		event:         processedBlockEv,
		expectedState: expected,
		data: bReactorEventData{
			err: reactorError,
		},
		errWanted: reactorError,
	}
}

func makeStepStatusEv(current, expected string, peerID p2p.ID, height int64, err error) fsmStepTestValues {
	return fsmStepTestValues{
		currentState:  current,
		event:         statusResponseEv,
		data:          bReactorEventData{peerId: peerID, height: height},
		expectedState: expected,
		errWanted:     err}
}

func makeStepMakeRequestsEv(current, expected string, maxPendingRequests int32) fsmStepTestValues {
	return fsmStepTestValues{
		currentState:      current,
		event:             makeRequestsEv,
		data:              bReactorEventData{maxNumRequests: maxPendingRequests},
		expectedState:     expected,
		blockReqIncreased: true,
	}
}

func makeStepMakeRequestsEvErrored(current, expected string,
	maxPendingRequests int32, err error, peersRemoved []p2p.ID) fsmStepTestValues {
	return fsmStepTestValues{
		currentState:   current,
		event:          makeRequestsEv,
		data:           bReactorEventData{maxNumRequests: maxPendingRequests},
		expectedState:  expected,
		errWanted:      err,
		peersNotInPool: peersRemoved,
	}
}

func makeStepBlockRespEv(current, expected string, peerID p2p.ID, height int64, prevBlocks []int64) fsmStepTestValues {
	txs := []types.Tx{types.Tx("foo"), types.Tx("bar")}
	return fsmStepTestValues{
		currentState: current,
		event:        blockResponseEv,
		data: bReactorEventData{
			peerId: peerID,
			height: height,
			block:  types.MakeBlock(int64(height), txs, nil, nil),
			length: 100},
		expectedState: expected,
		blocksAdded:   append(prevBlocks, height),
	}
}

func makeStepBlockRespEvErrored(current, expected string,
	peerID p2p.ID, height int64, prevBlocks []int64, errWanted error, peersRemoved []p2p.ID) fsmStepTestValues {
	txs := []types.Tx{types.Tx("foo"), types.Tx("bar")}

	return fsmStepTestValues{
		currentState: current,
		event:        blockResponseEv,
		data: bReactorEventData{
			peerId: peerID,
			height: height,
			block:  types.MakeBlock(int64(height), txs, nil, nil),
			length: 100},
		expectedState:  expected,
		errWanted:      errWanted,
		peersNotInPool: peersRemoved,
		blocksAdded:    prevBlocks,
	}
}

func makeStepPeerRemoveEv(current, expected string, peerID p2p.ID, err error, peersRemoved []p2p.ID) fsmStepTestValues {
	return fsmStepTestValues{
		currentState: current,
		event:        peerRemoveEv,
		data: bReactorEventData{
			peerId: peerID,
			err:    err,
		},
		expectedState:  expected,
		peersNotInPool: peersRemoved,
	}
}

// --------------------------------------------

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

func shouldApplyProcessedBlockEvStep(step *fsmStepTestValues, testBcR *testReactor) bool {
	if step.event == processedBlockEv {
		_, err := testBcR.fsm.pool.getBlockAndPeerAtHeight(testBcR.fsm.pool.height)
		if err == errMissingBlocks {
			return false
		}
		_, err = testBcR.fsm.pool.getBlockAndPeerAtHeight(testBcR.fsm.pool.height + 1)
		if err == errMissingBlocks {
			return false
		}
	}
	return true
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
				makeStepStartFSMEv(),
				// statusResponseEv
				makeStepStatusEv("waitForPeer", "waitForBlock", "P1", 2, nil),
				// makeRequestEv
				makeStepMakeRequestsEv("waitForBlock", "waitForBlock", maxNumPendingRequests),
				// blockResponseEv for height 1
				makeStepBlockRespEv("waitForBlock", "waitForBlock",
					"P1", 1, []int64{}),
				// blockResponseEv for height 2
				makeStepBlockRespEv("waitForBlock", "waitForBlock",
					"P2", 2, []int64{1}),
				// processedBlockEv
				makeStepProcessedBlockEv("waitForBlock", "finished", nil),
			},
		},
		{
			name:               "multi block, multi peer",
			startingHeight:     1,
			maxRequestsPerPeer: 2,
			steps: []fsmStepTestValues{
				// startFSMEv
				makeStepStartFSMEv(),
				// statusResponseEv from P1
				makeStepStatusEv("waitForPeer", "waitForBlock", "P1", 4, nil),
				// statusResponseEv from P2
				makeStepStatusEv("waitForBlock", "waitForBlock", "P2", 4, nil),
				// makeRequestEv
				makeStepMakeRequestsEv("waitForBlock", "waitForBlock", maxNumPendingRequests),

				// blockResponseEv for height 1
				makeStepBlockRespEv("waitForBlock", "waitForBlock", "P1", 1, []int64{}),
				// blockResponseEv for height 2
				makeStepBlockRespEv("waitForBlock", "waitForBlock", "P1", 2, []int64{1}),
				// blockResponseEv for height 3
				makeStepBlockRespEv("waitForBlock", "waitForBlock", "P2", 3, []int64{1, 2}),
				// blockResponseEv for height 4
				makeStepBlockRespEv("waitForBlock", "waitForBlock", "P2", 4, []int64{1, 2, 3}),
				// processedBlockEv
				makeStepProcessedBlockEv("waitForBlock", "waitForBlock", nil),
				makeStepProcessedBlockEv("waitForBlock", "waitForBlock", nil),
				makeStepProcessedBlockEv("waitForBlock", "finished", nil),
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
				if step.expectedState == "finished" {
					assert.True(t, testBcR.fsm.isCaughtUp())
				}
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
			maxRequestsPerPeer: 3,
			steps: []fsmStepTestValues{
				// startFSMEv
				makeStepStartFSMEv(),
				// statusResponseEv from P1
				makeStepStatusEv("waitForPeer", "waitForBlock", "P1", 3, nil),

				// makeRequestEv
				makeStepMakeRequestsEv("waitForBlock", "waitForBlock", maxNumPendingRequests),
				// blockResponseEv for height 1
				makeStepBlockRespEv("waitForBlock", "waitForBlock",
					"P1", 1, []int64{}),
				// blockResponseEv for height 2
				makeStepBlockRespEv("waitForBlock", "waitForBlock",
					"P1", 2, []int64{1}),
				// blockResponseEv for height 3
				makeStepBlockRespEv("waitForBlock", "waitForBlock",
					"P1", 3, []int64{1, 2}),

				// statusResponseEv from P2
				makeStepStatusEv("waitForBlock", "waitForBlock", "P2", 3, nil),

				// processedBlockEv with Error
				makeStepProcessedBlockEv("waitForBlock", "waitForBlock", errBlockVerificationFailure),

				// makeRequestEv
				makeStepMakeRequestsEv("waitForBlock", "waitForBlock", maxNumPendingRequests),
				makeStepBlockRespEv("waitForBlock", "waitForBlock",
					"P2", 1, []int64{}),
				// blockResponseEv for height 2
				makeStepBlockRespEv("waitForBlock", "waitForBlock", "P2", 2, []int64{1}),
				// blockResponseEv for height 3
				makeStepBlockRespEv("waitForBlock", "waitForBlock", "P2", 3, []int64{1, 2}),

				makeStepProcessedBlockEv("waitForBlock", "waitForBlock", nil),
				makeStepProcessedBlockEv("waitForBlock", "finished", nil),
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

				oldNumStatusRequests := numStatusRequests
				oldNumBlockRequests := numBlockRequests

				var heightBefore int64
				if step.event == processedBlockEv && step.data.err == errBlockVerificationFailure {
					heightBefore = testBcR.fsm.pool.height
				}

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
					assert.Nil(t, firstAfter)
					assert.Nil(t, secondAfter)
				}
				assert.Equal(t, step.expectedState, testBcR.fsm.state.name)
				if step.expectedState == "finished" {
					assert.True(t, testBcR.fsm.isCaughtUp())
				}
			}
		})
	}
}

func TestFSMBadBlockFromPeer(t *testing.T) {
	tests := []struct {
		name               string
		startingHeight     int64
		maxRequestsPerPeer int32
		steps              []fsmStepTestValues
	}{
		{
			name:               "block we haven't asked for",
			startingHeight:     1,
			maxRequestsPerPeer: 3,
			steps: []fsmStepTestValues{
				// startFSMEv
				makeStepStartFSMEv(),
				// statusResponseEv from P1
				makeStepStatusEv("waitForPeer", "waitForBlock", "P1", 300, nil),
				// makeRequestEv
				makeStepMakeRequestsEv("waitForBlock", "waitForBlock", maxNumPendingRequests),
				// blockResponseEv for height 100
				makeStepBlockRespEvErrored("waitForBlock", "waitForBlock",
					"P1", 100, []int64{}, errBadDataFromPeer, []p2p.ID{}),
			},
		},
		{
			name:               "block we already have",
			startingHeight:     1,
			maxRequestsPerPeer: 3,
			steps: []fsmStepTestValues{
				// startFSMEv
				makeStepStartFSMEv(),
				// statusResponseEv from P1
				makeStepStatusEv("waitForPeer", "waitForBlock", "P1", 100, nil),

				// makeRequestEv
				makeStepMakeRequestsEv("waitForBlock", "waitForBlock", maxNumPendingRequests),
				makeStepBlockRespEv("waitForBlock", "waitForBlock",
					"P1", 1, []int64{}),

				// simulate error in this step. Since peer is removed together with block 1,
				// the blocks present in the pool will be {}
				makeStepBlockRespEvErrored("waitForBlock", "waitForBlock",
					"P1", 1, []int64{}, errBadDataFromPeer, []p2p.ID{"P1"}),
			},
		},
		{
			name:               "block from unknown peer",
			startingHeight:     1,
			maxRequestsPerPeer: 3,
			steps: []fsmStepTestValues{
				// startFSMEv
				makeStepStartFSMEv(),
				// statusResponseEv from P1
				makeStepStatusEv("waitForPeer", "waitForBlock", "P1", 3, nil),

				// makeRequestEv
				makeStepMakeRequestsEv("waitForBlock", "waitForBlock", maxNumPendingRequests),
				makeStepBlockRespEvErrored("waitForBlock", "waitForBlock",
					"P2", 1, []int64{}, errBadDataFromPeer, []p2p.ID{"P2"}),
			},
		},
		{
			name:               "block from wrong peer",
			startingHeight:     1,
			maxRequestsPerPeer: 3,
			steps: []fsmStepTestValues{
				// startFSMEv
				makeStepStartFSMEv(),
				// statusResponseEv from P1
				makeStepStatusEv("waitForPeer", "waitForBlock", "P1", 3, nil),
				// makeRequestEv
				makeStepMakeRequestsEv("waitForBlock", "waitForBlock", maxNumPendingRequests),

				// add another peer
				makeStepStatusEv("waitForBlock", "waitForBlock", "P2", 3, nil),

				makeStepBlockRespEvErrored("waitForBlock", "waitForBlock",
					"P2", 1, []int64{}, errBadDataFromPeer, []p2p.ID{"P2"}),
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

				oldNumStatusRequests := numStatusRequests
				oldNumBlockRequests := numBlockRequests

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

func TestFSMBlockAtCurrentHeightDoesNotArriveInTime(t *testing.T) {
	tests := []struct {
		name               string
		startingHeight     int64
		maxRequestsPerPeer int32
		steps              []fsmStepTestValues
	}{
		{
			name:               "block at current height undelivered",
			startingHeight:     1,
			maxRequestsPerPeer: 3,
			steps: []fsmStepTestValues{
				// startFSMEv
				makeStepStartFSMEv(),
				// statusResponseEv from P1
				makeStepStatusEv("waitForPeer", "waitForBlock", "P1", 3, nil),
				// make some requests
				makeStepMakeRequestsEv("waitForBlock", "waitForBlock", maxNumPendingRequests),
				// block received for height 1
				makeStepBlockRespEv("waitForBlock", "waitForBlock",
					"P1", 1, []int64{}),
				// block recevied for height 2
				makeStepBlockRespEv("waitForBlock", "waitForBlock",
					"P1", 2, []int64{1}),
				// processed block at height 1
				makeStepProcessedBlockEv("waitForBlock", "waitForBlock", nil),

				// add peer P2
				makeStepStatusEv("waitForBlock", "waitForBlock", "P2", 3, nil),
				// timeout on block at height 3
				makeStepStateTimeoutEv("waitForBlock", "waitForBlock", "waitForBlock", errNoPeerResponse),

				// make some requests (includes redo-s for blocks 2 and 3)
				makeStepMakeRequestsEv("waitForBlock", "waitForBlock", maxNumPendingRequests),
				// block received for height 2 from P2
				makeStepBlockRespEv("waitForBlock", "waitForBlock", "P2", 2, []int64{}),
				// block received for height 3 from P2
				makeStepBlockRespEv("waitForBlock", "waitForBlock", "P2", 3, []int64{2}),
				makeStepProcessedBlockEv("waitForBlock", "finished", nil),
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

				oldNumStatusRequests := numStatusRequests
				oldNumBlockRequests := numBlockRequests

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

func TestFSMPeerRelatedErrors(t *testing.T) {
	tests := []struct {
		name               string
		startingHeight     int64
		maxRequestsPerPeer int32
		steps              []fsmStepTestValues
	}{
		{
			name:           "peer remove event with no blocks",
			startingHeight: 1,
			steps: []fsmStepTestValues{
				// startFSMEv
				makeStepStartFSMEv(),
				// statusResponseEv from P1
				makeStepStatusEv("waitForPeer", "waitForBlock", "P1", 3, nil),
				// statusResponseEv from P2
				makeStepStatusEv("waitForBlock", "waitForBlock", "P2", 3, nil),
				// statusResponseEv from P3
				makeStepStatusEv("waitForBlock", "waitForBlock", "P3", 3, nil),
				makeStepPeerRemoveEv("waitForBlock", "waitForBlock", "P2", errSwitchRemovesPeer, []p2p.ID{"P2"}),
			},
		},
		{
			name:           "new peer with low height while in waitForPeer state",
			startingHeight: 100,
			steps: []fsmStepTestValues{
				// startFSMEv
				makeStepStartFSMEv(),
				// statusResponseEv from P1
				makeStepStatusEv("waitForPeer", "waitForPeer", "P1", 3, errPeerTooShort),
			},
		},
		{
			name:           "new peer added with low height while in waitForBlock state",
			startingHeight: 100,
			steps: []fsmStepTestValues{
				// startFSMEv
				makeStepStartFSMEv(),
				// statusResponseEv from P1
				makeStepStatusEv("waitForPeer", "waitForBlock", "P1", 200, nil),
				// statusResponseEv from P2
				makeStepStatusEv("waitForBlock", "waitForBlock", "P2", 3, errPeerTooShort),
			},
		},
		{
			name:           "only peer updated with low height while in waitForBlock state",
			startingHeight: 100,
			steps: []fsmStepTestValues{
				// startFSMEv
				makeStepStartFSMEv(),
				// statusResponseEv from P1
				makeStepStatusEv("waitForPeer", "waitForBlock", "P1", 200, nil),
				// statusResponseEv from P1
				makeStepStatusEv("waitForBlock", "waitForPeer", "P1", 3, errPeerTooShort),
			},
		},
		{
			name:               "peer remove event with no blocks",
			startingHeight:     9999999,
			maxRequestsPerPeer: 3,
			steps: []fsmStepTestValues{
				// startFSMEv
				makeStepStartFSMEv(),
				// statusResponseEv from P1
				makeStepStatusEv("waitForPeer", "waitForBlock", "P1", 20000000, nil),
				makeStepMakeRequestsEvErrored("waitForBlock", "waitForBlock",
					maxNumPendingRequests, nil, []p2p.ID{"P1"}),
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

				oldNumStatusRequests := numStatusRequests

				fsmErr := sendEventToFSM(testBcR.fsm, step.event, step.data)
				assert.Equal(t, step.errWanted, fsmErr)

				if step.shouldSendStatusReq {
					assert.Equal(t, oldNumStatusRequests+1, numStatusRequests)
				} else {
					assert.Equal(t, oldNumStatusRequests, numStatusRequests)
				}

				for _, peerID := range step.peersNotInPool {
					_, ok := testBcR.fsm.pool.peers[peerID]
					assert.False(t, ok)
				}
				assert.Equal(t, step.expectedState, testBcR.fsm.state.name)
			}
		})
	}
}

func TestFSMStopFSM(t *testing.T) {
	tests := []struct {
		name           string
		startingHeight int64
		steps          []fsmStepTestValues
	}{
		{
			name: "stopFSMEv in unknown",
			steps: []fsmStepTestValues{
				// startFSMEv
				makeStepStopFSMEv("unknown", "finished"),
			},
		},
		{
			name:           "stopFSMEv in waitForPeer",
			startingHeight: 1,
			steps: []fsmStepTestValues{
				// startFSMEv
				makeStepStartFSMEv(),
				// stopFSMEv
				makeStepStopFSMEv("waitForPeer", "finished"),
			},
		},
		{
			name:           "stopFSMEv in waitForBlock",
			startingHeight: 1,
			steps: []fsmStepTestValues{
				// startFSMEv
				makeStepStartFSMEv(),
				// statusResponseEv from P1
				makeStepStatusEv("waitForPeer", "waitForBlock", "P1", 3, nil),
				makeStepStopFSMEv("waitForBlock", "finished"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test reactor
			testBcR := newTestReactor(tt.startingHeight)
			resetTestValues()

			for _, step := range tt.steps {
				assert.Equal(t, step.currentState, testBcR.fsm.state.name)
				fsmErr := sendEventToFSM(testBcR.fsm, step.event, step.data)
				assert.Equal(t, step.errWanted, fsmErr)
				assert.Equal(t, step.expectedState, testBcR.fsm.state.name)
			}
		})
	}
}

func TestFSMUnknownElements(t *testing.T) {
	tests := []struct {
		name           string
		startingHeight int64
		steps          []fsmStepTestValues
	}{
		{
			name: "unknown event for state unknown",
			steps: []fsmStepTestValues{
				makeStepUnknownFSMEv("unknown"),
			},
		},
		{
			name: "unknown event for state waitForPeer",
			steps: []fsmStepTestValues{
				makeStepStartFSMEv(),
				makeStepUnknownFSMEv("waitForPeer"),
			},
		},
		{
			name:           "unknown event for state waitForBlock",
			startingHeight: 1,
			steps: []fsmStepTestValues{
				makeStepStartFSMEv(),
				makeStepStatusEv("waitForPeer", "waitForBlock", "P1", 3, nil),
				makeStepUnknownFSMEv("waitForBlock"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test reactor
			testBcR := newTestReactor(tt.startingHeight)
			resetTestValues()

			for _, step := range tt.steps {
				assert.Equal(t, step.currentState, testBcR.fsm.state.name)
				fsmErr := sendEventToFSM(testBcR.fsm, step.event, step.data)
				assert.Equal(t, step.errWanted, fsmErr)
				assert.Equal(t, step.expectedState, testBcR.fsm.state.name)
			}
		})
	}
}

func TestFSMPeerStateTimeoutEvent(t *testing.T) {
	tests := []struct {
		name               string
		startingHeight     int64
		maxRequestsPerPeer int32
		steps              []fsmStepTestValues
	}{
		{
			name:               "timeout event for state waitForPeer while in state waitForPeer",
			startingHeight:     1,
			maxRequestsPerPeer: 3,
			steps: []fsmStepTestValues{
				makeStepStartFSMEv(),
				makeStepStateTimeoutEv("waitForPeer", "finished", "waitForPeer", errNoPeerResponse),
			},
		},
		{
			name:               "timeout event for state waitForPeer while in a state != waitForPeer",
			startingHeight:     1,
			maxRequestsPerPeer: 3,
			steps: []fsmStepTestValues{
				makeStepStartFSMEv(),
				makeStepStateTimeoutEv("waitForPeer", "waitForPeer", "waitForBlock", errTimeoutEventWrongState),
			},
		},
		{
			name:               "timeout event for state waitForBlock while in state waitForBlock ",
			startingHeight:     1,
			maxRequestsPerPeer: 3,
			steps: []fsmStepTestValues{
				makeStepStartFSMEv(),
				makeStepStatusEv("waitForPeer", "waitForBlock", "P1", 3, nil),
				makeStepMakeRequestsEv("waitForBlock", "waitForBlock", maxNumPendingRequests),
				makeStepStateTimeoutEv("waitForBlock", "waitForPeer", "waitForBlock", errNoPeerResponse),
			},
		},
		{
			name:               "timeout event for state waitForBlock while in a state != waitForBlock",
			startingHeight:     1,
			maxRequestsPerPeer: 3,
			steps: []fsmStepTestValues{
				makeStepStartFSMEv(),
				makeStepStatusEv("waitForPeer", "waitForBlock", "P1", 3, nil),
				makeStepMakeRequestsEv("waitForBlock", "waitForBlock", maxNumPendingRequests),
				makeStepStateTimeoutEv("waitForBlock", "waitForBlock", "waitForPeer", errTimeoutEventWrongState),
			},
		},
		{
			name:               "timeout event for state waitForBlock with multiple peers",
			startingHeight:     1,
			maxRequestsPerPeer: 3,
			steps: []fsmStepTestValues{
				makeStepStartFSMEv(),
				makeStepStatusEv("waitForPeer", "waitForBlock", "P1", 3, nil),
				makeStepMakeRequestsEv("waitForBlock", "waitForBlock", maxNumPendingRequests),
				makeStepStatusEv("waitForBlock", "waitForBlock", "P2", 3, nil),
				makeStepStateTimeoutEv("waitForBlock", "waitForBlock", "waitForBlock", errNoPeerResponse),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test reactor
			testBcR := newTestReactor(tt.startingHeight)
			resetTestValues()

			for _, step := range tt.steps {
				assert.Equal(t, step.currentState, testBcR.fsm.state.name)
				fsmErr := sendEventToFSM(testBcR.fsm, step.event, step.data)
				assert.Equal(t, step.errWanted, fsmErr)
				assert.Equal(t, step.expectedState, testBcR.fsm.state.name)
			}
		})
	}
}

type testFields struct {
	name               string
	startingHeight     int64
	maxRequestsPerPeer int32
	maxPendingRequests int32
	steps              []fsmStepTestValues
}

func makeCorrectTransitionSequence(startingHeight int64, numBlocks int64, numPeers int, randomPeerHeights bool,
	maxRequestsPerPeer int32, maxPendingRequests int32) testFields {

	// Generate numPeers peers with random or numBlocks heights according to the randomPeerHeights flag.
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
	testSteps = append(testSteps, makeStepStartFSMEv())

	// For each peer, add statusResponseEv step.
	for i := 0; i < numPeers; i++ {
		peerName := fmt.Sprintf("P%d", i)
		if i == 0 {
			testSteps = append(
				testSteps,
				makeStepStatusEv("waitForPeer", "waitForBlock", p2p.ID(peerName), peerHeights[i], nil))
		} else {
			testSteps = append(testSteps,
				makeStepStatusEv("waitForBlock", "waitForBlock", p2p.ID(peerName), peerHeights[i], nil))
		}
	}

	height := startingHeight
	numBlocksReceived := 0
	prevBlocks := make([]int64, 0, maxPendingRequests)

forLoop:
	for i := 0; i < int(numBlocks); i++ {

		// Add the makeRequestEv step periodically.
		if i%int(maxRequestsPerPeer) == 0 {
			testSteps = append(
				testSteps,
				makeStepMakeRequestsEv("waitForBlock", "waitForBlock", maxNumPendingRequests),
			)
		}

		// Add the blockRespEv step
		testSteps = append(
			testSteps,
			makeStepBlockRespEv("waitForBlock", "waitForBlock",
				"P0", height, prevBlocks))
		prevBlocks = append(prevBlocks, height)
		height++
		numBlocksReceived++

		// Add the processedBlockEv step periodically.
		if numBlocksReceived >= int(maxRequestsPerPeer) || height >= numBlocks {
			for j := int(height) - numBlocksReceived; j < int(height); j++ {
				if j >= int(numBlocks) {
					// This is the last block that is processed, we should be in "finished" state.
					testSteps = append(
						testSteps,
						makeStepProcessedBlockEv("waitForBlock", "finished", nil))
					break forLoop
				}
				testSteps = append(
					testSteps,
					makeStepProcessedBlockEv("waitForBlock", "waitForBlock", nil))
			}
			numBlocksReceived = 0
			prevBlocks = make([]int64, 0, maxPendingRequests)
		}
	}

	return testFields{
		name:               testName,
		startingHeight:     startingHeight,
		maxRequestsPerPeer: maxRequestsPerPeer,
		maxPendingRequests: maxPendingRequests,
		steps:              testSteps,
	}
}

const (
	maxStartingHeightTest       = 100
	maxRequestsPerPeerTest      = 20
	maxTotalPendingRequestsTest = 600
	maxNumPeersTest             = 1000
	maxNumBlocksInChainTest     = 10000 //should be smaller than 9999999
)

func makeCorrectTransitionSequenceWithRandomParameters() testFields {
	// Generate a starting height for fast sync.
	startingHeight := int64(cmn.RandIntn(maxStartingHeightTest) + 1)

	// Generate the number of requests per peer.
	maxRequestsPerPeer := int32(cmn.RandIntn(maxRequestsPerPeerTest) + 1)

	// Generate the maximum number of total pending requests, >= maxRequestsPerPeer.
	maxPendingRequests := int32(cmn.RandIntn(maxTotalPendingRequestsTest-int(maxRequestsPerPeer))) + maxRequestsPerPeer

	// Generate the number of blocks to be synced.
	numBlocks := int64(cmn.RandIntn(maxNumBlocksInChainTest)) + startingHeight

	// Generate a number of peers.
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

				oldNumStatusRequests := numStatusRequests
				fixBlockResponseEvStep(&step, testBcR)
				if !shouldApplyProcessedBlockEvStep(&step, testBcR) {
					continue
				}

				fsmErr := sendEventToFSM(testBcR.fsm, step.event, step.data)
				assert.Equal(t, step.errWanted, fsmErr)

				if step.shouldSendStatusReq {
					assert.Equal(t, oldNumStatusRequests+1, numStatusRequests)
				} else {
					assert.Equal(t, oldNumStatusRequests, numStatusRequests)
				}

				assert.Equal(t, step.expectedState, testBcR.fsm.state.name)
				if step.expectedState == "finished" {
					assert.True(t, testBcR.fsm.isCaughtUp())
				}
			}

		})
	}
}

func TestFSMPeerTimeout(t *testing.T) {
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
	peerID := p2p.ID("P1")
	sendStatusResponse(fsm, peerID, 10)
	time.Sleep(5 * time.Millisecond)

	if err := fsm.handle(&bcReactorMessage{
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
	testMutex.Lock()
	defer testMutex.Unlock()
	assert.Equal(t, peerID, lastPeerError.peerID)
	assert.Equal(t, errNoPeerResponse, lastPeerError.err)

	peerTimeout = 15 * time.Second
	maxRequestsPerPeer = 40

}

// ----------------------------------------
// implementation for the test reactor APIs

func (testR *testReactor) sendPeerError(err error, peerID p2p.ID) {
	testMutex.Lock()
	defer testMutex.Unlock()
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
	if height == 9999999 {
		// simulate switch does not have peer
		return errNilPeerForBlockRequest
	}
	return nil
}

func (testR *testReactor) resetStateTimer(name string, timer **time.Timer, timeout time.Duration) {
	testR.logger.Info("Reactor received resetStateTimer call from FSM", "state", name, "timeout", timeout)
	if _, ok := stateTimerStarts[name]; !ok {
		stateTimerStarts[name] = 1
	} else {
		stateTimerStarts[name]++
	}
}

// ----------------------------------------

// -------------------------------------------------------
// helper functions for tests to simulate different events
func sendStatusResponse(fsm *bReactorFSM, peerID p2p.ID, height int64) {
	msgBytes := makeStatusResponseMessage(height)

	_ = fsm.handle(&bcReactorMessage{
		event: statusResponseEv,
		data: bReactorEventData{
			peerId: peerID,
			height: height,
			length: len(msgBytes),
		},
	})
}

// -------------------------------------------------------

// ----------------------------------------------------
// helper functions to make blockchain reactor messages
func makeStatusResponseMessage(height int64) []byte {
	msgBytes := cdc.MustMarshalBinaryBare(&bcStatusResponseMessage{height})
	return msgBytes
}

// ----------------------------------------------------
