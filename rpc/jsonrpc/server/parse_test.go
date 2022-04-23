package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/libs/bytes"
)

func TestParseJSONMap(t *testing.T) {
	input := []byte(`{"value":"1234","height":22}`)

	// naive is float,string
	var p1 map[string]interface{}
	err := json.Unmarshal(input, &p1)
	if assert.NoError(t, err) {
		h, ok := p1["height"].(float64)
		if assert.True(t, ok, "%#v", p1["height"]) {
			assert.EqualValues(t, 22, h)
		}
		v, ok := p1["value"].(string)
		if assert.True(t, ok, "%#v", p1["value"]) {
			assert.EqualValues(t, "1234", v)
		}
	}

	// preloading map with values doesn't help
	tmp := 0
	p2 := map[string]interface{}{
		"value":  &bytes.HexBytes{},
		"height": &tmp,
	}
	err = json.Unmarshal(input, &p2)
	if assert.NoError(t, err) {
		h, ok := p2["height"].(float64)
		if assert.True(t, ok, "%#v", p2["height"]) {
			assert.EqualValues(t, 22, h)
		}
		v, ok := p2["value"].(string)
		if assert.True(t, ok, "%#v", p2["value"]) {
			assert.EqualValues(t, "1234", v)
		}
	}

	// preload here with *pointers* to the desired types
	// struct has unknown types, but hard-coded keys
	tmp = 0
	p3 := struct {
		Value  interface{} `json:"value"`
		Height interface{} `json:"height"`
	}{
		Height: &tmp,
		Value:  &bytes.HexBytes{},
	}
	err = json.Unmarshal(input, &p3)
	if assert.NoError(t, err) {
		h, ok := p3.Height.(*int)
		if assert.True(t, ok, "%#v", p3.Height) {
			assert.Equal(t, 22, *h)
		}
		v, ok := p3.Value.(*bytes.HexBytes)
		if assert.True(t, ok, "%#v", p3.Value) {
			assert.EqualValues(t, []byte{0x12, 0x34}, *v)
		}
	}

	// simplest solution, but hard-coded
	p4 := struct {
		Value  bytes.HexBytes `json:"value"`
		Height int            `json:"height"`
	}{}
	err = json.Unmarshal(input, &p4)
	if assert.NoError(t, err) {
		assert.EqualValues(t, 22, p4.Height)
		assert.EqualValues(t, []byte{0x12, 0x34}, p4.Value)
	}

	// so, let's use this trick...
	// dynamic keys on map, and we can deserialize to the desired types
	var p5 map[string]*json.RawMessage
	err = json.Unmarshal(input, &p5)
	if assert.NoError(t, err) {
		var h int
		err = json.Unmarshal(*p5["height"], &h)
		if assert.NoError(t, err) {
			assert.Equal(t, 22, h)
		}

		var v bytes.HexBytes
		err = json.Unmarshal(*p5["value"], &v)
		if assert.NoError(t, err) {
			assert.Equal(t, bytes.HexBytes{0x12, 0x34}, v)
		}
	}
}

func TestParseJSONArray(t *testing.T) {
	input := []byte(`["1234",22]`)

	// naive is float,string
	var p1 []interface{}
	err := json.Unmarshal(input, &p1)
	if assert.NoError(t, err) {
		v, ok := p1[0].(string)
		if assert.True(t, ok, "%#v", p1[0]) {
			assert.EqualValues(t, "1234", v)
		}
		h, ok := p1[1].(float64)
		if assert.True(t, ok, "%#v", p1[1]) {
			assert.EqualValues(t, 22, h)
		}
	}

	// preloading map with values helps here (unlike map - p2 above)
	tmp := 0
	p2 := []interface{}{&bytes.HexBytes{}, &tmp}
	err = json.Unmarshal(input, &p2)
	if assert.NoError(t, err) {
		v, ok := p2[0].(*bytes.HexBytes)
		if assert.True(t, ok, "%#v", p2[0]) {
			assert.EqualValues(t, []byte{0x12, 0x34}, *v)
		}
		h, ok := p2[1].(*int)
		if assert.True(t, ok, "%#v", p2[1]) {
			assert.EqualValues(t, 22, *h)
		}
	}
}

func TestParseJSONRPC(t *testing.T) {
	type demoArgs struct {
		Height int    `json:"height,string"`
		Name   string `json:"name"`
	}
	demo := func(ctx context.Context, _ *demoArgs) error { return nil }
	call := NewRPCFunc(demo)

	cases := []struct {
		raw    string
		height int64
		name   string
		fail   bool
	}{
		// should parse
		{`["7", "flew"]`, 7, "flew", false},
		{`{"name": "john", "height": "22"}`, 22, "john", false},
		// defaults
		{`{"name": "solo", "unused": "stuff"}`, 0, "solo", false},
		// should fail - wrong types/length
		{`["flew", 7]`, 0, "", true},
		{`[7,"flew",100]`, 0, "", true},
		{`{"name": -12, "height": "fred"}`, 0, "", true},
	}
	ctx := context.Background()
	for idx, tc := range cases {
		i := strconv.Itoa(idx)
		vals, err := parseParams(ctx, call, []byte(tc.raw))
		if tc.fail {
			assert.Error(t, err, i)
		} else {
			assert.NoError(t, err, "%s: %+v", i, err)
			assert.Equal(t, 2, len(vals), i)
			p, ok := vals[1].Interface().(*demoArgs)
			if assert.True(t, ok) {
				assert.Equal(t, tc.height, int64(p.Height), i)
				assert.Equal(t, tc.name, p.Name, i)
			}
		}

	}
}

func TestParseURI(t *testing.T) {
	type args struct {
		Height json.Number `json:"height"`
		Name   string      `json:"name"`
	}
	demo := func(ctx context.Context, arg *args) error { return nil }
	call := NewRPCFunc(demo)

	cases := []struct {
		raw    []interface{}
		height int64
		name   string
		fail   bool
	}{
		// can parse numbers unquoted and strings quoted
		{[]interface{}{"7", `"flew"`}, 7, "flew", false},
		{[]interface{}{"22", `"john"`}, 22, "john", false},
		{[]interface{}{"-10", `"bob"`}, -10, "bob", false},
		// can parse numbers quoted, too
		{[]interface{}{`"7"`, `"flew"`}, 7, "flew", false},
		{[]interface{}{`"-10"`, `"bob"`}, -10, "bob", false},
		// can parse strings hex-escaped, in either case
		{[]interface{}{`-9`, `0x626f62`}, -9, "bob", false},
		{[]interface{}{`-9`, `0X646F7567`}, -9, "doug", false},
		// can parse strings unquoted (as per OpenAPI docs)
		{[]interface{}{`0`, `hey you`}, 0, "hey you", false},
		// fail for invalid numbers, strings, hex
		{[]interface{}{`"-xx"`, `bob`}, 0, "", true},  // bad number
		{[]interface{}{`"95""`, `"bob`}, 0, "", true}, // bad string
		{[]interface{}{`15`, `0xa`}, 0, "", true},     // bad hex
	}
	for idx, tc := range cases {
		i := strconv.Itoa(idx)
		t.Run(i, func(t *testing.T) {
			url := fmt.Sprintf("http://test.com/method?height=%v&name=%v", tc.raw...)
			req, err := http.NewRequest("GET", url, nil)
			require.NoError(t, err)

			vals, err := parseURLParams(context.Background(), call, req)
			if tc.fail {
				require.Error(t, err, i)
				return
			}
			require.NoError(t, err)
			require.Equal(t, 2, len(vals)) // ctx, *args
			arg := vals[1].Interface().(*args)
			h, err := arg.Height.Int64()
			require.NoError(t, err)
			require.Equal(t, tc.height, h)
			require.Equal(t, tc.name, arg.Name)
		})
	}
}
