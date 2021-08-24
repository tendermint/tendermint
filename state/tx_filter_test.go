package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/pkg/mempool"
	sm "github.com/tendermint/tendermint/state"
)

func TestTxFilter(t *testing.T) {
	genDoc := randomGenesisDoc()
	genDoc.ConsensusParams.Block.MaxBytes = 3000
	genDoc.ConsensusParams.Evidence.MaxBytes = 1500

	// Max size of Txs is much smaller than size of block,
	// since we need to account for commits and evidence.
	testCases := []struct {
		tx    mempool.Tx
		isErr bool
	}{
		{mempool.Tx(tmrand.Bytes(2155)), false},
		{mempool.Tx(tmrand.Bytes(2156)), true},
		{mempool.Tx(tmrand.Bytes(3000)), true},
	}

	for i, tc := range testCases {
		state, err := sm.MakeGenesisState(genDoc)
		require.NoError(t, err)

		f := sm.TxPreCheck(state)
		if tc.isErr {
			assert.NotNil(t, f(tc.tx), "#%v", i)
		} else {
			assert.Nil(t, f(tc.tx), "#%v", i)
		}
	}
}
