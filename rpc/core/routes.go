package core

import (
	data "github.com/tendermint/go-wire/data"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpc "github.com/tendermint/tendermint/rpc/lib/server"
	"github.com/tendermint/tendermint/rpc/lib/types"
	"github.com/tendermint/tendermint/types"
)

// TODO: better system than "unsafe" prefix
var Routes = map[string]*rpc.RPCFunc{
	// subscribe/unsubscribe are reserved for websocket events.
	"subscribe":   rpc.NewWSRPCFunc(SubscribeResult, "event"),
	"unsubscribe": rpc.NewWSRPCFunc(UnsubscribeResult, "event"),

	// info API
	"status":               rpc.NewRPCFunc(StatusResult, ""),
	"net_info":             rpc.NewRPCFunc(NetInfoResult, ""),
	"blockchain":           rpc.NewRPCFunc(BlockchainInfoResult, "minHeight,maxHeight"),
	"genesis":              rpc.NewRPCFunc(GenesisResult, ""),
	"block":                rpc.NewRPCFunc(BlockResult, "height"),
	"commit":               rpc.NewRPCFunc(CommitResult, "height"),
	"tx":                   rpc.NewRPCFunc(TxResult, "hash,prove"),
	"validators":           rpc.NewRPCFunc(ValidatorsResult, ""),
	"dump_consensus_state": rpc.NewRPCFunc(DumpConsensusStateResult, ""),
	"unconfirmed_txs":      rpc.NewRPCFunc(UnconfirmedTxsResult, ""),
	"num_unconfirmed_txs":  rpc.NewRPCFunc(NumUnconfirmedTxsResult, ""),

	// broadcast API
	"broadcast_tx_commit": rpc.NewRPCFunc(BroadcastTxCommitResult, "tx"),
	"broadcast_tx_sync":   rpc.NewRPCFunc(BroadcastTxSyncResult, "tx"),
	"broadcast_tx_async":  rpc.NewRPCFunc(BroadcastTxAsyncResult, "tx"),

	// abci API
	"abci_query": rpc.NewRPCFunc(ABCIQueryResult, "path,data,prove"),
	"abci_info":  rpc.NewRPCFunc(ABCIInfoResult, ""),

	// control API
	"dial_seeds":           rpc.NewRPCFunc(UnsafeDialSeedsResult, "seeds"),
	"unsafe_flush_mempool": rpc.NewRPCFunc(UnsafeFlushMempool, ""),

	// config is not in general thread safe. expose specifics if you need em
	// "unsafe_set_config":    rpc.NewRPCFunc(UnsafeSetConfigResult, "type,key,value"),

	// profiler API
	"unsafe_start_cpu_profiler": rpc.NewRPCFunc(UnsafeStartCPUProfilerResult, "filename"),
	"unsafe_stop_cpu_profiler":  rpc.NewRPCFunc(UnsafeStopCPUProfilerResult, ""),
	"unsafe_write_heap_profile": rpc.NewRPCFunc(UnsafeWriteHeapProfileResult, "filename"),
}

func SubscribeResult(wsCtx rpctypes.WSRPCContext, event string) (ctypes.TMResult, error) {
	res, err := Subscribe(wsCtx, event)
	return ctypes.TMResult{res}, err
}

func UnsubscribeResult(wsCtx rpctypes.WSRPCContext, event string) (ctypes.TMResult, error) {
	res, err := Unsubscribe(wsCtx, event)
	return ctypes.TMResult{res}, err
}

func StatusResult() (ctypes.TMResult, error) {
	res, err := Status()
	return ctypes.TMResult{res}, err
}

func NetInfoResult() (ctypes.TMResult, error) {
	res, err := NetInfo()
	return ctypes.TMResult{res}, err
}

func UnsafeDialSeedsResult(seeds []string) (ctypes.TMResult, error) {
	res, err := UnsafeDialSeeds(seeds)
	return ctypes.TMResult{res}, err
}

func BlockchainInfoResult(min, max int) (ctypes.TMResult, error) {
	res, err := BlockchainInfo(min, max)
	return ctypes.TMResult{res}, err
}

func GenesisResult() (ctypes.TMResult, error) {
	res, err := Genesis()
	return ctypes.TMResult{res}, err
}

func BlockResult(height int) (ctypes.TMResult, error) {
	res, err := Block(height)
	return ctypes.TMResult{res}, err
}

func CommitResult(height int) (ctypes.TMResult, error) {
	res, err := Commit(height)
	return ctypes.TMResult{res}, err
}

func ValidatorsResult() (ctypes.TMResult, error) {
	res, err := Validators()
	return ctypes.TMResult{res}, err
}

func DumpConsensusStateResult() (ctypes.TMResult, error) {
	res, err := DumpConsensusState()
	return ctypes.TMResult{res}, err
}

func UnconfirmedTxsResult() (ctypes.TMResult, error) {
	res, err := UnconfirmedTxs()
	return ctypes.TMResult{res}, err
}

func NumUnconfirmedTxsResult() (ctypes.TMResult, error) {
	res, err := NumUnconfirmedTxs()
	return ctypes.TMResult{res}, err
}

// Tx allow user to query the transaction results. `nil` could mean the
// transaction is in the mempool, invalidated, or was not send in the first
// place.
func TxResult(hash []byte, prove bool) (ctypes.TMResult, error) {
	res, err := Tx(hash, prove)
	return ctypes.TMResult{res}, err
}

func BroadcastTxCommitResult(tx types.Tx) (ctypes.TMResult, error) {
	res, err := BroadcastTxCommit(tx)
	return ctypes.TMResult{res}, err
}

func BroadcastTxSyncResult(tx types.Tx) (ctypes.TMResult, error) {
	res, err := BroadcastTxSync(tx)
	return ctypes.TMResult{res}, err
}

func BroadcastTxAsyncResult(tx types.Tx) (ctypes.TMResult, error) {
	res, err := BroadcastTxAsync(tx)
	return ctypes.TMResult{res}, err
}

func ABCIQueryResult(path string, data data.Bytes, prove bool) (ctypes.TMResult, error) {
	res, err := ABCIQuery(path, data, prove)
	return ctypes.TMResult{res}, err
}

func ABCIInfoResult() (ctypes.TMResult, error) {
	res, err := ABCIInfo()
	return ctypes.TMResult{res}, err
}

func UnsafeFlushMempoolResult() (ctypes.TMResult, error) {
	res, err := UnsafeFlushMempool()
	return ctypes.TMResult{res}, err
}

func UnsafeSetConfigResult(typ, key, value string) (ctypes.TMResult, error) {
	res, err := UnsafeSetConfig(typ, key, value)
	return ctypes.TMResult{res}, err
}

func UnsafeStartCPUProfilerResult(filename string) (ctypes.TMResult, error) {
	res, err := UnsafeStartCPUProfiler(filename)
	return ctypes.TMResult{res}, err
}

func UnsafeStopCPUProfilerResult() (ctypes.TMResult, error) {
	res, err := UnsafeStopCPUProfiler()
	return ctypes.TMResult{res}, err
}

func UnsafeWriteHeapProfileResult(filename string) (ctypes.TMResult, error) {
	res, err := UnsafeWriteHeapProfile(filename)
	return ctypes.TMResult{res}, err
}
