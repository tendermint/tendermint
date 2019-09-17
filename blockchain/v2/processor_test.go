package v2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/p2p"
	tdState "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

// TODO
// * Implement tests using the new FSM API
// * Create a new PR

/*
# Processor tests

## TestBlockPoolFirstTwoBlocksAndPeers
* both blocks missing
* second block missing
* first block missing
* both blocks present

## TestBlockPoolInvalidateFirstTwoBlocks
* both blocks missing
* second block missing
* first block missing
* both blocks present

## TestProcessedCurrentHeightBlock
* one peer
* multiple peers

## TestRemovePeerAtCurrentHeight
* one peer, remove peer for block at H
* one peer, remove peer for block at H+1
* multiple peers, remove peer for block at H
* multiple peers, remove peer for block at H+1
**/

func TestBlockResponse(t *testing.T) {
	var (
		peerID         p2p.ID = "peer"
		initHeight     int64  = 0
		initBlock             = &types.Block{Header: types.Header{Height: initHeight}}
		tdState               = tdState.State{}
		applicationBL         = []types.BlockID{}
		verificationBL        = []types.BlockID{}
		context               = newMockProcessorContext(verificationBL, applicationBL)
		state                 = &pcState{
			bq:           newBlockQueue(initBlock),
			draining:     false,
			blocksSynced: 0,
			context:      context,
			tdState:      tdState,
		}
	)

	event, nextState, err := pcHandle(&bcBlockResponse{
		peerID: peerID,
		block:  initBlock,
		height: initHeight,
	}, state)

	assert.Equal(t, pcDuplicateBlock{}, event, "expected duplicate block error")
	assert.Equal(t, state, nextState, "expected state to go unchanged")

	nextHeight := initHeight + 1
	nextBlock := &types.Block{Header: types.Header{Height: nextHeight}}
	nextEvent := &bcBlockResponse{
		peerID: peerID,
		height: nextHeight,
		block:  nextBlock,
	}

	event, state, err = pcHandle(nextEvent, state)
	assert.NoError(t, err)
	assert.Equal(t, state, &pcState{
		draining: false,
		bq: &blockQueue{
			height: nextHeight,
			queue: map[int64]*queueItem{
				initHeight: &queueItem{block: initBlock, peerID: peerID},
				nextHeight: &queueItem{block: nextBlock, peerID: peerID},
			},
		},
		context: context,
		tdState: tdState,
	})
}

func TestProcessBlock(t *testing.T) {
}

func TestPeerError(t *testing.T) {
}

func TestBlockProcessed(t *testing.T) {
}

func TestStop(t *testing.T) {
}

/*
func TestProcessorStop(t *testing.T) {
	var (
		initHeight     int64 = 0
		initBlock            = types.Block{Header: types.Header{Height: initHeight}}
		state                = state.State{}
		chainID              = "TestChain"
		applicationBL        = []types.BlockID{}
		verificationBL       = []types.BlockID{}
		context              = newMockProcessorContext(verificationBL, applicationBL)
		processor            = newProcessor(&initBlock, state, chainID, context)
	)
	processor.setLogger(log.TestingLogger())

	assert.False(t, processor.isRunning(),
		"expected an initialized processor to not be running")
	go processor.start()
	go processor.feedback()
	<-processor.ready()

	assert.True(t, processor.trySend(pcStop{}),
		"expected stopping to a ready processor to succeed")

	assert.Equal(t, pcFinished{}, <-processor.final(),
		"expected the final event to be done")
}

// XXX: It would be much better here if:
// 1. we used synchronous `send` method
// 2. Each even produced at most one event

// what should the correct behaviour be here?

func TestProcessorBlockReceived(t *testing.T) {
	var (
		initHeight     int64  = 0
		initBlock             = types.Block{Header: types.Header{Height: initHeight}}
		state                 = state.State{}
		chainID               = "TestChain"
		applicationBL         = []types.BlockID{}
		verificationBL        = []types.BlockID{}
		context               = newMockProcessorContext(verificationBL, applicationBL)
		processor             = newProcessor(&initBlock, state, chainID, context)
		peerID         p2p.ID = "1"
	)
	processor.setLogger(log.TestingLogger())

	assert.False(t, processor.isRunning(),
		"expected an initialized processor to not be running")
	go processor.start()
	go processor.feedback()
	<-processor.ready()

	// XXX: Blocks have a Height field
	firstHeader := types.Header{Height: initHeight + 1}
	firstBlock := types.Block{Header: firstHeader}
	assert.True(t, processor.trySend(&bcBlockResponse{peerID: peerID, height: initHeight + 1, block: &firstBlock}),
		"expected sending a block to succeed")

	secondHeader := types.Header{Height: initHeight + 2}
	secondBlock := types.Block{Header: secondHeader}
	assert.True(t, processor.trySend(&bcBlockResponse{peerID: peerID, height: initHeight + 2, block: &secondBlock}),
		"expected sending a block to succeed")

	assert.True(t, processor.trySend(pcStop{}),
		"expected stopping to a ready processor to succeed")

	assert.Equal(t, pcFinished{height: initHeight + 1}, <-processor.final(),
		"expected the final event to be done")
}
*/
