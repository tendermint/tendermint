package merkle

import (
	"bytes"
	"fmt"

	"github.com/tendermint/tendermint/crypto/tmhash"
	cmn "github.com/tendermint/tmlibs/common"
)

const ProofOpSimpleValue = "simple:v"

// SimpleValueOp takes a key and a single value as argument and
// produces the root hash.  The corresponding tree structure is
// the SimpleMap tree.  SimpleMap takes a Hasher, and currently
// Tendermint uses aminoHasher.  SimpleValueOp should support
// the hash function as used in aminoHasher.  TODO support
// additional hash functions here as options/args to this
// operator.
//
// If the produced root hash matches the expected hash, the
// proof is good.
type SimpleValueOp struct {
	// Encoded in ProofOp.Key.
	key []byte

	// To encode in ProofOp.Data
	Proof *SimpleProof `json:"simple-proof"`
}

var _ ProofOperator = SimpleValueOp{}

func NewSimpleValueOp(key []byte, proof *SimpleProof) SimpleValueOp {
	return SimpleValueOp{
		key:   key,
		Proof: proof,
	}
}

func SimpleValueOpDecoder(pop ProofOp) (ProofOperator, error) {
	if pop.Type != ProofOpSimpleValue {
		return nil, cmn.NewError("unexpected ProofOp.Type; got %v, want %v", pop.Type, ProofOpSimpleValue)
	}
	var op SimpleValueOp // a bit strange as we'll discard this, but it works.
	err := cdc.UnmarshalBinary(pop.Data, &op)
	if err != nil {
		return nil, cmn.ErrorWrap(err, "decoding ProofOp.Data into SimpleValueOp")
	}
	return NewSimpleValueOp(pop.Key, op.Proof), nil
}

func (op SimpleValueOp) ProofOp() ProofOp {
	bz := cdc.MustMarshalBinary(op)
	return ProofOp{
		Type: ProofOpSimpleValue,
		Key:  op.key,
		Data: bz,
	}
}

func (op SimpleValueOp) String() string {
	return fmt.Sprintf("SimpleValueOp{%v}", op.GetKey())
}

func (op SimpleValueOp) Run(args [][]byte) ([][]byte, error) {
	if len(args) != 1 {
		return nil, cmn.NewError("expected 1 arg, got %v", len(args))
	}
	value := args[0]
	hasher := tmhash.New()
	hasher.Write(value) // does not error
	vhash := hasher.Sum(nil)

	// Wrap <op.Key, vhash> to hash the KVPair.
	hasher = tmhash.New()
	encodeByteSlice(hasher, []byte(op.key)) // does not error
	encodeByteSlice(hasher, []byte(vhash))  // does not error
	kvhash := hasher.Sum(nil)

	if !bytes.Equal(kvhash, op.Proof.LeafHash) {
		return nil, cmn.NewError("leaf hash mismatch: want %X got %X", op.Proof.LeafHash, kvhash)
	}

	return [][]byte{
		op.Proof.ComputeRootHash(),
	}, nil
}

func (op SimpleValueOp) GetKey() []byte {
	return op.key
}
