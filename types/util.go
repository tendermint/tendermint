package types

import (
	"bytes"
	"encoding/json"
	"sort"
)

//------------------------------------------------------------------------------

// Validators is a list of validators that implements the Sort interface
type Validators []Validator

var _ sort.Interface = (Validators)(nil)

// All these methods for Validators:
//    Len, Less and Swap
// are for Validators to implement sort.Interface
// which will be used by the sort package.
// See Issue https://github.com/tendermint/abci/issues/212

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

func ValidatorsString(vs Validators) string {
	s := make([]validatorPretty, len(vs))
	for i, v := range vs {
		s[i] = validatorPretty(v)
	}
	b, err := json.Marshal(s)
	if err != nil {
		panic(err.Error())
	}
	return string(b)
}

type validatorPretty struct {
	PubKey []byte `json:"pub_key"`
	Power  int64  `json:"power"`
}
