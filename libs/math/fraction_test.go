package math

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseFraction(t *testing.T) {

	testCases := []struct {
		f   string
		exp Fraction
		err bool
	}{
		{
			f:   "2/3",
			exp: Fraction{2, 3},
			err: false,
		},
		{
			f:   "15/5",
			exp: Fraction{15, 5},
			err: false,
		},
		{
			f:   "-1/2",
			exp: Fraction{-1, 2},
			err: false,
		},
		{
			f:   "1/-2",
			exp: Fraction{1, -2},
			err: false,
		},
		{
			f:   "2/3/4",
			exp: Fraction{},
			err: true,
		},
		{
			f:   "123",
			exp: Fraction{},
			err: true,
		},
		{
			f:   "1a2/4",
			exp: Fraction{},
			err: true,
		},
		{
			f:   "1/3bc4",
			exp: Fraction{},
			err: true,
		},
	}

	for idx, tc := range testCases {
		output, err := ParseFraction(tc.f)
		if tc.err {
			assert.Error(t, err, idx)
		} else {
			assert.NoError(t, err, idx)
		}
		assert.Equal(t, tc.exp, output, idx)
	}

}
