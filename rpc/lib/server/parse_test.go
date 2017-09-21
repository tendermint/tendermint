package rpcserver

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/go-wire/data"
)

func TestParseJSONMap(t *testing.T) {
	assert := assert.New(t)

	input := []byte(`{"value":"1234","height":22}`)

	// naive is float,string
	var p1 map[string]interface{}
	err := json.Unmarshal(input, &p1)
	if assert.Nil(err) {
		h, ok := p1["height"].(float64)
		if assert.True(ok, "%#v", p1["height"]) {
			assert.EqualValues(22, h)
		}
		v, ok := p1["value"].(string)
		if assert.True(ok, "%#v", p1["value"]) {
			assert.EqualValues("1234", v)
		}
	}

	// preloading map with values doesn't help
	tmp := 0
	p2 := map[string]interface{}{
		"value":  &data.Bytes{},
		"height": &tmp,
	}
	err = json.Unmarshal(input, &p2)
	if assert.Nil(err) {
		h, ok := p2["height"].(float64)
		if assert.True(ok, "%#v", p2["height"]) {
			assert.EqualValues(22, h)
		}
		v, ok := p2["value"].(string)
		if assert.True(ok, "%#v", p2["value"]) {
			assert.EqualValues("1234", v)
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
		Value:  &data.Bytes{},
	}
	err = json.Unmarshal(input, &p3)
	if assert.Nil(err) {
		h, ok := p3.Height.(*int)
		if assert.True(ok, "%#v", p3.Height) {
			assert.Equal(22, *h)
		}
		v, ok := p3.Value.(*data.Bytes)
		if assert.True(ok, "%#v", p3.Value) {
			assert.EqualValues([]byte{0x12, 0x34}, *v)
		}
	}

	// simplest solution, but hard-coded
	p4 := struct {
		Value  data.Bytes `json:"value"`
		Height int        `json:"height"`
	}{}
	err = json.Unmarshal(input, &p4)
	if assert.Nil(err) {
		assert.EqualValues(22, p4.Height)
		assert.EqualValues([]byte{0x12, 0x34}, p4.Value)
	}

	// so, let's use this trick...
	// dynamic keys on map, and we can deserialize to the desired types
	var p5 map[string]*json.RawMessage
	err = json.Unmarshal(input, &p5)
	if assert.Nil(err) {
		var h int
		err = json.Unmarshal(*p5["height"], &h)
		if assert.Nil(err) {
			assert.Equal(22, h)
		}

		var v data.Bytes
		err = json.Unmarshal(*p5["value"], &v)
		if assert.Nil(err) {
			assert.Equal(data.Bytes{0x12, 0x34}, v)
		}
	}
}

func TestParseJSONArray(t *testing.T) {
	assert := assert.New(t)

	input := []byte(`["1234",22]`)

	// naive is float,string
	var p1 []interface{}
	err := json.Unmarshal(input, &p1)
	if assert.Nil(err) {
		v, ok := p1[0].(string)
		if assert.True(ok, "%#v", p1[0]) {
			assert.EqualValues("1234", v)
		}
		h, ok := p1[1].(float64)
		if assert.True(ok, "%#v", p1[1]) {
			assert.EqualValues(22, h)
		}
	}

	// preloading map with values helps here (unlike map - p2 above)
	tmp := 0
	p2 := []interface{}{&data.Bytes{}, &tmp}
	err = json.Unmarshal(input, &p2)
	if assert.Nil(err) {
		v, ok := p2[0].(*data.Bytes)
		if assert.True(ok, "%#v", p2[0]) {
			assert.EqualValues([]byte{0x12, 0x34}, *v)
		}
		h, ok := p2[1].(*int)
		if assert.True(ok, "%#v", p2[1]) {
			assert.EqualValues(22, *h)
		}
	}
}

func TestParseRPC(t *testing.T) {
	assert := assert.New(t)

	demo := func(height int, name string) {}
	call := NewRPCFunc(demo, "height,name")

	cases := []struct {
		raw    string
		height int64
		name   string
		fail   bool
	}{
		// should parse
		{`[7, "flew"]`, 7, "flew", false},
		{`{"name": "john", "height": 22}`, 22, "john", false},
		// defaults
		{`{"name": "solo", "unused": "stuff"}`, 0, "solo", false},
		// should fail - wrong types/length
		{`["flew", 7]`, 0, "", true},
		{`[7,"flew",100]`, 0, "", true},
		{`{"name": -12, "height": "fred"}`, 0, "", true},
	}
	for idx, tc := range cases {
		i := strconv.Itoa(idx)
		data := []byte(tc.raw)
		vals, err := jsonParamsToArgs(call, data, 0)
		if tc.fail {
			assert.NotNil(err, i)
		} else {
			assert.Nil(err, "%s: %+v", i, err)
			if assert.Equal(2, len(vals), i) {
				assert.Equal(tc.height, vals[0].Int(), i)
				assert.Equal(tc.name, vals[1].String(), i)
			}
		}

	}

}
