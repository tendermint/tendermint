package math

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseFraction(t *testing.T) {

	testCases := []struct {
		inputString string
		expFraction Fraction
		err         bool
	}{
		{
			inputString: "2/3",
			expFraction: Fraction{2, 3},
			err:         false,
		},
		{
			inputString: "15/5",
			expFraction: Fraction{15, 5},
			err:         false,
		},
		{
			inputString: "-1/2",
			expFraction: Fraction{-1, 2},
			err:         false,
		},
		{
			inputString: "1/-2",
			expFraction: Fraction{1, -2},
			err:         false,
		},
		{
			inputString: "2/3/4",
			expFraction: Fraction{},
			err:         true,
		},
		{
			inputString: "123",
			expFraction: Fraction{},
			err:         true,
		},
		{
			inputString: "1a2/4",
			expFraction: Fraction{},
			err:         true,
		},
		{
			inputString: "1/3bc4",
			expFraction: Fraction{},
			err:         true,
		},
	}

	for idx, tc := range testCases {
		output, err := ParseFraction(tc.inputString)
		if tc.err {
			assert.Error(t, err, idx)
		} else {
			assert.NoError(t, err, idx)
		}
		assert.Equal(t, tc.expFraction, output, idx)
	}

}
