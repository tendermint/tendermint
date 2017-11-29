package core

import (
	"fmt"

	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/state/txindex/null"
	"github.com/tendermint/tendermint/types"
	tmquery "github.com/tendermint/tmlibs/pubsub/query"
)

// Tx allows you to query the transaction results. `nil` could mean the
// transaction is in the mempool, invalidated, or was not sent in the first
// place.
//
// ```shell
// curl "localhost:46657/tx?hash=0x2B8EC32BA2579B3B8606E42C06DE2F7AFA2556EF"
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:46657", "/websocket")
// tx, err := client.Tx([]byte("2B8EC32BA2579B3B8606E42C06DE2F7AFA2556EF"), true)
// ```
//
// > The above command returns JSON structured like this:
//
// ```json
// {
// 	"error": "",
// 	"result": {
// 		"proof": {
// 			"Proof": {
// 				"aunts": []
// 			},
// 			"Data": "YWJjZA==",
// 			"RootHash": "2B8EC32BA2579B3B8606E42C06DE2F7AFA2556EF",
// 			"Total": 1,
// 			"Index": 0
// 		},
// 		"tx": "YWJjZA==",
// 		"tx_result": {
// 			"log": "",
// 			"data": "",
// 			"code": 0
// 		},
// 		"index": 0,
// 		"height": 52
// 	},
// 	"id": "",
// 	"jsonrpc": "2.0"
// }
// ```
//
// Returns a transaction matching the given transaction hash.
//
// ### Query Parameters
//
// | Parameter | Type   | Default | Required | Description                                               |
// |-----------+--------+---------+----------+-----------------------------------------------------------|
// | hash      | []byte | nil     | true     | The transaction hash                                      |
// | prove     | bool   | false   | false    | Include a proof of the transaction inclusion in the block |
//
// ### Returns
//
// - `proof`: the `types.TxProof` object
// - `tx`: `[]byte` - the transaction
// - `tx_result`: the `abci.Result` object
// - `index`: `int` - index of the transaction
// - `height`: `int` - height of the block where this transaction was in
func Tx(hash []byte, prove bool) (*ctypes.ResultTx, error) {

	// if index is disabled, return error
	if _, ok := txIndexer.(*null.TxIndex); ok {
		return nil, fmt.Errorf("Transaction indexing is disabled.")
	}

	r, err := txIndexer.Get(hash)
	if err != nil {
		return nil, err
	}

	if r == nil {
		return nil, fmt.Errorf("Tx (%X) not found", hash)
	}

	height := r.Height
	index := r.Index

	var proof types.TxProof
	if prove {
		// TODO: handle overflow
		block := blockStore.LoadBlock(int(height))
		proof = block.Data.Txs.Proof(int(index))
	}

	return &ctypes.ResultTx{
		Height:   height,
		Index:    index,
		TxResult: r.Result,
		Tx:       r.Tx,
		Proof:    proof,
	}, nil
}

func TxSearch(query string, prove bool) ([]*ctypes.ResultTx, error) {
	// if index is disabled, return error
	if _, ok := txIndexer.(*null.TxIndex); ok {
		return nil, fmt.Errorf("Transaction indexing is disabled.")
	}

	q, err := tmquery.New(query)
	if err != nil {
		return nil, err
	}

	results, err := txIndexer.Search(q)
	if err != nil {
		return nil, err
	}

	// TODO: we may want to consider putting a maximum on this length and somehow
	// informing the user that things were truncated.
	apiResults := make([]*ctypes.ResultTx, len(results))
	var proof types.TxProof
	for i, r := range results {
		height := r.Height
		index := r.Index

		if prove {
			// TODO: handle overflow
			block := blockStore.LoadBlock(int(height))
			proof = block.Data.Txs.Proof(int(index))
		}

		apiResults[i] = &ctypes.ResultTx{
			Height:   height,
			Index:    index,
			TxResult: r.Result,
			Tx:       r.Tx,
			Proof:    proof,
		}
	}

	return apiResults, nil
}
