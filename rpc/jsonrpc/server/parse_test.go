package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

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
	rfunc := NewRPCFunc(demo)

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
		vals, err := rfunc.parseParams(ctx, []byte(tc.raw))
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
	// URI parameter parsing happens in two phases:
	//
	// Phase 1 swizzles the query parameters into JSON.  The result of this
	// phase must be valid JSON, but may fail the second stage.
	//
	// Phase 2 decodes the JSON to obtain the actual arguments. A failure at
	// this stage means the JSON is not compatible with the target.

	t.Run("Swizzle", func(t *testing.T) {
		tests := []struct {
			name string
			url  string
			args []argInfo
			want string
			fail bool
		}{
			{
				name: "quoted numbers and strings",
				url:  `http://localhost?num="7"&str="flew"&neg="-10"`,
				args: []argInfo{{name: "neg"}, {name: "num"}, {name: "str"}, {name: "other"}},
				want: `{"neg":-10,"num":7,"str":"flew"}`,
			},
			{
				name: "unquoted numbers and strings",
				url:  `http://localhost?num1=7&str1=cabbage&num2=-199&str2=hey+you`,
				args: []argInfo{{name: "num1"}, {name: "num2"}, {name: "str1"}, {name: "str2"}, {name: "other"}},
				want: `{"num1":7,"num2":-199,"str1":"cabbage","str2":"hey you"}`,
			},
			{
				name: "quoted byte strings",
				url:  `http://localhost?left="Fahrvergnügen"&right="Applesauce"`,
				args: []argInfo{{name: "left", isBinary: true}, {name: "right", isBinary: false}},
				want: `{"left":"RmFocnZlcmduw7xnZW4=","right":"Applesauce"}`,
			},
			{
				name: "hexadecimal byte strings",
				url:  `http://localhost?lower=0x626f62&upper=0X646F7567`,
				args: []argInfo{{name: "upper", isBinary: true}, {name: "lower", isBinary: false}, {name: "other"}},
				want: `{"lower":"bob","upper":"ZG91Zw=="}`,
			},
			{
				name: "invalid hex odd length",
				url:  `http://localhost?bad=0xa`,
				args: []argInfo{{name: "bad"}, {name: "superbad"}},
				fail: true,
			},
			{
				name: "invalid hex empty",
				url:  `http://localhost?bad=0x`,
				args: []argInfo{{name: "bad"}},
				fail: true,
			},
			{
				name: "invalid quoted string",
				url:  `http://localhost?bad="double""`,
				args: []argInfo{{name: "bad"}},
				fail: true,
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				hreq, err := http.NewRequest("GET", test.url, nil)
				if err != nil {
					t.Fatalf("NewRequest for %q: %v", test.url, err)
				}

				bits, err := parseURLParams(test.args, hreq)
				if err != nil && !test.fail {
					t.Fatalf("Parse %q: unexpected error: %v", test.url, err)
				} else if err == nil && test.fail {
					t.Fatalf("Parse %q: got %#q, wanted error", test.url, string(bits))
				}
				if got := string(bits); got != test.want {
					t.Errorf("Parse %q: got %#q, want %#q", test.url, got, test.want)
				}
			})
		}
	})

	t.Run("Decode", func(t *testing.T) {
		type argValue struct {
			Height json.Number `json:"height"`
			Name   string      `json:"name"`
			Flag   bool        `json:"flag"`
		}

		echo := NewRPCFunc(func(_ context.Context, arg *argValue) (*argValue, error) {
			return arg, nil
		})

		tests := []struct {
			name string
			url  string
			fail string
			want interface{}
		}{
			{
				name: "valid all args",
				url:  `http://localhost?height=235&flag=true&name="bogart"`,
				want: &argValue{
					Height: "235",
					Flag:   true,
					Name:   "bogart",
				},
			},
			{
				name: "valid partial args",
				url:  `http://localhost?height="1987"&name=free+willy`,
				want: &argValue{
					Height: "1987",
					Name:   "free willy",
				},
			},
			{
				name: "invalid quoted number",
				url:  `http://localhost?height="-xx"`,
				fail: "invalid number literal",
			},
			{
				name: "invalid unquoted number",
				url:  `http://localhost?height=25*q`,
				fail: "invalid number literal",
			},
			{
				name: "invalid boolean",
				url:  `http://localhost?flag="garbage"`,
				fail: "flag of type bool",
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				hreq, err := http.NewRequest("GET", test.url, nil)
				if err != nil {
					t.Fatalf("NewRequest for %q: %v", test.url, err)
				}
				bits, err := parseURLParams(echo.args, hreq)
				if err != nil {
					t.Fatalf("Parse %#q: unexpected error: %v", test.url, err)
				}
				rsp, err := echo.Call(context.Background(), bits)
				if test.want != nil {
					assert.Equal(t, test.want, rsp)
				}
				if test.fail != "" {
					assert.ErrorContains(t, err, test.fail)
				}
			})
		}
	})
}
