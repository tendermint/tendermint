// Copyright 2015 Tendermint. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package core

import (
	rpc "github.com/tendermint/tendermint/rpc/lib/server"
)

// TODO: better system than "unsafe" prefix
var Routes = map[string]*rpc.RPCFunc{
	// subscribe/unsubscribe are reserved for websocket events.
	"subscribe":   rpc.NewWSRPCFunc(Subscribe, "event"),
	"unsubscribe": rpc.NewWSRPCFunc(Unsubscribe, "event"),

	// info API
	"status":               rpc.NewRPCFunc(Status, ""),
	"net_info":             rpc.NewRPCFunc(NetInfo, ""),
	"blockchain":           rpc.NewRPCFunc(BlockchainInfo, "minHeight,maxHeight"),
	"genesis":              rpc.NewRPCFunc(Genesis, ""),
	"block":                rpc.NewRPCFunc(Block, "height"),
	"commit":               rpc.NewRPCFunc(Commit, "height"),
	"tx":                   rpc.NewRPCFunc(Tx, "hash,prove"),
	"validators":           rpc.NewRPCFunc(Validators, "height"),
	"dump_consensus_state": rpc.NewRPCFunc(DumpConsensusState, ""),
	"unconfirmed_txs":      rpc.NewRPCFunc(UnconfirmedTxs, ""),
	"num_unconfirmed_txs":  rpc.NewRPCFunc(NumUnconfirmedTxs, ""),

	// broadcast API
	"broadcast_tx_commit": rpc.NewRPCFunc(BroadcastTxCommit, "tx"),
	"broadcast_tx_sync":   rpc.NewRPCFunc(BroadcastTxSync, "tx"),
	"broadcast_tx_async":  rpc.NewRPCFunc(BroadcastTxAsync, "tx"),

	// abci API
	"abci_query": rpc.NewRPCFunc(ABCIQuery, "path,data,prove"),
	"abci_info":  rpc.NewRPCFunc(ABCIInfo, ""),
}

func AddUnsafeRoutes() {
	// control API
	Routes["dial_seeds"] = rpc.NewRPCFunc(UnsafeDialSeeds, "seeds")
	Routes["unsafe_flush_mempool"] = rpc.NewRPCFunc(UnsafeFlushMempool, "")

	// profiler API
	Routes["unsafe_start_cpu_profiler"] = rpc.NewRPCFunc(UnsafeStartCPUProfiler, "filename")
	Routes["unsafe_stop_cpu_profiler"] = rpc.NewRPCFunc(UnsafeStopCPUProfiler, "")
	Routes["unsafe_write_heap_profile"] = rpc.NewRPCFunc(UnsafeWriteHeapProfile, "filename")
}
