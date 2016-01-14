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
	"blockchain":           rpc.NewRPCFunc(BlockchainInfoResult, "minHeight,maxHeight"),
	"genesis":              rpc.NewRPCFunc(GenesisResult, ""),
	"get_block":            rpc.NewRPCFunc(GetBlockResult, "height"),
	"list_validators":      rpc.NewRPCFunc(ListValidatorsResult, ""),
	"dump_consensus_state": rpc.NewRPCFunc(DumpConsensusStateResult, ""),
	"broadcast_tx":         rpc.NewRPCFunc(BroadcastTxResult, "tx"),
	"list_unconfirmed_txs": rpc.NewRPCFunc(ListUnconfirmedTxsResult, ""),
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

func GetBlockResult(height int) (ctypes.TMResult, error) {
	if r, err := GetBlock(height); err != nil {
		return nil, err
	} else {
		return r, nil
	}
}

func ListValidatorsResult() (ctypes.TMResult, error) {
	if r, err := ListValidators(); err != nil {
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

func ListUnconfirmedTxsResult() (ctypes.TMResult, error) {
	if r, err := ListUnconfirmedTxs(); err != nil {
		return nil, err
	} else {
		return r, nil
	}
}

func BroadcastTxResult(tx []byte) (ctypes.TMResult, error) {
	if r, err := BroadcastTx(tx); err != nil {
		return nil, err
	} else {
		return r, nil
	}
}
