package v1

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

type lastBlockRequestT struct {
	peerID p2p.ID
	height int64
}

type lastPeerErrorT struct {
	peerID p2p.ID
	err    error
}

// reactor for FSM testing
type testReactor struct {
	logger            log.Logger
	fsm               *BcReactorFSM
	numStatusRequests int
	numBlockRequests  int
	lastBlockRequest  lastBlockRequestT
	lastPeerError     lastPeerErrorT
	stateTimerStarts  map[string]int
}

func sendEventToFSM(fsm *BcReactorFSM, ev bReactorEvent, data bReactorEventData) error {
	return fsm.Handle(&bcReactorMessage{event: ev, data: data})
}

type fsmStepTestValues struct {
	currentState string
	event        bReactorEvent
	data         bReactorEventData

	wantErr           error
	wantState         string
	wantStatusReqSent bool
	wantReqIncreased  bool
	wantNewBlocks     []int64
	wantRemovedPeers  []p2p.ID
}

// ---------------------------------------------------------------------------
// helper test function for different FSM events, state and expected behavior
func sStopFSMEv(current, expected string) fsmStepTestValues {
	return fsmStepTestValues{
		currentState: current,
		event:        stopFSMEv,
		wantState:    expected,
		wantErr:      errNoErrorFinished}
}

func sUnknownFSMEv(current string) fsmStepTestValues {
	return fsmStepTestValues{
		currentState: current,
		event:        1234,
		wantState:    current,
		wantErr:      errInvalidEvent}
}

func sStartFSMEv() fsmStepTestValues {
	return fsmStepTestValues{
		currentState:      "unknown",
		event:             startFSMEv,
		wantState:         "waitForPeer",
		wantStatusReqSent: true}
}

func sStateTimeoutEv(current, expected string, timedoutState string, wantErr error) fsmStepTestValues {
	return fsmStepTestValues{
		currentState: current,
		event:        stateTimeoutEv,
		data: bReactorEventData{
			stateName: timedoutState,
		},
		wantState: expected,
		wantErr:   wantErr,
	}
}

func sProcessedBlockEv(current, expected string, reactorError error) fsmStepTestValues {
	return fsmStepTestValues{
		currentState: current,
		event:        processedBlockEv,
		data: bReactorEventData{
			err: reactorError,
		},
		wantState: expected,
		wantErr:   reactorError,
	}
}

func sStatusEv(current, expected string, peerID p2p.ID, height int64, err error) fsmStepTestValues {
	return fsmStepTestValues{
		currentState: current,
		event:        statusResponseEv,
		data:         bReactorEventData{peerID: peerID, height: height},
		wantState:    expected,
		wantErr:      err}
}

func sMakeRequestsEv(current, expected string, maxPendingRequests int) fsmStepTestValues {
	return fsmStepTestValues{
		currentState:     current,
		event:            makeRequestsEv,
		data:             bReactorEventData{maxNumRequests: maxPendingRequests},
		wantState:        expected,
		wantReqIncreased: true,
	}
}

func sMakeRequestsEvErrored(current, expected string,
	maxPendingRequests int, err error, peersRemoved []p2p.ID) fsmStepTestValues {
	return fsmStepTestValues{
		currentState:     current,
		event:            makeRequestsEv,
		data:             bReactorEventData{maxNumRequests: maxPendingRequests},
		wantState:        expected,
		wantErr:          err,
		wantRemovedPeers: peersRemoved,
		wantReqIncreased: true,
	}
}

func sBlockRespEv(current, expected string, peerID p2p.ID, height int64, prevBlocks []int64) fsmStepTestValues {
	txs := []types.Tx{types.Tx("foo"), types.Tx("bar")}
	return fsmStepTestValues{
		currentState: current,
		event:        blockResponseEv,
		data: bReactorEventData{
			peerID: peerID,
			height: height,
			block:  types.MakeBlock(height, txs, nil, nil),
			length: 100},
		wantState:     expected,
		wantNewBlocks: append(prevBlocks, height),
	}
}

func sBlockRespEvErrored(current, expected string,
	peerID p2p.ID, height int64, prevBlocks []int64, wantErr error, peersRemoved []p2p.ID) fsmStepTestValues {
	txs := []types.Tx{types.Tx("foo"), types.Tx("bar")}

	return fsmStepTestValues{
		currentState: current,
		event:        blockResponseEv,
		data: bReactorEventData{
			peerID: peerID,
			height: height,
			block:  types.MakeBlock(height, txs, nil, nil),
			length: 100},
		wantState:        expected,
		wantErr:          wantErr,
		wantRemovedPeers: peersRemoved,
		wantNewBlocks:    prevBlocks,
	}
}

func sPeerRemoveEv(current, expected string, peerID p2p.ID, err error, peersRemoved []p2p.ID) fsmStepTestValues {
	return fsmStepTestValues{
		currentState: current,
		event:        peerRemoveEv,
		data: bReactorEventData{
			peerID: peerID,
			err:    err,
		},
		wantState:        expected,
		wantRemovedPeers: peersRemoved,
	}
}

// --------------------------------------------

func newTestReactor(height int64) *testReactor {
	testBcR := &testReactor{logger: log.TestingLogger(), stateTimerStarts: make(map[string]int)}
	testBcR.fsm = NewFSM(height, testBcR)
	testBcR.fsm.SetLogger(testBcR.logger)
	return testBcR
}

func fixBlockResponseEvStep(step *fsmStepTestValues, testBcR *testReactor) {
	// There is currently no good way to know to which peer a block request was sent.
	// So in some cases where it does not matter, before we simulate a block response
	// we cheat and look where it is expected from.
	if step.event == blockResponseEv {
		height := step.data.height
		peerID, ok := testBcR.fsm.pool.blocks[height]
		if ok {
			step.data.peerID = peerID
		}
	}
}

type testFields struct {
	name               string
	startingHeight     int64
	maxRequestsPerPeer int
	maxPendingRequests int
	steps              []fsmStepTestValues
}

func executeFSMTests(t *testing.T, tests []testFields, matchRespToReq bool) {
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// Create test reactor
			testBcR := newTestReactor(tt.startingHeight)

			if tt.maxRequestsPerPeer != 0 {
				maxRequestsPerPeer = tt.maxRequestsPerPeer
			}

			for _, step := range tt.steps {
				step := step
				assert.Equal(t, step.currentState, testBcR.fsm.state.name)

				var heightBefore int64
				if step.event == processedBlockEv && step.data.err == errBlockVerificationFailure {
					heightBefore = testBcR.fsm.pool.Height
				}
				oldNumStatusRequests := testBcR.numStatusRequests
				oldNumBlockRequests := testBcR.numBlockRequests
				if matchRespToReq {
					fixBlockResponseEvStep(&step, testBcR)
				}

				fsmErr := sendEventToFSM(testBcR.fsm, step.event, step.data)
				assert.Equal(t, step.wantErr, fsmErr)

				if step.wantStatusReqSent {
					assert.Equal(t, oldNumStatusRequests+1, testBcR.numStatusRequests)
				} else {
					assert.Equal(t, oldNumStatusRequests, testBcR.numStatusRequests)
				}

				if step.wantReqIncreased {
					assert.True(t, oldNumBlockRequests < testBcR.numBlockRequests)
				} else {
					assert.Equal(t, oldNumBlockRequests, testBcR.numBlockRequests)
				}

				for _, height := range step.wantNewBlocks {
					_, err := testBcR.fsm.pool.BlockAndPeerAtHeight(height)
					assert.Nil(t, err)
				}
				if step.event == processedBlockEv && step.data.err == errBlockVerificationFailure {
					heightAfter := testBcR.fsm.pool.Height
					assert.Equal(t, heightBefore, heightAfter)
					firstAfter, err1 := testBcR.fsm.pool.BlockAndPeerAtHeight(testBcR.fsm.pool.Height)
					secondAfter, err2 := testBcR.fsm.pool.BlockAndPeerAtHeight(testBcR.fsm.pool.Height + 1)
					assert.NotNil(t, err1)
					assert.NotNil(t, err2)
					assert.Nil(t, firstAfter)
					assert.Nil(t, secondAfter)
				}

				assert.Equal(t, step.wantState, testBcR.fsm.state.name)

				if step.wantState == "finished" {
					assert.True(t, testBcR.fsm.isCaughtUp())
				}
			}
		})
	}
}

func TestFSMBasic(t *testing.T) {
	tests := []testFields{
		{
			name:               "one block, one peer - TS2",
			startingHeight:     1,
			maxRequestsPerPeer: 2,
			steps: []fsmStepTestValues{
				sStartFSMEv(),
				sStatusEv("waitForPeer", "waitForBlock", "P1", 2, nil),
				sMakeRequestsEv("waitForBlock", "waitForBlock", maxNumRequests),
				sBlockRespEv("waitForBlock", "waitForBlock", "P1", 1, []int64{}),
				sBlockRespEv("waitForBlock", "waitForBlock", "P2", 2, []int64{1}),
				sProcessedBlockEv("waitForBlock", "finished", nil),
			},
		},
		{
			name:               "multi block, multi peer - TS2",
			startingHeight:     1,
			maxRequestsPerPeer: 2,
			steps: []fsmStepTestValues{
				sStartFSMEv(),
				sStatusEv("waitForPeer", "waitForBlock", "P1", 4, nil),
				sStatusEv("waitForBlock", "waitForBlock", "P2", 4, nil),
				sMakeRequestsEv("waitForBlock", "waitForBlock", maxNumRequests),

				sBlockRespEv("waitForBlock", "waitForBlock", "P1", 1, []int64{}),
				sBlockRespEv("waitForBlock", "waitForBlock", "P1", 2, []int64{1}),
				sBlockRespEv("waitForBlock", "waitForBlock", "P2", 3, []int64{1, 2}),
				sBlockRespEv("waitForBlock", "waitForBlock", "P2", 4, []int64{1, 2, 3}),

				sProcessedBlockEv("waitForBlock", "waitForBlock", nil),
				sProcessedBlockEv("waitForBlock", "waitForBlock", nil),
				sProcessedBlockEv("waitForBlock", "finished", nil),
			},
		},
	}

	executeFSMTests(t, tests, true)
}

func TestFSMBlockVerificationFailure(t *testing.T) {
	tests := []testFields{
		{
			name:               "block verification failure - TS2 variant",
			startingHeight:     1,
			maxRequestsPerPeer: 3,
			steps: []fsmStepTestValues{
				sStartFSMEv(),

				// add P1 and get blocks 1-3 from it
				sStatusEv("waitForPeer", "waitForBlock", "P1", 3, nil),
				sMakeRequestsEv("waitForBlock", "waitForBlock", maxNumRequests),
				sBlockRespEv("waitForBlock", "waitForBlock", "P1", 1, []int64{}),
				sBlockRespEv("waitForBlock", "waitForBlock", "P1", 2, []int64{1}),
				sBlockRespEv("waitForBlock", "waitForBlock", "P1", 3, []int64{1, 2}),

				// add P2
				sStatusEv("waitForBlock", "waitForBlock", "P2", 3, nil),

				// process block failure, should remove P1 and all blocks
				sProcessedBlockEv("waitForBlock", "waitForBlock", errBlockVerificationFailure),

				// get blocks 1-3 from P2
				sMakeRequestsEv("waitForBlock", "waitForBlock", maxNumRequests),
				sBlockRespEv("waitForBlock", "waitForBlock", "P2", 1, []int64{}),
				sBlockRespEv("waitForBlock", "waitForBlock", "P2", 2, []int64{1}),
				sBlockRespEv("waitForBlock", "waitForBlock", "P2", 3, []int64{1, 2}),

				// finish after processing blocks 1 and 2
				sProcessedBlockEv("waitForBlock", "waitForBlock", nil),
				sProcessedBlockEv("waitForBlock", "finished", nil),
			},
		},
	}

	executeFSMTests(t, tests, false)
}

func TestFSMBadBlockFromPeer(t *testing.T) {
	tests := []testFields{
		{
			name:               "block we haven't asked for",
			startingHeight:     1,
			maxRequestsPerPeer: 3,
			steps: []fsmStepTestValues{
				sStartFSMEv(),
				// add P1 and ask for blocks 1-3
				sStatusEv("waitForPeer", "waitForBlock", "P1", 300, nil),
				sMakeRequestsEv("waitForBlock", "waitForBlock", maxNumRequests),

				// blockResponseEv for height 100 should cause an error
				sBlockRespEvErrored("waitForBlock", "waitForPeer",
					"P1", 100, []int64{}, errMissingBlock, []p2p.ID{}),
			},
		},
		{
			name:               "block we already have",
			startingHeight:     1,
			maxRequestsPerPeer: 3,
			steps: []fsmStepTestValues{
				sStartFSMEv(),
				// add P1 and get block 1
				sStatusEv("waitForPeer", "waitForBlock", "P1", 100, nil),
				sMakeRequestsEv("waitForBlock", "waitForBlock", maxNumRequests),
				sBlockRespEv("waitForBlock", "waitForBlock",
					"P1", 1, []int64{}),

				// Get block 1 again. Since peer is removed together with block 1,
				// the blocks present in the pool should be {}
				sBlockRespEvErrored("waitForBlock", "waitForPeer",
					"P1", 1, []int64{}, errDuplicateBlock, []p2p.ID{"P1"}),
			},
		},
		{
			name:               "block from unknown peer",
			startingHeight:     1,
			maxRequestsPerPeer: 3,
			steps: []fsmStepTestValues{
				sStartFSMEv(),
				// add P1 and get block 1
				sStatusEv("waitForPeer", "waitForBlock", "P1", 3, nil),

				// get block 1 from unknown peer P2
				sMakeRequestsEv("waitForBlock", "waitForBlock", maxNumRequests),
				sBlockRespEvErrored("waitForBlock", "waitForBlock",
					"P2", 1, []int64{}, errBadDataFromPeer, []p2p.ID{"P2"}),
			},
		},
		{
			name:               "block from wrong peer",
			startingHeight:     1,
			maxRequestsPerPeer: 3,
			steps: []fsmStepTestValues{
				sStartFSMEv(),
				// add P1, make requests for blocks 1-3 to P1
				sStatusEv("waitForPeer", "waitForBlock", "P1", 3, nil),
				sMakeRequestsEv("waitForBlock", "waitForBlock", maxNumRequests),

				// add P2
				sStatusEv("waitForBlock", "waitForBlock", "P2", 3, nil),

				// receive block 1 from P2
				sBlockRespEvErrored("waitForBlock", "waitForBlock",
					"P2", 1, []int64{}, errBadDataFromPeer, []p2p.ID{"P2"}),
			},
		},
	}

	executeFSMTests(t, tests, false)
}

func TestFSMBlockAtCurrentHeightDoesNotArriveInTime(t *testing.T) {
	tests := []testFields{
		{
			name:               "block at current height undelivered - TS5",
			startingHeight:     1,
			maxRequestsPerPeer: 3,
			steps: []fsmStepTestValues{
				sStartFSMEv(),
				// add P1, get blocks 1 and 2, process block 1
				sStatusEv("waitForPeer", "waitForBlock", "P1", 3, nil),
				sMakeRequestsEv("waitForBlock", "waitForBlock", maxNumRequests),
				sBlockRespEv("waitForBlock", "waitForBlock",
					"P1", 1, []int64{}),
				sBlockRespEv("waitForBlock", "waitForBlock",
					"P1", 2, []int64{1}),
				sProcessedBlockEv("waitForBlock", "waitForBlock", nil),

				// add P2
				sStatusEv("waitForBlock", "waitForBlock", "P2", 3, nil),

				// timeout on block 3, P1 should be removed
				sStateTimeoutEv("waitForBlock", "waitForBlock", "waitForBlock", errNoPeerResponseForCurrentHeights),

				// make requests and finish by receiving blocks 2 and 3 from P2
				sMakeRequestsEv("waitForBlock", "waitForBlock", maxNumRequests),
				sBlockRespEv("waitForBlock", "waitForBlock", "P2", 2, []int64{}),
				sBlockRespEv("waitForBlock", "waitForBlock", "P2", 3, []int64{2}),
				sProcessedBlockEv("waitForBlock", "finished", nil),
			},
		},
		{
			name:               "block at current height undelivered, at maxPeerHeight after peer removal - TS3",
			startingHeight:     1,
			maxRequestsPerPeer: 3,
			steps: []fsmStepTestValues{
				sStartFSMEv(),
				// add P1, request blocks 1-3 from P1
				sStatusEv("waitForPeer", "waitForBlock", "P1", 3, nil),
				sMakeRequestsEv("waitForBlock", "waitForBlock", maxNumRequests),

				// add P2 (tallest)
				sStatusEv("waitForBlock", "waitForBlock", "P2", 30, nil),
				sMakeRequestsEv("waitForBlock", "waitForBlock", maxNumRequests),

				// receive blocks 1-3 from P1
				sBlockRespEv("waitForBlock", "waitForBlock", "P1", 1, []int64{}),
				sBlockRespEv("waitForBlock", "waitForBlock", "P1", 2, []int64{1}),
				sBlockRespEv("waitForBlock", "waitForBlock", "P1", 3, []int64{1, 2}),

				// process blocks at heights 1 and 2
				sProcessedBlockEv("waitForBlock", "waitForBlock", nil),
				sProcessedBlockEv("waitForBlock", "waitForBlock", nil),

				// timeout on block at height 4
				sStateTimeoutEv("waitForBlock", "finished", "waitForBlock", nil),
			},
		},
	}

	executeFSMTests(t, tests, true)
}

func TestFSMPeerRelatedEvents(t *testing.T) {
	tests := []testFields{
		{
			name:           "peer remove event with no blocks",
			startingHeight: 1,
			steps: []fsmStepTestValues{
				sStartFSMEv(),
				// add P1, P2, P3
				sStatusEv("waitForPeer", "waitForBlock", "P1", 3, nil),
				sStatusEv("waitForBlock", "waitForBlock", "P2", 3, nil),
				sStatusEv("waitForBlock", "waitForBlock", "P3", 3, nil),

				// switch removes P2
				sPeerRemoveEv("waitForBlock", "waitForBlock", "P2", errSwitchRemovesPeer, []p2p.ID{"P2"}),
			},
		},
		{
			name:           "only peer removed while in waitForBlock state",
			startingHeight: 100,
			steps: []fsmStepTestValues{
				sStartFSMEv(),
				// add P1
				sStatusEv("waitForPeer", "waitForBlock", "P1", 200, nil),

				// switch removes P1
				sPeerRemoveEv("waitForBlock", "waitForPeer", "P1", errSwitchRemovesPeer, []p2p.ID{"P1"}),
			},
		},
		{
			name:               "highest peer removed while in waitForBlock state, node reaches maxPeerHeight - TS4 ",
			startingHeight:     100,
			maxRequestsPerPeer: 3,
			steps: []fsmStepTestValues{
				sStartFSMEv(),
				// add P1 and make requests
				sStatusEv("waitForPeer", "waitForBlock", "P1", 101, nil),
				sMakeRequestsEv("waitForBlock", "waitForBlock", maxNumRequests),
				// add P2
				sStatusEv("waitForBlock", "waitForBlock", "P2", 200, nil),

				// get blocks 100 and 101 from P1 and process block at height 100
				sBlockRespEv("waitForBlock", "waitForBlock", "P1", 100, []int64{}),
				sBlockRespEv("waitForBlock", "waitForBlock", "P1", 101, []int64{100}),
				sProcessedBlockEv("waitForBlock", "waitForBlock", nil),

				// switch removes peer P1, should be finished
				sPeerRemoveEv("waitForBlock", "finished", "P2", errSwitchRemovesPeer, []p2p.ID{"P2"}),
			},
		},
		{
			name:               "highest peer lowers its height in waitForBlock state, node reaches maxPeerHeight - TS4",
			startingHeight:     100,
			maxRequestsPerPeer: 3,
			steps: []fsmStepTestValues{
				sStartFSMEv(),
				// add P1 and make requests
				sStatusEv("waitForPeer", "waitForBlock", "P1", 101, nil),
				sMakeRequestsEv("waitForBlock", "waitForBlock", maxNumRequests),

				// add P2
				sStatusEv("waitForBlock", "waitForBlock", "P2", 200, nil),

				// get blocks 100 and 101 from P1
				sBlockRespEv("waitForBlock", "waitForBlock", "P1", 100, []int64{}),
				sBlockRespEv("waitForBlock", "waitForBlock", "P1", 101, []int64{100}),

				// processed block at heights 100
				sProcessedBlockEv("waitForBlock", "waitForBlock", nil),

				// P2 becomes short
				sStatusEv("waitForBlock", "finished", "P2", 100, errPeerLowersItsHeight),
			},
		},
		{
			name:           "new short peer while in waitForPeer state",
			startingHeight: 100,
			steps: []fsmStepTestValues{
				sStartFSMEv(),
				sStatusEv("waitForPeer", "waitForPeer", "P1", 3, errPeerTooShort),
			},
		},
		{
			name:           "new short peer while in waitForBlock state",
			startingHeight: 100,
			steps: []fsmStepTestValues{
				sStartFSMEv(),
				sStatusEv("waitForPeer", "waitForBlock", "P1", 200, nil),
				sStatusEv("waitForBlock", "waitForBlock", "P2", 3, errPeerTooShort),
			},
		},
		{
			name:           "only peer updated with low height while in waitForBlock state",
			startingHeight: 100,
			steps: []fsmStepTestValues{
				sStartFSMEv(),
				sStatusEv("waitForPeer", "waitForBlock", "P1", 200, nil),
				sStatusEv("waitForBlock", "waitForPeer", "P1", 3, errPeerLowersItsHeight),
			},
		},
		{
			name:               "peer does not exist in the switch",
			startingHeight:     9999999,
			maxRequestsPerPeer: 3,
			steps: []fsmStepTestValues{
				sStartFSMEv(),
				// add P1
				sStatusEv("waitForPeer", "waitForBlock", "P1", 20000000, nil),
				// send request for block 9999999
				// Note: For this block request the "switch missing the peer" error is simulated,
				// see implementation of bcReactor interface, sendBlockRequest(), in this file.
				sMakeRequestsEvErrored("waitForBlock", "waitForBlock",
					maxNumRequests, nil, []p2p.ID{"P1"}),
			},
		},
	}

	executeFSMTests(t, tests, true)
}

func TestFSMStopFSM(t *testing.T) {
	tests := []testFields{
		{
			name: "stopFSMEv in unknown",
			steps: []fsmStepTestValues{
				sStopFSMEv("unknown", "finished"),
			},
		},
		{
			name:           "stopFSMEv in waitForPeer",
			startingHeight: 1,
			steps: []fsmStepTestValues{
				sStartFSMEv(),
				sStopFSMEv("waitForPeer", "finished"),
			},
		},
		{
			name:           "stopFSMEv in waitForBlock",
			startingHeight: 1,
			steps: []fsmStepTestValues{
				sStartFSMEv(),
				sStatusEv("waitForPeer", "waitForBlock", "P1", 3, nil),
				sStopFSMEv("waitForBlock", "finished"),
			},
		},
	}

	executeFSMTests(t, tests, false)
}

func TestFSMUnknownElements(t *testing.T) {
	tests := []testFields{
		{
			name: "unknown event for state unknown",
			steps: []fsmStepTestValues{
				sUnknownFSMEv("unknown"),
			},
		},
		{
			name: "unknown event for state waitForPeer",
			steps: []fsmStepTestValues{
				sStartFSMEv(),
				sUnknownFSMEv("waitForPeer"),
			},
		},
		{
			name:           "unknown event for state waitForBlock",
			startingHeight: 1,
			steps: []fsmStepTestValues{
				sStartFSMEv(),
				sStatusEv("waitForPeer", "waitForBlock", "P1", 3, nil),
				sUnknownFSMEv("waitForBlock"),
			},
		},
	}

	executeFSMTests(t, tests, false)
}

func TestFSMPeerStateTimeoutEvent(t *testing.T) {
	tests := []testFields{
		{
			name:               "timeout event for state waitForPeer while in state waitForPeer - TS1",
			startingHeight:     1,
			maxRequestsPerPeer: 3,
			steps: []fsmStepTestValues{
				sStartFSMEv(),
				sStateTimeoutEv("waitForPeer", "finished", "waitForPeer", errNoTallerPeer),
			},
		},
		{
			name:               "timeout event for state waitForPeer while in a state != waitForPeer",
			startingHeight:     1,
			maxRequestsPerPeer: 3,
			steps: []fsmStepTestValues{
				sStartFSMEv(),
				sStateTimeoutEv("waitForPeer", "waitForPeer", "waitForBlock", errTimeoutEventWrongState),
			},
		},
		{
			name:               "timeout event for state waitForBlock while in state waitForBlock ",
			startingHeight:     1,
			maxRequestsPerPeer: 3,
			steps: []fsmStepTestValues{
				sStartFSMEv(),
				sStatusEv("waitForPeer", "waitForBlock", "P1", 3, nil),
				sMakeRequestsEv("waitForBlock", "waitForBlock", maxNumRequests),
				sStateTimeoutEv("waitForBlock", "waitForPeer", "waitForBlock", errNoPeerResponseForCurrentHeights),
			},
		},
		{
			name:               "timeout event for state waitForBlock while in a state != waitForBlock",
			startingHeight:     1,
			maxRequestsPerPeer: 3,
			steps: []fsmStepTestValues{
				sStartFSMEv(),
				sStatusEv("waitForPeer", "waitForBlock", "P1", 3, nil),
				sMakeRequestsEv("waitForBlock", "waitForBlock", maxNumRequests),
				sStateTimeoutEv("waitForBlock", "waitForBlock", "waitForPeer", errTimeoutEventWrongState),
			},
		},
		{
			name:               "timeout event for state waitForBlock with multiple peers",
			startingHeight:     1,
			maxRequestsPerPeer: 3,
			steps: []fsmStepTestValues{
				sStartFSMEv(),
				sStatusEv("waitForPeer", "waitForBlock", "P1", 3, nil),
				sMakeRequestsEv("waitForBlock", "waitForBlock", maxNumRequests),
				sStatusEv("waitForBlock", "waitForBlock", "P2", 3, nil),
				sStateTimeoutEv("waitForBlock", "waitForBlock", "waitForBlock", errNoPeerResponseForCurrentHeights),
			},
		},
	}

	executeFSMTests(t, tests, false)
}

func makeCorrectTransitionSequence(startingHeight int64, numBlocks int64, numPeers int, randomPeerHeights bool,
	maxRequestsPerPeer int, maxPendingRequests int) testFields {

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

	testName := fmt.Sprintf("%v-blocks %v-startingHeight %v-peers %v-maxRequestsPerPeer %v-maxNumRequests",
		numBlocks, startingHeight, numPeers, maxRequestsPerPeer, maxPendingRequests)

	// Add startFSMEv step.
	testSteps = append(testSteps, sStartFSMEv())

	// For each peer, add statusResponseEv step.
	for i := 0; i < numPeers; i++ {
		peerName := fmt.Sprintf("P%d", i)
		if i == 0 {
			testSteps = append(
				testSteps,
				sStatusEv("waitForPeer", "waitForBlock", p2p.ID(peerName), peerHeights[i], nil))
		} else {
			testSteps = append(testSteps,
				sStatusEv("waitForBlock", "waitForBlock", p2p.ID(peerName), peerHeights[i], nil))
		}
	}

	height := startingHeight
	numBlocksReceived := 0
	prevBlocks := make([]int64, 0, maxPendingRequests)

forLoop:
	for i := 0; i < int(numBlocks); i++ {

		// Add the makeRequestEv step periodically.
		if i%maxRequestsPerPeer == 0 {
			testSteps = append(
				testSteps,
				sMakeRequestsEv("waitForBlock", "waitForBlock", maxNumRequests),
			)
		}

		// Add the blockRespEv step
		testSteps = append(
			testSteps,
			sBlockRespEv("waitForBlock", "waitForBlock",
				"P0", height, prevBlocks))
		prevBlocks = append(prevBlocks, height)
		height++
		numBlocksReceived++

		// Add the processedBlockEv step periodically.
		if numBlocksReceived >= maxRequestsPerPeer || height >= numBlocks {
			for j := int(height) - numBlocksReceived; j < int(height); j++ {
				if j >= int(numBlocks) {
					// This is the last block that is processed, we should be in "finished" state.
					testSteps = append(
						testSteps,
						sProcessedBlockEv("waitForBlock", "finished", nil))
					break forLoop
				}
				testSteps = append(
					testSteps,
					sProcessedBlockEv("waitForBlock", "waitForBlock", nil))
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
	maxRequestsPerPeer := cmn.RandIntn(maxRequestsPerPeerTest) + 1

	// Generate the maximum number of total pending requests, >= maxRequestsPerPeer.
	maxPendingRequests := cmn.RandIntn(maxTotalPendingRequestsTest-maxRequestsPerPeer) + maxRequestsPerPeer

	// Generate the number of blocks to be synced.
	numBlocks := int64(cmn.RandIntn(maxNumBlocksInChainTest)) + startingHeight

	// Generate a number of peers.
	numPeers := cmn.RandIntn(maxNumPeersTest) + 1

	return makeCorrectTransitionSequence(startingHeight, numBlocks, numPeers, true, maxRequestsPerPeer, maxPendingRequests)
}

func shouldApplyProcessedBlockEvStep(step *fsmStepTestValues, testBcR *testReactor) bool {
	if step.event == processedBlockEv {
		_, err := testBcR.fsm.pool.BlockAndPeerAtHeight(testBcR.fsm.pool.Height)
		if err == errMissingBlock {
			return false
		}
		_, err = testBcR.fsm.pool.BlockAndPeerAtHeight(testBcR.fsm.pool.Height + 1)
		if err == errMissingBlock {
			return false
		}
	}
	return true
}

func TestFSMCorrectTransitionSequences(t *testing.T) {

	tests := []testFields{
		makeCorrectTransitionSequence(1, 100, 10, true, 10, 40),
		makeCorrectTransitionSequenceWithRandomParameters(),
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// Create test reactor
			testBcR := newTestReactor(tt.startingHeight)

			if tt.maxRequestsPerPeer != 0 {
				maxRequestsPerPeer = tt.maxRequestsPerPeer
			}

			for _, step := range tt.steps {
				step := step
				assert.Equal(t, step.currentState, testBcR.fsm.state.name)

				oldNumStatusRequests := testBcR.numStatusRequests
				fixBlockResponseEvStep(&step, testBcR)
				if !shouldApplyProcessedBlockEvStep(&step, testBcR) {
					continue
				}

				fsmErr := sendEventToFSM(testBcR.fsm, step.event, step.data)
				assert.Equal(t, step.wantErr, fsmErr)

				if step.wantStatusReqSent {
					assert.Equal(t, oldNumStatusRequests+1, testBcR.numStatusRequests)
				} else {
					assert.Equal(t, oldNumStatusRequests, testBcR.numStatusRequests)
				}

				assert.Equal(t, step.wantState, testBcR.fsm.state.name)
				if step.wantState == "finished" {
					assert.True(t, testBcR.fsm.isCaughtUp())
				}
			}

		})
	}
}

// ----------------------------------------
// implements the bcRNotifier
func (testR *testReactor) sendPeerError(err error, peerID p2p.ID) {
	testR.logger.Info("Reactor received sendPeerError call from FSM", "peer", peerID, "err", err)
	testR.lastPeerError.peerID = peerID
	testR.lastPeerError.err = err
}

func (testR *testReactor) sendStatusRequest() {
	testR.logger.Info("Reactor received sendStatusRequest call from FSM")
	testR.numStatusRequests++
}

func (testR *testReactor) sendBlockRequest(peerID p2p.ID, height int64) error {
	testR.logger.Info("Reactor received sendBlockRequest call from FSM", "peer", peerID, "height", height)
	testR.numBlockRequests++
	testR.lastBlockRequest.peerID = peerID
	testR.lastBlockRequest.height = height
	if height == 9999999 {
		// simulate switch does not have peer
		return errNilPeerForBlockRequest
	}
	return nil
}

func (testR *testReactor) resetStateTimer(name string, timer **time.Timer, timeout time.Duration) {
	testR.logger.Info("Reactor received resetStateTimer call from FSM", "state", name, "timeout", timeout)
	if _, ok := testR.stateTimerStarts[name]; !ok {
		testR.stateTimerStarts[name] = 1
	} else {
		testR.stateTimerStarts[name]++
	}
}

func (testR *testReactor) switchToConsensus() {
}

// ----------------------------------------
