package merkle

import (
	"bytes"

	cmn "github.com/tendermint/tmlibs/common"
)

//----------------------------------------
// ProofOp gets converted to an instance of ProofOperator:

// ProofOperator is a layer for calculating intermediate Merkle root
// Run() takes a list of bytes because it can be more than one
// for example in range proofs
// ProofOp() defines custom encoding which can be decoded later with
// OpDecoder
type ProofOperator interface {
	Run([][]byte) ([][]byte, error)
	GetKey() []byte
	ProofOp() ProofOp
}

//----------------------------------------
// Operations on a list of ProofOperators

// ProofOperators is a slice of ProofOperator(s)
// Each operator will be applied to the input value sequencially
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
			if !bytes.Equal(keys[0], key) {
				return cmn.NewError("Key mismatch on operation #%d: expected %+v but %+v", i, []byte(keys[0]), []byte(key))
			}
			keys = keys[1:]
		}
		args, err = op.Run(args)
		if err != nil {
			return
		}
	}
	if !bytes.Equal(root, args[0]) {
		return cmn.NewError("Calculated root hash is invalid: expected %+v but %+v", root, args[0])
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

func (prt *ProofRuntime) DecodeProof(proof *Proof) (poz ProofOperators, err error) {
	poz = ProofOperators(nil)
	for _, pop := range proof.Ops {
		operator, err := prt.Decode(pop)
		if err != nil {
			return nil, cmn.ErrorWrap(err, "decoding a proof operator")
		}
		poz = append(poz, operator)
	}
	return
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
