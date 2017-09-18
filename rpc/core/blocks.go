// Copyright 2015 Tendermint. All Rights Reserved.
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
	"github.com/tendermint/tendermint/types"
	. "github.com/tendermint/tmlibs/common"
)

// Get block headers for minHeight <= height <= maxHeight.
//
// ```shell
// curl 'localhost:46657/blockchain?minHeight=10&maxHeight=10'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:46657", "/websocket")
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
func BlockchainInfo(minHeight, maxHeight int) (*ctypes.ResultBlockchainInfo, error) {
	if maxHeight == 0 {
		maxHeight = blockStore.Height()
	} else {
		maxHeight = MinInt(blockStore.Height(), maxHeight)
	}
	if minHeight == 0 {
		minHeight = MaxInt(1, maxHeight-20)
	} else {
		minHeight = MaxInt(minHeight, maxHeight-20)
	}

	logger.Debug("BlockchainInfoHandler", "maxHeight", maxHeight, "minHeight", minHeight)

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
// curl 'localhost:46657/block?height=10'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:46657", "/websocket")
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
func Block(heightPtr *int) (*ctypes.ResultBlock, error) {
	if heightPtr == nil {
		height := blockStore.Height()
		blockMeta := blockStore.LoadBlockMeta(height)
		block := blockStore.LoadBlock(height)
		return &ctypes.ResultBlock{blockMeta, block}, nil
	}

	height := *heightPtr
	if height <= 0 {
		return nil, fmt.Errorf("Height must be greater than 0")
	}
	if height > blockStore.Height() {
		return nil, fmt.Errorf("Height must be less than the current blockchain height")
	}

	blockMeta := blockStore.LoadBlockMeta(height)
	block := blockStore.LoadBlock(height)
	return &ctypes.ResultBlock{blockMeta, block}, nil
}

// Get block commit at a given height.
// If no height is provided, it will fetch the commit for the latest block.
//
// ```shell
// curl 'localhost:46657/commit?height=11'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:46657", "/websocket")
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
func Commit(heightPtr *int) (*ctypes.ResultCommit, error) {
	if heightPtr == nil {
		height := blockStore.Height()
		header := blockStore.LoadBlockMeta(height).Header
		commit := blockStore.LoadSeenCommit(height)
		return &ctypes.ResultCommit{header, commit, false}, nil
	}

	height := *heightPtr
	if height <= 0 {
		return nil, fmt.Errorf("Height must be greater than 0")
	}
	storeHeight := blockStore.Height()
	if height > storeHeight {
		return nil, fmt.Errorf("Height must be less than or equal to the current blockchain height")
	}

	header := blockStore.LoadBlockMeta(height).Header

	// If the next block has not been committed yet,
	// use a non-canonical commit
	if height == storeHeight {
		commit := blockStore.LoadSeenCommit(height)
		return &ctypes.ResultCommit{header, commit, false}, nil
	}

	// Return the canonical commit (comes from the block at height+1)
	commit := blockStore.LoadBlockCommit(height)
	return &ctypes.ResultCommit{header, commit, true}, nil
}
