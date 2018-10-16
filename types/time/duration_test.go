package time

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDurationPretty(t *testing.T) {
	testCases := []struct {
		jsonBytes []byte
		expErr    bool
		expValue  time.Duration
	}{
		{[]byte(`"10s"`), false, 10 * time.Second},
		{[]byte(`"48h0m0s"`), false, 48 * time.Hour},
		{[]byte(`"10kkk"`), true, 0},
		{[]byte(`"kkk"`), true, 0},
	}

	for i, tc := range testCases {
		var d DurationPretty
		if tc.expErr {
			assert.Error(t, d.UnmarshalJSON(tc.jsonBytes), "#%d", i)
		} else {
			assert.NoError(t, d.UnmarshalJSON(tc.jsonBytes), "#%d", i)
			assert.Equal(t, tc.expValue, d.Duration, "#%d", i)
		}
	}
}
