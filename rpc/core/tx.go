package core

import (
	"fmt"

	cmn "github.com/tendermint/tendermint/libs/common"

	tmquery "github.com/tendermint/tendermint/libs/pubsub/query"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
	"github.com/tendermint/tendermint/state/txindex/null"
	"github.com/tendermint/tendermint/types"
	sm "github.com/tendermint/tendermint/state"
	abci"github.com/tendermint/abci/types"

)

// Tx allows you to query the transaction results. `nil` could mean the
// transaction is in the mempool, invalidated, or was not sent in the first
// place.
//
// ```shell
// curl "localhost:26657/tx?hash=0xF87370F68C82D9AC7201248ECA48CEC5F16FFEC99C461C1B2961341A2FE9C1C8"
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:26657", "/websocket")
// err := client.Start()
// if err != nil {
//   // handle error
// }
// defer client.Stop()
// hashBytes, err := hex.DecodeString("F87370F68C82D9AC7201248ECA48CEC5F16FFEC99C461C1B2961341A2FE9C1C8")
// tx, err := client.Tx(hashBytes, true)
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
// 			"Total": "1",
// 			"Index": "0"
// 		},
// 		"tx": "YWJjZA==",
// 		"tx_result": {
// 			"log": "",
// 			"data": "",
// 			"code": "0"
// 		},
// 		"index": "0",
// 		"height": "52",
//		"hash": "2B8EC32BA2579B3B8606E42C06DE2F7AFA2556EF"
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
// - `hash`: `[]byte` - hash of the transaction

// statecode 1 under checkTxing,2 finished checkTx but don't do deliverTx
// 3/5 finished deliverTx

func Tx(ctx *rpctypes.Context, hash cmn.HexBytes, prove bool) (*ctypes.ResultTx, error) {

	var stateCode uint32
	var height int64
	check := true
	dResult, err := sm.LoadABCITxResponses(stateDB,cmn.HexBytes(hash))

	if err == nil {
		height = dResult.Height
	}
	var checkResult abci.ResponseCheckTx
	checkRes,errCheck := mempool.TxSearch(hash)
	if errCheck == nil {
		if checkRes != nil {
			checkResult = *checkRes
			height = checkResult.Height
		}
	} else {
		check = false
	}

	if dResult.Height == 0 {
		if checkRes == nil {
			if !check {
				return nil,errCheck //can't find tx maybe checkTx failed and resCache timeout
			} else {
				stateCode = 1 //checking return ResponseCheckTx is null
				checkResult = abci.ResponseCheckTx{}
			}
		} else {
			stateCode = 2  //finished checkTx just now return ResponseCheckTx
		}
	} else {
		if checkRes == nil {
			if !check {
				stateCode = 3 // deliverTx successfully return ResponseDeliverTx but don't return ResponseCheckTx 'cause rescache timeout
				checkResult = abci.ResponseCheckTx{}
			} else {
				stateCode = 4 //it should be a bug,lol
			}
		} else {
			stateCode = 5  //both checkTx and deliverTx successfully and return ResponseDeliverTx and ResponseCheckTx
		}

	}

	return &ctypes.ResultTx{
		Hash:     cmn.HexBytes(hash),
		Height:   height,
		//Index:    uint32(index),
		DeliverResult: dResult,
		CheckResult: checkResult,
		StateCode:       stateCode,
	}, nil
}

// TxSearch allows you to query for multiple transactions results. It returns a
// list of transactions (maximum ?per_page entries) and the total count.
//
// ```shell
// curl "localhost:26657/tx_search?query=\"account.owner='Ivan'\"&prove=true"
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:26657", "/websocket")
// err := client.Start()
// if err != nil {
//   // handle error
// }
// defer client.Stop()
// q, err := tmquery.New("account.owner='Ivan'")
// tx, err := client.TxSearch(q, true)
// ```
//
// > The above command returns JSON structured like this:
//
// ```json
// {
//   "jsonrpc": "2.0",
//   "id": "",
//   "result": {
// 	   "txs": [
//       {
//         "proof": {
//           "Proof": {
//             "aunts": [
//               "J3LHbizt806uKnABNLwG4l7gXCA=",
//               "iblMO/M1TnNtlAefJyNCeVhjAb0=",
//               "iVk3ryurVaEEhdeS0ohAJZ3wtB8=",
//               "5hqMkTeGqpct51ohX0lZLIdsn7Q=",
//               "afhsNxFnLlZgFDoyPpdQSe0bR8g="
//             ]
//           },
//           "Data": "mvZHHa7HhZ4aRT0xMDA=",
//           "RootHash": "F6541223AA46E428CB1070E9840D2C3DF3B6D776",
//           "Total": "32",
//           "Index": "31"
//         },
//         "tx": "mvZHHa7HhZ4aRT0xMDA=",
//         "tx_result": {},
//         "index": "31",
//         "height": "12",
//         "hash": "2B8EC32BA2579B3B8606E42C06DE2F7AFA2556EF"
//       }
//     ],
//     "total_count": "1"
//   }
// }
// ```
//
// ### Query Parameters
//
// | Parameter | Type   | Default | Required | Description                                               |
// |-----------+--------+---------+----------+-----------------------------------------------------------|
// | query     | string | ""      | true     | Query                                                     |
// | prove     | bool   | false   | false    | Include proofs of the transactions inclusion in the block |
// | page      | int    | 1       | false    | Page number (1-based)                                     |
// | per_page  | int    | 30      | false    | Number of entries per page (max: 100)                     |
//
// ### Returns
//
// - `proof`: the `types.TxProof` object
// - `tx`: `[]byte` - the transaction
// - `tx_result`: the `abci.Result` object
// - `index`: `int` - index of the transaction
// - `height`: `int` - height of the block where this transaction was in
// - `hash`: `[]byte` - hash of the transaction
func TxSearch(ctx *rpctypes.Context, query string, prove bool, page, perPage int) (*ctypes.ResultTxSearch, error) {
	// if index is disabled, return error
	if _, ok := txIndexer.(*null.TxIndex); ok {
		return nil, fmt.Errorf("Transaction indexing is disabled")
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
	page = validatePage(page, perPage, totalCount)
	skipCount := validateSkipCount(page, perPage)

	apiResults := make([]*ctypes.ResultTx, cmn.MinInt(perPage, totalCount-skipCount))
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
