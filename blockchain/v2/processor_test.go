package v2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/p2p"
	tdState "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

// Block Response
func TestBlockAddition(t *testing.T) {
	var (
		peerID         p2p.ID = "peer"
		initHeight     int64  = 0
		initBlock             = &types.Block{Header: types.Header{Height: initHeight}}
		nextHeight            = initHeight + 1
		nextBlock             = &types.Block{Header: types.Header{Height: nextHeight}}
		tdState               = tdState.State{}
		applicationBL         = []types.BlockID{}
		verificationBL        = []types.BlockID{}
		context               = newMockProcessorContext(verificationBL, applicationBL)
		state                 = &pcState{
			height: initHeight,
			queue: blockQueue{
				initHeight: queueItem{block: initBlock, peerID: peerID},
			},
			draining:     false,
			blocksSynced: 0,
			context:      context,
			tdState:      tdState,
		}
		event = &bcBlockResponse{
			peerID: peerID,
			block:  nextBlock,
			height: nextHeight,
		}
	)

	nextEvent, err := state.handle(event)
	assert.NoError(t, err, "expected no error")

	assert.Equal(t, state, &pcState{
		draining:     false,
		blocksSynced: 0,
		height:       initHeight,
		queue: blockQueue{
			initHeight: queueItem{block: initBlock, peerID: peerID},
			nextHeight: queueItem{block: nextBlock, peerID: peerID},
		},
		context: context,
		tdState: tdState,
	}, "expected the addition of a new block to the queue")
	assert.Equal(t, noOp, nextEvent, "expected noOp event")
}

func TestProcessDuplicateBlock(t *testing.T) {
	var (
		peerID         p2p.ID = "peer"
		initHeight     int64  = 0
		initBlock             = &types.Block{Header: types.Header{Height: initHeight}}
		tdState               = tdState.State{}
		applicationBL         = []types.BlockID{}
		verificationBL        = []types.BlockID{}
		context               = newMockProcessorContext(verificationBL, applicationBL)
		state                 = &pcState{
			height: initHeight,
			queue: blockQueue{
				initHeight: queueItem{block: initBlock, peerID: peerID},
			},
			draining:     false,
			blocksSynced: 0,
			context:      context,
			tdState:      tdState,
		}
		event = &bcBlockResponse{
			peerID: peerID,
			block:  initBlock,
			height: initHeight,
		}
	)

	nextEvent, err := state.handle(event)

	assert.NoError(t, err, "expected no error")
	assert.Equal(t, state, &pcState{
		height: initHeight,
		queue: blockQueue{
			initHeight: queueItem{block: initBlock, peerID: peerID},
		},
		draining:     false,
		blocksSynced: 0,
		context:      context,
		tdState:      tdState,
	}, "expected state to go unchanged")
	assert.Equal(t, pcDuplicateBlock{}, nextEvent, "expected duplicate block event")
}

func TestProcessSingleBlock(t *testing.T) {
	var (
		peerID         p2p.ID = "peer"
		initHeight     int64  = 0
		initBlock             = &types.Block{Header: types.Header{Height: initHeight}}
		tdState               = tdState.State{}
		applicationBL         = []types.BlockID{}
		verificationBL        = []types.BlockID{}
		context               = newMockProcessorContext(verificationBL, applicationBL)
		state                 = &pcState{
			height: initHeight,
			queue: blockQueue{
				initHeight: queueItem{block: initBlock, peerID: peerID},
			},
			draining:     false,
			blocksSynced: 0,
			context:      context,
			tdState:      tdState,
		}
		event = &pcProcessBlock{}
	)

	nextEvent, err := state.handle(event)
	assert.NoError(t, err, "expected no error")
	assert.Equal(t, noOp, nextEvent, "expected noOp event")
	assert.Equal(t, state, &pcState{
		height: initHeight,
		queue: blockQueue{
			initHeight: queueItem{block: initBlock, peerID: peerID},
		},
		draining:     false,
		blocksSynced: 0,
		context:      context,
		tdState:      tdState,
	}, "expected state to go unchanged")
}

func TestProcessBlockEmptyBlock(t *testing.T) {
	var (
		peerID         p2p.ID = "peer"
		initHeight     int64  = 0
		initBlock             = &types.Block{Header: types.Header{Height: initHeight}}
		tdState               = tdState.State{}
		applicationBL         = []types.BlockID{}
		verificationBL        = []types.BlockID{}
		context               = newMockProcessorContext(verificationBL, applicationBL)
		nextNextHeight        = initHeight + 2
		nextNextBlock         = &types.Block{Header: types.Header{Height: nextNextHeight}}
		event                 = pcProcessBlock{}
		state                 = &pcState{
			height: initHeight,
			queue: blockQueue{
				initHeight:     queueItem{block: initBlock, peerID: peerID},
				nextNextHeight: queueItem{block: nextNextBlock, peerID: peerID},
			},
			draining:     false,
			blocksSynced: 0,
			context:      context,
			tdState:      tdState,
		}
	)

	nextEvent, err := state.handle(event)

	assert.NoError(t, err, "expected no error")
	assert.Equal(t, noOp, nextEvent, "expected noOp event")
	assert.Equal(t, state, &pcState{
		height: initHeight,
		queue: blockQueue{
			initHeight:     queueItem{block: initBlock, peerID: peerID},
			nextNextHeight: queueItem{block: nextNextBlock, peerID: peerID},
		},
		draining:     false,
		blocksSynced: 0,
		context:      context,
		tdState:      tdState,
	}, "expected state to go unchanged")
}

// Test with verificationBL
// Test with applicationBL

func TestProcessAdvance(t *testing.T) {
	var (
		peerID         p2p.ID = "peer"
		initHeight     int64  = 0
		initBlock             = &types.Block{Header: types.Header{Height: initHeight}}
		tdState               = tdState.State{}
		applicationBL         = []types.BlockID{}
		verificationBL        = []types.BlockID{}
		context               = newMockProcessorContext(verificationBL, applicationBL)
		nextHeight            = initHeight + 1
		nextBlock             = &types.Block{Header: types.Header{Height: nextHeight}}
		nextNextHeight        = initHeight + 2
		nextNextBlock         = &types.Block{Header: types.Header{Height: nextNextHeight}}
		state                 = &pcState{
			height: initHeight,
			queue: blockQueue{
				initHeight:     queueItem{block: initBlock, peerID: peerID},
				nextHeight:     queueItem{block: nextBlock, peerID: peerID},
				nextNextHeight: queueItem{block: nextNextBlock, peerID: peerID},
			},
			draining:     false,
			blocksSynced: 0,
			context:      context,
			tdState:      tdState,
		}
		event = pcProcessBlock{}
	)

	nextEvent, err := state.handle(event)

	assert.NoError(t, err, "expected no error")
	assert.Equal(t, nextEvent, pcBlockProcessed{
		height: nextHeight,
		peerID: peerID}, "expected the correct bcBlockProcessed event")
	assert.Equal(t, state, &pcState{
		height: nextHeight,
		queue: blockQueue{
			nextHeight:     queueItem{block: nextBlock, peerID: peerID},
			nextNextHeight: queueItem{block: nextNextBlock, peerID: peerID},
		},
		draining:     false,
		blocksSynced: 1,
		context:      context,
		tdState:      tdState,
	}, "expected the state to have advanced")
}

func TestPeerError(t *testing.T) {
	// Test queue removal
}

func TestStop(t *testing.T) {
	// test with empty queue
	// test with non empty queue
}
