package rpc

import (
	block_ "github.com/tendermint/tendermint/block"
	"github.com/tendermint/tendermint/consensus"
	mempool_ "github.com/tendermint/tendermint/mempool"
)

var blockStore *block_.BlockStore
var consensusState *consensus.ConsensusState
var mempoolReactor *mempool_.MempoolReactor

func SetRPCBlockStore(bs *block_.BlockStore) {
	blockStore = bs
}

func SetRPCConsensusState(cs *consensus.ConsensusState) {
	consensusState = cs
}

func SetRPCMempoolReactor(mr *mempool_.MempoolReactor) {
	mempoolReactor = mr
}
