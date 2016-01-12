package core

import (
	rpc "github.com/tendermint/go-rpc/server"
)

// TODO: eliminate redundancy between here and reading code from core/
var Routes = map[string]*rpc.RPCFunc{
	"subscribe":            rpc.NewWSRPCFunc(Subscribe, []string{"event"}),
	"unsubscribe":          rpc.NewWSRPCFunc(Unsubscribe, []string{"event"}),
	"status":               rpc.NewRPCFunc(Status, []string{}),
	"net_info":             rpc.NewRPCFunc(NetInfo, []string{}),
	"blockchain":           rpc.NewRPCFunc(BlockchainInfo, []string{"minHeight", "maxHeight"}),
	"genesis":              rpc.NewRPCFunc(Genesis, []string{}),
	"get_block":            rpc.NewRPCFunc(GetBlock, []string{"height"}),
	"list_validators":      rpc.NewRPCFunc(ListValidators, []string{}),
	"dump_consensus_state": rpc.NewRPCFunc(DumpConsensusState, []string{}),
	"broadcast_tx":         rpc.NewRPCFunc(BroadcastTx, []string{"tx"}),
	"list_unconfirmed_txs": rpc.NewRPCFunc(ListUnconfirmedTxs, []string{}),
	// subscribe/unsubscribe are reserved for websocket events.
}
