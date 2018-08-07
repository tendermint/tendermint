package multisig

import "github.com/tendermint/tendermint/crypto"

// Multisignature is used to represent the signature object used in the multisigs.
// Sigs is a list of signatures, sorted by corresponding index.
type Multisignature struct {
	BitArray *CompactBitArray
	Sigs     [][]byte
}

// NewMultisig returns a new Multisignature of size n.
func NewMultisig(n int) *Multisignature {
	// Default the signature list to have a capacity of two, since we can
	// expect that most multisigs will require multiple signers.
	return &Multisignature{NewCompactBitArray(n), make([][]byte, 0, 2)}
}

// GetIndex returns the index of pk in keys. Returns -1 if not found
func GetIndex(pk crypto.PubKey, keys []crypto.PubKey) int {
	for i := 0; i < len(keys); i++ {
		if pk.Equals(keys[i]) {
			return i
		}
	}
	return -1
}

// AddSignature adds a signature to the multisig, at the corresponding index.
func (mSig *Multisignature) AddSignature(sig []byte, index int) {
	i := mSig.BitArray.trueIndex(index)
	// Signature already exists, just replace the value there
	if mSig.BitArray.GetIndex(index) {
		mSig.Sigs[i] = sig
		return
	}
	mSig.BitArray.SetIndex(index, true)
	// Optimization if the index is the greatest index
	if i > len(mSig.Sigs) {
		mSig.Sigs = append(mSig.Sigs, sig)
		return
	}
	// Expand slice by one with a dummy element, move all elements after i
	// over by one, then place the new signature in that gap.
	mSig.Sigs = append(mSig.Sigs, make([]byte, 0))
	copy(mSig.Sigs[i+1:], mSig.Sigs[i:])
	mSig.Sigs[i] = sig
}

// AddSignatureFromPubkey adds a signature to the multisig,
// at the index in keys corresponding to the provided pubkey.
func (mSig *Multisignature) AddSignatureFromPubkey(sig []byte, pubkey crypto.PubKey, keys []crypto.PubKey) {
	index := GetIndex(pubkey, keys)
	mSig.AddSignature(sig, index)
}

// Marshal the multisignature with amino
func (mSig *Multisignature) Marshal() []byte {
	return cdc.MustMarshalBinary(mSig)
}
