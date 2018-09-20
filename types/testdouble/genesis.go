package testdouble

import (
	"github.com/tendermint/tendermint/crypto/ed25519"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

// RandomGenesisDoc returns a random genesis document with one validator.
// Genesis time is set to now().
func RandomGenesisDoc() *types.GenesisDoc {
	pubkey := ed25519.GenPrivKey().PubKey()
	return &types.GenesisDoc{
		GenesisTime: tmtime.Now(),
		ChainID:     cmn.RandStr(types.MaxChainIDLen),
		Validators: []types.GenesisValidator{{
			Address: pubkey.Address(),
			PubKey:  pubkey,
			Power:   cmn.RandInt63(),
			Name:    cmn.RandStr(10),
		}},
		ConsensusParams: types.DefaultConsensusParams(),
	}
}
