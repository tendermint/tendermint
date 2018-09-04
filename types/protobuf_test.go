package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	tmtime "github.com/tendermint/tendermint/types/time"
)

func TestABCIPubKey(t *testing.T) {
	pkEd := ed25519.GenPrivKey().PubKey()
	pkSecp := secp256k1.GenPrivKey().PubKey()
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
	pkEd := ed25519.GenPrivKey().PubKey()

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

	abciVal := TM2PB.ValidatorUpdate(tmVal)
	tmVals, err := PB2TM.ValidatorUpdates([]abci.ValidatorUpdate{abciVal})
	assert.Nil(t, err)
	assert.Equal(t, tmValExpected, tmVals[0])

	abciVals := TM2PB.ValidatorUpdates(NewValidatorSet(tmVals))
	assert.Equal(t, []abci.ValidatorUpdate{abciVal}, abciVals)

	// val with address
	tmVal.Address = pkEd.Address()

	abciVal = TM2PB.ValidatorUpdate(tmVal)
	tmVals, err = PB2TM.ValidatorUpdates([]abci.ValidatorUpdate{abciVal})
	assert.Nil(t, err)
	assert.Equal(t, tmValExpected, tmVals[0])

	// val with incorrect pubkey data
	abciVal = TM2PB.ValidatorUpdate(tmVal)
	abciVal.PubKey.Data = []byte("incorrect!")
	tmVals, err = PB2TM.ValidatorUpdates([]abci.ValidatorUpdate{abciVal})
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

func TestABCIHeader(t *testing.T) {
	header := &Header{
		Height:          int64(3),
		Time:            tmtime.Now(),
		NumTxs:          int64(10),
		ProposerAddress: []byte("cloak"),
	}
	abciHeader := TM2PB.Header(header)

	assert.Equal(t, int64(3), abciHeader.Height)
	assert.Equal(t, []byte("cloak"), abciHeader.ProposerAddress)
}

func TestABCIEvidence(t *testing.T) {
	val := NewMockPV()
	blockID := makeBlockID([]byte("blockhash"), 1000, []byte("partshash"))
	blockID2 := makeBlockID([]byte("blockhash2"), 1000, []byte("partshash"))
	const chainID = "mychain"
	ev := &DuplicateVoteEvidence{
		PubKey: val.GetPubKey(),
		VoteA:  makeVote(val, chainID, 0, 10, 2, 1, blockID),
		VoteB:  makeVote(val, chainID, 0, 10, 2, 1, blockID2),
	}
	abciEv := TM2PB.Evidence(
		ev,
		NewValidatorSet([]*Validator{NewValidator(val.GetPubKey(), 10)}),
		time.Now(),
	)

	assert.Equal(t, "duplicate/vote", abciEv.Type)
}

type pubKeyEddie struct{}

func (pubKeyEddie) Address() Address                        { return []byte{} }
func (pubKeyEddie) Bytes() []byte                           { return []byte{} }
func (pubKeyEddie) VerifyBytes(msg []byte, sig []byte) bool { return false }
func (pubKeyEddie) Equals(crypto.PubKey) bool               { return false }

func TestABCIValidatorFromPubKeyAndPower(t *testing.T) {
	pubkey := ed25519.GenPrivKey().PubKey()

	abciVal := TM2PB.NewValidatorUpdate(pubkey, 10)
	assert.Equal(t, int64(10), abciVal.Power)

	assert.Panics(t, func() { TM2PB.NewValidatorUpdate(nil, 10) })
	assert.Panics(t, func() { TM2PB.NewValidatorUpdate(pubKeyEddie{}, 10) })
}

func TestABCIValidatorWithoutPubKey(t *testing.T) {
	pkEd := ed25519.GenPrivKey().PubKey()

	abciVal := TM2PB.Validator(&Validator{
		Address:     pkEd.Address(),
		PubKey:      pkEd,
		VotingPower: 10,
	})

	// pubkey must be nil
	tmValExpected := abci.Validator{
		Address: pkEd.Address(),
		Power:   10,
	}

	assert.Equal(t, tmValExpected, abciVal)
}
