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

func TestMarshal(t *testing.T) {
	i64 := int64(64)
	testcases := map[string]struct {
		value  interface{}
		output string
	}{
		"string":      {"foo", `"foo"`},
		"float64":     {float64(3.14), `3.14`},
		"float64 neg": {float64(-3.14), `-3.14`},
		"int32":       {int32(32), `32`},
		"int64":       {int64(64), `"64"`},
		"int64 neg":   {int64(-64), `"-64"`},
		"int64 ptr":   {&i64, `"64"`},
		"uint64":      {uint64(64), `"64"`},
		"nil":         {nil, `null`},
		"time.Time": {
			time.Date(2020, 6, 2, 18, 5, 13, 4346374, time.FixedZone("UTC+2", 2*60*60)),
			`"2020-06-02T16:05:13.004346374Z"`,
		},
		"ptrcustom ptr":   {&PtrCustom{Value: "x"}, `"custom"`},
		"ptrcustom bare":  {PtrCustom{Value: "x"}, `{"Value":"x"}`}, // same as encoding/json
		"barecustom ptr":  {&BareCustom{Value: "x"}, `"custom"`},
		"barecustom bare": {BareCustom{Value: "x"}, `"custom"`},
	}
	for name, tc := range testcases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			var s strings.Builder
			err := encodeJSON(&s, tc.value)
			require.NoError(t, err)
			assert.EqualValues(t, tc.output, s.String())
		})
	}
}
