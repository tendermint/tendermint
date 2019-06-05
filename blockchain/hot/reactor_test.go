package hot

import (
	"os"
	"testing"
	"time"

	"github.com/tendermint/tendermint/mock"

	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tendermint/blockchain"
	cfg "github.com/tendermint/tendermint/config"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-cmn/db"
)

func TestHotSyncReactorBasic(t *testing.T) {
	config = cfg.ResetTestRoot("blockchain_reactor_test")
	defer os.RemoveAll(config.RootDir)
	genDoc, privVals := randGenesisDoc(1, false, 30)

	maxBlockHeight := int64(65)

	reactorPairs := make([]BlockChainReactorPair, 2)

	eventBus1 := types.NewEventBus()
	err := eventBus1.Start()
	defer eventBus1.Stop()
	assert.NoError(t, err)
	eventBus2 := types.NewEventBus()
	err = eventBus2.Start()
	defer eventBus2.Stop()
	assert.NoError(t, err)
	reactorPairs[0] = newBlockchainReactorPair(log.TestingLogger(), genDoc, privVals, maxBlockHeight, eventBus1, true, false)
	reactorPairs[1] = newBlockchainReactorPair(log.TestingLogger(), genDoc, privVals, 0, eventBus2, true, false)

	p2p.MakeConnectedSwitches(config.P2P, 2, func(i int, s *p2p.Switch) *p2p.Switch {
		s.AddReactor("hot", reactorPairs[i].reactor)
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
		if reactorPairs[1].reactor.pool.currentHeight() == maxBlockHeight-1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	for _, tt := range tests {
		block := reactorPairs[1].reactor.pool.store.LoadBlock(tt.height)
		if tt.existent {
			assert.True(t, block != nil)
		} else {
			assert.True(t, block == nil)
		}
	}
}

func TestHotSyncReactorSwitch(t *testing.T) {
	config = cfg.ResetTestRoot("blockchain_reactor_test")
	defer os.RemoveAll(config.RootDir)
	genDoc, privVals := randGenesisDoc(1, false, 30)

	maxBlockHeight := int64(65)

	reactorPairs := make([]BlockChainReactorPair, 2)

	eventBus0 := types.NewEventBus()
	err := eventBus0.Start()
	defer eventBus0.Stop()
	assert.NoError(t, err)
	eventBus1 := types.NewEventBus()
	err = eventBus1.Start()
	defer eventBus1.Stop()
	assert.NoError(t, err)
	reactorPairs[0] = newBlockchainReactorPair(log.TestingLogger(), genDoc, privVals, maxBlockHeight, eventBus0, true, false)
	reactorPairs[1] = newBlockchainReactorPair(log.TestingLogger(), genDoc, privVals, 0, eventBus1, true, true)

	p2p.MakeConnectedSwitches(config.P2P, 2, func(i int, s *p2p.Switch) *p2p.Switch {
		s.AddReactor("hot", reactorPairs[i].reactor)
		return s

	}, p2p.Connect2Switches)

	defer func() {
		for _, r := range reactorPairs {
			r.reactor.Stop()
			r.app.Stop()
		}
	}()

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, reactorPairs[1].reactor.pool.currentHeight(), int64(0))

	for i := int64(0); i < 10; i++ {
		first := reactorPairs[0].reactor.pool.store.LoadBlock(i + 1)
		bId := makeBlockID(first)
		second := reactorPairs[0].reactor.pool.store.LoadBlock(i + 2)
		reactorPairs[1].reactor.pool.applyBlock(&blockState{block: first, commit: second.LastCommit, blockId: bId})
	}

	reactorPairs[1].reactor.SwitchToHotSync(reactorPairs[1].reactor.pool.state, 10)

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
		if reactorPairs[1].reactor.pool.currentHeight() == maxBlockHeight-1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	for _, tt := range tests {
		block := reactorPairs[1].reactor.pool.store.LoadBlock(tt.height)
		if tt.existent {
			assert.True(t, block != nil)
		} else {
			assert.True(t, block == nil)
		}
	}

	reactorPairs[0].reactor.SwitchToConsensusSync(reactorPairs[0].reactor.pool.state)

	pool := reactorPairs[0].reactor.pool
	consensusHeight := int64(100)
	for i := int64(0); i < consensusHeight; i++ {
		height := pool.currentHeight()
		block, commit, vs := nextBlock(pool.state, pool.store, pool.blockExec, reactorPairs[0].privVals[0])

		err := eventBus0.PublishEventCompleteProposal(types.EventDataCompleteProposal{Height: block.Height, Block: *block})
		assert.NoError(t, err)
		err = eventBus0.PublishEventMajorPrecommits(types.EventDataMajorPrecommits{Height: block.Height, Votes: *vs})
		assert.NoError(t, err)
		pool.applyBlock(&blockState{
			block:   block,
			commit:  commit,
			blockId: makeBlockID(block),
		})
		for {
			if height+1 != pool.currentHeight() {
				time.Sleep(10 * time.Millisecond)
			} else {
				break
			}
		}
	}

	tests2 := []struct {
		height   int64
		existent bool
	}{
		{reactorPairs[1].reactor.pool.currentHeight(), true},
		{122, true},
		{50, true},
		{200, false},
	}

	for {
		if reactorPairs[1].reactor.pool.currentHeight() == maxBlockHeight+consensusHeight {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	for _, tt := range tests2 {
		block := reactorPairs[1].reactor.pool.store.LoadBlock(tt.height)
		if tt.existent {
			assert.True(t, block != nil)
		} else {
			assert.True(t, block == nil)
		}
	}
}

type BlockChainReactorPair struct {
	reactor  *BlockchainReactor
	app      proxy.AppConns
	privVals []types.PrivValidator
}

func newBlockchainReactorPair(logger log.Logger, genDoc *types.GenesisDoc, privVals []types.PrivValidator, maxBlockHeight int64, eventBus *types.EventBus, hotsync, fastSync bool) BlockChainReactorPair {
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
	blockStore := blockchain.NewBlockStore(blockDB)
	state, err := sm.LoadStateFromDBOrGenesisDoc(stateDB, genDoc)
	if err != nil {
		panic(cmn.ErrorWrap(err, "error constructing state from genesis file"))
	}

	// Make the BlockPool itself.
	// NOTE we have to create and commit the blocks first because
	// pool.height is determined from the store.
	db := dbm.NewMemDB()
	blockExec := sm.NewBlockExecutor(db, log.TestingLogger(), proxyApp.Consensus(),
		mock.Mempool{}, sm.MockEvidencePool{})
	sm.SaveState(db, state)
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
	bcReactor := NewBlockChainReactor(state.Copy(), blockExec, blockStore, hotsync, fastSync, 2*time.Second, WithEventBus(eventBus))
	bcReactor.SetLogger(logger.With("module", "hotsync"))
	return BlockChainReactorPair{bcReactor, proxyApp, privVals}
}
