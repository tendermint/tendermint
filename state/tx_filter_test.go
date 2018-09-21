package state

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto/ed25519"
	cmn "github.com/tendermint/tendermint/libs/common"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

func TestTxFilter(t *testing.T) {
	genDoc := randomGenesisDoc()
	genDoc.ConsensusParams.BlockSize.MaxBytes = 3000

	testCases := []struct {
		tx        types.Tx
		isTxValid bool
	}{
		{types.Tx(cmn.RandBytes(250)), true},
		{types.Tx(cmn.RandBytes(3001)), false},
	}

	for i, tc := range testCases {
		stateDB := dbm.NewDB("state", "memdb", os.TempDir())
		state, err := LoadStateFromDBOrGenesisDoc(stateDB, genDoc)
		require.NoError(t, err)

		f := TxFilter(state)
		assert.Equal(t, tc.isTxValid, f(tc.tx), "#%v", i)
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
