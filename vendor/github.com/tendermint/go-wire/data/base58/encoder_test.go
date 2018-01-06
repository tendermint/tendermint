package base58_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	data "github.com/tendermint/go-wire/data"
	"github.com/tendermint/go-wire/data/base58"
)

func TestEncoders(t *testing.T) {
	assert := assert.New(t)

	// TODO: also test other alphabets???
	btc := base58.BTCEncoder

	cases := []struct {
		encoder         data.ByteEncoder
		input, expected []byte
	}{
		{btc, []byte(`"3mJr7AoUXx2Wqd"`), []byte("1234598760")},
		{btc, []byte(`"3yxU3u1igY8WkgtjK92fbJQCd4BZiiT1v25f"`), []byte("abcdefghijklmnopqrstuvwxyz")},
		// these are errors
		{btc, []byte(`0123`), nil},    // not in quotes
		{btc, []byte(`"3mJr0"`), nil}, // invalid chars
	}

	for _, tc := range cases {
		var output []byte
		err := tc.encoder.Unmarshal(&output, tc.input)
		if tc.expected == nil {
			assert.NotNil(err, tc.input)
		} else if assert.Nil(err, "%s: %+v", tc.input, err) {
			assert.Equal(tc.expected, output, tc.input)
		}
	}
}
