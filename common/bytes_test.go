package common

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// This is a trivial test for protobuf compatibility.
func TestMarshal(t *testing.T) {
	b := []byte("hello world")
	dataB := Bytes(b)
	b2, err := dataB.Marshal()
	assert.Nil(t, err)
	assert.Equal(t, b, b2)

	var dataB2 Bytes
	err = (&dataB2).Unmarshal(b)
	assert.Nil(t, err)
	assert.Equal(t, dataB, dataB2)
}

// Test that the hex encoding works.
func TestJSONMarshal(t *testing.T) {

	type TestStruct struct {
		B1 []byte
		B2 Bytes
	}

	cases := []struct {
		input    []byte
		expected string
	}{
		{[]byte(``), `{"B1":"","B2":""}`},
		{[]byte(``), `{"B1":"","B2":""}`},
		{[]byte(``), `{"B1":"","B2":""}`},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("Case %d", i), func(t *testing.T) {
			ts := TestStruct{B1: tc.input, B2: tc.input}

			// Test that it marshals correctly to JSON.
			jsonBytes, err := json.Marshal(ts)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, string(jsonBytes), tc.expected)

			// TODO do fuzz testing to ensure that unmarshal fails

			// Test that unmarshaling works correctly.
			ts2 := TestStruct{}
			err = json.Unmarshal(jsonBytes, &ts2)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, ts2.B1, tc.input)
			assert.Equal(t, ts2.B2, Bytes(tc.input))
		})
	}
}
