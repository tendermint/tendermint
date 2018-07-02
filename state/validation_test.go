package state

import (
	"testing"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/libs/log"
)

func TestValidateBlock(t *testing.T) {
	state, _ := state(1, 1)

	blockExec := NewBlockExecutor(dbm.NewMemDB(), log.TestingLogger(), nil, nil, nil)

	// proper block must pass
	block := makeBlock(state, 1)
	err := blockExec.ValidateBlock(state, block)
	require.NoError(t, err)

	// wrong chain fails
	block = makeBlock(state, 1)
	block.ChainID = "not-the-real-one"
	err = blockExec.ValidateBlock(state, block)
	require.Error(t, err)

	// wrong height fails
	block = makeBlock(state, 1)
	block.Height += 10
	err = blockExec.ValidateBlock(state, block)
	require.Error(t, err)

	// wrong total tx fails
	block = makeBlock(state, 1)
	block.TotalTxs += 10
	err = blockExec.ValidateBlock(state, block)
	require.Error(t, err)

	// wrong blockid fails
	block = makeBlock(state, 1)
	block.LastBlockID.PartsHeader.Total += 10
	err = blockExec.ValidateBlock(state, block)
	require.Error(t, err)

	// wrong app hash fails
	block = makeBlock(state, 1)
	block.AppHash = []byte("wrong app hash")
	err = blockExec.ValidateBlock(state, block)
	require.Error(t, err)

	// wrong consensus hash fails
	block = makeBlock(state, 1)
	block.ConsensusHash = []byte("wrong consensus hash")
	err = blockExec.ValidateBlock(state, block)
	require.Error(t, err)

	// wrong results hash fails
	block = makeBlock(state, 1)
	block.LastResultsHash = []byte("wrong results hash")
	err = blockExec.ValidateBlock(state, block)
	require.Error(t, err)

	// wrong validators hash fails
	block = makeBlock(state, 1)
	block.ValidatorsHash = []byte("wrong validators hash")
	err = blockExec.ValidateBlock(state, block)
	require.Error(t, err)
}
