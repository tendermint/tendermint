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
	if r, err := Subscribe(wsCtx, event); err != nil {
		return nil, err
	} else {
		return r, nil
	}
}

func UnsubscribeResult(wsCtx rpctypes.WSRPCContext, event string) (ctypes.TMResult, error) {
	if r, err := Unsubscribe(wsCtx, event); err != nil {
		return nil, err
	} else {
		return r, nil
	}
}

func StatusResult() (ctypes.TMResult, error) {
	if r, err := Status(); err != nil {
		return nil, err
	} else {
		return r, nil
	}
}

func NetInfoResult() (ctypes.TMResult, error) {
	if r, err := NetInfo(); err != nil {
		return nil, err
	} else {
		return r, nil
	}
}

func UnsafeDialSeedsResult(seeds []string) (ctypes.TMResult, error) {
	if r, err := UnsafeDialSeeds(seeds); err != nil {
		return nil, err
	} else {
		return r, nil
	}
}

func BlockchainInfoResult(min, max int) (ctypes.TMResult, error) {
	if r, err := BlockchainInfo(min, max); err != nil {
		return nil, err
	} else {
		return r, nil
	}
}

func GenesisResult() (ctypes.TMResult, error) {
	if r, err := Genesis(); err != nil {
		return nil, err
	} else {
		return r, nil
	}
}

func BlockResult(height int) (ctypes.TMResult, error) {
	if r, err := Block(height); err != nil {
		return nil, err
	} else {
		return r, nil
	}
}

func CommitResult(height int) (ctypes.TMResult, error) {
	if r, err := Commit(height); err != nil {
		return nil, err
	} else {
		return r, nil
	}
}

func ValidatorsResult() (ctypes.TMResult, error) {
	if r, err := Validators(); err != nil {
		return nil, err
	} else {
		return r, nil
	}
}

func DumpConsensusStateResult() (ctypes.TMResult, error) {
	if r, err := DumpConsensusState(); err != nil {
		return nil, err
	} else {
		return r, nil
	}
}

func UnconfirmedTxsResult() (ctypes.TMResult, error) {
	if r, err := UnconfirmedTxs(); err != nil {
		return nil, err
	} else {
		return r, nil
	}
}

func NumUnconfirmedTxsResult() (ctypes.TMResult, error) {
	if r, err := NumUnconfirmedTxs(); err != nil {
		return nil, err
	} else {
		return r, nil
	}
}

func BroadcastTxCommitResult(tx []byte) (ctypes.TMResult, error) {
	if r, err := BroadcastTxCommit(tx); err != nil {
		return nil, err
	} else {
		return r, nil
	}
}

func BroadcastTxSyncResult(tx []byte) (ctypes.TMResult, error) {
	if r, err := BroadcastTxSync(tx); err != nil {
		return nil, err
	} else {
		return r, nil
	}
}

func BroadcastTxAsyncResult(tx []byte) (ctypes.TMResult, error) {
	if r, err := BroadcastTxAsync(tx); err != nil {
		return nil, err
	} else {
		return r, nil
	}
}

func ABCIQueryResult(path string, data []byte, prove bool) (ctypes.TMResult, error) {
	if r, err := ABCIQuery(path, data, prove); err != nil {
		return nil, err
	} else {
		return r, nil
	}
}

func ABCIInfoResult() (ctypes.TMResult, error) {
	if r, err := ABCIInfo(); err != nil {
		return nil, err
	} else {
		return r, nil
	}
}

func UnsafeFlushMempoolResult() (ctypes.TMResult, error) {
	if r, err := UnsafeFlushMempool(); err != nil {
		return nil, err
	} else {
		return r, nil
	}
}

func UnsafeSetConfigResult(typ, key, value string) (ctypes.TMResult, error) {
	if r, err := UnsafeSetConfig(typ, key, value); err != nil {
		return nil, err
	} else {
		return r, nil
	}
}

func UnsafeStartCPUProfilerResult(filename string) (ctypes.TMResult, error) {
	if r, err := UnsafeStartCPUProfiler(filename); err != nil {
		return nil, err
	} else {
		return r, nil
	}
}

func UnsafeStopCPUProfilerResult() (ctypes.TMResult, error) {
	if r, err := UnsafeStopCPUProfiler(); err != nil {
		return nil, err
	} else {
		return r, nil
	}
}

func UnsafeWriteHeapProfileResult(filename string) (ctypes.TMResult, error) {
	if r, err := UnsafeWriteHeapProfile(filename); err != nil {
		return nil, err
	} else {
		return r, nil
	}
}
