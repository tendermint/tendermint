package types

import (
	"testing"

	"github.com/dashevo/dashd-go/btcjson"

	"github.com/tendermint/tendermint/crypto/bls12381"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/encoding"
)

func TestABCIPubKey(t *testing.T) {
	pkBLS := bls12381.GenPrivKey().PubKey()
	err := testABCIPubKey(t, pkBLS, ABCIPubKeyTypeBLS12381)
	assert.NoError(t, err)
}

func testABCIPubKey(t *testing.T, pk crypto.PubKey, typeStr string) error {
	abciPubKey, err := encoding.PubKeyToProto(pk)
	require.NoError(t, err)
	pk2, err := encoding.PubKeyFromProto(abciPubKey)
	require.NoError(t, err)
	require.Equal(t, pk, pk2)
	return nil
}

func TestABCIValidators(t *testing.T) {
	pkBLS := bls12381.GenPrivKey().PubKey()
	proTxHash := crypto.RandProTxHash()
	quorumHash := crypto.RandQuorumHash()

	// correct validator
	tmValExpected := NewValidatorDefaultVotingPower(pkBLS, proTxHash)

	tmVal := NewValidatorDefaultVotingPower(pkBLS, proTxHash)

	abciVal := TM2PB.ValidatorUpdate(tmVal)
	tmVals, err := PB2TM.ValidatorUpdates([]abci.ValidatorUpdate{abciVal})
	assert.Nil(t, err)
	assert.Equal(t, tmValExpected, tmVals[0])

	abciVals := TM2PB.ValidatorUpdates(NewValidatorSet(tmVals, tmVal.PubKey, btcjson.LLMQType_5_60, quorumHash, true))
	assert.Equal(t, abci.ValidatorSetUpdate{
		ValidatorUpdates:   []abci.ValidatorUpdate{abciVal},
		ThresholdPublicKey: *abciVal.PubKey,
		QuorumHash:         quorumHash,
	}, abciVals)

	abciVal = TM2PB.ValidatorUpdate(tmVal)
	tmVals, err = PB2TM.ValidatorUpdates([]abci.ValidatorUpdate{abciVal})
	assert.Nil(t, err)
	assert.Equal(t, tmValExpected, tmVals[0])
}

func TestABCIValidatorWithoutPubKey(t *testing.T) {
	pkBLS := bls12381.GenPrivKey().PubKey()
	proTxHash := crypto.RandProTxHash()

	abciVal := TM2PB.Validator(NewValidatorDefaultVotingPower(pkBLS, proTxHash))

	// pubkey must be nil
	tmValExpected := abci.Validator{
		Power:     DefaultDashVotingPower,
		ProTxHash: proTxHash,
	}

	assert.Equal(t, tmValExpected, abciVal)
}
