package core

import (
	"bytes"
	"time"

	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/p2p"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpctypes "github.com/tendermint/tendermint/rpc/lib/types"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

// Get Tendermint status including node info, pubkey, latest block
// hash, app hash, block height and time.
//
// ```shell
// curl 'localhost:26657/status'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:26657", "/websocket")
// err := client.Start()
// if err != nil {
//   // handle error
// }
// defer client.Stop()
// result, err := client.Status()
// ```
//
// > The above command returns JSON structured like this:
//
// ```json
// {
// "jsonrpc": "2.0",
// "id": "",
// "result": {
//   "node_info": {
//   		"protocol_version": {
//   			"p2p": "4",
//   			"block": "7",
//   			"app": "0"
//   		},
//   		"id": "53729852020041b956e86685e24394e0bee4373f",
//   		"listen_addr": "10.0.2.15:26656",
//   		"network": "test-chain-Y1OHx6",
//   		"version": "0.24.0-2ce1abc2",
//   		"channels": "4020212223303800",
//   		"moniker": "ubuntu-xenial",
//   		"other": {
//   			"tx_index": "on",
//   			"rpc_addr": "tcp://0.0.0.0:26657"
//   		}
//   	},
//   	"sync_info": {
//   		"latest_block_hash": "F51538DA498299F4C57AC8162AAFA0254CE08286",
//   		"latest_app_hash": "0000000000000000",
//   		"latest_block_height": "18",
//   		"latest_block_time": "2018-09-17T11:42:19.149920551Z",
//   		"catching_up": false
//   	},
//   	"validator_info": {
//   		"address": "D9F56456D7C5793815D0E9AF07C3A355D0FC64FD",
//   		"pub_key": {
//   			"type": "tendermint/PubKeyEd25519",
//   			"value": "wVxKNtEsJmR4vvh651LrVoRguPs+6yJJ9Bz174gw9DM="
//   		},
//   		"voting_power": "10"
//   	}
//   }
// }
// ```
func Status(ctx *rpctypes.Context) (*ctypes.ResultStatus, error) {
	var latestHeight int64
	if consensusReactor.FastSync() {
		latestHeight = blockStore.Height()
	} else {
		latestHeight = consensusState.GetLastHeight()
	}
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

	var votingPower int64
	if val := validatorAtHeight(latestHeight); val != nil {
		votingPower = val.VotingPower
	}

	result := &ctypes.ResultStatus{
		NodeInfo: p2pTransport.NodeInfo().(p2p.DefaultNodeInfo),
		SyncInfo: ctypes.SyncInfo{
			LatestBlockHash:   latestBlockHash,
			LatestAppHash:     latestAppHash,
			LatestBlockHeight: latestHeight,
			LatestBlockTime:   latestBlockTime,
			CatchingUp:        consensusReactor.FastSync(),
		},
		ValidatorInfo: ctypes.ValidatorInfo{
			Address:     pubKey.Address(),
			PubKey:      pubKey,
			VotingPower: votingPower,
		},
	}

	return result, nil
}

func validatorAtHeight(h int64) *types.Validator {
	privValAddress := pubKey.Address()

	// If we're still at height h, search in the current validator set.
	lastBlockHeight, vals := consensusState.GetValidators()
	if lastBlockHeight == h {
		for _, val := range vals {
			if bytes.Equal(val.Address, privValAddress) {
				return val
			}
		}
	}

	// If we've moved to the next height, retrieve the validator set from DB.
	if lastBlockHeight > h {
		vals, err := sm.LoadValidators(stateDB, h)
		if err != nil {
			return nil // should not happen
		}
		_, val := vals.GetByAddress(privValAddress)
		return val
	}

	return nil
}
