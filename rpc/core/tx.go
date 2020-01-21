package core

import (
	"fmt"

	tmmath "github.com/tendermint/tendermint/libs/math"

	tmquery "github.com/tendermint/tendermint/libs/pubsub/query"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
	"github.com/tendermint/tendermint/state/txindex/null"
	"github.com/tendermint/tendermint/types"
)

// Tx allows you to query the transaction results. `nil` could mean the
// transaction is in the mempool, invalidated, or was not sent in the first
// place.
// More: https://docs.tendermint.com/master/rpc/#/Info/tx
func Tx(ctx *rpctypes.Context, hash []byte, prove bool) (*ctypes.ResultTx, error) {
	// if index is disabled, return error
	if _, ok := txIndexer.(*null.TxIndex); ok {
		return nil, fmt.Errorf("transaction indexing is disabled")
	}

	r, err := txIndexer.Get(hash)
	if err != nil {
		return nil, err
	}

	if r == nil {
		return nil, fmt.Errorf("tx (%X) not found", hash)
	}

	height := r.Height
	index := r.Index

	var proof types.TxProof
	if prove {
		block := blockStore.LoadBlock(height)
		proof = block.Data.Txs.Proof(int(index)) // XXX: overflow on 32-bit machines
	}

	return &ctypes.ResultTx{
		Hash:     hash,
		Height:   height,
		Index:    index,
		TxResult: r.Result,
		Tx:       r.Tx,
		Proof:    proof,
	}, nil
}

// TxSearch allows you to query for multiple transactions results. It returns a
// list of transactions (maximum ?per_page entries) and the total count.
// More: https://docs.tendermint.com/master/rpc/#/Info/tx_search
func TxSearch(ctx *rpctypes.Context, query string, prove bool, page, perPage int) (*ctypes.ResultTxSearch, error) {
	// if index is disabled, return error
	if _, ok := txIndexer.(*null.TxIndex); ok {
		return nil, fmt.Errorf("transaction indexing is disabled")
	}

	q, err := tmquery.New(query)
	if err != nil {
		return nil, err
	}

	results, err := txIndexer.Search(q)
	if err != nil {
		return nil, err
	}

	totalCount := len(results)
	perPage = validatePerPage(perPage)
	page, err = validatePage(page, perPage, totalCount)
	if err != nil {
		return nil, err
	}
	skipCount := validateSkipCount(page, perPage)

	apiResults := make([]*ctypes.ResultTx, tmmath.MinInt(perPage, totalCount-skipCount))
	var proof types.TxProof
	// if there's no tx in the results array, we don't need to loop through the apiResults array
	for i := 0; i < len(apiResults); i++ {
		r := results[skipCount+i]
		height := r.Height
		index := r.Index

		if prove {
			block := blockStore.LoadBlock(height)
			proof = block.Data.Txs.Proof(int(index)) // XXX: overflow on 32-bit machines
		}

		apiResults[i] = &ctypes.ResultTx{
			Hash:     r.Tx.Hash(),
			Height:   height,
			Index:    index,
			TxResult: r.Result,
			Tx:       r.Tx,
			Proof:    proof,
		}
	}

	return &ctypes.ResultTxSearch{Txs: apiResults, TotalCount: totalCount}, nil
}
