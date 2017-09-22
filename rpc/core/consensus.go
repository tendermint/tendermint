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
	wire "github.com/tendermint/go-wire"
	cm "github.com/tendermint/tendermint/consensus"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
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
func Validators(heightPtr *int) (*ctypes.ResultValidators, error) {
	if heightPtr == nil {
		blockHeight, validators := consensusState.GetValidators()
		return &ctypes.ResultValidators{blockHeight, validators}, nil
	}

	height := *heightPtr
	state := consensusState.GetState()
	validators, err := state.LoadValidators(height)
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultValidators{height, validators.Validators}, nil
}

// Dump consensus state.
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
// {
// 	"error": "",
// 	"result": {
// 		"peer_round_states": [],
// 		"round_state": "RoundState{\n  H:3537 R:0 S:RoundStepNewHeight\n  StartTime:     2017-05-31 12:32:31.178653883 +0000 UTC\n  CommitTime:    2017-05-31 12:32:30.178653883 +0000 UTC\n  Validators:    ValidatorSet{\n      Proposer: Validator{E89A51D60F68385E09E716D353373B11F8FACD62 {PubKeyEd25519{68DFDA7E50F82946E7E8546BED37944A422CD1B831E70DF66BA3B8430593944D}} VP:10 A:0}\n      Validators:\n        Validator{E89A51D60F68385E09E716D353373B11F8FACD62 {PubKeyEd25519{68DFDA7E50F82946E7E8546BED37944A422CD1B831E70DF66BA3B8430593944D}} VP:10 A:0}\n    }\n  Proposal:      <nil>\n  ProposalBlock: nil-PartSet nil-Block\n  LockedRound:   0\n  LockedBlock:   nil-PartSet nil-Block\n  Votes:         HeightVoteSet{H:3537 R:0~0\n      VoteSet{H:3537 R:0 T:1 +2/3:<nil> BA{1:_} map[]}\n      VoteSet{H:3537 R:0 T:2 +2/3:<nil> BA{1:_} map[]}\n    }\n  LastCommit: VoteSet{H:3536 R:0 T:2 +2/3:B7F988FBCDC68F9320E346EECAA76E32F6054654:1:673BE7C01F74 BA{1:X} map[]}\n  LastValidators:    ValidatorSet{\n      Proposer: Validator{E89A51D60F68385E09E716D353373B11F8FACD62 {PubKeyEd25519{68DFDA7E50F82946E7E8546BED37944A422CD1B831E70DF66BA3B8430593944D}} VP:10 A:0}\n      Validators:\n        Validator{E89A51D60F68385E09E716D353373B11F8FACD62 {PubKeyEd25519{68DFDA7E50F82946E7E8546BED37944A422CD1B831E70DF66BA3B8430593944D}} VP:10 A:0}\n    }\n}"
// 	},
// 	"id": "",
// 	"jsonrpc": "2.0"
// }
// ```
func DumpConsensusState() (*ctypes.ResultDumpConsensusState, error) {
	roundState := consensusState.GetRoundState()
	peerRoundStates := []string{}
	for _, peer := range p2pSwitch.Peers().List() {
		// TODO: clean this up?
		peerState := peer.Get(types.PeerStateKey).(*cm.PeerState)
		peerRoundState := peerState.GetRoundState()
		peerRoundStateStr := peer.Key() + ":" + string(wire.JSONBytes(peerRoundState))
		peerRoundStates = append(peerRoundStates, peerRoundStateStr)
	}
	return &ctypes.ResultDumpConsensusState{roundState.String(), peerRoundStates}, nil
}
