package merkle

import (
	"bytes"
	"fmt"

	"github.com/tendermint/tendermint/crypto/tmhash"
)

const ProofOpValue = "simple:v"

// ValueOp takes a key and a single value as argument and
// produces the root hash.  The corresponding tree structure is
// the SimpleMap tree.  SimpleMap takes a Hasher, and currently
// Tendermint uses aminoHasher.  ValueOp should support
// the hash function as used in aminoHasher.  TODO support
// additional hash functions here as options/args to this
// operator.
//
// If the produced root hash matches the expected hash, the
// proof is good.
type ValueOp struct {
	// Encoded in ProofOp.Key.
	key []byte

	// To encode in ProofOp.Data
	Proof *SimpleProof `json:"simple_proof"`
}

var _ ProofOperator = ValueOp{}

func NewValueOp(key []byte, proof *SimpleProof) ValueOp {
	return ValueOp{
		key:   key,
		Proof: proof,
	}
}

func ValueOpDecoder(pop ProofOp) (ProofOperator, error) {
	if pop.Type != ProofOpValue {
		return nil, fmt.Errorf("unexpected ProofOp.Type; got %v, want %v", pop.Type, ProofOpValue)
	}
	var op ValueOp // a bit strange as we'll discard this, but it works.
	err := cdc.UnmarshalBinaryLengthPrefixed(pop.Data, &op)
	if err != nil {
		return nil, fmt.Errorf("decoding ProofOp.Data into SimpleValueOp: %w", err)
	}
	return NewValueOp(pop.Key, op.Proof), nil
}

func (op ValueOp) ProofOp() ProofOp {
	bz := cdc.MustMarshalBinaryLengthPrefixed(op)
	return ProofOp{
		Type: ProofOpValue,
		Key:  op.key,
		Data: bz,
	}
}

func (op ValueOp) String() string {
	return fmt.Sprintf("SimpleValueOp{%v}", op.GetKey())
}

func (op ValueOp) Run(args [][]byte) ([][]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("expected 1 arg, got %v", len(args))
	}
	value := args[0]
	hasher := tmhash.New()
	hasher.Write(value) // does not error
	vhash := hasher.Sum(nil)

	bz := new(bytes.Buffer)
	// Wrap <op.Key, vhash> to hash the KVPair.
	encodeByteSlice(bz, op.key) // does not error
	encodeByteSlice(bz, vhash)  // does not error
	kvhash := leafHash(bz.Bytes())

	if !bytes.Equal(kvhash, op.Proof.LeafHash) {
		return nil, fmt.Errorf("leaf hash mismatch: want %X got %X", op.Proof.LeafHash, kvhash)
	}

	return [][]byte{
		op.Proof.ComputeRootHash(),
	}, nil
}

func (op ValueOp) GetKey() []byte {
	return op.key
}
