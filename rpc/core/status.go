package core

import (
	data "github.com/tendermint/go-wire/data"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
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
//   "error": "",
//   "result": {
//     "latest_block_time": 1.49631773695e+18,
//     "latest_block_height": 22924,
//     "latest_app_hash": "9D16177BC71E445476174622EA559715C293740C",
//     "latest_block_hash": "75B36EEF96C277A592D8B14867098C58F68BB180",
//     "pub_key": {
//       "data": "68DFDA7E50F82946E7E8546BED37944A422CD1B831E70DF66BA3B8430593944D",
//       "type": "ed25519"
//     },
//     "node_info": {
//       "other": [
//         "wire_version=0.6.2",
//         "p2p_version=0.5.0",
//         "consensus_version=v1/0.2.2",
//         "rpc_version=0.7.0/3",
//         "tx_index=on",
//         "rpc_addr=tcp://0.0.0.0:46657"
//       ],
//       "version": "0.10.0-rc1-aa22bd84",
//       "listen_addr": "10.0.2.15:46656",
//       "remote_addr": "",
//       "network": "test-chain-6UTNIN",
//       "moniker": "anonymous",
//       "pub_key": "659B9E54DD6EF9FEF28FAD40629AF0E4BD3C2563BB037132B054A176E00F1D94"
//     }
//   },
//   "id": "",
//   "jsonrpc": "2.0"
// }
// ```
func Status() (*ctypes.ResultStatus, error) {
	latestHeight := blockStore.Height()
	var (
		latestBlockMeta *types.BlockMeta
		latestBlockHash data.Bytes
		latestAppHash   data.Bytes
		latestBlockTime int64
	)
	if latestHeight != 0 {
		latestBlockMeta = blockStore.LoadBlockMeta(latestHeight)
		latestBlockHash = latestBlockMeta.BlockID.Hash
		latestAppHash = latestBlockMeta.Header.AppHash
		latestBlockTime = latestBlockMeta.Header.Time.UnixNano()
	}

	return &ctypes.ResultStatus{
		NodeInfo:          p2pSwitch.NodeInfo(),
		PubKey:            pubKey,
		LatestBlockHash:   latestBlockHash,
		LatestAppHash:     latestAppHash,
		LatestBlockHeight: latestHeight,
		LatestBlockTime:   latestBlockTime,
		Syncing:           consensusReactor.FastSync()}, nil
}
