package rpc

import (
	blk "github.com/tendermint/tendermint/block"
	"github.com/tendermint/tendermint/consensus"
	mempl "github.com/tendermint/tendermint/mempool"
)

var blockStore *blk.BlockStore
var consensusState *consensus.ConsensusState
var mempoolReactor *mempl.MempoolReactor

func SetRPCBlockStore(bs *blk.BlockStore) {
	blockStore = bs
}

func SetRPCConsensusState(cs *consensus.ConsensusState) {
	consensusState = cs
}

func SetRPCMempoolReactor(mr *mempl.MempoolReactor) {
	mempoolReactor = mr
}
