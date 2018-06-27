package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	abci "github.com/tendermint/tendermint/abci/types"
	crypto "github.com/tendermint/tendermint/crypto"
)

func TestABCIPubKey(t *testing.T) {
	pkEd := crypto.GenPrivKeyEd25519().PubKey()
	pkSecp := crypto.GenPrivKeySecp256k1().PubKey()
	testABCIPubKey(t, pkEd, ABCIPubKeyTypeEd25519)
	testABCIPubKey(t, pkSecp, ABCIPubKeyTypeSecp256k1)
}

func testABCIPubKey(t *testing.T, pk crypto.PubKey, typeStr string) {
	abciPubKey := TM2PB.PubKey(pk)
	pk2, err := PB2TM.PubKey(abciPubKey)
	assert.Nil(t, err)
	assert.Equal(t, pk, pk2)
}

func TestABCIValidators(t *testing.T) {
	pkEd := crypto.GenPrivKeyEd25519().PubKey()

	// correct validator
	tmValExpected := &Validator{
		Address:     pkEd.Address(),
		PubKey:      pkEd,
		VotingPower: 10,
	}

	tmVal := &Validator{
		Address:     pkEd.Address(),
		PubKey:      pkEd,
		VotingPower: 10,
	}

	abciVal := TM2PB.Validator(tmVal)
	tmVals, err := PB2TM.Validators([]abci.Validator{abciVal})
	assert.Nil(t, err)
	assert.Equal(t, tmValExpected, tmVals[0])

	// val with address
	tmVal.Address = pkEd.Address()

	abciVal = TM2PB.Validator(tmVal)
	tmVals, err = PB2TM.Validators([]abci.Validator{abciVal})
	assert.Nil(t, err)
	assert.Equal(t, tmValExpected, tmVals[0])

	// val with incorrect address
	abciVal = TM2PB.Validator(tmVal)
	abciVal.Address = []byte("incorrect!")
	tmVals, err = PB2TM.Validators([]abci.Validator{abciVal})
	assert.NotNil(t, err)
	assert.Nil(t, tmVals)
}

func TestABCIConsensusParams(t *testing.T) {
	cp := DefaultConsensusParams()
	cp.EvidenceParams.MaxAge = 0 // TODO add this to ABCI
	abciCP := TM2PB.ConsensusParams(cp)
	cp2 := PB2TM.ConsensusParams(abciCP)

	assert.Equal(t, *cp, cp2)
}
