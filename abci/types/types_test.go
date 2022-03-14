package types_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/merkle"
)

func TestHashAndProveResults(t *testing.T) {
	trs := []*abci.ExecTxResult{
		{Code: 0, Data: nil},
		{Code: 0, Data: []byte{}},
		{Code: 0, Data: []byte("one")},
		{Code: 14, Data: nil},
		{Code: 14, Data: []byte("foo")},
		{Code: 14, Data: []byte("bar")},
	}

	// Nil and []byte{} should produce the same bytes
	bz0, err := trs[0].Marshal()
	require.NoError(t, err)
	bz1, err := trs[1].Marshal()
	require.NoError(t, err)
	require.Equal(t, bz0, bz1)

	// Make sure that we can get a root hash from results and verify proofs.
	rs, err := abci.TxResultsToByteSlices(trs)
	require.NoError(t, err)
	root := merkle.HashFromByteSlices(rs)
	assert.NotEmpty(t, root)

	_, proofs := merkle.ProofsFromByteSlices(rs)
	for i, tr := range trs {
		bz, err := tr.Marshal()
		require.NoError(t, err)

		valid := proofs[i].Verify(root, bz)
		assert.NoError(t, valid, "%d", i)
	}
}
