package types

import (
	"github.com/dashevo/dashd-go/btcjson"
	"testing"

	"github.com/tendermint/tendermint/crypto/bls12381"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
)

func TestABCIPubKey(t *testing.T) {
	pkBLS := bls12381.GenPrivKey().PubKey()
	err := testABCIPubKey(t, pkBLS, ABCIPubKeyTypeBLS12381)
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
		ThresholdPublicKey: abciVal.PubKey,
		QuorumHash:         quorumHash,
	}, abciVals)

	// val with address
	tmVal.Address = pkBLS.Address()

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

type pubKeyBLS struct{}

func (pubKeyBLS) Address() Address                            { return []byte{} }
func (pubKeyBLS) Bytes() []byte                               { return []byte{} }
func (pubKeyBLS) VerifySignature(msg []byte, sig []byte) bool { return false }
func (pubKeyBLS) VerifySignatureDigest(msg []byte, sig []byte) bool { return false }
func (pubKeyBLS) AggregateSignatures(sigSharesData [][]byte, messages [][]byte) ([]byte, error) {
	return []byte{}, nil
}
func (pubKeyBLS) VerifyAggregateSignature(msgs [][]byte, sig []byte) bool { return false }
func (pubKeyBLS) Equals(crypto.PubKey) bool                               { return false }
func (pubKeyBLS) String() string                                          { return "" }
func (pubKeyBLS) Type() string                                            { return "pubKeyBLS12381" }
func (pubKeyBLS) TypeValue() crypto.KeyType                               { return crypto.BLS12381 }

func TestABCIValidatorFromPubKeyAndPower(t *testing.T) {
	pubkey := bls12381.GenPrivKey().PubKey()

	abciVal := TM2PB.NewValidatorUpdate(pubkey, DefaultDashVotingPower, crypto.RandProTxHash())
	assert.Equal(t, DefaultDashVotingPower, abciVal.Power)

	assert.Panics(t, func() { TM2PB.NewValidatorUpdate(nil, DefaultDashVotingPower, crypto.RandProTxHash()) })
	assert.Panics(t, func() { TM2PB.NewValidatorUpdate(pubKeyBLS{}, DefaultDashVotingPower, crypto.RandProTxHash()) })
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
