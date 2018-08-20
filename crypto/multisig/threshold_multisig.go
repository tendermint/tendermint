package multisig

import (
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
)

// ThresholdMultiSignaturePubKey implements a K of N threshold multisig.
type ThresholdMultiSignaturePubKey struct {
	K       uint            `json:"threshold"`
	Pubkeys []crypto.PubKey `json:"pubkeys"`
}

var _ crypto.PubKey = &ThresholdMultiSignaturePubKey{}

// NewThresholdMultiSignaturePubKey returns a new ThresholdMultiSignaturePubKey.
// Panics if len(pubkeys) < k or 0 >= k.
func NewThresholdMultiSignaturePubKey(k int, pubkeys []crypto.PubKey) crypto.PubKey {
	if k <= 0 {
		panic("threshold k of n multisignature: k <= 0")
	}
	if len(pubkeys) < k {
		panic("threshold k of n multisignature: len(pubkeys) < k")
	}
	return &ThresholdMultiSignaturePubKey{uint(k), pubkeys}
}

// VerifyBytes expects sig to be an amino encoded version of a MultiSignature.
// Returns true iff the multisignature contains k or more signatures
// for the correct corresponding keys,
// and all signatures are valid. (Not just k of the signatures)
// The multisig uses a bitarray, so multiple signatures for the same key is not
// a concern.
func (pk *ThresholdMultiSignaturePubKey) VerifyBytes(msg []byte, marshalledSig []byte) bool {
	var sig *Multisignature
	err := cdc.UnmarshalBinaryBare(marshalledSig, &sig)
	if err != nil {
		return false
	}
	size := sig.BitArray.Size()
	// ensure bit array is the correct size
	if len(pk.Pubkeys) != size {
		return false
	}
	// ensure size of signature list
	if len(sig.Sigs) < int(pk.K) || len(sig.Sigs) > size {
		return false
	}
	// ensure at least k signatures are set
	if sig.BitArray.NumTrueBitsBefore(size) < int(pk.K) {
		return false
	}
	// index in the list of signatures which we are concerned with.
	sigIndex := 0
	for i := 0; i < size; i++ {
		if sig.BitArray.GetIndex(i) {
			if !pk.Pubkeys[i].VerifyBytes(msg, sig.Sigs[sigIndex]) {
				return false
			}
			sigIndex++
		}
	}
	return true
}

// Bytes returns the amino encoded version of the ThresholdMultiSignaturePubKey
func (pk *ThresholdMultiSignaturePubKey) Bytes() []byte {
	return cdc.MustMarshalBinaryBare(pk)
}

// Address returns tmhash(ThresholdMultiSignaturePubKey.Bytes())
func (pk *ThresholdMultiSignaturePubKey) Address() crypto.Address {
	return crypto.Address(tmhash.Sum(pk.Bytes()))
}

// Equals returns true iff pk and other both have the same number of keys, and
// all constituent keys are the same, and in the same order.
func (pk *ThresholdMultiSignaturePubKey) Equals(other crypto.PubKey) bool {
	otherKey, sameType := other.(*ThresholdMultiSignaturePubKey)
	if !sameType {
		return false
	}
	if pk.K != otherKey.K || len(pk.Pubkeys) != len(otherKey.Pubkeys) {
		return false
	}
	for i := 0; i < len(pk.Pubkeys); i++ {
		if !pk.Pubkeys[i].Equals(otherKey.Pubkeys[i]) {
			return false
		}
	}
	return true
}
