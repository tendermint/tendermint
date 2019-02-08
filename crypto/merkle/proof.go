package merkle

import (
	"bytes"

	cmn "github.com/tendermint/tendermint/libs/common"
)

//----------------------------------------
// ProofOp gets converted to an instance of ProofOperator:

// ProofOperator is a layer for calculating intermediate Merkle roots
// when a series of Merkle trees are chained together.
// Run() takes leaf values from a tree and returns the Merkle
// root for the corresponding tree. It takes and returns a list of bytes
// to allow multiple leaves to be part of a single proof, for instance in a range proof.
// ProofOp() encodes the ProofOperator in a generic way so it can later be
// decoded with OpDecoder.
type ProofOperator interface {
	Run([][]byte) ([][]byte, error)
	GetKey() []byte
	ProofOp() ProofOp
}

//----------------------------------------
// Operations on a list of ProofOperators

// ProofOperators is a slice of ProofOperator(s).
// Each operator will be applied to the input value sequentially
// and the last Merkle root will be verified with already known data
type ProofOperators []ProofOperator

func (poz ProofOperators) VerifyValue(root []byte, keypath string, value []byte) (err error) {
	return poz.Verify(root, keypath, [][]byte{value})
}

func (poz ProofOperators) Verify(root []byte, keypath string, args [][]byte) (err error) {
	keys, err := KeyPathToKeys(keypath)
	if err != nil {
		return
	}

	for i, op := range poz {
		key := op.GetKey()
		if len(key) != 0 {
			if len(keys) == 0 {
				return cmn.NewError("Key path has insufficient # of parts: expected no more keys but got %+v", string(key))
			}
			lastKey := keys[len(keys)-1]
			if !bytes.Equal(lastKey, key) {
				return cmn.NewError("Key mismatch on operation #%d: expected %+v but got %+v", i, string(lastKey), string(key))
			}
			keys = keys[:len(keys)-1]
		}
		args, err = op.Run(args)
		if err != nil {
			return
		}
	}
	if !bytes.Equal(root, args[0]) {
		return cmn.NewError("Calculated root hash is invalid: expected %+v but got %+v", root, args[0])
	}
	if len(keys) != 0 {
		return cmn.NewError("Keypath not consumed all")
	}
	return nil
}

//----------------------------------------
// ProofRuntime - main entrypoint

type OpDecoder func(ProofOp) (ProofOperator, error)

type ProofRuntime struct {
	decoders map[string]OpDecoder
}

func NewProofRuntime() *ProofRuntime {
	return &ProofRuntime{
		decoders: make(map[string]OpDecoder),
	}
}

func (prt *ProofRuntime) RegisterOpDecoder(typ string, dec OpDecoder) {
	_, ok := prt.decoders[typ]
	if ok {
		panic("already registered for type " + typ)
	}
	prt.decoders[typ] = dec
}

func (prt *ProofRuntime) Decode(pop ProofOp) (ProofOperator, error) {
	decoder := prt.decoders[pop.Type]
	if decoder == nil {
		return nil, cmn.NewError("unrecognized proof type %v", pop.Type)
	}
	return decoder(pop)
}

func (prt *ProofRuntime) DecodeProof(proof *Proof) (ProofOperators, error) {
	poz := make(ProofOperators, 0, len(proof.Ops))
	for _, pop := range proof.Ops {
		operator, err := prt.Decode(pop)
		if err != nil {
			return nil, cmn.ErrorWrap(err, "decoding a proof operator")
		}
		poz = append(poz, operator)
	}
	return poz, nil
}

func (prt *ProofRuntime) VerifyValue(proof *Proof, root []byte, keypath string, value []byte) (err error) {
	return prt.Verify(proof, root, keypath, [][]byte{value})
}

// TODO In the long run we'll need a method of classifcation of ops,
// whether existence or absence or perhaps a third?
func (prt *ProofRuntime) VerifyAbsence(proof *Proof, root []byte, keypath string) (err error) {
	return prt.Verify(proof, root, keypath, nil)
}

func (prt *ProofRuntime) Verify(proof *Proof, root []byte, keypath string, args [][]byte) (err error) {
	poz, err := prt.DecodeProof(proof)
	if err != nil {
		return cmn.ErrorWrap(err, "decoding proof")
	}
	return poz.Verify(root, keypath, args)
}

// DefaultProofRuntime only knows about Simple value
// proofs.
// To use e.g. IAVL proofs, register op-decoders as
// defined in the IAVL package.
func DefaultProofRuntime() (prt *ProofRuntime) {
	prt = NewProofRuntime()
	prt.RegisterOpDecoder(ProofOpSimpleValue, SimpleValueOpDecoder)
	return
}
