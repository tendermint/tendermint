package core

import (
	"bytes"
	"time"

	cmn "github.com/tendermint/tendermint/libs/common"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
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
// result, err := client.Status()
// ```
//
// > The above command returns JSON structured like this:
//
// ```json
//{
//  "jsonrpc": "2.0",
//  "id": "",
//  "result": {
//    "node_info": {
//      "id": "562dd7f579f0ecee8c94a11a3c1e378c1876f433",
//      "listen_addr": "192.168.1.2:26656",
//      "network": "test-chain-I6zScH",
//      "version": "0.19.0",
//      "channels": "4020212223303800",
//      "moniker": "Ethans-MacBook-Pro.local",
//      "other": [
//        "amino_version=0.9.8",
//        "p2p_version=0.5.0",
//        "consensus_version=v1/0.2.2",
//        "rpc_version=0.7.0/3",
//        "tx_index=on",
//        "rpc_addr=tcp://0.0.0.0:26657"
//      ]
//    },
//    "sync_info": {
//      "latest_block_hash": "2D4D7055BE685E3CB2410603C92AD37AE557AC59",
//      "latest_app_hash": "0000000000000000",
//      "latest_block_height": 231,
//      "latest_block_time": "2018-04-27T23:18:08.459766485-04:00",
//      "catching_up": false
//    },
//    "validator_info": {
//      "address": "5875562FF0FFDECC895C20E32FC14988952E99E7",
//      "pub_key": {
//        "type": "tendermint/PubKeyEd25519",
//        "value": "PpDJRUrLG2RgFqYYjawfn/AcAgacSXpLFrmfYYQnuzE="
//      },
//      "voting_power": 10
//    }
//  }
//}
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

	var votingPower int64
	if val := validatorAtHeight(latestHeight); val != nil {
		votingPower = val.VotingPower
	}

	result := &ctypes.ResultStatus{
		NodeInfo: p2pSwitch.NodeInfo(),
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

	lastBlockHeight, vals := consensusState.GetValidators()

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
