package mempool

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/types"
)

func TestTxStore_GetTxBySender(t *testing.T) {
	txs := NewTxStore()
	wtx := &WrappedTx{
		tx:        []byte("test_tx"),
		sender:    "foo",
		priority:  1,
		timestamp: time.Now(),
	}

	res := txs.GetTxBySender(wtx.sender)
	require.Nil(t, res)

	txs.SetTx(wtx)

	res = txs.GetTxBySender(wtx.sender)
	require.NotNil(t, res)
	require.Equal(t, wtx, res)
}

func TestTxStore_GetTxByHash(t *testing.T) {
	txs := NewTxStore()
	wtx := &WrappedTx{
		tx:        []byte("test_tx"),
		sender:    "foo",
		priority:  1,
		timestamp: time.Now(),
	}

	key := wtx.tx.Key()
	res := txs.GetTxByHash(key)
	require.Nil(t, res)

	txs.SetTx(wtx)

	res = txs.GetTxByHash(key)
	require.NotNil(t, res)
	require.Equal(t, wtx, res)
}

func TestTxStore_SetTx(t *testing.T) {
	txs := NewTxStore()
	wtx := &WrappedTx{
		tx:        []byte("test_tx"),
		priority:  1,
		timestamp: time.Now(),
	}

	key := wtx.tx.Key()
	txs.SetTx(wtx)

	res := txs.GetTxByHash(key)
	require.NotNil(t, res)
	require.Equal(t, wtx, res)

	wtx.sender = "foo"
	txs.SetTx(wtx)

	res = txs.GetTxByHash(key)
	require.NotNil(t, res)
	require.Equal(t, wtx, res)
}

func TestTxStore_GetOrSetPeerByTxHash(t *testing.T) {
	txs := NewTxStore()
	wtx := &WrappedTx{
		tx:        []byte("test_tx"),
		priority:  1,
		timestamp: time.Now(),
	}

	key := wtx.tx.Key()
	txs.SetTx(wtx)

	res, ok := txs.GetOrSetPeerByTxHash(types.Tx([]byte("test_tx_2")).Key(), 15)
	require.Nil(t, res)
	require.False(t, ok)

	res, ok = txs.GetOrSetPeerByTxHash(key, 15)
	require.NotNil(t, res)
	require.False(t, ok)

	res, ok = txs.GetOrSetPeerByTxHash(key, 15)
	require.NotNil(t, res)
	require.True(t, ok)

	require.True(t, txs.TxHasPeer(key, 15))
	require.False(t, txs.TxHasPeer(key, 16))
}

func TestTxStore_RemoveTx(t *testing.T) {
	txs := NewTxStore()
	wtx := &WrappedTx{
		tx:        []byte("test_tx"),
		priority:  1,
		timestamp: time.Now(),
	}

	txs.SetTx(wtx)

	key := wtx.tx.Key()
	res := txs.GetTxByHash(key)
	require.NotNil(t, res)

	txs.RemoveTx(res)

	res = txs.GetTxByHash(key)
	require.Nil(t, res)
}

func TestTxStore_Size(t *testing.T) {
	txStore := NewTxStore()
	numTxs := 1000

	for i := 0; i < numTxs; i++ {
		txStore.SetTx(&WrappedTx{
			tx:        []byte(fmt.Sprintf("test_tx_%d", i)),
			priority:  int64(i),
			timestamp: time.Now(),
		})
	}

	require.Equal(t, numTxs, txStore.Size())
}

func TestWrappedTxList_Reset(t *testing.T) {
	list := NewWrappedTxList(func(wtx1, wtx2 *WrappedTx) bool {
		return wtx1.height >= wtx2.height
	})

	require.Zero(t, list.Size())

	for i := 0; i < 100; i++ {
		list.Insert(&WrappedTx{height: int64(i)})
	}

	require.Equal(t, 100, list.Size())

	list.Reset()
	require.Zero(t, list.Size())
}

func TestWrappedTxList_Insert(t *testing.T) {
	list := NewWrappedTxList(func(wtx1, wtx2 *WrappedTx) bool {
		return wtx1.height >= wtx2.height
	})

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	var expected []int
	for i := 0; i < 100; i++ {
		height := rng.Int63n(10000)
		expected = append(expected, int(height))
		list.Insert(&WrappedTx{height: height})

		if i%10 == 0 {
			list.Insert(&WrappedTx{height: height})
			expected = append(expected, int(height))
		}
	}

	got := make([]int, list.Size())
	for i, wtx := range list.txs {
		got[i] = int(wtx.height)
	}

	sort.Ints(expected)
	require.Equal(t, expected, got)
}

func TestWrappedTxList_Remove(t *testing.T) {
	list := NewWrappedTxList(func(wtx1, wtx2 *WrappedTx) bool {
		return wtx1.height >= wtx2.height
	})

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	var txs []*WrappedTx
	for i := 0; i < 100; i++ {
		height := rng.Int63n(10000)
		tx := &WrappedTx{height: height}

		txs = append(txs, tx)
		list.Insert(tx)

		if i%10 == 0 {
			tx = &WrappedTx{height: height}
			list.Insert(tx)
			txs = append(txs, tx)
		}
	}

	// remove a tx that does not exist
	list.Remove(&WrappedTx{height: 20000})

	// remove a tx that exists (by height) but not referenced
	list.Remove(&WrappedTx{height: txs[0].height})

	// remove a few existing txs
	for i := 0; i < 25; i++ {
		j := rng.Intn(len(txs))
		list.Remove(txs[j])
		txs = append(txs[:j], txs[j+1:]...)
	}

	expected := make([]int, len(txs))
	for i, tx := range txs {
		expected[i] = int(tx.height)
	}

	got := make([]int, list.Size())
	for i, wtx := range list.txs {
		got[i] = int(wtx.height)
	}

	sort.Ints(expected)
	require.Equal(t, expected, got)
}
