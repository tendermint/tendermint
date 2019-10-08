package v2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/p2p"
	tdState "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

// bl is a test helper structure with short name and simple types. Its purpose is to help with test readability.
type bl struct {
	pid    string
	height int64
}

// params is a test structure used to create processor state.
type params struct {
	height       int64
	items        []bl
	blocksSynced int64
	verBL        []int64
	appBL        []int64
	draining     bool
}

// makePcBlock makes an empty block.
func makePcBlock(height int64) *types.Block {
	return &types.Block{Header: types.Header{Height: height}}
}

// mst takes test parameters and creates a specific processor state. Name is kept short for test readability.
func mst(p params) *pcState {
	var (
		tdState = tdState.State{}
		context = newMockProcessorContext(p.verBL, p.appBL)
	)
	state := newPcState(p.height, tdState, "test", context)

	for _, item := range p.items {
		_ = state.enqueue(p2p.ID(item.pid), makePcBlock(item.height), item.height)
	}

	state.blocksSynced = p.blocksSynced
	state.draining = p.draining
	return state
}

func mBlockResponse(peerID p2p.ID, height int64) *bcBlockResponse {
	return &bcBlockResponse{
		peerID: peerID,
		block:  makePcBlock(height),
		height: height,
	}
}

type pcFsmStepTestValues struct {
	currentState  *pcState
	event         Event
	wantState     *pcState
	wantNextEvent Event
	wantErr       error
	wantPanic     bool
}

type testFields struct {
	name  string
	steps []pcFsmStepTestValues
}

func executeProcessorTests(t *testing.T, tests []testFields) {
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var state *pcState
			for _, step := range tt.steps {
				defer func() {
					r := recover()
					if (r != nil) != step.wantPanic {
						t.Errorf("recover = %v, wantPanic = %v", r, step.wantPanic)
					}
				}()
				// First step must always initialise the currentState as state.
				if step.currentState != nil {
					state = step.currentState
				}
				if state == nil {
					panic("Bad (initial?) step")
				}

				nextEvent, err := state.handle(step.event)
				t.Log(state)
				assert.Equal(t, step.wantErr, err)
				assert.Equal(t, step.wantState, state)
				assert.Equal(t, step.wantNextEvent, nextEvent)
				// Next step may use the wantedState as their currentState.
				state = step.wantState
			}
		})
	}
}

func TestPcBlockResponse(t *testing.T) {
	tests := []testFields{
		{
			name: "add one block",
			steps: []pcFsmStepTestValues{
				{
					currentState: mst(params{}), event: mBlockResponse("P1", 1),
					wantState: mst(params{items: []bl{{"P1", 1}}}), wantNextEvent: noOp,
				},
			},
		},
		{
			name: "add two blocks",
			steps: []pcFsmStepTestValues{
				{
					currentState: mst(params{}), event: mBlockResponse("P1", 3),
					wantState: mst(params{items: []bl{{"P1", 3}}}), wantNextEvent: noOp,
				},
				{ // use previous wantState as currentState,
					event:     mBlockResponse("P1", 4),
					wantState: mst(params{items: []bl{{"P1", 3}, {"P1", 4}}}), wantNextEvent: noOp,
				},
			},
		},
		{
			name: "add duplicate block from same peer",
			steps: []pcFsmStepTestValues{
				{
					currentState: mst(params{}), event: mBlockResponse("P1", 3),
					wantState: mst(params{items: []bl{{"P1", 3}}}), wantNextEvent: noOp,
				},
				{ // use previous wantState as currentState,
					event:     mBlockResponse("P1", 3),
					wantState: mst(params{items: []bl{{"P1", 3}}}), wantNextEvent: pcDuplicateBlock{},
				},
			},
		},
		{
			name: "add duplicate block from different peer",
			steps: []pcFsmStepTestValues{
				{
					currentState: mst(params{}), event: mBlockResponse("P1", 3),
					wantState: mst(params{items: []bl{{"P1", 3}}}), wantNextEvent: noOp,
				},
				{ // use previous wantState as currentState,
					event:     mBlockResponse("P2", 3),
					wantState: mst(params{items: []bl{{"P1", 3}}}), wantNextEvent: pcDuplicateBlock{},
				},
			},
		},
	}

	executeProcessorTests(t, tests)
}

func TestPcProcessBlockSuccess(t *testing.T) {
	tests := []testFields{
		{
			name: "noop - no blocks over current height",
			steps: []pcFsmStepTestValues{
				{
					currentState: mst(params{}), event: pcProcessBlock{},
					wantState: mst(params{}), wantNextEvent: noOp,
				},
			},
		},
		{
			name: "noop - high new blocks",
			steps: []pcFsmStepTestValues{
				{
					currentState: mst(params{height: 5, items: []bl{{"P1", 30}, {"P2", 31}}}), event: pcProcessBlock{},
					wantState: mst(params{height: 5, items: []bl{{"P1", 30}, {"P2", 31}}}), wantNextEvent: noOp,
				},
			},
		},
		{
			name: "blocks H+1 and H+2 present",
			steps: []pcFsmStepTestValues{
				{
					currentState: mst(params{items: []bl{{"P1", 1}, {"P2", 2}}}), event: pcProcessBlock{},
					wantState:     mst(params{height: 1, items: []bl{{"P2", 2}}, blocksSynced: 1}),
					wantNextEvent: pcBlockProcessed{height: 1, peerID: "P1"},
				},
			},
		},
	}

	executeProcessorTests(t, tests)
}

func TestPcProcessBlockFailures(t *testing.T) {
	tests := []testFields{
		{
			name: "blocks H+1 and H+2 present - H+1 verification fails ",
			steps: []pcFsmStepTestValues{
				{
					currentState: mst(params{items: []bl{{"P1", 1}, {"P2", 2}}, verBL: []int64{1}}), event: pcProcessBlock{},
					wantState:     mst(params{items: []bl{{"P1", 1}, {"P2", 2}}, verBL: []int64{1}}),
					wantNextEvent: pcBlockVerificationFailure{peerID: "P1", height: 1},
				},
			},
		},
		{
			name: "blocks H+1 and H+2 present - H+1 applyBlock fails ",
			steps: []pcFsmStepTestValues{
				{
					currentState: mst(params{items: []bl{{"P1", 1}, {"P2", 2}}, appBL: []int64{1}}), event: pcProcessBlock{},
					wantState: mst(params{items: []bl{{"P1", 1}, {"P2", 2}}, appBL: []int64{1}}),
					wantPanic: true,
				},
			},
		},
	}

	executeProcessorTests(t, tests)
}

func TestPcPeerError(t *testing.T) {
	tests := []testFields{
		{
			name: "peer not present",
			steps: []pcFsmStepTestValues{
				{
					currentState: mst(params{items: []bl{{"P1", 1}, {"P2", 2}}}), event: &peerError{peerID: "P3"},
					wantState:     mst(params{items: []bl{{"P1", 1}, {"P2", 2}}}),
					wantNextEvent: noOp,
				},
			},
		},
		{
			name: "some blocks are from errored peer",
			steps: []pcFsmStepTestValues{
				{
					currentState: mst(params{items: []bl{{"P1", 100}, {"P1", 99}, {"P2", 101}}}), event: &peerError{peerID: "P1"},
					wantState:     mst(params{items: []bl{{"P2", 101}}}),
					wantNextEvent: noOp,
				},
			},
		},
		{
			name: "all blocks are from errored peer",
			steps: []pcFsmStepTestValues{
				{
					currentState: mst(params{items: []bl{{"P1", 100}, {"P1", 99}}}), event: &peerError{peerID: "P1"},
					wantState:     mst(params{}),
					wantNextEvent: noOp,
				},
			},
		},
	}

	executeProcessorTests(t, tests)
}

func TestStop(t *testing.T) {
	tests := []testFields{
		{
			name: "no blocks",
			steps: []pcFsmStepTestValues{
				{
					currentState: mst(params{height: 100, items: []bl{}, blocksSynced: 100}), event: pcStop{},
					wantState:     mst(params{height: 100, items: []bl{}, blocksSynced: 100}),
					wantNextEvent: noOp,
					wantErr:       pcFinished{height: 100, blocksSynced: 100},
				},
			},
		},
		{
			name: "maxHeight+1 block present",
			steps: []pcFsmStepTestValues{
				{
					currentState: mst(params{height: 100, items: []bl{{"P1", 101}}, blocksSynced: 100}), event: pcStop{},
					wantState:     mst(params{height: 100, items: []bl{{"P1", 101}}, blocksSynced: 100}),
					wantNextEvent: noOp,
					wantErr:       pcFinished{height: 100, blocksSynced: 100},
				},
			},
		},
		{
			name: "more blocks present",
			steps: []pcFsmStepTestValues{
				{
					currentState: mst(params{height: 100, items: []bl{{"P1", 101}, {"P1", 102}}, blocksSynced: 100}), event: pcStop{},
					wantState:     mst(params{height: 100, items: []bl{{"P1", 101}, {"P1", 102}}, blocksSynced: 100, draining: true}),
					wantNextEvent: noOp,
					wantErr:       nil,
				},
			},
		},
	}

	executeProcessorTests(t, tests)
}
