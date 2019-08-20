package v2

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

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

// TestBlockReceive

func TestProcessorStop(t *testing.T) {
	// create a mockContext
	var (
		initHeight     int64 = 0
		state                = state.State{}
		chainID              = "TestChain"
		applicationBL        = []types.BlockID{}
		verificationBL       = []types.BlockID{}
		context              = newMockProcessorContext(verificationBL, applicationBL)
		processor            = newProcessor(initHeight, state, chainID, context)
	)
	processor.setLogger(log.TestingLogger())

	assert.False(t, processor.isRunning(),
		"expected an initialized processor to not be running")
	go processor.start()
	<-processor.ready()

	assert.True(t, processor.trySend(pcStop{}),
		"expected stopping to a ready processor to succeed")

	assert.Equal(t, pcFinished{}, <-processor.final(),
		"expected the final event to be done")
}
