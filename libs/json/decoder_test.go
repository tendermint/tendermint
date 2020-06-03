package json

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecode(t *testing.T) {
	var i64Nil *int64
	var stNil *Struct
	i32 := int32(32)
	i64 := int64(64)
	str := string("foo")
	testcases := map[string]struct {
		json  string
		value interface{}
		err   bool
	}{
		"bool true":          {"true", true, false},
		"bool false":         {"false", false, false},
		"float32":            {"3.14", float32(3.14), false},
		"float64":            {"3.14", float64(3.14), false},
		"int32":              {`32`, int32(32), false},
		"int32 string":       {`"32"`, int32(32), true},
		"int32 ptr":          {`32`, &i32, false},
		"int64":              {`"64"`, int64(64), false},
		"int64 noend":        {`"64`, int64(64), true},
		"int64 number":       {`64`, int64(64), true},
		"int64 ptr":          {`"64"`, &i64, false},
		"int64 ptr nil":      {`null`, i64Nil, false},
		"string":             {`"foo"`, "foo", false},
		"string noend":       {`"foo`, "foo", true},
		"string ptr":         {`"foo"`, &str, false},
		"slice byte":         {`"AQID"`, []byte{1, 2, 3}, false},
		"slice bytes":        {`["AQID"]`, [][]byte{{1, 2, 3}}, false},
		"slice int32":        {`[1,2,3]`, []int32{1, 2, 3}, false},
		"slice int64":        {`["1","2","3"]`, []int64{1, 2, 3}, false},
		"slice int64 number": {`[1,2,3]`, []int64{1, 2, 3}, true},
		"slice int64 ptr":    {`["64"]`, []*int64{&i64}, false},
		"slice int64 empty":  {`[]`, []int64(nil), false},
		"slice int64 null":   {`null`, []int64(nil), false},
		"array byte":         {`"AQID"`, [3]byte{1, 2, 3}, false},
		"array byte large":   {`"AQID"`, [4]byte{1, 2, 3, 4}, true},
		"array byte small":   {`"AQID"`, [2]byte{1, 2}, true},
		"array int32":        {`[1,2,3]`, [3]int32{1, 2, 3}, false},
		"array int64":        {`["1","2","3"]`, [3]int64{1, 2, 3}, false},
		"array int64 number": {`[1,2,3]`, [3]int64{1, 2, 3}, true},
		"array int64 large":  {`["1","2","3"]`, [4]int64{1, 2, 3, 4}, true},
		"array int64 small":  {`["1","2","3"]`, [2]int64{1, 2}, true},
		"map bytes":          {`{"b":"AQID"}`, map[string][]byte{"b": {1, 2, 3}}, false},
		"map int32":          {`{"a":1,"b":2}`, map[string]int32{"a": 1, "b": 2}, false},
		"map int64":          {`{"a":"1","b":"2"}`, map[string]int64{"a": 1, "b": 2}, false},
		"map int64 empty":    {`{}`, map[string]int64{}, false},
		"map int64 null":     {`null`, map[string]int64(nil), false},
		"map int key":        {`{}`, map[int]int{}, true},
		"time":               {`"2020-06-03T17:35:30Z"`, time.Date(2020, 6, 3, 17, 35, 30, 0, time.UTC), false},
		"time non-utc":       {`"2020-06-03T17:35:30+02:00"`, time.Time{}, true},
		"time nozone":        {`"2020-06-03T17:35:30"`, time.Time{}, true},
		"struct": {
			`{"name":"foo","Value":"42","Child":{"name":"bar","Value":"7"}}`,
			Struct{Name: "foo", Value: 42, Child: &Struct{Name: "bar", Value: 7}},
			false,
		},
		"struct ptr": {
			`{"name":"foo","Value":"42","Child":{"name":"bar","Value":"7"}}`,
			&Struct{Name: "foo", Value: 42, Child: &Struct{Name: "bar", Value: 7}},
			false,
		},
		"struct ptr null": {`null`, stNil, false},
		"custom value":    {`{"Value":"foo"}`, BareCustom{}, false},
		"custom ptr":      {`"foo"`, &PtrCustom{Value: "custom"}, false},
		"car testa":       {`{"type":"car/tesla","value":{"Color":"blue"}}`, Tesla{Color: "blue"}, false},
		// FIXME Handle this
		//"car testa ptr":   {`{"type":"car/tesla","value":{"Color":"blue"}}`, &Tesla{Color: "blue"}, false},
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

			// Unwrap the target pointer and get the value behind the interface.
			actual := reflect.ValueOf(target).Elem().Interface()
			assert.Equal(t, tc.value, actual)
		})
	}
}
