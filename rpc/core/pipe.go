package core

import (
	cfg "github.com/tendermint/go-config"
	"github.com/tendermint/go-p2p"

	"github.com/tendermint/go-events"
	bc "github.com/tendermint/tendermint/blockchain"
	"github.com/tendermint/tendermint/consensus"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
)

var eventSwitch *events.EventSwitch
var blockStore *bc.BlockStore
var consensusState *consensus.ConsensusState
var consensusReactor *consensus.ConsensusReactor
var mempoolReactor *mempl.MempoolReactor
var p2pSwitch *p2p.Switch
var privValidator *types.PrivValidator
var genDoc *types.GenesisDoc // cache the genesis structure
var proxyAppQuery proxy.AppConnQuery

var config cfg.Config = nil

func SetConfig(c cfg.Config) {
	config = c
}

func SetEventSwitch(evsw *events.EventSwitch) {
	eventSwitch = evsw
}

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

func SetProxyAppQuery(appConn proxy.AppConnQuery) {
	proxyAppQuery = appConn
}
