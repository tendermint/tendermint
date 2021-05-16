package v1

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/mempool"
)

func TestTxMap_GetTxBySender(t *testing.T) {
	txm := NewTxMap()
	wtx := &WrappedTx{
		Tx:        []byte("test_tx"),
		Sender:    "foo",
		Priority:  1,
		Timestamp: time.Now(),
	}

	res := txm.GetTxBySender(wtx.Sender)
	require.Nil(t, res)

	txm.SetTx(wtx)

	res = txm.GetTxBySender(wtx.Sender)
	require.NotNil(t, res)
	require.Equal(t, wtx, res)
}

func TestTxMap_GetTxByHash(t *testing.T) {
	txm := NewTxMap()
	wtx := &WrappedTx{
		Tx:        []byte("test_tx"),
		Sender:    "foo",
		Priority:  1,
		Timestamp: time.Now(),
	}
	key := mempool.TxKey(wtx.Tx)

	res := txm.GetTxByHash(key)
	require.Nil(t, res)

	txm.SetTx(wtx)

	res = txm.GetTxByHash(key)
	require.NotNil(t, res)
	require.Equal(t, wtx, res)
}

func TestTxMap_SetTx(t *testing.T) {
	txm := NewTxMap()
	wtx := &WrappedTx{
		Tx:        []byte("test_tx"),
		Priority:  1,
		Timestamp: time.Now(),
	}
	key := mempool.TxKey(wtx.Tx)

	txm.SetTx(wtx)

	res := txm.GetTxByHash(key)
	require.NotNil(t, res)
	require.Equal(t, wtx, res)

	wtx.Sender = "foo"
	txm.SetTx(wtx)

	res = txm.GetTxByHash(key)
	require.NotNil(t, res)
	require.Equal(t, wtx, res)
}

func TestTxMap_GetOrSetPeerByTxHash(t *testing.T) {
	txm := NewTxMap()
	wtx := &WrappedTx{
		Tx:        []byte("test_tx"),
		Priority:  1,
		Timestamp: time.Now(),
	}
	key := mempool.TxKey(wtx.Tx)

	txm.SetTx(wtx)

	res, ok := txm.GetOrSetPeerByTxHash(mempool.TxKey([]byte("test_tx_2")), 15)
	require.Nil(t, res)
	require.False(t, ok)

	res, ok = txm.GetOrSetPeerByTxHash(key, 15)
	require.NotNil(t, res)
	require.False(t, ok)

	res, ok = txm.GetOrSetPeerByTxHash(key, 15)
	require.NotNil(t, res)
	require.True(t, ok)
}
