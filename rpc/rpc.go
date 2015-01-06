package rpc

import (
	block_ "github.com/tendermint/tendermint/block"
	mempool_ "github.com/tendermint/tendermint/mempool"
	state_ "github.com/tendermint/tendermint/state"
)

var blockStore *block_.BlockStore
var state *state_.State
var mempoolReactor *mempool_.MempoolReactor

func SetRPCBlockStore(bs *block_.BlockStore) {
	blockStore = bs
}

func SetRPCState(s *state_.State) {
	state = s
}

func SetRPCMempoolReactor(mr *mempool_.MempoolReactor) {
	mempoolReactor = mr
}
