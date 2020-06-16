package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
)

func TestABCIPubKey(t *testing.T) {
	pkEd := ed25519.GenPrivKey().PubKey()
	err := testABCIPubKey(t, pkEd, ABCIPubKeyTypeEd25519)
	assert.NoError(t, err)
}

func testABCIPubKey(t *testing.T, pk crypto.PubKey, typeStr string) error {
	abciPubKey, err := cryptoenc.PubKeyToProto(pk)
	require.NoError(t, err)
	pk2, err := cryptoenc.PubKeyFromProto(abciPubKey)
	require.NoError(t, err)
	require.Equal(t, pk, pk2)
	return nil
}

func TestABCIValidators(t *testing.T) {
	pkEd := ed25519.GenPrivKey().PubKey()

	// correct validator
	tmValExpected := NewValidator(pkEd, 10)

	tmVal := NewValidator(pkEd, 10)

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
}

func TestABCIConsensusParams(t *testing.T) {
	cp := DefaultConsensusParams()
	abciCP := TM2PB.ConsensusParams(cp)
	cp2 := UpdateConsensusParams(*cp, abciCP)

	assert.Equal(t, *cp, cp2)
}

func TestABCIEvidence(t *testing.T) {
	val := NewMockPV()
	blockID := makeBlockID([]byte("blockhash"), 1000, []byte("partshash"))
	blockID2 := makeBlockID([]byte("blockhash2"), 1000, []byte("partshash"))
	const chainID = "mychain"
	pubKey, err := val.GetPubKey()
	require.NoError(t, err)
	ev := &DuplicateVoteEvidence{
		VoteA: makeVote(t, val, chainID, 0, 10, 2, 1, blockID, defaultVoteTime),
		VoteB: makeVote(t, val, chainID, 0, 10, 2, 1, blockID2, defaultVoteTime),
	}
	abciEv := TM2PB.Evidence(
		ev,
		NewValidatorSet([]*Validator{NewValidator(pubKey, 10)}),
		time.Now(),
	)

	assert.Equal(t, "duplicate/vote", abciEv.Type)
}

type pubKeyEddie struct{}

func (pubKeyEddie) Address() Address                        { return []byte{} }
func (pubKeyEddie) Bytes() []byte                           { return []byte{} }
func (pubKeyEddie) VerifyBytes(msg []byte, sig []byte) bool { return false }
func (pubKeyEddie) Equals(crypto.PubKey) bool               { return false }
func (pubKeyEddie) String() string                          { return "" }
func (pubKeyEddie) Type() string                            { return "pubKeyEddie" }

func TestABCIValidatorFromPubKeyAndPower(t *testing.T) {
	pubkey := ed25519.GenPrivKey().PubKey()

	abciVal := TM2PB.NewValidatorUpdate(pubkey, 10)
	assert.Equal(t, int64(10), abciVal.Power)

	assert.Panics(t, func() { TM2PB.NewValidatorUpdate(nil, 10) })
	assert.Panics(t, func() { TM2PB.NewValidatorUpdate(pubKeyEddie{}, 10) })
}

func TestABCIValidatorWithoutPubKey(t *testing.T) {
	pkEd := ed25519.GenPrivKey().PubKey()

	abciVal := TM2PB.Validator(NewValidator(pkEd, 10))

	// pubkey must be nil
	tmValExpected := abci.Validator{
		Address: pkEd.Address(),
		Power:   10,
	}

	assert.Equal(t, tmValExpected, abciVal)
}
