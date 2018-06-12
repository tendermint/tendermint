package core

import (
	"fmt"

	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tmlibs/common"
)

// Get block headers for minHeight <= height <= maxHeight.
// Block headers are returned in descending order (highest first).
//
// ```shell
// curl 'localhost:26657/blockchain?minHeight=10&maxHeight=10'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:26657", "/websocket")
// info, err := client.BlockchainInfo(10, 10)
// ```
//
// > The above command returns JSON structured like this:
//
// ```json
// {
// 	"error": "",
// 	"result": {
// 		"block_metas": [
// 			{
// 				"header": {
// 					"app_hash": "",
// 					"chain_id": "test-chain-6UTNIN",
// 					"height": 10,
// 					"time": "2017-05-29T15:05:53.877Z",
// 					"num_txs": 0,
// 					"last_block_id": {
// 						"parts": {
// 							"hash": "3C78F00658E06744A88F24FF97A0A5011139F34A",
// 							"total": 1
// 						},
// 						"hash": "F70588DAB36BDA5A953D548A16F7D48C6C2DFD78"
// 					},
// 					"last_commit_hash": "F31CC4282E50B3F2A58D763D233D76F26D26CABE",
// 					"data_hash": "",
// 					"validators_hash": "9365FC80F234C967BD233F5A3E2AB2F1E4B0E5AA"
// 				},
// 				"block_id": {
// 					"parts": {
// 						"hash": "277A4DBEF91483A18B85F2F5677ABF9694DFA40F",
// 						"total": 1
// 					},
// 					"hash": "96B1D2F2D201BA4BC383EB8224139DB1294944E5"
// 				}
// 			}
// 		],
// 		"last_height": 5493
// 	},
// 	"id": "",
// 	"jsonrpc": "2.0"
// }
// ```
//
// <aside class="notice">Returns at most 20 items.</aside>
func BlockchainInfo(minHeight, maxHeight int64) (*ctypes.ResultBlockchainInfo, error) {
	if minHeight == 0 {
		minHeight = 1
	}

	if maxHeight == 0 {
		maxHeight = blockStore.Height()
	} else {
		maxHeight = cmn.MinInt64(blockStore.Height(), maxHeight)
	}

	// maximum 20 block metas
	const limit int64 = 20
	minHeight = cmn.MaxInt64(minHeight, maxHeight-limit)

	logger.Debug("BlockchainInfoHandler", "maxHeight", maxHeight, "minHeight", minHeight)

	if minHeight > maxHeight {
		return nil, fmt.Errorf("min height %d can't be greater than max height %d", minHeight, maxHeight)
	}

	blockMetas := []*types.BlockMeta{}
	for height := maxHeight; height >= minHeight; height-- {
		blockMeta := blockStore.LoadBlockMeta(height)
		blockMetas = append(blockMetas, blockMeta)
	}

	return &ctypes.ResultBlockchainInfo{blockStore.Height(), blockMetas}, nil
}

// Get block at a given height.
// If no height is provided, it will fetch the latest block.
//
// ```shell
// curl 'localhost:26657/block?height=10'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:26657", "/websocket")
// info, err := client.Block(10)
// ```
//
// > The above command returns JSON structured like this:
//
// ```json
// {
//   "error": "",
//   "result": {
//     "block": {
//       "last_commit": {
//         "precommits": [
//           {
//             "signature": {
//               "data": "12C0D8893B8A38224488DC1DE6270DF76BB1A5E9DB1C68577706A6A97C6EC34FFD12339183D5CA8BC2F46148773823DE905B7F6F5862FD564038BB7AE03BF50D",
//               "type": "ed25519"
//             },
//             "block_id": {
//               "parts": {
//                 "hash": "3C78F00658E06744A88F24FF97A0A5011139F34A",
//                 "total": 1
//               },
//               "hash": "F70588DAB36BDA5A953D548A16F7D48C6C2DFD78"
//             },
//             "type": 2,
//             "round": 0,
//             "height": 9,
//             "validator_index": 0,
//             "validator_address": "E89A51D60F68385E09E716D353373B11F8FACD62"
//           }
//         ],
//         "blockID": {
//           "parts": {
//             "hash": "3C78F00658E06744A88F24FF97A0A5011139F34A",
//             "total": 1
//           },
//           "hash": "F70588DAB36BDA5A953D548A16F7D48C6C2DFD78"
//         }
//       },
//       "data": {
//         "txs": []
//       },
//       "header": {
//         "app_hash": "",
//         "chain_id": "test-chain-6UTNIN",
//         "height": 10,
//         "time": "2017-05-29T15:05:53.877Z",
//         "num_txs": 0,
//         "last_block_id": {
//           "parts": {
//             "hash": "3C78F00658E06744A88F24FF97A0A5011139F34A",
//             "total": 1
//           },
//           "hash": "F70588DAB36BDA5A953D548A16F7D48C6C2DFD78"
//         },
//         "last_commit_hash": "F31CC4282E50B3F2A58D763D233D76F26D26CABE",
//         "data_hash": "",
//         "validators_hash": "9365FC80F234C967BD233F5A3E2AB2F1E4B0E5AA"
//       }
//     },
//     "block_meta": {
//       "header": {
//         "app_hash": "",
//         "chain_id": "test-chain-6UTNIN",
//         "height": 10,
//         "time": "2017-05-29T15:05:53.877Z",
//         "num_txs": 0,
//         "last_block_id": {
//           "parts": {
//             "hash": "3C78F00658E06744A88F24FF97A0A5011139F34A",
//             "total": 1
//           },
//           "hash": "F70588DAB36BDA5A953D548A16F7D48C6C2DFD78"
//         },
//         "last_commit_hash": "F31CC4282E50B3F2A58D763D233D76F26D26CABE",
//         "data_hash": "",
//         "validators_hash": "9365FC80F234C967BD233F5A3E2AB2F1E4B0E5AA"
//       },
//       "block_id": {
//         "parts": {
//           "hash": "277A4DBEF91483A18B85F2F5677ABF9694DFA40F",
//           "total": 1
//         },
//         "hash": "96B1D2F2D201BA4BC383EB8224139DB1294944E5"
//       }
//     }
//   },
//   "id": "",
//   "jsonrpc": "2.0"
// }
// ```
func Block(heightPtr *int64) (*ctypes.ResultBlock, error) {
	storeHeight := blockStore.Height()
	height, err := getHeight(storeHeight, heightPtr)
	if err != nil {
		return nil, err
	}

	blockMeta := blockStore.LoadBlockMeta(height)
	block := blockStore.LoadBlock(height)
	return &ctypes.ResultBlock{blockMeta, block}, nil
}

// Get block commit at a given height.
// If no height is provided, it will fetch the commit for the latest block.
//
// ```shell
// curl 'localhost:26657/commit?height=11'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:26657", "/websocket")
// info, err := client.Commit(11)
// ```
//
// > The above command returns JSON structured like this:
//
// ```json
// {
//   "error": "",
//   "result": {
//     "canonical": true,
//     "commit": {
//       "precommits": [
//         {
//           "signature": {
//             "data": "00970429FEC652E9E21D106A90AE8C5413759A7488775CEF4A3F44DC46C7F9D941070E4FBE9ED54DF247FA3983359A0C3A238D61DE55C75C9116D72ABC9CF50F",
//             "type": "ed25519"
//           },
//           "block_id": {
//             "parts": {
//               "hash": "9E37CBF266BC044A779E09D81C456E653B89E006",
//               "total": 1
//             },
//             "hash": "CC6E861E31CA4334E9888381B4A9137D1458AB6A"
//           },
//           "type": 2,
//           "round": 0,
//           "height": 11,
//           "validator_index": 0,
//           "validator_address": "E89A51D60F68385E09E716D353373B11F8FACD62"
//         }
//       ],
//       "blockID": {
//         "parts": {
//           "hash": "9E37CBF266BC044A779E09D81C456E653B89E006",
//           "total": 1
//         },
//         "hash": "CC6E861E31CA4334E9888381B4A9137D1458AB6A"
//       }
//     },
//     "header": {
//       "app_hash": "",
//       "chain_id": "test-chain-6UTNIN",
//       "height": 11,
//       "time": "2017-05-29T15:05:54.893Z",
//       "num_txs": 0,
//       "last_block_id": {
//         "parts": {
//           "hash": "277A4DBEF91483A18B85F2F5677ABF9694DFA40F",
//           "total": 1
//         },
//         "hash": "96B1D2F2D201BA4BC383EB8224139DB1294944E5"
//       },
//       "last_commit_hash": "3CE0C9727CE524BA9CB7C91E28F08E2B94001087",
//       "data_hash": "",
//       "validators_hash": "9365FC80F234C967BD233F5A3E2AB2F1E4B0E5AA"
//     }
//   },
//   "id": "",
//   "jsonrpc": "2.0"
// }
// ```
func Commit(heightPtr *int64) (*ctypes.ResultCommit, error) {
	storeHeight := blockStore.Height()
	height, err := getHeight(storeHeight, heightPtr)
	if err != nil {
		return nil, err
	}

	header := blockStore.LoadBlockMeta(height).Header

	// If the next block has not been committed yet,
	// use a non-canonical commit
	if height == storeHeight {
		commit := blockStore.LoadSeenCommit(height)
		return ctypes.NewResultCommit(header, commit, false), nil
	}

	// Return the canonical commit (comes from the block at height+1)
	commit := blockStore.LoadBlockCommit(height)
	return ctypes.NewResultCommit(header, commit, true), nil
}

// BlockResults gets ABCIResults at a given height.
// If no height is provided, it will fetch results for the latest block.
//
// Results are for the height of the block containing the txs.
// Thus response.results[5] is the results of executing getBlock(h).Txs[5]
//
// ```shell
// curl 'localhost:26657/block_results?height=10'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:26657", "/websocket")
// info, err := client.BlockResults(10)
// ```
//
//
// > The above command returns JSON structured like this:
//
// ```json
// {
//  "height": 10,
//  "results": [
//   {
//    "code": 0,
//    "data": "CAFE00F00D"
//   },
//   {
//    "code": 102,
//    "data": ""
//   }
//  ]
// }
// ```
func BlockResults(heightPtr *int64) (*ctypes.ResultBlockResults, error) {
	storeHeight := blockStore.Height()
	height, err := getHeight(storeHeight, heightPtr)
	if err != nil {
		return nil, err
	}

	// load the results
	results, err := sm.LoadABCIResponses(stateDB, height)
	if err != nil {
		return nil, err
	}

	res := &ctypes.ResultBlockResults{
		Height:  height,
		Results: results,
	}
	return res, nil
}

func getHeight(storeHeight int64, heightPtr *int64) (int64, error) {
	if heightPtr != nil {
		height := *heightPtr
		if height <= 0 {
			return 0, fmt.Errorf("Height must be greater than 0")
		}
		if height > storeHeight {
			return 0, fmt.Errorf("Height must be less than or equal to the current blockchain height")
		}
		return height, nil
	}
	return storeHeight, nil
}
