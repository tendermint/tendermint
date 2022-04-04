package types

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	ctest "github.com/tendermint/tendermint/internal/libs/test"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

func makeTxs(cnt, size int) Txs {
	txs := make(Txs, cnt)
	for i := 0; i < cnt; i++ {
		txs[i] = tmrand.Bytes(size)
	}
	return txs
}

func TestTxIndex(t *testing.T) {
	for i := 0; i < 20; i++ {
		txs := makeTxs(15, 60)
		for j := 0; j < len(txs); j++ {
			tx := txs[j]
			idx := txs.Index(tx)
			assert.Equal(t, j, idx)
		}
		assert.Equal(t, -1, txs.Index(nil))
		assert.Equal(t, -1, txs.Index(Tx("foodnwkf")))
	}
}

func TestTxIndexByHash(t *testing.T) {
	for i := 0; i < 20; i++ {
		txs := makeTxs(15, 60)
		for j := 0; j < len(txs); j++ {
			tx := txs[j]
			idx := txs.IndexByHash(tx.Hash())
			assert.Equal(t, j, idx)
		}
		assert.Equal(t, -1, txs.IndexByHash(nil))
		assert.Equal(t, -1, txs.IndexByHash(Tx("foodnwkf").Hash()))
	}
}

func TestValidateTxRecordSet(t *testing.T) {
	t.Run("should error on total transaction size exceeding max data size", func(t *testing.T) {
		trs := []*abci.TxRecord{
			{
				Action: abci.TxRecord_ADDED,
				Tx:     Tx([]byte{1, 2, 3, 4, 5}),
			},
			{
				Action: abci.TxRecord_ADDED,
				Tx:     Tx([]byte{6, 7, 8, 9, 10}),
			},
		}
		txrSet := NewTxRecordSet(trs)
		err := txrSet.Validate(9, []Tx{})
		require.Error(t, err)
	})
	t.Run("should not error on removed transaction size exceeding max data size", func(t *testing.T) {
		trs := []*abci.TxRecord{
			{
				Action: abci.TxRecord_ADDED,
				Tx:     Tx([]byte{1, 2, 3, 4, 5}),
			},
			{
				Action: abci.TxRecord_ADDED,
				Tx:     Tx([]byte{6, 7, 8, 9}),
			},
			{
				Action: abci.TxRecord_REMOVED,
				Tx:     Tx([]byte{10}),
			},
		}
		txrSet := NewTxRecordSet(trs)
		err := txrSet.Validate(9, []Tx{[]byte{10}})
		require.NoError(t, err)
	})
	t.Run("should error on duplicate transactions with the same action", func(t *testing.T) {
		trs := []*abci.TxRecord{
			{
				Action: abci.TxRecord_ADDED,
				Tx:     Tx([]byte{1, 2, 3, 4, 5}),
			},
			{
				Action: abci.TxRecord_ADDED,
				Tx:     Tx([]byte{100}),
			},
			{
				Action: abci.TxRecord_ADDED,
				Tx:     Tx([]byte{1, 2, 3, 4, 5}),
			},
			{
				Action: abci.TxRecord_ADDED,
				Tx:     Tx([]byte{200}),
			},
		}
		txrSet := NewTxRecordSet(trs)
		err := txrSet.Validate(100, []Tx{})
		require.Error(t, err)
	})
	t.Run("should error on duplicate transactions with mixed actions", func(t *testing.T) {
		trs := []*abci.TxRecord{
			{
				Action: abci.TxRecord_ADDED,
				Tx:     Tx([]byte{1, 2, 3, 4, 5}),
			},
			{
				Action: abci.TxRecord_ADDED,
				Tx:     Tx([]byte{100}),
			},
			{
				Action: abci.TxRecord_REMOVED,
				Tx:     Tx([]byte{1, 2, 3, 4, 5}),
			},
			{
				Action: abci.TxRecord_ADDED,
				Tx:     Tx([]byte{200}),
			},
		}
		txrSet := NewTxRecordSet(trs)
		err := txrSet.Validate(100, []Tx{})
		require.Error(t, err)
	})
	t.Run("should error on new transactions marked UNMODIFIED", func(t *testing.T) {
		trs := []*abci.TxRecord{
			{
				Action: abci.TxRecord_UNMODIFIED,
				Tx:     Tx([]byte{1, 2, 3, 4, 5}),
			},
		}
		txrSet := NewTxRecordSet(trs)
		err := txrSet.Validate(100, []Tx{})
		require.Error(t, err)
	})
	t.Run("should error on new transactions marked REMOVED", func(t *testing.T) {
		trs := []*abci.TxRecord{
			{
				Action: abci.TxRecord_REMOVED,
				Tx:     Tx([]byte{1, 2, 3, 4, 5}),
			},
		}
		txrSet := NewTxRecordSet(trs)
		err := txrSet.Validate(100, []Tx{})
		require.Error(t, err)
	})
	t.Run("should error on existing transaction marked as ADDED", func(t *testing.T) {
		trs := []*abci.TxRecord{
			{
				Action: abci.TxRecord_ADDED,
				Tx:     Tx([]byte{5, 4, 3, 2, 1}),
			},
			{
				Action: abci.TxRecord_ADDED,
				Tx:     Tx([]byte{6}),
			},
			{
				Action: abci.TxRecord_ADDED,
				Tx:     Tx([]byte{1, 2, 3, 4, 5}),
			},
		}
		txrSet := NewTxRecordSet(trs)
		err := txrSet.Validate(100, []Tx{{0}, {1, 2, 3, 4, 5}})
		require.Error(t, err)
	})
	t.Run("should error if any transaction marked as UNKNOWN", func(t *testing.T) {
		trs := []*abci.TxRecord{
			{
				Action: abci.TxRecord_UNKNOWN,
				Tx:     Tx([]byte{1, 2, 3, 4, 5}),
			},
		}
		txrSet := NewTxRecordSet(trs)
		err := txrSet.Validate(100, []Tx{})
		require.Error(t, err)
	})
	t.Run("TxRecordSet preserves order", func(t *testing.T) {
		trs := []*abci.TxRecord{
			{
				Action: abci.TxRecord_ADDED,
				Tx:     Tx([]byte{100}),
			},
			{
				Action: abci.TxRecord_ADDED,
				Tx:     Tx([]byte{99}),
			},
			{
				Action: abci.TxRecord_ADDED,
				Tx:     Tx([]byte{55}),
			},
			{
				Action: abci.TxRecord_ADDED,
				Tx:     Tx([]byte{12}),
			},
			{
				Action: abci.TxRecord_ADDED,
				Tx:     Tx([]byte{66}),
			},
			{
				Action: abci.TxRecord_ADDED,
				Tx:     Tx([]byte{9}),
			},
			{
				Action: abci.TxRecord_ADDED,
				Tx:     Tx([]byte{17}),
			},
		}
		txrSet := NewTxRecordSet(trs)
		err := txrSet.Validate(100, []Tx{})
		require.NoError(t, err)
		for i, tx := range txrSet.IncludedTxs() {
			require.Equal(t, Tx(trs[i].Tx), tx)
		}
	})
}

func TestValidTxProof(t *testing.T) {
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
			tx := []byte(txs[i])
			proof := txs.Proof(i)
			assert.EqualValues(t, i, proof.Proof.Index, "%d: %d", h, i)
			assert.EqualValues(t, len(txs), proof.Proof.Total, "%d: %d", h, i)
			assert.EqualValues(t, root, proof.RootHash, "%d: %d", h, i)
			assert.EqualValues(t, tx, proof.Data, "%d: %d", h, i)
			assert.EqualValues(t, txs[i].Hash(), proof.Leaf(), "%d: %d", h, i)
			assert.Nil(t, proof.Validate(root), "%d: %d", h, i)
			assert.NotNil(t, proof.Validate([]byte("foobar")), "%d: %d", h, i)

			// read-write must also work
			var (
				p2  TxProof
				pb2 tmproto.TxProof
			)
			pbProof := proof.ToProto()
			bin, err := pbProof.Marshal()
			require.NoError(t, err)

			err = pb2.Unmarshal(bin)
			require.NoError(t, err)

			p2, err = TxProofFromProto(pb2)
			if assert.NoError(t, err, "%d: %d: %+v", h, i, err) {
				assert.Nil(t, p2.Validate(root), "%d: %d", h, i)
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
	// make some proof
	txs := makeTxs(randInt(2, 100), randInt(16, 128))
	root := txs.Hash()
	i := randInt(0, len(txs)-1)
	proof := txs.Proof(i)

	// make sure it is valid to start with
	assert.Nil(t, proof.Validate(root))
	pbProof := proof.ToProto()
	bin, err := pbProof.Marshal()
	require.NoError(t, err)

	// try mutating the data and make sure nothing breaks
	for j := 0; j < 500; j++ {
		bad := ctest.MutateByteSlice(bin)
		if !bytes.Equal(bad, bin) {
			assertBadProof(t, root, bad, proof)
		}
	}
}

// This makes sure that the proof doesn't deserialize into something valid.
func assertBadProof(t *testing.T, root []byte, bad []byte, good TxProof) {

	var (
		proof   TxProof
		pbProof tmproto.TxProof
	)
	err := pbProof.Unmarshal(bad)
	if err == nil {
		proof, err = TxProofFromProto(pbProof)
		if err == nil {
			err = proof.Validate(root)
			if err == nil {
				// XXX Fix simple merkle proofs so the following is *not* OK.
				// This can happen if we have a slightly different total (where the
				// path ends up the same). If it is something else, we have a real
				// problem.
				assert.NotEqual(t, proof.Proof.Total, good.Proof.Total, "bad: %#v\ngood: %#v", proof, good)
			}
		}
	}
}

func randInt(low, high int) int {
	return rand.Intn(high-low) + low
}
