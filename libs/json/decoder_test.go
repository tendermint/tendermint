package json

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecode(t *testing.T) {
	var i64_nil *int64
	i32 := int32(32)
	i64 := int64(64)
	str := string("foo")
	testcases := map[string]struct {
		json  string
		value interface{}
		err   bool
	}{
		"bool true":     {"true", true, false},
		"bool false":    {"false", false, false},
		"float32":       {"3.14", float32(3.14), false},
		"float64":       {"3.14", float64(3.14), false},
		"int32":         {`32`, int32(32), false},
		"int32 string":  {`"32"`, int32(32), true},
		"int32 ptr":     {`32`, &i32, false},
		"int64":         {`"64"`, int64(64), false},
		"int64 noend":   {`"64`, int64(64), true},
		"int64 number":  {`64`, int64(64), true},
		"int64 ptr":     {`"64"`, &i64, false},
		"int64 ptr nil": {`null`, i64_nil, false},
		"string":        {`"foo"`, "foo", false},
		"string noend":  {`"foo`, "foo", true},
		"string ptr":    {`"foo"`, &str, false},
	}
	for name, tc := range testcases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			// Create a target variable as a pointer to the zero value of the tc.value type,
			// and wrap it in an empty interface. Decode into that interface.
			target := reflect.New(reflect.TypeOf(tc.value)).Interface()
			err := decodeJSON([]byte(tc.json), target)
			if tc.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Unwrap the pointer and get the bare value behind the interface.
			actual := reflect.ValueOf(target).Elem().Interface()
			assert.Equal(t, tc.value, actual)
		})
	}
}
