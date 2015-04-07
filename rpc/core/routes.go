package core

import (
	"github.com/tendermint/tendermint/rpc"
)

/*
TODO: support Call && GetStorage.
*/

var Routes = map[string]*rpc.RPCFunc{
	"status":                  rpc.NewRPCFunc(Status, []string{}),
	"net_info":                rpc.NewRPCFunc(NetInfo, []string{}),
	"blockchain":              rpc.NewRPCFunc(BlockchainInfo, []string{"min_height", "max_height"}),
	"get_block":               rpc.NewRPCFunc(GetBlock, []string{"height"}),
	"get_account":             rpc.NewRPCFunc(GetAccount, []string{"address"}),
	"get_storage":             rpc.NewRPCFunc(GetStorage, []string{"address", "storage"}),
	"call":                    rpc.NewRPCFunc(Call, []string{"address", "data"}),
	"list_validators":         rpc.NewRPCFunc(ListValidators, []string{}),
	"dump_storage":            rpc.NewRPCFunc(DumpStorage, []string{"address"}),
	"broadcast_tx":            rpc.NewRPCFunc(BroadcastTx, []string{"tx"}),
	"list_accounts":           rpc.NewRPCFunc(ListAccounts, []string{}),
	"unsafe/gen_priv_account": rpc.NewRPCFunc(GenPrivAccount, []string{}),
	"unsafe/sign_tx":          rpc.NewRPCFunc(SignTx, []string{"tx", "privAccounts"}),
}
