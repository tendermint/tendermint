package merkle

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	amino "github.com/tendermint/go-amino"
)

const ProofOpDomino = "test:domino"

// Expects given input, produces given output.
// Like the game dominos.
type DominoOp struct {
	key    string // unexported, may be empty
	Input  string
	Output string
}

func NewDominoOp(key, input, output string) DominoOp {
	return DominoOp{
		key:    key,
		Input:  input,
		Output: output,
	}
}

//nolint:unused
func DominoOpDecoder(pop ProofOp) (ProofOperator, error) {
	if pop.Type != ProofOpDomino {
		panic("unexpected proof op type")
	}
	var op DominoOp // a bit strange as we'll discard this, but it works.
	err := amino.UnmarshalBinaryLengthPrefixed(pop.Data, &op)
	if err != nil {
		return nil, errors.Wrap(err, "decoding ProofOp.Data into SimpleValueOp")
	}
	return NewDominoOp(string(pop.Key), op.Input, op.Output), nil
}

func (dop DominoOp) ProofOp() ProofOp {
	bz := amino.MustMarshalBinaryLengthPrefixed(dop)
	return ProofOp{
		Type: ProofOpDomino,
		Key:  []byte(dop.key),
		Data: bz,
	}
}

func (dop DominoOp) Run(input [][]byte) (output [][]byte, err error) {
	if len(input) != 1 {
		return nil, errors.New("Expected input of length 1")
	}
	if string(input[0]) != dop.Input {
		return nil, errors.Errorf("Expected input %v, got %v",
			dop.Input, string(input[0]))
	}
	return [][]byte{[]byte(dop.Output)}, nil
}

func (dop DominoOp) GetKey() []byte {
	return []byte(dop.key)
}

//----------------------------------------

func TestProofOperators(t *testing.T) {
	var err error

	// ProofRuntime setup
	// TODO test this somehow.
	// prt := NewProofRuntime()
	// prt.RegisterOpDecoder(ProofOpDomino, DominoOpDecoder)

	// ProofOperators setup
	op1 := NewDominoOp("KEY1", "INPUT1", "INPUT2")
	op2 := NewDominoOp("KEY2", "INPUT2", "INPUT3")
	op3 := NewDominoOp("", "INPUT3", "INPUT4")
	op4 := NewDominoOp("KEY4", "INPUT4", "OUTPUT4")

	// Good
	popz := ProofOperators([]ProofOperator{op1, op2, op3, op4})
	err = popz.Verify(bz("OUTPUT4"), "/KEY4/KEY2/KEY1", [][]byte{bz("INPUT1")})
	assert.Nil(t, err)
	err = popz.VerifyValue(bz("OUTPUT4"), "/KEY4/KEY2/KEY1", bz("INPUT1"))
	assert.Nil(t, err)

	// BAD INPUT
	err = popz.Verify(bz("OUTPUT4"), "/KEY4/KEY2/KEY1", [][]byte{bz("INPUT1_WRONG")})
	assert.NotNil(t, err)
	err = popz.VerifyValue(bz("OUTPUT4"), "/KEY4/KEY2/KEY1", bz("INPUT1_WRONG"))
	assert.NotNil(t, err)

	// BAD KEY 1
	err = popz.Verify(bz("OUTPUT4"), "/KEY3/KEY2/KEY1", [][]byte{bz("INPUT1")})
	assert.NotNil(t, err)

	// BAD KEY 2
	err = popz.Verify(bz("OUTPUT4"), "KEY4/KEY2/KEY1", [][]byte{bz("INPUT1")})
	assert.NotNil(t, err)

	// BAD KEY 3
	err = popz.Verify(bz("OUTPUT4"), "/KEY4/KEY2/KEY1/", [][]byte{bz("INPUT1")})
	assert.NotNil(t, err)

	// BAD KEY 4
	err = popz.Verify(bz("OUTPUT4"), "//KEY4/KEY2/KEY1", [][]byte{bz("INPUT1")})
	assert.NotNil(t, err)

	// BAD KEY 5
	err = popz.Verify(bz("OUTPUT4"), "/KEY2/KEY1", [][]byte{bz("INPUT1")})
	assert.NotNil(t, err)

	// BAD OUTPUT 1
	err = popz.Verify(bz("OUTPUT4_WRONG"), "/KEY4/KEY2/KEY1", [][]byte{bz("INPUT1")})
	assert.NotNil(t, err)

	// BAD OUTPUT 2
	err = popz.Verify(bz(""), "/KEY4/KEY2/KEY1", [][]byte{bz("INPUT1")})
	assert.NotNil(t, err)

	// BAD POPZ 1
	popz = []ProofOperator{op1, op2, op4}
	err = popz.Verify(bz("OUTPUT4"), "/KEY4/KEY2/KEY1", [][]byte{bz("INPUT1")})
	assert.NotNil(t, err)

	// BAD POPZ 2
	popz = []ProofOperator{op4, op3, op2, op1}
	err = popz.Verify(bz("OUTPUT4"), "/KEY4/KEY2/KEY1", [][]byte{bz("INPUT1")})
	assert.NotNil(t, err)

	// BAD POPZ 3
	popz = []ProofOperator{}
	err = popz.Verify(bz("OUTPUT4"), "/KEY4/KEY2/KEY1", [][]byte{bz("INPUT1")})
	assert.NotNil(t, err)
}

func bz(s string) []byte {
	return []byte(s)
}
