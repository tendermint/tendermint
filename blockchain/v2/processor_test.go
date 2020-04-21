package v2

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tendermint/p2p"
	tmState "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

// pcBlock is a test helper structure with simple types. Its purpose is to help with test readability.
type pcBlock struct {
	pid    string
	height int64
}

// params is a test structure used to create processor state.
type params struct {
	height       int64
	items        []pcBlock
	blocksSynced int
	verBL        []int64
	appBL        []int64
	draining     bool
}

// makePcBlock makes an empty block.
func makePcBlock(height int64) *types.Block {
	return &types.Block{Header: types.Header{Height: height}}
}

// makeState takes test parameters and creates a specific processor state.
func makeState(p *params) *pcState {
	var (
		tmState = tmState.State{LastBlockHeight: p.height}
		context = newMockProcessorContext(tmState, p.verBL, p.appBL)
	)
	state := newPcState(context)

	for _, item := range p.items {
		state.enqueue(p2p.ID(item.pid), makePcBlock(item.height), item.height)
	}

	state.blocksSynced = p.blocksSynced
	state.draining = p.draining
	return state
}

func mBlockResponse(peerID p2p.ID, height int64) scBlockReceived {
	return scBlockReceived{
		peerID: peerID,
		block:  makePcBlock(height),
	}
}

type pcFsmMakeStateValues struct {
	currentState  *params
	event         Event
	wantState     *params
	wantNextEvent Event
	wantErr       error
	wantPanic     bool
}

type testFields struct {
	name  string
	steps []pcFsmMakeStateValues
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
					state = makeState(step.currentState)
				}
				if state == nil {
					panic("Bad (initial?) step")
				}

				nextEvent, err := state.handle(step.event)
				t.Log(state)
				assert.Equal(t, step.wantErr, err)
				assert.Equal(t, makeState(step.wantState), state)
				assert.Equal(t, step.wantNextEvent, nextEvent)
				// Next step may use the wantedState as their currentState.
				state = makeState(step.wantState)
			}
		})
	}
}

func TestRProcessPeerError(t *testing.T) {
	tests := []testFields{
		{
			name: "error for existing peer",
			steps: []pcFsmMakeStateValues{
				{
					currentState:  &params{items: []pcBlock{{"P1", 1}, {"P2", 2}}},
					event:         scPeerError{peerID: "P2"},
					wantState:     &params{items: []pcBlock{{"P1", 1}}},
					wantNextEvent: noOp,
				},
			},
		},
		{
			name: "error for unknown peer",
			steps: []pcFsmMakeStateValues{
				{
					currentState:  &params{items: []pcBlock{{"P1", 1}, {"P2", 2}}},
					event:         scPeerError{peerID: "P3"},
					wantState:     &params{items: []pcBlock{{"P1", 1}, {"P2", 2}}},
					wantNextEvent: noOp,
				},
			},
		},
	}

	executeProcessorTests(t, tests)
}

func TestPcBlockResponse(t *testing.T) {
	tests := []testFields{
		{
			name: "add one block",
			steps: []pcFsmMakeStateValues{
				{
					currentState: &params{}, event: mBlockResponse("P1", 1),
					wantState: &params{items: []pcBlock{{"P1", 1}}}, wantNextEvent: noOp,
				},
			},
		},

		{
			name: "add two blocks",
			steps: []pcFsmMakeStateValues{
				{
					currentState: &params{}, event: mBlockResponse("P1", 3),
					wantState: &params{items: []pcBlock{{"P1", 3}}}, wantNextEvent: noOp,
				},
				{ // use previous wantState as currentState,
					event:     mBlockResponse("P1", 4),
					wantState: &params{items: []pcBlock{{"P1", 3}, {"P1", 4}}}, wantNextEvent: noOp,
				},
			},
		},
	}

	executeProcessorTests(t, tests)
}

func TestRProcessBlockSuccess(t *testing.T) {
	tests := []testFields{
		{
			name: "noop - no blocks over current height",
			steps: []pcFsmMakeStateValues{
				{
					currentState: &params{}, event: rProcessBlock{},
					wantState: &params{}, wantNextEvent: noOp,
				},
			},
		},
		{
			name: "noop - high new blocks",
			steps: []pcFsmMakeStateValues{
				{
					currentState: &params{height: 5, items: []pcBlock{{"P1", 30}, {"P2", 31}}}, event: rProcessBlock{},
					wantState: &params{height: 5, items: []pcBlock{{"P1", 30}, {"P2", 31}}}, wantNextEvent: noOp,
				},
			},
		},
		{
			name: "blocks H+1 and H+2 present",
			steps: []pcFsmMakeStateValues{
				{
					currentState: &params{items: []pcBlock{{"P1", 1}, {"P2", 2}}}, event: rProcessBlock{},
					wantState:     &params{height: 1, items: []pcBlock{{"P2", 2}}, blocksSynced: 1},
					wantNextEvent: pcBlockProcessed{height: 1, peerID: "P1"},
				},
			},
		},
		{
			name: "blocks H+1 and H+2 present after draining",
			steps: []pcFsmMakeStateValues{
				{ // some contiguous blocks - on stop check draining is set
					currentState:  &params{items: []pcBlock{{"P1", 1}, {"P2", 2}, {"P1", 4}}},
					event:         scFinishedEv{},
					wantState:     &params{items: []pcBlock{{"P1", 1}, {"P2", 2}, {"P1", 4}}, draining: true},
					wantNextEvent: noOp,
				},
				{
					event:         rProcessBlock{},
					wantState:     &params{height: 1, items: []pcBlock{{"P2", 2}, {"P1", 4}}, blocksSynced: 1, draining: true},
					wantNextEvent: pcBlockProcessed{height: 1, peerID: "P1"},
				},
				{ // finish when H+1 or/and H+2 are missing
					event:         rProcessBlock{},
					wantState:     &params{height: 1, items: []pcBlock{{"P2", 2}, {"P1", 4}}, blocksSynced: 1, draining: true},
					wantNextEvent: pcFinished{tmState: tmState.State{LastBlockHeight: 1}, blocksSynced: 1},
				},
			},
		},
	}

	executeProcessorTests(t, tests)
}

func TestRProcessBlockFailures(t *testing.T) {
	tests := []testFields{
		{
			name: "blocks H+1 and H+2 present from different peers - H+1 verification fails ",
			steps: []pcFsmMakeStateValues{
				{
					currentState: &params{items: []pcBlock{{"P1", 1}, {"P2", 2}}, verBL: []int64{1}}, event: rProcessBlock{},
					wantState:     &params{items: []pcBlock{}, verBL: []int64{1}},
					wantNextEvent: pcBlockVerificationFailure{height: 1, firstPeerID: "P1", secondPeerID: "P2"},
				},
			},
		},
		{
			name: "blocks H+1 and H+2 present from same peer - H+1 applyBlock fails ",
			steps: []pcFsmMakeStateValues{
				{
					currentState: &params{items: []pcBlock{{"P1", 1}, {"P2", 2}}, appBL: []int64{1}}, event: rProcessBlock{},
					wantState: &params{items: []pcBlock{}, appBL: []int64{1}}, wantPanic: true,
				},
			},
		},
		{
			name: "blocks H+1 and H+2 present from same peers - H+1 verification fails ",
			steps: []pcFsmMakeStateValues{
				{
					currentState: &params{height: 0, items: []pcBlock{{"P1", 1}, {"P1", 2}, {"P2", 3}},
						verBL: []int64{1}}, event: rProcessBlock{},
					wantState:     &params{height: 0, items: []pcBlock{{"P2", 3}}, verBL: []int64{1}},
					wantNextEvent: pcBlockVerificationFailure{height: 1, firstPeerID: "P1", secondPeerID: "P1"},
				},
			},
		},
		{
			name: "blocks H+1 and H+2 present from different peers - H+1 applyBlock fails ",
			steps: []pcFsmMakeStateValues{
				{
					currentState: &params{items: []pcBlock{{"P1", 1}, {"P2", 2}, {"P2", 3}}, appBL: []int64{1}},
					event:        rProcessBlock{},
					wantState:    &params{items: []pcBlock{{"P2", 3}}, appBL: []int64{1}}, wantPanic: true,
				},
			},
		},
	}

	executeProcessorTests(t, tests)
}

func TestScFinishedEv(t *testing.T) {
	tests := []testFields{
		{
			name: "no blocks",
			steps: []pcFsmMakeStateValues{
				{
					currentState: &params{height: 100, items: []pcBlock{}, blocksSynced: 100}, event: scFinishedEv{},
					wantState:     &params{height: 100, items: []pcBlock{}, blocksSynced: 100},
					wantNextEvent: pcFinished{tmState: tmState.State{LastBlockHeight: 100}, blocksSynced: 100},
				},
			},
		},
		{
			name: "maxHeight+1 block present",
			steps: []pcFsmMakeStateValues{
				{
					currentState: &params{height: 100, items: []pcBlock{
						{"P1", 101}}, blocksSynced: 100}, event: scFinishedEv{},
					wantState:     &params{height: 100, items: []pcBlock{{"P1", 101}}, blocksSynced: 100},
					wantNextEvent: pcFinished{tmState: tmState.State{LastBlockHeight: 100}, blocksSynced: 100},
				},
			},
		},
		{
			name: "more blocks present",
			steps: []pcFsmMakeStateValues{
				{
					currentState: &params{height: 100, items: []pcBlock{
						{"P1", 101}, {"P1", 102}}, blocksSynced: 100}, event: scFinishedEv{},
					wantState: &params{height: 100, items: []pcBlock{
						{"P1", 101}, {"P1", 102}}, blocksSynced: 100, draining: true},
					wantNextEvent: noOp,
					wantErr:       nil,
				},
			},
		},
	}

	executeProcessorTests(t, tests)
}
