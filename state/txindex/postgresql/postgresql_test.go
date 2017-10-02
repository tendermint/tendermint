package postgresql

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/abci/types"
	"github.com/tendermint/tendermint/state/txindex"
	"github.com/tendermint/tendermint/types"
)

func TestTxIndex(t *testing.T) {
	db, err := sql.Open("postgres", "user=postgres password=mysecretpassword dbname=tendermint sslmode=disable")
	require.NoError(t, err)
	err = db.Ping()
	if err != nil {
		t.Skip(fmt.Sprintf("this test requires PG database. got error: %v", err))
	}

	indexer := &TxIndex{db: db}

	tx := types.Tx("HELLO WORLD")
	txResult := &types.TxResult{1, 0, tx, abci.ResponseDeliverTx{Data: []byte{0}, Code: abci.CodeType_OK, Log: ""}}
	hash := tx.Hash()

	batch := txindex.NewBatch(1)
	batch.Add(*txResult)
	err = indexer.AddBatch(batch)
	require.NoError(t, err)

	loadedTxResult, err := indexer.Get(hash)
	require.NoError(t, err)
	assert.Equal(t, txResult, loadedTxResult)
}
