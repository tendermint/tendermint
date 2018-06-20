package types

import (
	"testing"

	asrt "github.com/stretchr/testify/assert"
)

func TestConsensusParams(t *testing.T) {
	assert := asrt.New(t)

	params := &ConsensusParams{
		BlockSize:   &BlockSize{MaxGas: 12345},
		BlockGossip: &BlockGossip{BlockPartSizeBytes: 54321},
	}
	var noParams *ConsensusParams // nil

	// no error with nil fields
	assert.Nil(noParams.GetBlockSize())
	assert.EqualValues(noParams.GetBlockSize().GetMaxGas(), 0)

	// get values with real fields
	assert.NotNil(params.GetBlockSize())
	assert.EqualValues(params.GetBlockSize().GetMaxTxs(), 0)
	assert.EqualValues(params.GetBlockSize().GetMaxGas(), 12345)
	assert.NotNil(params.GetBlockGossip())
	assert.EqualValues(params.GetBlockGossip().GetBlockPartSizeBytes(), 54321)
	assert.Nil(params.GetTxSize())
	assert.EqualValues(params.GetTxSize().GetMaxBytes(), 0)

}
