package json

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshal(t *testing.T) {
	testcases := []struct {
		value  interface{}
		output string
	}{
		{"foo", `"foo"`},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(fmt.Sprintf("%v", tc.value), func(t *testing.T) {
			var s strings.Builder
			rv := reflect.ValueOf(tc.value)
			err := encodeReflectJSON(&s, rv)
			require.NoError(t, err)
			assert.EqualValues(t, tc.output, s.String())
		})
	}
}
