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
// UNSTABLE
func DumpConsensusState() (*ctypes.ResultDumpConsensusState, error) {
	peers := p2pSwitch.Peers().List()
	peerRoundStates := make([]ctypes.PeerRoundStateInfo, len(peers))
	for i, peer := range peers {
		peerState := peer.Get(types.PeerStateKey).(*cm.PeerState)
		peerRoundState, err := peerState.GetRoundStateJSON()
		if err != nil {
			return nil, err
		}
		peerRoundStates[i] = ctypes.PeerRoundStateInfo{
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
