package core

import (
	bc "github.com/tendermint/tendermint2/blockchain"
	"github.com/tendermint/tendermint2/consensus"
	mempl "github.com/tendermint/tendermint2/mempool"
	"github.com/tendermint/tendermint2/p2p"
)

var blockStore *bc.BlockStore
var consensusState *consensus.ConsensusState
var mempoolReactor *mempl.MempoolReactor
var p2pSwitch *p2p.Switch

func SetBlockStore(bs *bc.BlockStore) {
	blockStore = bs
}

func SetConsensusState(cs *consensus.ConsensusState) {
	consensusState = cs
}

func SetMempoolReactor(mr *mempl.MempoolReactor) {
	mempoolReactor = mr
}

func SetSwitch(sw *p2p.Switch) {
	p2pSwitch = sw
}
