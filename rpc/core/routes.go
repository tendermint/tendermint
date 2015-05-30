package core

import (
	rpc "github.com/tendermint/tendermint/rpc/server"
)

var Routes = map[string]*rpc.RPCFunc{
	"status":                  rpc.NewRPCFunc(Status, []string{}),
	"net_info":                rpc.NewRPCFunc(NetInfo, []string{}),
	"blockchain":              rpc.NewRPCFunc(BlockchainInfo, []string{"minHeight", "maxHeight"}),
	"genesis":                 rpc.NewRPCFunc(Genesis, []string{}),
	"get_block":               rpc.NewRPCFunc(GetBlock, []string{"height"}),
	"get_account":             rpc.NewRPCFunc(GetAccount, []string{"address"}),
	"get_storage":             rpc.NewRPCFunc(GetStorage, []string{"address", "key"}),
	"call":                    rpc.NewRPCFunc(Call, []string{"address", "data"}),
	"call_code":               rpc.NewRPCFunc(CallCode, []string{"code", "data"}),
	"list_validators":         rpc.NewRPCFunc(ListValidators, []string{}),
	"dump_consensus_state":    rpc.NewRPCFunc(DumpConsensusState, []string{}),
	"dump_storage":            rpc.NewRPCFunc(DumpStorage, []string{"address"}),
	"broadcast_tx":            rpc.NewRPCFunc(BroadcastTx, []string{"tx"}),
	"list_unconfirmed_txs":    rpc.NewRPCFunc(ListUnconfirmedTxs, []string{}),
	"list_accounts":           rpc.NewRPCFunc(ListAccounts, []string{}),
	"unsafe/gen_priv_account": rpc.NewRPCFunc(GenPrivAccount, []string{}),
	"unsafe/sign_tx":          rpc.NewRPCFunc(SignTx, []string{"tx", "privAccounts"}),
}
