package core

import (
	"github.com/tendermint/go-p2p"
	bc "github.com/tendermint/tendermint/blockchain"
	"github.com/tendermint/tendermint/consensus"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/types"
)

var blockStore *bc.BlockStore
var consensusState *consensus.ConsensusState
var consensusReactor *consensus.ConsensusReactor
var mempoolReactor *mempl.MempoolReactor
var p2pSwitch *p2p.Switch
var privValidator *types.PrivValidator
var genDoc *types.GenesisDoc // cache the genesis structure

func SetBlockStore(bs *bc.BlockStore) {
	blockStore = bs
}

func SetConsensusState(cs *consensus.ConsensusState) {
	consensusState = cs
}

func SetConsensusReactor(cr *consensus.ConsensusReactor) {
	consensusReactor = cr
}

func SetMempoolReactor(mr *mempl.MempoolReactor) {
	mempoolReactor = mr
}

func SetSwitch(sw *p2p.Switch) {
	p2pSwitch = sw
}

func SetPrivValidator(pv *types.PrivValidator) {
	privValidator = pv
}

func SetGenesisDoc(doc *types.GenesisDoc) {
	genDoc = doc
}
