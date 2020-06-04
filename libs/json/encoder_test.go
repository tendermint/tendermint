package json_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/json"
)

func TestMarshal(t *testing.T) {
	s := "string"
	sPtr := &s
	i64 := int64(64)
	ti := time.Date(2020, 6, 2, 18, 5, 13, 4346374, time.FixedZone("UTC+2", 2*60*60))
	tesla := &Tesla{Color: "blue"}

	testcases := map[string]struct {
		value  interface{}
		output string
	}{
		"nil":             {nil, `null`},
		"string":          {"foo", `"foo"`},
		"float32":         {float32(3.14), `3.14`},
		"float32 neg":     {float32(-3.14), `-3.14`},
		"float64":         {float64(3.14), `3.14`},
		"float64 neg":     {float64(-3.14), `-3.14`},
		"int32":           {int32(32), `32`},
		"int64":           {int64(64), `"64"`},
		"int64 neg":       {int64(-64), `"-64"`},
		"int64 ptr":       {&i64, `"64"`},
		"uint64":          {uint64(64), `"64"`},
		"time":            {ti, `"2020-06-02T16:05:13.004346374Z"`},
		"time empty":      {time.Time{}, `"0001-01-01T00:00:00Z"`},
		"time ptr":        {&ti, `"2020-06-02T16:05:13.004346374Z"`},
		"customptr":       {CustomPtr{Value: "x"}, `{"Value":"x"}`}, // same as encoding/json
		"customptr ptr":   {&CustomPtr{Value: "x"}, `"custom"`},
		"customvalue":     {CustomValue{Value: "x"}, `"custom"`},
		"customvalue ptr": {&CustomValue{Value: "x"}, `"custom"`},
		"slice nil":       {[]int(nil), `null`},
		"slice bytes":     {[]byte{1, 2, 3}, `"AQID"`},
		"slice int64":     {[]int64{1, 2, 3}, `["1","2","3"]`},
		"slice int64 ptr": {[]*int64{&i64, nil}, `["64",null]`},
		"array int64":     {[3]int64{1, 2, 3}, `["1","2","3"]`},
		"map int64":       {map[string]int64{"a": 1, "b": 2, "c": 3}, `{"a":"1","b":"2","c":"3"}`},
		"tesla":           {tesla, `{"type":"car/tesla","value":{"Color":"blue"}}`},
		"tesla value":     {*tesla, `{"type":"car/tesla","value":{"Color":"blue"}}`},
		"tesla interface": {Car(tesla), `{"type":"car/tesla","value":{"Color":"blue"}}`},
		"tags": {
			Tags{JSONName: "name", OmitEmpty: "foo", Hidden: "bar", Tags: &Tags{JSONName: "child"}},
			`{"name":"name","OmitEmpty":"foo","tags":{"name":"child"}}`,
		},
		"tags empty": {Tags{}, `{"name":""}`},
		"struct": {
			Struct{
				Bool: true, Float64: 3.14, Int32: 32, Int64: 64, Int64Ptr: &i64,
				String: "foo", StringPtrPtr: &sPtr, Bytes: []byte{1, 2, 3},
				Time: ti, Car: tesla, Child: &Struct{Bool: false, String: "child"},
				private: "private",
			},
			`{
				"Bool":true, "Float64":3.14, "Int32":32, "Int64":"64", "Int64Ptr":"64",
				"String":"foo", "StringPtrPtr": "string", "Bytes":"AQID",
				"Time":"2020-06-02T16:05:13.004346374Z",
				"Car":{"type":"car/tesla","value":{"Color":"blue"}},
				"Child":{
					"Bool":false, "Float64":0, "Int32":0, "Int64":"0", "Int64Ptr":null,
					"String":"child", "StringPtrPtr":null, "Bytes":null,
					"Time":"0001-01-01T00:00:00Z", "Car":null, "Child":null
				}
			}`,
		},
	}
	for name, tc := range testcases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			bz, err := json.Marshal(tc.value)
			require.NoError(t, err)
			assert.JSONEq(t, tc.output, string(bz))
		})
	}
}
