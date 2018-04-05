package core

import (
	"bytes"
	"time"

	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tmlibs/common"
)

// Get Tendermint status including node info, pubkey, latest block
// hash, app hash, block height and time.
//
// ```shell
// curl 'localhost:46657/status'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:46657", "/websocket")
// result, err := client.Status()
// ```
//
// > The above command returns JSON structured like this:
//
// ```json
// {
// 	"result": {
// 		"syncing": false,
// 		"latest_block_time": "2017-12-07T18:19:47.617Z",
// 		"latest_block_height": 6,
// 		"latest_app_hash": "",
// 		"latest_block_hash": "A63D0C3307DEDCCFCC82ED411AE9108B70B29E02",
// 		"pub_key": {
// 			"data": "8C9A68070CBE33F9C445862BA1E9D96A75CEB68C0CF6ADD3652D07DCAC5D0380",
// 			"type": "ed25519"
// 		},
// 		"node_info": {
// 			"other": [
// 				"wire_version=0.7.2",
// 				"p2p_version=0.5.0",
// 				"consensus_version=v1/0.2.2",
// 				"rpc_version=0.7.0/3",
// 				"tx_index=on",
// 				"rpc_addr=tcp://0.0.0.0:46657"
// 			],
// 			"version": "0.13.0-14ccc8b",
// 			"listen_addr": "10.0.2.15:46656",
// 			"remote_addr": "",
// 			"network": "test-chain-qhVCa2",
// 			"moniker": "vagrant-ubuntu-trusty-64",
// 			"pub_key": "844981FE99ABB19F7816F2D5E94E8A74276AB1153760A7799E925C75401856C6",
//			"validator_status": {
//				"voting_power": 10
//			}
// 		}
// 	},
// 	"id": "",
// 	"jsonrpc": "2.0"
// }
// ```
func Status() (*ctypes.ResultStatus, error) {
	latestHeight := blockStore.Height()
	var (
		latestBlockMeta     *types.BlockMeta
		latestBlockHash     cmn.HexBytes
		latestAppHash       cmn.HexBytes
		latestBlockTimeNano int64
	)
	if latestHeight != 0 {
		latestBlockMeta = blockStore.LoadBlockMeta(latestHeight)
		latestBlockHash = latestBlockMeta.BlockID.Hash
		latestAppHash = latestBlockMeta.Header.AppHash
		latestBlockTimeNano = latestBlockMeta.Header.Time.UnixNano()
	}

	latestBlockTime := time.Unix(0, latestBlockTimeNano)

	result := &ctypes.ResultStatus{
		NodeInfo:          p2pSwitch.NodeInfo(),
		PubKey:            pubKey,
		LatestBlockHash:   latestBlockHash,
		LatestAppHash:     latestAppHash,
		LatestBlockHeight: latestHeight,
		LatestBlockTime:   latestBlockTime,
		Syncing:           consensusReactor.FastSync(),
	}

	// add ValidatorStatus if node is a validator
	if val := validatorAtHeight(latestHeight); val != nil {
		result.ValidatorStatus = ctypes.ValidatorStatus{
			VotingPower: val.VotingPower,
		}
	}

	return result, nil
}

func validatorAtHeight(h int64) *types.Validator {
	lastBlockHeight, vals := consensusState.GetValidators()

	privValAddress := pubKey.Address()

	// if we're still at height h, search in the current validator set
	if lastBlockHeight == h {
		for _, val := range vals {
			if bytes.Equal(val.Address, privValAddress) {
				return val
			}
		}
	}

	// if we've moved to the next height, retrieve the validator set from DB
	if lastBlockHeight > h {
		vals, err := sm.LoadValidators(stateDB, h)
		if err != nil {
			// should not happen
			return nil
		}
		_, val := vals.GetByAddress(privValAddress)
		return val
	}

	return nil
}
