package v1

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/mempool"
)

func Testtxsap_GetTxBySender(t *testing.T) {
	txs := NewTxStore()
	wtx := &WrappedTx{
		Tx:        []byte("test_tx"),
		Sender:    "foo",
		Priority:  1,
		Timestamp: time.Now(),
	}

	res := txs.GetTxBySender(wtx.Sender)
	require.Nil(t, res)

	txs.SetTx(wtx)

	res = txs.GetTxBySender(wtx.Sender)
	require.NotNil(t, res)
	require.Equal(t, wtx, res)
}

func Testtxsap_GetTxByHash(t *testing.T) {
	txs := NewTxStore()
	wtx := &WrappedTx{
		Tx:        []byte("test_tx"),
		Sender:    "foo",
		Priority:  1,
		Timestamp: time.Now(),
	}
	key := mempool.TxKey(wtx.Tx)

	res := txs.GetTxByHash(key)
	require.Nil(t, res)

	txs.SetTx(wtx)

	res = txs.GetTxByHash(key)
	require.NotNil(t, res)
	require.Equal(t, wtx, res)
}

func Testtxsap_SetTx(t *testing.T) {
	txs := NewTxStore()
	wtx := &WrappedTx{
		Tx:        []byte("test_tx"),
		Priority:  1,
		Timestamp: time.Now(),
	}
	key := mempool.TxKey(wtx.Tx)

	txs.SetTx(wtx)

	res := txs.GetTxByHash(key)
	require.NotNil(t, res)
	require.Equal(t, wtx, res)

	wtx.Sender = "foo"
	txs.SetTx(wtx)

	res = txs.GetTxByHash(key)
	require.NotNil(t, res)
	require.Equal(t, wtx, res)
}

func Testtxsap_GetOrSetPeerByTxHash(t *testing.T) {
	txs := NewTxStore()
	wtx := &WrappedTx{
		Tx:        []byte("test_tx"),
		Priority:  1,
		Timestamp: time.Now(),
	}
	key := mempool.TxKey(wtx.Tx)

	txs.SetTx(wtx)

	res, ok := txs.GetOrSetPeerByTxHash(mempool.TxKey([]byte("test_tx_2")), 15)
	require.Nil(t, res)
	require.False(t, ok)

	res, ok = txs.GetOrSetPeerByTxHash(key, 15)
	require.NotNil(t, res)
	require.False(t, ok)

	res, ok = txs.GetOrSetPeerByTxHash(key, 15)
	require.NotNil(t, res)
	require.True(t, ok)
}
