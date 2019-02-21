package core

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/tendermint/tendermint/p2p"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

// Get network info.
//
// ```shell
// curl 'localhost:26657/net_info'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:26657", "/websocket")
// err := client.Start()
// if err != nil {
//   // handle error
// }
// defer client.Stop()
// info, err := client.NetInfo()
// ```
//
// > The above command returns JSON structured like this:
//
// ```json
// {
//   "jsonrpc": "2.0",
//   "id": "",
//   "result": {
//   	"listening": true,
//   	"listeners": [
//   		"Listener(@)"
//   	],
//   	"n_peers": "3",
//   	"peers": [
//   		{
//   			"node_info": {
//   				"protocol_version": {
//   					"p2p": "7",
//   					"block": "8",
//   					"app": "1"
//   				},
//   				"id": "93529da3435c090d02251a050342b6a488d4ab56",
//   				"listen_addr": "tcp://0.0.0.0:26656",
//   				"network": "chain-RFo6qC",
//   				"version": "0.30.0",
//   				"channels": "4020212223303800",
//   				"moniker": "fc89e4ed23f2",
//   				"other": {
//   					"tx_index": "on",
//   					"rpc_address": "tcp://0.0.0.0:26657"
//   				}
//   			},
//   			"is_outbound": true,
//   			"connection_status": {
//   				"Duration": "3475230558",
//   				"SendMonitor": {
//   					"Active": true,
//   					"Start": "2019-02-14T12:40:47.52Z",
//   					"Duration": "3480000000",
//   					"Idle": "240000000",
//   					"Bytes": "4512",
//   					"Samples": "9",
//   					"InstRate": "1338",
//   					"CurRate": "2046",
//   					"AvgRate": "1297",
//   					"PeakRate": "6570",
//   					"BytesRem": "0",
//   					"TimeRem": "0",
//   					"Progress": 0
//   				},
//   				"RecvMonitor": {
//   					"Active": true,
//   					"Start": "2019-02-14T12:40:47.52Z",
//   					"Duration": "3480000000",
//   					"Idle": "280000000",
//   					"Bytes": "4489",
//   					"Samples": "10",
//   					"InstRate": "1821",
//   					"CurRate": "1663",
//   					"AvgRate": "1290",
//   					"PeakRate": "5512",
//   					"BytesRem": "0",
//   					"TimeRem": "0",
//   					"Progress": 0
//   				},
//   				"Channels": [
//   					{
//   						"ID": 48,
//   						"SendQueueCapacity": "1",
//   						"SendQueueSize": "0",
//   						"Priority": "5",
//   						"RecentlySent": "0"
//   					},
//   					{
//   						"ID": 64,
//   						"SendQueueCapacity": "1000",
//   						"SendQueueSize": "0",
//   						"Priority": "10",
//   						"RecentlySent": "14"
//   					},
//   					{
//   						"ID": 32,
//   						"SendQueueCapacity": "100",
//   						"SendQueueSize": "0",
//   						"Priority": "5",
//   						"RecentlySent": "619"
//   					},
//   					{
//   						"ID": 33,
//   						"SendQueueCapacity": "100",
//   						"SendQueueSize": "0",
//   						"Priority": "10",
//   						"RecentlySent": "1363"
//   					},
//   					{
//   						"ID": 34,
//   						"SendQueueCapacity": "100",
//   						"SendQueueSize": "0",
//   						"Priority": "5",
//   						"RecentlySent": "2145"
//   					},
//   					{
//   						"ID": 35,
//   						"SendQueueCapacity": "2",
//   						"SendQueueSize": "0",
//   						"Priority": "1",
//   						"RecentlySent": "0"
//   					},
//   					{
//   						"ID": 56,
//   						"SendQueueCapacity": "1",
//   						"SendQueueSize": "0",
//   						"Priority": "5",
//   						"RecentlySent": "0"
//   					},
//   					{
//   						"ID": 0,
//   						"SendQueueCapacity": "10",
//   						"SendQueueSize": "0",
//   						"Priority": "1",
//   						"RecentlySent": "10"
//   					}
//   				]
//   			},
//   			"remote_ip": "192.167.10.3"
//   		},
//      ...
//   }
// ```
func NetInfo() (*ctypes.ResultNetInfo, error) {
	out, in, _ := p2pPeers.NumPeers()
	peers := make([]ctypes.Peer, 0, out+in)
	for _, peer := range p2pPeers.Peers().List() {
		nodeInfo, ok := peer.NodeInfo().(p2p.DefaultNodeInfo)
		if !ok {
			return nil, fmt.Errorf("peer.NodeInfo() is not DefaultNodeInfo")
		}
		peers = append(peers, ctypes.Peer{
			NodeInfo:         nodeInfo,
			IsOutbound:       peer.IsOutbound(),
			ConnectionStatus: peer.Status(),
			RemoteIP:         peer.RemoteIP().String(),
		})
	}
	// TODO: Should we include PersistentPeers and Seeds in here?
	// PRO: useful info
	// CON: privacy
	return &ctypes.ResultNetInfo{
		Listening: p2pTransport.IsListening(),
		Listeners: p2pTransport.Listeners(),
		NPeers:    len(peers),
		Peers:     peers,
	}, nil
}

func UnsafeDialSeeds(seeds []string) (*ctypes.ResultDialSeeds, error) {
	if len(seeds) == 0 {
		return &ctypes.ResultDialSeeds{}, errors.New("No seeds provided")
	}
	// starts go routines to dial each peer after random delays
	logger.Info("DialSeeds", "addrBook", addrBook, "seeds", seeds)
	err := p2pPeers.DialPeersAsync(addrBook, seeds, false)
	if err != nil {
		return &ctypes.ResultDialSeeds{}, err
	}
	return &ctypes.ResultDialSeeds{Log: "Dialing seeds in progress. See /net_info for details"}, nil
}

func UnsafeDialPeers(peers []string, persistent bool) (*ctypes.ResultDialPeers, error) {
	if len(peers) == 0 {
		return &ctypes.ResultDialPeers{}, errors.New("No peers provided")
	}
	// starts go routines to dial each peer after random delays
	logger.Info("DialPeers", "addrBook", addrBook, "peers", peers, "persistent", persistent)
	err := p2pPeers.DialPeersAsync(addrBook, peers, persistent)
	if err != nil {
		return &ctypes.ResultDialPeers{}, err
	}
	return &ctypes.ResultDialPeers{Log: "Dialing peers in progress. See /net_info for details"}, nil
}

// Get genesis file.
//
// ```shell
// curl 'localhost:26657/genesis'
// ```
//
// ```go
// client := client.NewHTTP("tcp://0.0.0.0:26657", "/websocket")
// err := client.Start()
// if err != nil {
//   // handle error
// }
// defer client.Stop()
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
// 					"power": "10",
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
	return &ctypes.ResultGenesis{Genesis: genDoc}, nil
}
