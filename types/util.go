package types

import (
	"bytes"
	"encoding/json"

	"github.com/tendermint/go-wire/data"
	cmn "github.com/tendermint/tmlibs/common"
)

//------------------------------------------------------------------------------

// Validators is a list of validators that implements the Sort interface
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

func ValidatorsString(vs Validators) string {
	s := make([]validatorPretty, len(vs))
	for i, v := range vs {
		s[i] = validatorPretty{v.PubKey, v.Power}
	}
	b, err := json.Marshal(s)
	if err != nil {
		cmn.PanicSanity(err.Error())
	}
	return string(b)
}

type validatorPretty struct {
	PubKey data.Bytes `json:"pub_key"`
	Power  int64      `json:"power"`
}

//------------------------------------------------------------------------------

// KVPairInt is a helper method to build KV pair with an integer value.
func KVPairInt(key string, val int64) *KVPair {
	return &KVPair{
		Key:       key,
		ValueInt:  val,
		ValueType: KVPair_INT,
	}
}

// KVPairString is a helper method to build KV pair with a string value.
func KVPairString(key, val string) *KVPair {
	return &KVPair{
		Key:         key,
		ValueString: val,
		ValueType:   KVPair_STRING,
	}
}
