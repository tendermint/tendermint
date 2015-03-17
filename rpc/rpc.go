package rpc

import (
	blk "github.com/tendermint/tendermint/block"
	"github.com/tendermint/tendermint/consensus"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
)

var blockStore *blk.BlockStore
var consensusState *consensus.ConsensusState
var mempoolReactor *mempl.MempoolReactor
var p2pSwitch *p2p.Switch

func SetRPCBlockStore(bs *blk.BlockStore) {
	blockStore = bs
}

func SetRPCConsensusState(cs *consensus.ConsensusState) {
	consensusState = cs
}

func SetRPCMempoolReactor(mr *mempl.MempoolReactor) {
	mempoolReactor = mr
}

func SetSwitch(sw *p2p.Switch) {
	p2pSwitch = sw
}
