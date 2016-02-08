package core

import (
	rpc "github.com/tendermint/go-rpc/server"
	"github.com/tendermint/go-rpc/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

// TODO: eliminate redundancy between here and reading code from core/
var Routes = map[string]*rpc.RPCFunc{
	"subscribe":            rpc.NewWSRPCFunc(SubscribeResult, "event"),
	"unsubscribe":          rpc.NewWSRPCFunc(UnsubscribeResult, "event"),
	"status":               rpc.NewRPCFunc(StatusResult, ""),
	"net_info":             rpc.NewRPCFunc(NetInfoResult, ""),
	"dial_seeds":           rpc.NewRPCFunc(DialSeedsResult, "seeds"),
	"blockchain":           rpc.NewRPCFunc(BlockchainInfoResult, "minHeight,maxHeight"),
	"genesis":              rpc.NewRPCFunc(GenesisResult, ""),
	"block":                rpc.NewRPCFunc(BlockResult, "height"),
	"validators":           rpc.NewRPCFunc(ValidatorsResult, ""),
	"dump_consensus_state": rpc.NewRPCFunc(DumpConsensusStateResult, ""),
	"broadcast_tx_sync":    rpc.NewRPCFunc(BroadcastTxSyncResult, "tx"),
	"broadcast_tx_async":   rpc.NewRPCFunc(BroadcastTxAsyncResult, "tx"),
	"unconfirmed_txs":      rpc.NewRPCFunc(UnconfirmedTxsResult, ""),
	// subscribe/unsubscribe are reserved for websocket events.
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

func DialSeedsResult(seeds []string) (ctypes.TMResult, error) {
	if r, err := DialSeeds(seeds); err != nil {
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
