package core

import (
	bc "github.com/tendermint/tendermint/blockchain"
	"github.com/tendermint/tendermint/consensus"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	stypes "github.com/tendermint/tendermint/state/types"
	"github.com/tendermint/tendermint/types"
)

var blockStore *bc.BlockStore
var consensusState *consensus.ConsensusState
var consensusReactor *consensus.ConsensusReactor
var mempoolReactor *mempl.MempoolReactor
var p2pSwitch *p2p.Switch
var privValidator *types.PrivValidator
var genDoc *stypes.GenesisDoc // cache the genesis structure

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

func SetGenDoc(doc *stypes.GenesisDoc) {
	genDoc = doc
}
