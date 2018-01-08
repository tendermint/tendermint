package iavl

import (
	"testing"

	"github.com/stretchr/testify/require"

	cmn "github.com/tendermint/tmlibs/common"
)

func TestSerializeProofs(t *testing.T) {
	require := require.New(t)

	tree := NewTree(0, nil)
	for _, ikey := range []byte{0x17, 0x42, 0x99} {
		key := []byte{ikey}
		tree.Set(key, cmn.RandBytes(8))
	}
	root := tree.Hash()

	// test with key exists
	key := []byte{0x17}
	val, proof, err := tree.GetWithProof(key)
	require.Nil(err, "%+v", err)
	require.NotNil(val)
	bin := proof.Bytes()
	eproof, err := ReadKeyExistsProof(bin)
	require.Nil(err, "%+v", err)
	require.NoError(eproof.Verify(key, val, root))
	_, err = ReadKeyAbsentProof(bin)
	require.NotNil(err)

	// test with key absent
	key = []byte{0x38}
	val, proof, err = tree.GetWithProof(key)
	require.Nil(err, "%+v", err)
	require.Nil(val)
	bin = proof.Bytes()
	// I think this is ugly it works this way, but without type-bytes nothing we can do :(
	// eproof, err = ReadKeyExistsProof(bin)
	// require.NotNil(err)
	aproof, err := ReadKeyAbsentProof(bin)
	require.Nil(err, "%+v", err)
	require.NoError(aproof.Verify(key, val, root))
}
