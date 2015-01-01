package rpc

import (
	mempool_ "github.com/tendermint/tendermint/mempool"
	state_ "github.com/tendermint/tendermint/state"
)

var state *state_.State
var mempoolReactor *mempool_.MempoolReactor

func SetRPCState(state__ *state_.State) {
	state = state__
}

func SetRPCMempoolReactor(mempoolReactor_ *mempool_.MempoolReactor) {
	mempoolReactor = mempoolReactor_
}
