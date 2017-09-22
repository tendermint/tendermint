// Copyright 2017 Tendermint. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package core

import (
	"fmt"

	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/state/txindex/null"
	"github.com/tendermint/tendermint/types"
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

	height := int(r.Height) // XXX
	index := int(r.Index)

	var proof types.TxProof
	if prove {
		block := blockStore.LoadBlock(height)
		proof = block.Data.Txs.Proof(index)
	}

	return &ctypes.ResultTx{
		Height:   height,
		Index:    index,
		TxResult: r.Result.Result(),
		Tx:       r.Tx,
		Proof:    proof,
	}, nil
}
