package rpc

import (
	bc "github.com/tendermint/tendermint/blockchain"
	"github.com/tendermint/tendermint/consensus"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
)

var blockStore *bc.BlockStore
var consensusState *consensus.ConsensusState
var mempoolReactor *mempl.MempoolReactor
var p2pSwitch *p2p.Switch

func SetRPCBlockStore(bs *bc.BlockStore) {
	blockStore = bs
}

func SetRPCConsensusState(cs *consensus.ConsensusState) {
	consensusState = cs
}

func SetRPCMempoolReactor(mr *mempl.MempoolReactor) {
	mempoolReactor = mr
}

func SetRPCSwitch(sw *p2p.Switch) {
	p2pSwitch = sw
}
