package json

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type PtrCustom struct {
	Value string
}

func (c *PtrCustom) MarshalJSON() ([]byte, error) {
	return []byte("\"custom\""), nil
}

type BareCustom struct {
	Value string
}

func (c BareCustom) MarshalJSON() ([]byte, error) {
	return []byte("\"custom\""), nil
}

type Struct struct {
	Name  string `json:"name"`
	Value int64
	Child *Struct `json:"omitempty"`
}

func TestMarshal(t *testing.T) {
	i64 := int64(64)
	ti := time.Date(2020, 6, 2, 18, 5, 13, 4346374, time.FixedZone("UTC+2", 2*60*60))

	testcases := map[string]struct {
		value  interface{}
		output string
	}{
		"string":          {"foo", `"foo"`},
		"float64":         {float64(3.14), `3.14`},
		"float64 neg":     {float64(-3.14), `-3.14`},
		"int32":           {int32(32), `32`},
		"int64":           {int64(64), `"64"`},
		"int64 neg":       {int64(-64), `"-64"`},
		"int64 ptr":       {&i64, `"64"`},
		"uint64":          {uint64(64), `"64"`},
		"nil":             {nil, `null`},
		"time":            {ti, `"2020-06-02T16:05:13.004346374Z"`},
		"time ptr":        {&ti, `"2020-06-02T16:05:13.004346374Z"`},
		"ptrcustom ptr":   {&PtrCustom{Value: "x"}, `"custom"`},
		"ptrcustom bare":  {PtrCustom{Value: "x"}, `{"Value":"x"}`}, // same as encoding/json
		"barecustom ptr":  {&BareCustom{Value: "x"}, `"custom"`},
		"barecustom bare": {BareCustom{Value: "x"}, `"custom"`},
		"slice nil":       {[]int(nil), `null`},
		"slice bytes":     {[]byte{1, 2, 3}, `"AQID"`},
		"slice int64":     {[]int64{1, 2, 3}, `["1","2","3"]`},
		"slice int64 ptr": {[]*int64{&i64, nil}, `["64",null]`},
		"array int64":     {[3]int64{1, 2, 3}, `["1","2","3"]`},
		"map int64":       {map[string]int64{"a": 1, "b": 2, "c": 3}, `{"a":"1","b":"2","c":"3"}`},
		"struct int64": {
			Struct{Name: "a", Value: 1, Child: &Struct{Name: "b", Value: 2}},
			`{"Name":"a","Value":"1","Child":{"Name":"b","Value":"2","Child":null}}`,
		},
	}
	for name, tc := range testcases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			var s strings.Builder
			err := encodeJSON(&s, tc.value)
			require.NoError(t, err)
			assert.JSONEq(t, tc.output, s.String())
		})
	}
}
