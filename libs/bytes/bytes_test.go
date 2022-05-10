package bytes

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

// This is a trivial test for protobuf compatibility.
func TestMarshal(t *testing.T) {
	bz := []byte("hello world")
	dataB := HexBytes(bz)
	bz2, err := dataB.Marshal()
	assert.NoError(t, err)
	assert.Equal(t, bz, bz2)

	var dataB2 HexBytes
	err = (&dataB2).Unmarshal(bz)
	assert.NoError(t, err)
	assert.Equal(t, dataB, dataB2)
}

// Test that the hex encoding works.
func TestJSONMarshal(t *testing.T) {
	type TestStruct struct {
		B1 []byte
		B2 HexBytes
	}

	cases := []struct {
		input    []byte
		expected string
	}{
		{[]byte(``), `{"B1":"","B2":""}`},
		{[]byte(`a`), `{"B1":"YQ==","B2":"61"}`},
		{[]byte(`abc`), `{"B1":"YWJj","B2":"616263"}`},
		{[]byte("\x1a\x2b\x3c"), `{"B1":"Gis8","B2":"1A2B3C"}`},
	}

	for i, tc := range cases {
		tc := tc
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
			assert.Equal(t, string(ts2.B2), string(tc.input))
		})
	}
}

func TestHexBytes_String(t *testing.T) {
	hs := HexBytes([]byte("test me"))
	if _, err := strconv.ParseInt(hs.String(), 16, 64); err != nil {
		t.Fatal(err)
	}
}
