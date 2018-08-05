package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/secp256k1"
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

	abciVal := TM2PB.Validator(tmVal)
	tmVals, err := PB2TM.Validators([]abci.Validator{abciVal})
	assert.Nil(t, err)
	assert.Equal(t, tmValExpected, tmVals[0])

	abciVals := TM2PB.Validators(NewValidatorSet(tmVals))
	assert.Equal(t, []abci.Validator{abciVal}, abciVals)

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

func TestABCIHeader(t *testing.T) {
	header := &Header{
		Height: int64(3),
		Time:   time.Now(),
		NumTxs: int64(10),
	}
	abciHeader := TM2PB.Header(header)

	assert.Equal(t, int64(3), abciHeader.Height)
}

func TestABCIEvidence(t *testing.T) {
	val := NewMockPV()
	blockID := makeBlockID("blockhash", 1000, "partshash")
	blockID2 := makeBlockID("blockhash2", 1000, "partshash")
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

	// test we do not send pubkeys
	assert.Empty(t, abciEv.Validator.PubKey)
}

type pubKeyEddie struct{}

func (pubKeyEddie) Address() Address                                  { return []byte{} }
func (pubKeyEddie) Bytes() []byte                                     { return []byte{} }
func (pubKeyEddie) VerifyBytes(msg []byte, sig []byte) bool { return false }
func (pubKeyEddie) Equals(crypto.PubKey) bool                         { return false }

func TestABCIValidatorFromPubKeyAndPower(t *testing.T) {
	pubkey := ed25519.GenPrivKey().PubKey()

	abciVal := TM2PB.ValidatorFromPubKeyAndPower(pubkey, 10)
	assert.Equal(t, int64(10), abciVal.Power)

	assert.Panics(t, func() { TM2PB.ValidatorFromPubKeyAndPower(nil, 10) })
	assert.Panics(t, func() { TM2PB.ValidatorFromPubKeyAndPower(pubKeyEddie{}, 10) })
}

func TestABCIValidatorWithoutPubKey(t *testing.T) {
	pkEd := ed25519.GenPrivKey().PubKey()

	abciVal := TM2PB.ValidatorWithoutPubKey(&Validator{
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
