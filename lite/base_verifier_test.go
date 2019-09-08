package lite

import (
	"testing"

	"github.com/stretchr/testify/assert"

	lerr "github.com/tendermint/tendermint/lite/errors"
	"github.com/tendermint/tendermint/types"
)

func TestBaseCert(t *testing.T) {
	assert := assert.New(t)

	keys := genPrivKeys(4)
	// 20, 30, 40, 50 - the first 3 don't have 2/3, the last 3 do!
	vals := keys.ToValidators(20, 10)
	// and a Verifier based on our known set
	chainID := "test-static"
	cert := NewBaseVerifier(chainID, 2, vals)

	cases := []struct {
		keys        privKeys
		vals        *types.ValidatorSet
		height      int64
		first, last int  // who actually signs
		proper      bool // true -> expect no error
		changed     bool // true -> expect validator change error
	}{
		// height regression
		{keys, vals, 1, 0, len(keys), false, false},
		// perfect, signed by everyone
		{keys, vals, 2, 0, len(keys), true, false},
		// skip little guy is okay
		{keys, vals, 3, 1, len(keys), true, false},
		// but not the big guy
		{keys, vals, 4, 0, len(keys) - 1, false, false},
		// Changing the power a little bit breaks the static validator.
		// The sigs are enough, but the validator hash is unknown.
		{keys, keys.ToValidators(20, 11), 5, 0, len(keys), false, true},
	}

	for _, tc := range cases {
		sh := tc.keys.GenSignedHeader(chainID, tc.height, nil, tc.vals, tc.vals,
			[]byte("foo"), []byte("params"), []byte("results"), tc.first, tc.last)
		err := cert.Verify(sh)
		if tc.proper {
			assert.Nil(err, "%+v", err)
		} else {
			assert.NotNil(err)
			if tc.changed {
				assert.True(lerr.IsErrUnexpectedValidators(err), "%+v", err)
			}
		}
	}
}
