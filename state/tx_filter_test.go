package state_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cmn "github.com/tendermint/tendermint/libs/common"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

func TestTxFilter(t *testing.T) {
	genDoc := randomGenesisDoc()
	genDoc.ConsensusParams.Block.MaxBytes = 3000

	// Max size of Txs is much smaller than size of block,
	// since we need to account for commits and evidence.
	testCases := []struct {
		tx    types.Tx
		isErr bool
	}{
		{types.Tx(cmn.RandBytes(250)), false},
		{types.Tx(cmn.RandBytes(1809)), false},
		{types.Tx(cmn.RandBytes(1810)), false},
		{types.Tx(cmn.RandBytes(1811)), true},
		{types.Tx(cmn.RandBytes(1812)), true},
		{types.Tx(cmn.RandBytes(3000)), true},
	}

	for i, tc := range testCases {
		stateDB := dbm.NewDB("state", "memdb", os.TempDir())
		state, err := sm.LoadStateFromDBOrGenesisDoc(stateDB, genDoc)
		require.NoError(t, err)

		f := sm.TxPreCheck(state)
		if tc.isErr {
			assert.NotNil(t, f(tc.tx), "#%v", i)
		} else {
			assert.Nil(t, f(tc.tx), "#%v", i)
		}
	}
}
