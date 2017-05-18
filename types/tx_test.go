package types

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	wire "github.com/tendermint/go-wire"
	cmn "github.com/tendermint/tmlibs/common"
	ctest "github.com/tendermint/tmlibs/test"
)

func makeTxs(cnt, size int) Txs {
	txs := make(Txs, cnt)
	for i := 0; i < cnt; i++ {
		txs[i] = cmn.RandBytes(size)
	}
	return txs
}

func randInt(low, high int) int {
	off := cmn.RandInt() % (high - low)
	return low + off
}

func TestTxIndex(t *testing.T) {
	assert := assert.New(t)
	for i := 0; i < 20; i++ {
		txs := makeTxs(15, 60)
		for j := 0; j < len(txs); j++ {
			tx := txs[j]
			idx := txs.Index(tx)
			assert.Equal(j, idx)
		}
		assert.Equal(-1, txs.Index(nil))
		assert.Equal(-1, txs.Index(Tx("foodnwkf")))
	}
}

func TestValidTxProof(t *testing.T) {
	assert := assert.New(t)
	cases := []struct {
		txs Txs
	}{
		{Txs{{1, 4, 34, 87, 163, 1}}},
		{Txs{{5, 56, 165, 2}, {4, 77}}},
		{Txs{Tx("foo"), Tx("bar"), Tx("baz")}},
		{makeTxs(20, 5)},
		{makeTxs(7, 81)},
		{makeTxs(61, 15)},
	}

	for h, tc := range cases {
		txs := tc.txs
		root := txs.Hash()
		// make sure valid proof for every tx
		for i := range txs {
			leaf := txs[i]
			leafHash := leaf.Hash()
			proof := txs.Proof(i)
			assert.Equal(i, proof.Index, "%d: %d", h, i)
			assert.Equal(len(txs), proof.Total, "%d: %d", h, i)
			assert.EqualValues(root, proof.RootHash, "%d: %d", h, i)
			assert.EqualValues(leaf, proof.Data, "%d: %d", h, i)
			assert.EqualValues(leafHash, proof.LeafHash(), "%d: %d", h, i)
			assert.Nil(proof.Validate(root), "%d: %d", h, i)
			assert.NotNil(proof.Validate([]byte("foobar")), "%d: %d", h, i)

			// read-write must also work
			var p2 TxProof
			bin := wire.BinaryBytes(proof)
			err := wire.ReadBinaryBytes(bin, &p2)
			if assert.Nil(err, "%d: %d: %+v", h, i, err) {
				assert.Nil(p2.Validate(root), "%d: %d", h, i)
			}
		}
	}
}

func TestTxProofUnchangable(t *testing.T) {
	// run the other test a bunch...
	for i := 0; i < 40; i++ {
		testTxProofUnchangable(t)
	}
}

func testTxProofUnchangable(t *testing.T) {
	assert := assert.New(t)

	// make some proof
	txs := makeTxs(randInt(2, 100), randInt(16, 128))
	root := txs.Hash()
	i := randInt(0, len(txs)-1)
	proof := txs.Proof(i)

	// make sure it is valid to start with
	assert.Nil(proof.Validate(root))
	bin := wire.BinaryBytes(proof)

	// try mutating the data and make sure nothing breaks
	for j := 0; j < 500; j++ {
		bad := ctest.MutateByteSlice(bin)
		if !bytes.Equal(bad, bin) {
			assertBadProof(t, root, bad, proof)
		}
	}
}

// this make sure the proof doesn't deserialize into something valid
func assertBadProof(t *testing.T, root []byte, bad []byte, good TxProof) {
	var proof TxProof
	err := wire.ReadBinaryBytes(bad, &proof)
	if err == nil {
		err = proof.Validate(root)
		if err == nil {
			// okay, this can happen if we have a slightly different total
			// (where the path ends up the same), if it is something else, we have
			// a real problem
			assert.NotEqual(t, proof.Total, good.Total, "bad: %#v\ngood: %#v", proof, good)
		}
	}
}
