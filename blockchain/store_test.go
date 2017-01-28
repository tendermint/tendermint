package blockchain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	abci "github.com/tendermint/abci/types"
	db "github.com/tendermint/go-db"
	"github.com/tendermint/tendermint/types"
)

func TestBlockStoreLoadSaveTxResult(t *testing.T) {
	db := db.NewMemDB()
	store := NewBlockStore(db)

	tx := types.Tx("HELLO WORLD")
	txResult := &types.TxResult{tx, 1, abci.ResponseDeliverTx{Data: []byte{0}, Code: abci.CodeType_OK, Log: ""}}

	store.SaveTxResult(tx.Hash(), txResult)
	assert.Equal(t, store.LoadTxResult(tx.Hash()), txResult)
}
