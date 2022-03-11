package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	tmrand "github.com/tendermint/tendermint/libs/rand"
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
				Tx:     Tx([]byte{1, 2, 3, 4, 5}),
			},
		}
		txrSet := NewTxRecordSet(trs)
		err := txrSet.Validate(100, []Tx{{1, 2, 3, 4, 5}})
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
}
