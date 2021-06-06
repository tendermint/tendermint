package types

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/tendermint/tendermint/crypto/bls12381"

	"github.com/tendermint/tendermint/crypto"
	ce "github.com/tendermint/tendermint/crypto/encoding"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

// Volatile state for each Validator
// NOTE: The ProposerPriority is not included in Validator.Hash();
// make sure to update that method if changes are made here
// The ProTxHash is part of Dash additions required for BLS threshold signatures
type Validator struct {
	Address     Address       `json:"address"`
	PubKey      crypto.PubKey `json:"pub_key"`
	VotingPower int64         `json:"voting_power"`
	ProTxHash   ProTxHash     `json:"pro_tx_hash"`

	ProposerPriority int64 `json:"proposer_priority"`
}

func NewTestValidatorGeneratedFromAddress(address crypto.Address) *Validator {
	return &Validator{
		Address:          address,
		VotingPower:      DefaultDashVotingPower,
		ProposerPriority: 0,
		ProTxHash:        crypto.Sha256(address),
	}
}

func NewTestRemoveValidatorGeneratedFromAddress(address crypto.Address) *Validator {
	return &Validator{
		Address:          address,
		VotingPower:      0,
		ProposerPriority: 0,
		ProTxHash:        crypto.Sha256(address),
	}
}

func NewValidatorDefaultVotingPower(pubKey crypto.PubKey, proTxHash []byte) *Validator {
	return NewValidator(pubKey, DefaultDashVotingPower, proTxHash)
}

// NewValidator returns a new validator with the given pubkey and voting power.
func NewValidator(pubKey crypto.PubKey, votingPower int64, proTxHash []byte) *Validator {
	val := &Validator{
		PubKey:           pubKey,
		VotingPower:      votingPower,
		ProposerPriority: 0,
		ProTxHash:        proTxHash,
	}
	if pubKey != nil {
		val.Address = pubKey.Address()
	}
	return val
}

// ValidateBasic performs basic validation.
func (v *Validator) ValidateBasic() error {
	if v == nil {
		return errors.New("nil validator")
	}

	if v.ProTxHash == nil {
		return errors.New("validator does not have a provider transaction hash")
	}

	if v.VotingPower < 0 {
		return errors.New("validator has negative voting power")
	}

	if len(v.Address) != crypto.AddressSize {
		return fmt.Errorf("validator address is the wrong size: %v", v.Address)
	}

	return nil
}

// ValidatePubKey performs basic validation on the public key.
func (v *Validator) ValidatePubKey() error {
	if v.PubKey == nil {
		return errors.New("validator does not have a public key")
	}

	if len(v.PubKey.Bytes()) != bls12381.PubKeySize {
		return fmt.Errorf("validator PubKey is the wrong size: %X", v.PubKey.Bytes())
	}
	return nil
}

// Copy creates a new copy of the validator so we can mutate ProposerPriority.
// Panics if the validator is nil.
func (v *Validator) Copy() *Validator {
	vCopy := *v
	return &vCopy
}

// CompareProposerPriority Returns the one with higher ProposerPriority.
func (v *Validator) CompareProposerPriority(other *Validator) *Validator {
	if v == nil {
		return other
	}
	switch {
	case v.ProposerPriority > other.ProposerPriority:
		return v
	case v.ProposerPriority < other.ProposerPriority:
		return other
	default:
		result := bytes.Compare(v.ProTxHash, other.ProTxHash)
		switch {
		case result < 0:
			return v
		case result > 0:
			return other
		default:
			panic("Cannot compare identical validators")
		}
	}
}

// String returns a string representation of String.
//
// 1. address
// 2. public key
// 3. voting power
// 4. proposer priority
func (v *Validator) String() string {
	if v == nil {
		return "nil-Validator"
	}
	return fmt.Sprintf("Validator{%v %v VP:%v A:%v}",
		v.ProTxHash,
		v.PubKey,
		v.VotingPower,
		v.ProposerPriority)
}

// ValidatorListString returns a prettified validator list for logging purposes.
func ValidatorListString(vals []*Validator) string {
	chunks := make([]string, len(vals))
	for i, val := range vals {
		chunks[i] = fmt.Sprintf("%s:%s:%d", val.ProTxHash, val.PubKey, val.VotingPower)
	}

	return strings.Join(chunks, ",")
}

// Bytes computes the unique encoding of a validator with a given voting power.
// These are the bytes that gets hashed in consensus. It excludes address
// as its redundant with the pubkey. This also excludes ProposerPriority
// which changes every round.
func (v *Validator) Bytes() []byte {
	pk, err := ce.PubKeyToProto(v.PubKey)
	if err != nil {
		panic(err)
	}

	pbv := tmproto.SimpleValidator{
		PubKey:      &pk,
		VotingPower: v.VotingPower,
	}

	bz, err := pbv.Marshal()
	if err != nil {
		panic(err)
	}
	return bz
}

// ToProto converts Valiator to protobuf
func (v *Validator) ToProto() (*tmproto.Validator, error) {
	if v == nil {
		return nil, errors.New("nil validator")
	}

	pk, err := ce.PubKeyToProto(v.PubKey)
	if err != nil {
		return nil, err
	}

	if v.ProTxHash == nil {
		return nil, errors.New("the validator must have a proTxHash")
	}

	vp := tmproto.Validator{
		Address:          v.Address,
		PubKey:           pk,
		VotingPower:      v.VotingPower,
		ProposerPriority: v.ProposerPriority,
		ProTxHash:        v.ProTxHash,
	}

	return &vp, nil
}

// FromProto sets a protobuf Validator to the given pointer.
// It returns an error if the public key is invalid.
func ValidatorFromProto(vp *tmproto.Validator) (*Validator, error) {
	if vp == nil {
		return nil, errors.New("nil validator")
	}

	pk, err := ce.PubKeyFromProto(vp.PubKey)
	if err != nil {
		return nil, err
	}
	v := new(Validator)
	v.Address = vp.GetAddress()
	v.PubKey = pk
	v.VotingPower = vp.GetVotingPower()
	v.ProposerPriority = vp.GetProposerPriority()
	v.ProTxHash = vp.ProTxHash

	return v, nil
}

//----------------------------------------
// RandValidator

// RandValidator returns a randomized validator, useful for testing.
// UNSTABLE
func RandValidator() (*Validator, PrivValidator) {
	privVal := NewMockPV()
	proTxHash, err := privVal.GetProTxHash()
	if err != nil {
		panic(fmt.Errorf("could not retrieve proTxHash %w", err))
	}
	pubKey, err := privVal.GetPubKey(crypto.QuorumHash{})
	if err != nil {
		panic(fmt.Errorf("could not retrieve pubkey %w", err))
	}
	val := NewValidatorDefaultVotingPower(pubKey, proTxHash)
	return val, privVal
}
