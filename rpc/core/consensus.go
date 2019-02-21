package core

import (
	cm "github.com/tendermint/tendermint/consensus"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

// Get the validator set at the given block height.
// If no height is provided, it will fetch the current validator set.
//
// ```shell
// curl 'localhost:26657/validators'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:26657", "/websocket")
// err := client.Start()
// if err != nil {
//   // handle error
// }
// defer client.Stop()
// state, err := client.Validators()
// ```
//
// The above command returns JSON structured like this:
//
// ```json
// {
// 	"error": "",
// 	"result": {
// 		"validators": [
// 			{
// 				"proposer_priority": "0",
// 				"voting_power": "10",
// 				"pub_key": {
// 					"data": "68DFDA7E50F82946E7E8546BED37944A422CD1B831E70DF66BA3B8430593944D",
// 					"type": "ed25519"
// 				},
// 				"address": "E89A51D60F68385E09E716D353373B11F8FACD62"
// 			}
// 		],
// 		"block_height": "5241"
// 	},
// 	"id": "",
// 	"jsonrpc": "2.0"
// }
// ```
func Validators(heightPtr *int64) (*ctypes.ResultValidators, error) {
	// The latest validator that we know is the
	// NextValidator of the last block.
	height := consensusState.GetState().LastBlockHeight + 1
	height, err := getHeight(height, heightPtr)
	if err != nil {
		return nil, err
	}

	validators, err := sm.LoadValidators(stateDB, height)
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultValidators{
		BlockHeight: height,
		Validators:  validators.Validators}, nil
}

// DumpConsensusState dumps consensus state.
// UNSTABLE
//
// ```shell
// curl 'localhost:26657/dump_consensus_state'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:26657", "/websocket")
// err := client.Start()
// if err != nil {
//   // handle error
// }
// defer client.Stop()
// state, err := client.DumpConsensusState()
// ```
//
// The above command returns JSON structured like this:
//
// ```json
// {
//   "jsonrpc": "2.0",
//   "id": "",
//   "result": {
//     "round_state": {
//       "height": "7185",
//       "round": "0",
//       "step": "1",
//       "start_time": "2018-05-12T13:57:28.440293621-07:00",
//       "commit_time": "2018-05-12T13:57:27.440293621-07:00",
//       "validators": {
//         "validators": [
//           {
//             "address": "B5B3D40BE53982AD294EF99FF5A34C0C3E5A3244",
//             "pub_key": {
//               "type": "tendermint/PubKeyEd25519",
//               "value": "SBctdhRBcXtBgdI/8a/alTsUhGXqGs9k5ylV1u5iKHg="
//             },
//             "voting_power": "10",
//             "proposer_priority": "0"
//           }
//         ],
//         "proposer": {
//           "address": "B5B3D40BE53982AD294EF99FF5A34C0C3E5A3244",
//           "pub_key": {
//             "type": "tendermint/PubKeyEd25519",
//             "value": "SBctdhRBcXtBgdI/8a/alTsUhGXqGs9k5ylV1u5iKHg="
//           },
//           "voting_power": "10",
//           "proposer_priority": "0"
//         }
//       },
//       "proposal": null,
//       "proposal_block": null,
//       "proposal_block_parts": null,
//       "locked_round": "0",
//       "locked_block": null,
//       "locked_block_parts": null,
//       "valid_round": "0",
//       "valid_block": null,
//       "valid_block_parts": null,
//       "votes": [
//         {
//           "round": "0",
//           "prevotes": "_",
//           "precommits": "_"
//         }
//       ],
//       "commit_round": "-1",
//       "last_commit": {
//         "votes": [
//           "Vote{0:B5B3D40BE539 7184/00/2(Precommit) 14F946FA7EF0 /702B1B1A602A.../ @ 2018-05-12T20:57:27.342Z}"
//         ],
//         "votes_bit_array": "x",
//         "peer_maj_23s": {}
//       },
//       "last_validators": {
//         "validators": [
//           {
//             "address": "B5B3D40BE53982AD294EF99FF5A34C0C3E5A3244",
//             "pub_key": {
//               "type": "tendermint/PubKeyEd25519",
//               "value": "SBctdhRBcXtBgdI/8a/alTsUhGXqGs9k5ylV1u5iKHg="
//             },
//             "voting_power": "10",
//             "proposer_priority": "0"
//           }
//         ],
//         "proposer": {
//           "address": "B5B3D40BE53982AD294EF99FF5A34C0C3E5A3244",
//           "pub_key": {
//             "type": "tendermint/PubKeyEd25519",
//             "value": "SBctdhRBcXtBgdI/8a/alTsUhGXqGs9k5ylV1u5iKHg="
//           },
//           "voting_power": "10",
//           "proposer_priority": "0"
//         }
//       }
//     },
//     "peers": [
//       {
//         "node_address": "30ad1854af22506383c3f0e57fb3c7f90984c5e8@172.16.63.221:26656",
//         "peer_state": {
//           "round_state": {
//             "height": "7185",
//             "round": "0",
//             "step": "1",
//             "start_time": "2018-05-12T13:57:27.438039872-07:00",
//             "proposal": false,
//             "proposal_block_parts_header": {
//               "total": "0",
//               "hash": ""
//             },
//             "proposal_block_parts": null,
//             "proposal_pol_round": "-1",
//             "proposal_pol": "_",
//             "prevotes": "_",
//             "precommits": "_",
//             "last_commit_round": "0",
//             "last_commit": "x",
//             "catchup_commit_round": "-1",
//             "catchup_commit": "_"
//           },
//           "stats": {
//             "last_vote_height": "7184",
//             "votes": "255",
//             "last_block_part_height": "7184",
//             "block_parts": "255"
//           }
//         }
//       }
//     ]
//   }
// }
// ```
func DumpConsensusState() (*ctypes.ResultDumpConsensusState, error) {
	// Get Peer consensus states.
	peers := p2pPeers.Peers().List()
	peerStates := make([]ctypes.PeerStateInfo, len(peers))
	for i, peer := range peers {
		peerState, ok := peer.Get(types.PeerStateKey).(*cm.PeerState)
		if !ok { // peer does not have a state yet
			continue
		}
		peerStateJSON, err := peerState.ToJSON()
		if err != nil {
			return nil, err
		}
		peerStates[i] = ctypes.PeerStateInfo{
			// Peer basic info.
			NodeAddress: peer.NodeInfo().NetAddress().String(),
			// Peer consensus state.
			PeerState: peerStateJSON,
		}
	}
	// Get self round state.
	roundState, err := consensusState.GetRoundStateJSON()
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultDumpConsensusState{
		RoundState: roundState,
		Peers:      peerStates}, nil
}

// ConsensusState returns a concise summary of the consensus state.
// UNSTABLE
//
// ```shell
// curl 'localhost:26657/consensus_state'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:26657", "/websocket")
// err := client.Start()
// if err != nil {
//   // handle error
// }
// defer client.Stop()
// state, err := client.ConsensusState()
// ```
//
// The above command returns JSON structured like this:
//
// ```json
//{
//  "jsonrpc": "2.0",
//  "id": "",
//  "result": {
//    "round_state": {
//      "height/round/step": "9336/0/1",
//      "start_time": "2018-05-14T10:25:45.72595357-04:00",
//      "proposal_block_hash": "",
//      "locked_block_hash": "",
//      "valid_block_hash": "",
//      "height_vote_set": [
//        {
//          "round": "0",
//          "prevotes": [
//            "nil-Vote"
//          ],
//          "prevotes_bit_array": "BA{1:_} 0/10 = 0.00",
//          "precommits": [
//            "nil-Vote"
//          ],
//          "precommits_bit_array": "BA{1:_} 0/10 = 0.00"
//        }
//      ]
//    }
//  }
//}
//```
func ConsensusState() (*ctypes.ResultConsensusState, error) {
	// Get self round state.
	bz, err := consensusState.GetRoundStateSimpleJSON()
	return &ctypes.ResultConsensusState{RoundState: bz}, err
}

// Get the consensus parameters  at the given block height.
// If no height is provided, it will fetch the current consensus params.
//
// ```shell
// curl 'localhost:26657/consensus_params'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:26657", "/websocket")
// err := client.Start()
// if err != nil {
//   // handle error
// }
// defer client.Stop()
// state, err := client.ConsensusParams()
// ```
//
// The above command returns JSON structured like this:
//
// ```json
// {
//   "jsonrpc": "2.0",
//   "id": "",
//   "result": {
//     "block_height": "1",
//     "consensus_params": {
//       "block_size_params": {
//         "max_txs_bytes": "22020096",
//         "max_gas": "-1"
//       },
//       "evidence_params": {
//         "max_age": "100000"
//       }
//     }
//   }
// }
// ```
func ConsensusParams(heightPtr *int64) (*ctypes.ResultConsensusParams, error) {
	height := consensusState.GetState().LastBlockHeight + 1
	height, err := getHeight(height, heightPtr)
	if err != nil {
		return nil, err
	}

	consensusparams, err := sm.LoadConsensusParams(stateDB, height)
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultConsensusParams{
		BlockHeight:     height,
		ConsensusParams: consensusparams}, nil
}
