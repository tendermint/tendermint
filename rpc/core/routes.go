package core

import (
	rpc "github.com/tendermint/go-rpc/server"
	"github.com/tendermint/go-rpc/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
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
	"tx":                   rpc.NewRPCFunc(TxResult, "hash"),
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
	"unsafe_set_config":    rpc.NewRPCFunc(UnsafeSetConfigResult, "type,key,value"),

	// profiler API
	"unsafe_start_cpu_profiler": rpc.NewRPCFunc(UnsafeStartCPUProfilerResult, "filename"),
	"unsafe_stop_cpu_profiler":  rpc.NewRPCFunc(UnsafeStopCPUProfilerResult, ""),
	"unsafe_write_heap_profile": rpc.NewRPCFunc(UnsafeWriteHeapProfileResult, "filename"),
}

func SubscribeResult(wsCtx rpctypes.WSRPCContext, event string) (ctypes.TMResult, error) {
	return Subscribe(wsCtx, event)
}

func UnsubscribeResult(wsCtx rpctypes.WSRPCContext, event string) (ctypes.TMResult, error) {
	return Unsubscribe(wsCtx, event)
}

func StatusResult() (ctypes.TMResult, error) {
	return Status()
}

func NetInfoResult() (ctypes.TMResult, error) {
	return NetInfo()
}

func UnsafeDialSeedsResult(seeds []string) (ctypes.TMResult, error) {
	return UnsafeDialSeeds(seeds)
}

func BlockchainInfoResult(min, max int) (ctypes.TMResult, error) {
	return BlockchainInfo(min, max)
}

func GenesisResult() (ctypes.TMResult, error) {
	return Genesis()
}

func BlockResult(height int) (ctypes.TMResult, error) {
	return Block(height)
}

func CommitResult(height int) (ctypes.TMResult, error) {
	return Commit(height)
}

func ValidatorsResult() (ctypes.TMResult, error) {
	return Validators()
}

func DumpConsensusStateResult() (ctypes.TMResult, error) {
	return DumpConsensusState()
}

func UnconfirmedTxsResult() (ctypes.TMResult, error) {
	return UnconfirmedTxs()
}

func NumUnconfirmedTxsResult() (ctypes.TMResult, error) {
	return NumUnconfirmedTxs()
}

// Tx allow user to query the transaction results. `nil` could mean the
// transaction is in the mempool, invalidated, or was not send in the first
// place.
func TxResult(hash []byte) (ctypes.TMResult, error) {
	return Tx(hash)
}

func BroadcastTxCommitResult(tx []byte) (ctypes.TMResult, error) {
	return BroadcastTxCommit(tx)
}

func BroadcastTxSyncResult(tx []byte) (ctypes.TMResult, error) {
	return BroadcastTxSync(tx)
}

func BroadcastTxAsyncResult(tx []byte) (ctypes.TMResult, error) {
	return BroadcastTxAsync(tx)
}

func ABCIQueryResult(path string, data []byte, prove bool) (ctypes.TMResult, error) {
	return ABCIQuery(path, data, prove)
}

func ABCIInfoResult() (ctypes.TMResult, error) {
	return ABCIInfo()
}

func UnsafeFlushMempoolResult() (ctypes.TMResult, error) {
	return UnsafeFlushMempool()
}

func UnsafeSetConfigResult(typ, key, value string) (ctypes.TMResult, error) {
	return UnsafeSetConfig(typ, key, value)
}

func UnsafeStartCPUProfilerResult(filename string) (ctypes.TMResult, error) {
	return UnsafeStartCPUProfiler(filename)
}

func UnsafeStopCPUProfilerResult() (ctypes.TMResult, error) {
	return UnsafeStopCPUProfiler()
}

func UnsafeWriteHeapProfileResult(filename string) (ctypes.TMResult, error) {
	return UnsafeWriteHeapProfile(filename)
}
