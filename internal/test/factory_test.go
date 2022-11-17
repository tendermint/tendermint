package test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/types"
)

func TestMakeHeader(t *testing.T) {
	header := MakeHeader(t, &types.Header{})
	require.NotNil(t, header)

	require.NoError(t, header.ValidateBasic())
}

func TestValidatorSet(t *testing.T) {
	valSet, privVals := ValidatorSet(t, 5, 10)
	for idx, val := range valSet.Validators {
		pk, err := privVals[idx].GetPubKey()
		require.NoError(t, err)
		require.Equal(t, val.PubKey, pk)
	}
}
