package types

import (
	"bytes"

	"github.com/tendermint/go-wire"
)

// validators implements sort

type Validators []*Validator

func (v Validators) Len() int {
	return len(v)
}

// XXX: doesn't distinguish same validator with different power
func (v Validators) Less(i, j int) bool {
	return bytes.Compare(v[i].PubKey, v[j].PubKey) <= 0
}

func (v Validators) Swap(i, j int) {
	v1 := v[i]
	v[i] = v[j]
	v[j] = v1
}

//-------------------------------------

type validatorPretty struct {
	PubKey []byte `json:"pub_key"`
	Power  uint64 `json:"power"`
}

func ValidatorsString(vs Validators) string {
	s := make([]validatorPretty, len(vs))
	for i, v := range vs {
		s[i] = validatorPretty{v.PubKey, v.Power}
	}
	return string(wire.JSONBytes(s))
}
