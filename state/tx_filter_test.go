package state

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pakula/prism/crypto/ed25519"
	cmn "github.com/pakula/prism/libs/common"
	dbm "github.com/pakula/prism/libs/db"
	"github.com/pakula/prism/types"
	tmtime "github.com/pakula/prism/types/time"
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
		state, err := LoadStateFromDBOrGenesisDoc(stateDB, genDoc)
		require.NoError(t, err)

		f := TxPreCheck(state)
		if tc.isErr {
			assert.NotNil(t, f(tc.tx), "#%v", i)
		} else {
			assert.Nil(t, f(tc.tx), "#%v", i)
		}
	}
}

func randomGenesisDoc() *types.GenesisDoc {
	pubkey := ed25519.GenPrivKey().PubKey()
	return &types.GenesisDoc{
		GenesisTime:     tmtime.Now(),
		ChainID:         "abc",
		Validators:      []types.GenesisValidator{{pubkey.Address(), pubkey, 10, "myval"}},
		ConsensusParams: types.DefaultConsensusParams(),
	}
}
