package merkle

import (
	"bytes"
	"reflect"

	"github.com/tendermint/go-amino"
	cmn "github.com/tendermint/tmlibs/common"
)

//----------------------------------------
// Extension of ProofOp (defined in merkle.proto)

func (po ProofOp) Bytes() []byte {
	bz, err := amino.MarshalBinary(po)
	if err != nil {
		panic(err)
	}
	return bz
}

//----------------------------------------
// ProofOp gets converted to an instance of ProofOperator:

type ProofOperator interface {
	Run([][]byte) ([][]byte, error)
	GetKey() string
	ProofOp() ProofOp
}

//----------------------------------------
// Operations on a list of ProofOperators

type ProofOperators []ProofOperator

func (poz ProofOperators) VerifyValue(root []byte, value []byte, keys ...string) (err error) {
	return poz.Verify(root, [][]byte{value}, keys...)
}

func (poz ProofOperators) Verify(root []byte, args [][]byte, keys ...string) (err error) {
	for i, op := range poz {
		key := op.GetKey()
		if key != "" {
			if keys[0] != key {
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

func (prt *ProofRuntime) RegisterAminoOpDecoder(typ string, opType reflect.Type) {
	prt.RegisterOpDecoder(
		typ,
		func(pop ProofOp) (ProofOperator, error) {
			newOp := reflect.New(opType).Elem().Interface()
			err := cdc.UnmarshalBinary(pop.Data, &newOp)
			if err != nil {
				return nil, cmn.ErrorWrap(err, "decoding ProofOp.Data")
			}
			return newOp.(ProofOperator), nil
		},
	)
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

// XXX Reorder value/keys, and figure out how to merge keys into a single string
// after figuring out encoding between bytes/string.
func (prt *ProofRuntime) VerifyValue(proof *Proof, root []byte, value []byte, keys ...string) (err error) {
	return prt.Verify(proof, root, [][]byte{value}, keys...)
}

// XXX Reorder value/keys, and figure out how to merge keys into a single string
// after figuring out encoding between bytes/string.
func (prt *ProofRuntime) Verify(proof *Proof, root []byte, args [][]byte, keys ...string) (err error) {
	poz, err := prt.DecodeProof(proof)
	if err != nil {
		return cmn.ErrorWrap(err, "decoding proof")
	}
	return poz.Verify(root, args, keys...)
}

// DefaultProofRuntime only knows about Simple value
// proofs.
// To use e.g. IAVL proofs, register op-decoders as
// defined in the IAVL package.
func DefaultProofRuntime() (prt *ProofRuntime) {
	prt = NewProofRuntime()
	prt.RegisterAminoOpDecoder(
		ProofOpSimpleValue,
		reflect.TypeOf(SimpleValueOp{}),
	)
	return
}
