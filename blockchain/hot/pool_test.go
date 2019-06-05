package hot

import (
	"os"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/blockchain"
	cfg "github.com/tendermint/tendermint/config"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/mock"
	pmock "github.com/tendermint/tendermint/p2p/mock"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
	dbm "github.com/tendermint/tm-cmn/db"
)

var (
	config *cfg.Config
)

func TestBlockPoolHotSyncBasic(t *testing.T) {
	// prepare
	config = cfg.ResetTestRoot(t.Name())
	defer os.RemoveAll(config.RootDir)
	genDoc, privVals := randGenesisDoc(1, false, 30)
	initBlockHeight := int64(10)
	poolPair := newBlockchainPoolPair(log.TestingLogger(), genDoc, privVals, initBlockHeight)

	pool := poolPair.pool
	messageChan := poolPair.messageQueue
	pool.Start()
	defer pool.Stop()
	testPeer := pmock.NewPeer(nil)
	pool.AddPeer(testPeer)

	// consume subscribe message
	<-messageChan

	//handle subscribe message
	subscriber := pmock.NewPeer(nil)
	totalTestBlock := int64(10)
	pool.handleSubscribeBlock(initBlockHeight+1, initBlockHeight+totalTestBlock, subscriber)

	for i := int64(0); i < totalTestBlock; i++ {
		height := pool.currentHeight()
		block, commit, _ := nextBlock(poolPair.pool.state, poolPair.pool.store, poolPair.pool.blockExec, poolPair.privVals[0])
		expectedBlockMess1 := blockCommitResponseMessage{Block: block}
		expectedBlockMess2 := blockCommitResponseMessage{Commit: commit}
		pool.handleBlockCommit(block, commit, testPeer)
		var receive1, receive2 bool
		for m := range messageChan {
			if bm, ok := m.blockChainMessage.(*blockCommitResponseMessage); ok {
				if !receive1 {
					assert.Equal(t, *bm, expectedBlockMess1)
					receive1 = true
				} else if !receive2 {
					assert.Equal(t, *bm, expectedBlockMess2)
					receive2 = true
				}
				if receive1 && receive2 {
					break
				}
			}
		}
		for {
			if height+1 != pool.currentHeight() {
				time.Sleep(10 * time.Millisecond)
			} else {
				break
			}
		}
	}
	assert.Equal(t, int64(pool.blocksSynced), totalTestBlock)
}

func TestBlockPoolHotSyncTimeout(t *testing.T) {
	// prepare
	config = cfg.ResetTestRoot(t.Name())
	defer os.RemoveAll(config.RootDir)
	genDoc, privVals := randGenesisDoc(1, false, 30)
	initBlockHeight := int64(10)
	poolPair := newBlockchainPoolPair(log.TestingLogger(), genDoc, privVals, initBlockHeight)

	pool := poolPair.pool
	messageChan := poolPair.messageQueue
	pool.Start()
	defer pool.Stop()
	testPeer := pmock.NewPeer(nil)
	pool.AddPeer(testPeer)

	// send subscribe message for several times
	expectedMessages := make([]BlockchainMessage, 0, 1+2*maxSubscribeForesight)
	expectedMessages = append(expectedMessages, &blockSubscribeMessage{FromHeight: initBlockHeight + 1, ToHeight: initBlockHeight + maxSubscribeForesight})
	for j := 0; j < 3; j++ {
		for i := int64(0); i < maxSubscribeForesight; i++ {
			expectedMessages = append(expectedMessages, &blockUnSubscribeMessage{Height: initBlockHeight + i + 1})
			expectedMessages = append(expectedMessages, &blockSubscribeMessage{FromHeight: initBlockHeight + i + 1, ToHeight: initBlockHeight + i + 1})
		}
	}
	for _, expect := range expectedMessages {
		m := <-messageChan
		assert.Equal(t, m.blockChainMessage, expect)
	}
}

func TestBlockPoolHotSyncSubscribePastBlock(t *testing.T) {
	config = cfg.ResetTestRoot(t.Name())
	defer os.RemoveAll(config.RootDir)
	genDoc, privVals := randGenesisDoc(1, false, 30)
	initBlockHeight := int64(100)
	poolPair := newBlockchainPoolPair(log.TestingLogger(), genDoc, privVals, initBlockHeight)

	pool := poolPair.pool
	messageChan := poolPair.messageQueue
	pool.Start()
	defer pool.Stop()

	subscriber := pmock.NewPeer(nil)
	pool.handleSubscribeBlock(1, initBlockHeight, subscriber)

	expectBlockMessage := make([]blockCommitResponseMessage, initBlockHeight-1)
	for i := int64(1); i < int64(initBlockHeight); i++ {
		first := pool.store.LoadBlock(i)
		second := pool.store.LoadBlock(i + 1)
		expectBlockMessage[i-1].Block = first
		expectBlockMessage[i-1].Commit = second.LastCommit
	}
	var index int
	for m := range messageChan {
		if blockMes, ok := m.blockChainMessage.(*blockCommitResponseMessage); ok {
			assert.Equal(t, *blockMes, expectBlockMessage[index])
			index++
			if int64(index) >= initBlockHeight-1 {
				break
			}
		}
	}
}

func TestBlockPoolHotSyncSubscribeTooFarBlock(t *testing.T) {
	config = cfg.ResetTestRoot(t.Name())
	defer os.RemoveAll(config.RootDir)
	genDoc, privVals := randGenesisDoc(1, false, 30)
	initBlockHeight := int64(10)
	poolPair := newBlockchainPoolPair(log.TestingLogger(), genDoc, privVals, initBlockHeight)

	pool := poolPair.pool
	messageChan := poolPair.messageQueue
	pool.Start()
	defer pool.Stop()

	subscriber := pmock.NewPeer(nil)
	pool.handleSubscribeBlock(initBlockHeight+1, initBlockHeight+2*maxPublishForesight, subscriber)
	for m := range messageChan {
		if noBlock, ok := m.blockChainMessage.(*noBlockResponseMessage); ok {
			assert.Equal(t, noBlock.Height, initBlockHeight+maxPublishForesight)
			return
		}
	}
}

func TestBlockPoolHotSyncUnSubscribe(t *testing.T) {
	config = cfg.ResetTestRoot(t.Name())
	defer os.RemoveAll(config.RootDir)
	genDoc, privVals := randGenesisDoc(1, false, 30)
	initBlockHeight := int64(10)
	poolPair := newBlockchainPoolPair(log.TestingLogger(), genDoc, privVals, initBlockHeight)

	pool := poolPair.pool
	pool.Start()
	defer pool.Stop()

	subscriber := pmock.NewPeer(nil)
	totalTestBlock := int64(10)
	pool.handleSubscribeBlock(initBlockHeight+1, initBlockHeight+totalTestBlock, subscriber)
	for i := initBlockHeight + 1; i <= initBlockHeight+totalTestBlock; i++ {
		pool.handleUnSubscribeBlock(i, subscriber)
		assert.Zero(t, len(pool.subscriberPeerSets[i]))
	}
}

func TestBlockPoolHotSyncRemovePeer(t *testing.T) {
	config = cfg.ResetTestRoot(t.Name())
	defer os.RemoveAll(config.RootDir)
	genDoc, privVals := randGenesisDoc(1, false, 30)
	initBlockHeight := int64(10)
	poolPair := newBlockchainPoolPair(log.TestingLogger(), genDoc, privVals, initBlockHeight)

	pool := poolPair.pool
	messageChan := poolPair.messageQueue
	pool.Start()
	defer pool.Stop()

	testPeer := pmock.NewPeer(nil)
	pool.AddPeer(testPeer)
	// wait for peer have a try
	time.Sleep(2 * tryRepairInterval)
	pool.RemovePeer(testPeer)
	// drain subscribe message
	<-messageChan
	for i := int64(0); i < maxSubscribeForesight; i++ {
		m := <-messageChan
		noBlock, ok := m.blockChainMessage.(*blockUnSubscribeMessage)
		assert.True(t, ok)
		assert.Equal(t, noBlock.Height, initBlockHeight+i+1)
	}
}

func TestBlockPoolConsensusSyncBasic(t *testing.T) {
	eventBus := types.NewEventBus()
	err := eventBus.Start()
	require.NoError(t, err)
	defer eventBus.Stop()

	config = cfg.ResetTestRoot(t.Name())
	defer os.RemoveAll(config.RootDir)
	genDoc, privVals := randGenesisDoc(1, false, 30)
	initBlockHeight := int64(10)
	poolPair := newBlockchainPoolPair(log.TestingLogger(), genDoc, privVals, initBlockHeight)

	pool := poolPair.pool
	pool.st = Consensus
	pool.setEventBus(eventBus)
	messageChan := poolPair.messageQueue
	pool.Start()
	defer pool.Stop()
	//handle subscribe message
	subscriber := pmock.NewPeer(nil)
	totalTestBlock := int64(10)
	pool.handleSubscribeBlock(initBlockHeight+1, initBlockHeight+totalTestBlock, subscriber)

	for i := int64(0); i < totalTestBlock; i++ {
		height := pool.currentHeight()
		block, commit, vs := nextBlock(poolPair.pool.state, poolPair.pool.store, poolPair.pool.blockExec, poolPair.privVals[0])
		expectedBlockMess1 := blockCommitResponseMessage{Block: block}
		//generate hash
		commit.Hash()
		expectedBlockMess2 := blockCommitResponseMessage{Commit: commit}
		err := eventBus.PublishEventCompleteProposal(types.EventDataCompleteProposal{Height: block.Height, Block: *block})
		assert.NoError(t, err)
		err = eventBus.PublishEventMajorPrecommits(types.EventDataMajorPrecommits{Height: block.Height, Votes: *vs})
		assert.NoError(t, err)
		pool.applyBlock(&blockState{
			block:   block,
			commit:  commit,
			blockId: makeBlockID(block),
		})
		var receive1, receive2 bool
		for m := range messageChan {
			if bm, ok := m.blockChainMessage.(*blockCommitResponseMessage); ok {
				if !receive1 {
					assert.Equal(t, *bm, expectedBlockMess1)
					receive1 = true
				} else if !receive2 {
					assert.Equal(t, *bm, expectedBlockMess2)
					receive2 = true
				}
				if receive1 && receive2 {
					break
				}
			}
		}
		for {
			if height+1 != pool.currentHeight() {
				time.Sleep(10 * time.Millisecond)
			} else {
				break
			}
		}
	}
}

func TestBlockPoolSubscribeFromCache(t *testing.T) {
	config = cfg.ResetTestRoot(t.Name())
	defer os.RemoveAll(config.RootDir)
	genDoc, privVals := randGenesisDoc(1, false, 30)
	initBlockHeight := int64(10)
	poolPair := newBlockchainPoolPair(log.TestingLogger(), genDoc, privVals, initBlockHeight)

	pool := poolPair.pool
	messageChan := poolPair.messageQueue
	pool.Start()
	defer pool.Stop()
	testPeer := pmock.NewPeer(nil)
	pool.AddPeer(testPeer)

	// consume subscribe message
	<-messageChan

	totalTestBlock := int64(10)

	expectedBlockMess := make([]blockCommitResponseMessage, 0, totalTestBlock)
	for i := int64(0); i < totalTestBlock; i++ {
		height := pool.currentHeight()
		block, commit, _ := nextBlock(poolPair.pool.state, poolPair.pool.store, poolPair.pool.blockExec, poolPair.privVals[0])
		expectedBlockMess = append(expectedBlockMess, blockCommitResponseMessage{Block: block, Commit: commit})
		pool.handleBlockCommit(block, commit, testPeer)

		for {
			if height+1 != pool.currentHeight() {
				time.Sleep(10 * time.Millisecond)
			} else {
				break
			}
		}
	}
	//handle subscribe message
	subscriber := pmock.NewPeer(nil)
	pool.handleSubscribeBlock(initBlockHeight+1, initBlockHeight+totalTestBlock, subscriber)
	for i := int64(0); i < totalTestBlock; i++ {
		for m := range messageChan {
			if bm, ok := m.blockChainMessage.(*blockCommitResponseMessage); ok {
				assert.Equal(t, *bm, expectedBlockMess[i])
				break
			}
		}
	}
}

func TestBlockPoolSwitch(t *testing.T) {
	config = cfg.ResetTestRoot(t.Name())
	defer os.RemoveAll(config.RootDir)
	genDoc, privVals := randGenesisDoc(1, false, 30)
	initBlockHeight := int64(10)
	poolPair := newBlockchainPoolPair(log.TestingLogger(), genDoc, privVals, initBlockHeight)

	pool := poolPair.pool
	messageChan := poolPair.messageQueue
	pool.st = Mute
	pool.Start()
	defer pool.Stop()
	testPeer := pmock.NewPeer(nil)
	pool.AddPeer(testPeer)

	time.Sleep(10 * time.Millisecond)
	pool.SwitchToHotSync(pool.state, 100)
	// consume subscribe message
	<-messageChan

	//handle subscribe message
	subscriber := pmock.NewPeer(nil)
	totalTestBlock := int64(10)
	pool.handleSubscribeBlock(initBlockHeight+1, initBlockHeight+totalTestBlock, subscriber)

	for i := int64(0); i < totalTestBlock; i++ {
		height := pool.currentHeight()
		block, commit, _ := nextBlock(poolPair.pool.state, poolPair.pool.store, poolPair.pool.blockExec, poolPair.privVals[0])
		expectedBlockMess1 := blockCommitResponseMessage{Block: block}
		expectedBlockMess2 := blockCommitResponseMessage{Commit: commit}
		pool.handleBlockCommit(block, commit, testPeer)
		var receive1, receive2 bool
		for m := range messageChan {
			if bm, ok := m.blockChainMessage.(*blockCommitResponseMessage); ok {
				if !receive1 {
					assert.Equal(t, *bm, expectedBlockMess1)
					receive1 = true
				} else if !receive2 {
					assert.Equal(t, *bm, expectedBlockMess2)
					receive2 = true
				}
				if receive1 && receive2 {
					break
				}
			}
		}
		for {
			if height+1 != pool.currentHeight() {
				time.Sleep(10 * time.Millisecond)
			} else {
				break
			}
		}
	}
	assert.Equal(t, int64(pool.blocksSynced), totalTestBlock+100)
	eventBus := types.NewEventBus()
	err := eventBus.Start()
	require.NoError(t, err)
	defer eventBus.Stop()
	pool.setEventBus(eventBus)

	pool.SwitchToConsensusSync(nil)
	time.Sleep(10 * time.Millisecond)
	subscriber = pmock.NewPeer(nil)
	totalTestBlock = int64(10)
	pool.handleSubscribeBlock(pool.currentHeight()+1, pool.currentHeight()+totalTestBlock, subscriber)

	for i := int64(0); i < totalTestBlock; i++ {
		height := pool.currentHeight()
		block, commit, vs := nextBlock(poolPair.pool.state, poolPair.pool.store, poolPair.pool.blockExec, poolPair.privVals[0])
		expectedBlockMess1 := blockCommitResponseMessage{Block: block}
		//generate hash
		commit.Hash()
		expectedBlockMess2 := blockCommitResponseMessage{Commit: commit}
		err := eventBus.PublishEventCompleteProposal(types.EventDataCompleteProposal{Height: block.Height, Block: *block})
		assert.NoError(t, err)
		err = eventBus.PublishEventMajorPrecommits(types.EventDataMajorPrecommits{Height: block.Height, Votes: *vs})
		assert.NoError(t, err)
		pool.applyBlock(&blockState{
			block:   block,
			commit:  commit,
			blockId: makeBlockID(block),
		})
		var receive1, receive2 bool
		for m := range messageChan {
			if bm, ok := m.blockChainMessage.(*blockCommitResponseMessage); ok {
				if !receive1 {
					eq := assert.Equal(t, *bm, expectedBlockMess1)
					receive1 = true
					if !eq {
						return
					}
				} else if !receive2 {
					eq := assert.Equal(t, *bm, expectedBlockMess2)
					receive2 = true
					receive1 = true
					if !eq {
						return
					}
				}
				if receive1 && receive2 {
					break
				}
			}
		}
		for {
			if height+1 != pool.currentHeight() {
				time.Sleep(10 * time.Millisecond)
			} else {
				break
			}
		}
	}

}

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

func newBlockchainPoolPair(logger log.Logger, genDoc *types.GenesisDoc, privVals []types.PrivValidator, maxBlockHeight int64) BlockPoolPair {
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
	messagesCh := make(chan Message, messageQueueSize)
	bcPool := NewBlockPool(blockStore, blockExec, state.Copy(), messagesCh, Hot, 2*time.Second)
	bcPool.SetLogger(logger.With("module", "blockpool"))

	return BlockPoolPair{bcPool, proxyApp, messagesCh, privVals}
}

type BlockPoolPair struct {
	pool         *BlockPool
	app          proxy.AppConns
	messageQueue chan Message
	privVals     []types.PrivValidator
}

func nextBlock(state sm.State, blockStore *blockchain.BlockStore, blockExec *sm.BlockExecutor, pri types.PrivValidator) (*types.Block, *types.Commit, *types.VoteSet) {
	height := blockStore.Height()
	lastBlockMeta := blockStore.LoadBlockMeta(height)
	lastBlock := blockStore.LoadBlock(height)
	vote := makeVote(&lastBlock.Header, lastBlockMeta.BlockID, state.Validators, pri).CommitSig()
	lastCommit := types.NewCommit(lastBlockMeta.BlockID, []*types.CommitSig{vote})
	thisBlock := makeBlock(height+1, state, lastCommit)

	thisBlockId := makeBlockID(thisBlock)
	thisVote := makeVote(&thisBlock.Header, *thisBlockId, state.Validators, pri)
	thisCommitSig := thisVote.CommitSig()
	thisCommit := types.NewCommit(*makeBlockID(thisBlock), []*types.CommitSig{thisCommitSig})
	thisVoteSet := types.NewVoteSet(thisBlock.ChainID, thisBlock.Height, 1, types.PrecommitType, state.Validators)
	thisVoteSet.AddVote(thisVote)
	return thisBlock, thisCommit, thisVoteSet
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

func (app *testApp) DeliverTx(tx abci.RequestDeliverTx) abci.ResponseDeliverTx {
	return abci.ResponseDeliverTx{}
}

func (app *testApp) CheckTx(tx abci.RequestCheckTx) abci.ResponseCheckTx {
	return abci.ResponseCheckTx{}
}

func (app *testApp) Commit() abci.ResponseCommit {
	return abci.ResponseCommit{}
}

func (app *testApp) Query(reqQuery abci.RequestQuery) (resQuery abci.ResponseQuery) {
	return
}
