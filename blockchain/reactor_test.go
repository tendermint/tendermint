package blockchain

import (
	"os"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	abci "github.com/tendermint/tendermint/abci/types"
	cfg "github.com/tendermint/tendermint/config"
	cmn "github.com/tendermint/tendermint/libs/common"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/libs/log"
	memplmock "github.com/tendermint/tendermint/mempool/mock"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

var config *cfg.Config

func randGenesisDoc(numValidators int, randPower bool, minPower int64) (*types.GenesisDoc, []types.PrivValidator) {
	validators := make([]types.GenesisValidator, numValidators)
	privValidators := make([]types.PrivValidator, numValidators)
	for i := 0; i < numValidators; i++ {
		val, privVal := types.RandValidator(randPower, minPower)
		validators[i] = types.GenesisValidator{
			PubKey: val.PubKey,
			Power:  val.VotingPower,
		}
		privValidators[i] = privVal
	}
	sort.Sort(types.PrivValidatorsByAddress(privValidators))

	return &types.GenesisDoc{
		GenesisTime: tmtime.Now(),
		ChainID:     config.ChainID(),
		Validators:  validators,
	}, privValidators
}

func makeVote(header *types.Header, blockID types.BlockID, valset *types.ValidatorSet, privVal types.PrivValidator) *types.Vote {
	addr := privVal.GetPubKey().Address()
	idx, _ := valset.GetByAddress(addr)
	vote := &types.Vote{
		ValidatorAddress: addr,
		ValidatorIndex:   idx,
		Height:           header.Height,
		Round:            1,
		Timestamp:        tmtime.Now(),
		Type:             types.PrecommitType,
		BlockID:          blockID,
	}

	privVal.SignVote(header.ChainID, vote)

	return vote
}

type BlockchainReactorPair struct {
	reactor *BlockchainReactor
	app     proxy.AppConns
}

func newBlockchainReactor(logger log.Logger, genDoc *types.GenesisDoc, privVals []types.PrivValidator, maxBlockHeight int64) BlockchainReactorPair {
	if len(privVals) != 1 {
		panic("only support one validator")
	}

	app := &testApp{}
	cc := proxy.NewLocalClientCreator(app)
	proxyApp := proxy.NewAppConns(cc)
	err := proxyApp.Start()
	if err != nil {
		panic(cmn.ErrorWrap(err, "error start app"))
	}

	blockDB := dbm.NewMemDB()
	stateDB := dbm.NewMemDB()
	blockStore := NewBlockStore(blockDB)

	state, err := sm.LoadStateFromDBOrGenesisDoc(stateDB, genDoc)
	if err != nil {
		panic(cmn.ErrorWrap(err, "error constructing state from genesis file"))
	}

	// Make the BlockchainReactor itself.
	// NOTE we have to create and commit the blocks first because
	// pool.height is determined from the store.
	fastSync := true
	blockExec := sm.NewBlockExecutor(dbm.NewMemDB(), log.TestingLogger(), proxyApp.Consensus(),
		memplmock.Mempool{}, sm.MockEvidencePool{})

	// let's add some blocks in
	for blockHeight := int64(1); blockHeight <= maxBlockHeight; blockHeight++ {
		lastCommit := types.NewCommit(types.BlockID{}, nil)
		if blockHeight > 1 {
			lastBlockMeta := blockStore.LoadBlockMeta(blockHeight - 1)
			lastBlock := blockStore.LoadBlock(blockHeight - 1)

			vote := makeVote(&lastBlock.Header, lastBlockMeta.BlockID, state.Validators, privVals[0]).CommitSig()
			lastCommit = types.NewCommit(lastBlockMeta.BlockID, []*types.CommitSig{vote})
		}

		thisBlock := makeBlock(blockHeight, state, lastCommit)

		thisParts := thisBlock.MakePartSet(types.BlockPartSizeBytes)
		blockID := types.BlockID{thisBlock.Hash(), thisParts.Header()}

		state, err = blockExec.ApplyBlock(state, blockID, thisBlock)
		if err != nil {
			panic(cmn.ErrorWrap(err, "error apply block"))
		}

		blockStore.SaveBlock(thisBlock, thisParts, lastCommit)
	}

	bcReactor := NewBlockchainReactor(state.Copy(), blockExec, blockStore, fastSync)
	bcReactor.SetLogger(logger.With("module", "blockchain"))

	return BlockchainReactorPair{bcReactor, proxyApp}
}

func TestNoBlockResponse(t *testing.T) {
	config = cfg.ResetTestRoot("blockchain_reactor_test")
	defer os.RemoveAll(config.RootDir)
	genDoc, privVals := randGenesisDoc(1, false, 30)

	maxBlockHeight := int64(65)

	reactorPairs := make([]BlockchainReactorPair, 2)

	reactorPairs[0] = newBlockchainReactor(log.TestingLogger(), genDoc, privVals, maxBlockHeight)
	reactorPairs[1] = newBlockchainReactor(log.TestingLogger(), genDoc, privVals, 0)

	p2p.MakeConnectedSwitches(config.P2P, 2, func(i int, s *p2p.Switch) *p2p.Switch {
		s.AddReactor("BLOCKCHAIN", reactorPairs[i].reactor)
		return s

	}, p2p.Connect2Switches)

	defer func() {
		for _, r := range reactorPairs {
			r.reactor.Stop()
			r.app.Stop()
		}
	}()

	tests := []struct {
		height   int64
		existent bool
	}{
		{maxBlockHeight + 2, false},
		{10, true},
		{1, true},
		{100, false},
	}

	for {
		if reactorPairs[1].reactor.pool.IsCaughtUp() {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}

	assert.Equal(t, maxBlockHeight, reactorPairs[0].reactor.store.Height())

	for _, tt := range tests {
		block := reactorPairs[1].reactor.store.LoadBlock(tt.height)
		if tt.existent {
			assert.True(t, block != nil)
		} else {
			assert.True(t, block == nil)
		}
	}
}

// NOTE: This is too hard to test without
// an easy way to add test peer to switch
// or without significant refactoring of the module.
// Alternatively we could actually dial a TCP conn but
// that seems extreme.
func TestBadBlockStopsPeer(t *testing.T) {
	config = cfg.ResetTestRoot("blockchain_reactor_test")
	defer os.RemoveAll(config.RootDir)
	genDoc, privVals := randGenesisDoc(1, false, 30)

	maxBlockHeight := int64(148)

	otherChain := newBlockchainReactor(log.TestingLogger(), genDoc, privVals, maxBlockHeight)
	defer func() {
		otherChain.reactor.Stop()
		otherChain.app.Stop()
	}()

	reactorPairs := make([]BlockchainReactorPair, 4)

	reactorPairs[0] = newBlockchainReactor(log.TestingLogger(), genDoc, privVals, maxBlockHeight)
	reactorPairs[1] = newBlockchainReactor(log.TestingLogger(), genDoc, privVals, 0)
	reactorPairs[2] = newBlockchainReactor(log.TestingLogger(), genDoc, privVals, 0)
	reactorPairs[3] = newBlockchainReactor(log.TestingLogger(), genDoc, privVals, 0)

	switches := p2p.MakeConnectedSwitches(config.P2P, 4, func(i int, s *p2p.Switch) *p2p.Switch {
		s.AddReactor("BLOCKCHAIN", reactorPairs[i].reactor)
		return s

	}, p2p.Connect2Switches)

	defer func() {
		for _, r := range reactorPairs {
			r.reactor.Stop()
			r.app.Stop()
		}
	}()

	for {
		if reactorPairs[3].reactor.pool.IsCaughtUp() {
			break
		}

		time.Sleep(1 * time.Second)
	}

	//at this time, reactors[0-3] is the newest
	assert.Equal(t, 3, reactorPairs[1].reactor.Switch.Peers().Size())

	//mark reactorPairs[3] is an invalid peer
	reactorPairs[3].reactor.store = otherChain.reactor.store

	lastReactorPair := newBlockchainReactor(log.TestingLogger(), genDoc, privVals, 0)
	reactorPairs = append(reactorPairs, lastReactorPair)

	switches = append(switches, p2p.MakeConnectedSwitches(config.P2P, 1, func(i int, s *p2p.Switch) *p2p.Switch {
		s.AddReactor("BLOCKCHAIN", reactorPairs[len(reactorPairs)-1].reactor)
		return s

	}, p2p.Connect2Switches)...)

	for i := 0; i < len(reactorPairs)-1; i++ {
		p2p.Connect2Switches(switches, i, len(reactorPairs)-1)
	}

	for {
		if lastReactorPair.reactor.pool.IsCaughtUp() || lastReactorPair.reactor.Switch.Peers().Size() == 0 {
			break
		}

		time.Sleep(1 * time.Second)
	}

	assert.True(t, lastReactorPair.reactor.Switch.Peers().Size() < len(reactorPairs)-1)
}

//----------------------------------------------
// utility funcs

func makeTxs(height int64) (txs []types.Tx) {
	for i := 0; i < 10; i++ {
		txs = append(txs, types.Tx([]byte{byte(height), byte(i)}))
	}
	return txs
}

func makeBlock(height int64, state sm.State, lastCommit *types.Commit) *types.Block {
	block, _ := state.MakeBlock(height, makeTxs(height), lastCommit, nil, state.Validators.GetProposer().Address)
	return block
}

type testApp struct {
	abci.BaseApplication
}

var _ abci.Application = (*testApp)(nil)

func (app *testApp) Info(req abci.RequestInfo) (resInfo abci.ResponseInfo) {
	return abci.ResponseInfo{}
}

func (app *testApp) BeginBlock(req abci.RequestBeginBlock) abci.ResponseBeginBlock {
	return abci.ResponseBeginBlock{}
}

func (app *testApp) EndBlock(req abci.RequestEndBlock) abci.ResponseEndBlock {
	return abci.ResponseEndBlock{}
}

func (app *testApp) DeliverTx(tx []byte) abci.ResponseDeliverTx {
	return abci.ResponseDeliverTx{Tags: []cmn.KVPair{}}
}

func (app *testApp) CheckTx(tx []byte) abci.ResponseCheckTx {
	return abci.ResponseCheckTx{}
}

func (app *testApp) Commit() abci.ResponseCommit {
	return abci.ResponseCommit{}
}

func (app *testApp) Query(reqQuery abci.RequestQuery) (resQuery abci.ResponseQuery) {
	return
}
