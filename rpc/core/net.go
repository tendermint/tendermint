package core

import (
	"fmt"

	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

// Get network info.
//
// ```shell
// curl 'localhost:46657/net_info'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:46657", "/websocket")
// info, err := client.NetInfo()
// ```
//
// > The above command returns JSON structured like this:
//
// ```json
// {
// 	"error": "",
// 	"result": {
// 		"peers": [],
// 		"listeners": [
// 			"Listener(@10.0.2.15:46656)"
// 		],
// 		"listening": true
// 	},
// 	"id": "",
// 	"jsonrpc": "2.0"
// }
// ```
func NetInfo() (*ctypes.ResultNetInfo, error) {
	listening := p2pSwitch.IsListening()
	listeners := []string{}
	for _, listener := range p2pSwitch.Listeners() {
		listeners = append(listeners, listener.String())
	}
	peers := []ctypes.Peer{}
	for _, peer := range p2pSwitch.Peers().List() {
		peers = append(peers, ctypes.Peer{
			NodeInfo:         *peer.NodeInfo(),
			IsOutbound:       peer.IsOutbound(),
			ConnectionStatus: peer.Status(),
		})
	}
	return &ctypes.ResultNetInfo{
		Listening: listening,
		Listeners: listeners,
		Peers:     peers,
	}, nil
}

func UnsafeDialManualPeers(manual_peers []string) (*ctypes.ResultDialManualPeers, error) {

	if len(manual_peers) == 0 {
		return &ctypes.ResultDialManualPeers{}, fmt.Errorf("No manual peers provided")
	}
	// starts go routines to dial each peer after random delays
	logger.Info("DialManualPeers", "addrBook", addrBook, "manual_peers", manual_peers)
	err := p2pSwitch.DialPeersAsync(addrBook, manual_peers, true)
	if err != nil {
		return &ctypes.ResultDialManualPeers{}, err
	}
	return &ctypes.ResultDialManualPeers{"Dialing manual peers in progress. See /net_info for details"}, nil
}

// Get genesis file.
//
// ```shell
// curl 'localhost:46657/genesis'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:46657", "/websocket")
// genesis, err := client.Genesis()
// ```
//
// > The above command returns JSON structured like this:
//
// ```json
// {
// 	"error": "",
// 	"result": {
// 		"genesis": {
// 			"app_hash": "",
// 			"validators": [
// 				{
// 					"name": "",
// 					"power": 10,
// 					"pub_key": {
// 						"data": "68DFDA7E50F82946E7E8546BED37944A422CD1B831E70DF66BA3B8430593944D",
// 						"type": "ed25519"
// 					}
// 				}
// 			],
// 			"chain_id": "test-chain-6UTNIN",
// 			"genesis_time": "2017-05-29T15:05:41.671Z"
// 		}
// 	},
// 	"id": "",
// 	"jsonrpc": "2.0"
// }
// ```
func Genesis() (*ctypes.ResultGenesis, error) {
	return &ctypes.ResultGenesis{genDoc}, nil
}
