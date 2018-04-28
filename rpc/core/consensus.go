package core

import (
	cm "github.com/tendermint/tendermint/consensus"
	"github.com/tendermint/tendermint/p2p"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

// Get the validator set at the given block height.
// If no height is provided, it will fetch the current validator set.
//
// ```shell
// curl 'localhost:46657/validators'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:46657", "/websocket")
// state, err := client.Validators()
// ```
//
// > The above command returns JSON structured like this:
//
// ```json
// {
// 	"error": "",
// 	"result": {
// 		"validators": [
// 			{
// 				"accum": 0,
// 				"voting_power": 10,
// 				"pub_key": {
// 					"data": "68DFDA7E50F82946E7E8546BED37944A422CD1B831E70DF66BA3B8430593944D",
// 					"type": "ed25519"
// 				},
// 				"address": "E89A51D60F68385E09E716D353373B11F8FACD62"
// 			}
// 		],
// 		"block_height": 5241
// 	},
// 	"id": "",
// 	"jsonrpc": "2.0"
// }
// ```
func Validators(heightPtr *int64) (*ctypes.ResultValidators, error) {
	storeHeight := blockStore.Height()
	height, err := getHeight(storeHeight, heightPtr)
	if err != nil {
		return nil, err
	}

	validators, err := sm.LoadValidators(stateDB, height)
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultValidators{height, validators.Validators}, nil
}

// DumpConsensusState dumps consensus state.
//
// ```shell
// curl 'localhost:46657/dump_consensus_state'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:46657", "/websocket")
// state, err := client.DumpConsensusState()
// ```
//
// > The above command returns JSON structured like this:
//
// ```json
//{
//  "jsonrpc": "2.0",
//  "id": "",
//  "result": {
//    "round_state": {
//      "height": 138,
//      "round": 0,
//      "step": 1,
//      "start_time": "2018-04-27T23:16:34.472087096-04:00",
//      "commit_time": "2018-04-27T23:16:33.472087096-04:00",
//      "validators": {
//        "validators": [
//          {
//            "address": "5875562FF0FFDECC895C20E32FC14988952E99E7",
//            "pub_key": {
//              "type": "AC26791624DE60",
//              "value": "PpDJRUrLG2RgFqYYjawfn/AcAgacSXpLFrmfYYQnuzE="
//            },
//            "voting_power": 10,
//            "accum": 0
//          }
//        ],
//        "proposer": {
//          "address": "5875562FF0FFDECC895C20E32FC14988952E99E7",
//          "pub_key": {
//            "type": "AC26791624DE60",
//            "value": "PpDJRUrLG2RgFqYYjawfn/AcAgacSXpLFrmfYYQnuzE="
//          },
//          "voting_power": 10,
//          "accum": 0
//        }
//      },
//      "proposal": null,
//      "proposal_block": null,
//      "proposal_block_parts": null,
//      "locked_round": 0,
//      "locked_block": null,
//      "locked_block_parts": null,
//      "valid_round": 0,
//      "valid_block": null,
//      "valid_block_parts": null,
//      "votes": [
//        {
//          "round": 0,
//          "prevotes": "_",
//          "precommits": "_"
//        }
//      ],
//      "commit_round": -1,
//      "last_commit": {
//        "votes": [
//          "Vote{0:5875562FF0FF 137/00/2(Precommit) 5701C93659EA /ED3588D7AF29.../ @ 2018-04-28T03:16:33.469Z}"
//        ],
//        "votes_bit_array": "x",
//        "peer_maj_23s": {}
//      },
//      "last_validators": {
//        "validators": [
//          {
//            "address": "5875562FF0FFDECC895C20E32FC14988952E99E7",
//            "pub_key": {
//              "type": "AC26791624DE60",
//              "value": "PpDJRUrLG2RgFqYYjawfn/AcAgacSXpLFrmfYYQnuzE="
//            },
//            "voting_power": 10,
//            "accum": 0
//          }
//        ],
//        "proposer": {
//          "address": "5875562FF0FFDECC895C20E32FC14988952E99E7",
//          "pub_key": {
//            "type": "AC26791624DE60",
//            "value": "PpDJRUrLG2RgFqYYjawfn/AcAgacSXpLFrmfYYQnuzE="
//          },
//          "voting_power": 10,
//          "accum": 0
//        }
//      }
//    },
//    "peer_round_states": {
//      "d4bf26bfa5e390b94d98106ab858abf64db26d48": {
//        "Height": 136,
//        "Round": 0,
//        "Step": 1,
//        "StartTime": "2018-04-27T23:16:33.841163812-04:00",
//        "Proposal": false,
//        "ProposalBlockPartsHeader": {
//          "total": 1,
//          "hash": "E27F2D13298F7CB14090EE60CD9AB214D2F5161F"
//        },
//        "ProposalBlockParts": "x",
//        "ProposalPOLRound": -1,
//        "ProposalPOL": "_",
//        "Prevotes": "_",
//        "Precommits": "x",
//        "LastCommitRound": 0,
//        "LastCommit": null,
//        "CatchupCommitRound": 0,
//        "CatchupCommit": "_"
//      }
//    }
//  }
//}
// ```
// UNSTABLE
func DumpConsensusState() (*ctypes.ResultDumpConsensusState, error) {
	peers := p2pSwitch.Peers().List()
	peerRoundStates := make([]ctypes.PeerRoundState, len(peers))
	for i, peer := range peers {
		peerState := peer.Get(types.PeerStateKey).(*cm.PeerState)
		peerRoundState, err := peerState.GetRoundStateJSON()
		if err != nil {
			return nil, err
		}
		peerRoundStates[i] = ctypes.PeerRoundState{
			NodeAddress:    p2p.IDAddressString(peer.ID(), peer.NodeInfo().ListenAddr),
			PeerRoundState: peerRoundState,
		}
	}
	roundState, err := consensusState.GetRoundStateJSON()
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultDumpConsensusState{roundState, peerRoundStates}, nil
}
