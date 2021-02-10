package consensus

import (
	"context"
	"fmt"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	abcicli "github.com/tendermint/tendermint/abci/client"
	abci "github.com/tendermint/tendermint/abci/types"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	tmsync "github.com/tendermint/tendermint/libs/sync"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/p2ptest"
	tmcons "github.com/tendermint/tendermint/proto/tendermint/consensus"
	sm "github.com/tendermint/tendermint/state"
	statemocks "github.com/tendermint/tendermint/state/mocks"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

var (
	defaultTestTime = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
)

type reactorTestSuite struct {
	network             *p2ptest.Network
	reactors            map[p2p.NodeID]*Reactor
	subs                map[p2p.NodeID]types.Subscription
	stateChannels       map[p2p.NodeID]*p2p.Channel
	dataChannels        map[p2p.NodeID]*p2p.Channel
	voteChannels        map[p2p.NodeID]*p2p.Channel
	voteSetBitsChannels map[p2p.NodeID]*p2p.Channel
}

func setup(t *testing.T, states []*State, size int) *reactorTestSuite {
	t.Helper()

	numNodes := len(states)

	rts := &reactorTestSuite{
		network:  p2ptest.MakeNetwork(t, numNodes),
		reactors: make(map[p2p.NodeID]*Reactor, numNodes),
		subs:     make(map[p2p.NodeID]types.Subscription, numNodes),
	}

	rts.stateChannels = rts.network.MakeChannelsNoCleanup(t, StateChannel, new(tmcons.Message), size)
	rts.dataChannels = rts.network.MakeChannelsNoCleanup(t, DataChannel, new(tmcons.Message), size)
	rts.voteChannels = rts.network.MakeChannelsNoCleanup(t, VoteChannel, new(tmcons.Message), size)
	rts.voteSetBitsChannels = rts.network.MakeChannelsNoCleanup(t, VoteSetBitsChannel, new(tmcons.Message), size)

	i := 0
	for nodeID, node := range rts.network.Nodes {
		state := states[i]

		reactor := NewReactor(
			state.Logger.With("node", nodeID),
			state,
			rts.stateChannels[nodeID],
			rts.dataChannels[nodeID],
			rts.voteChannels[nodeID],
			rts.voteSetBitsChannels[nodeID],
			node.MakePeerUpdates(t),
			true,
		)

		reactor.SetEventBus(state.eventBus)

		blocksSub, err := state.eventBus.Subscribe(context.Background(), testSubscriber, types.EventQueryNewBlock)
		require.NoError(t, err)

		rts.subs[nodeID] = blocksSub
		rts.reactors[nodeID] = reactor

		// simulate handle initChain in handshake
		if state.state.LastBlockHeight == 0 {
			require.NoError(t, state.blockExec.Store().Save(state.state))
		}

		require.NoError(t, reactor.Start())
		require.True(t, reactor.IsRunning())

		i++
	}

	require.Len(t, rts.reactors, numNodes)

	// start the in-memory network and connect all peers with each other
	rts.network.Start(t)

	t.Cleanup(func() {
		i := 0
		for _, r := range rts.reactors {
			state := states[i]

			require.NoError(t, state.eventBus.Stop())
			require.NoError(t, r.Stop())
			require.False(t, r.IsRunning())

			i++
		}

		leaktest.Check(t)
	})

	return rts
}

func TestReactorBasic(t *testing.T) {
	configSetup(t)

	n := 4
	states, cleanup := randConsensusState(n, "consensus_reactor_test", newMockTickerFunc(true), newCounter)

	t.Cleanup(func() {
		cleanup()
	})

	rts := setup(t, states, 100) // buffer must be large enough to not deadlock

	for _, reactor := range rts.reactors {
		state := reactor.conS.GetState()
		reactor.SwitchToConsensus(state, false)
	}

	var wg sync.WaitGroup
	for _, sub := range rts.subs {
		wg.Add(1)

		// wait till everyone makes the first new block
		go func(s types.Subscription) {
			<-s.Out()
			wg.Done()
		}(sub)
	}

	wg.Wait()
}

func TestReactorWithEvidence(t *testing.T) {
	configSetup(t)

	n := 4
	testName := "consensus_reactor_test"
	tickerFunc := newMockTickerFunc(true)
	appFunc := newCounter

	genDoc, privVals := randGenesisDoc(n, false, 30)
	states := make([]*State, n)
	logger := consensusLogger()

	for i := 0; i < n; i++ {
		stateDB := dbm.NewMemDB() // each state needs its own db
		stateStore := sm.NewStore(stateDB)
		state, _ := stateStore.LoadFromDBOrGenesisDoc(genDoc)
		thisConfig := ResetConfig(fmt.Sprintf("%s_%d", testName, i))

		defer os.RemoveAll(thisConfig.RootDir)

		ensureDir(path.Dir(thisConfig.Consensus.WalFile()), 0700) // dir for wal
		app := appFunc()
		vals := types.TM2PB.ValidatorUpdates(state.Validators)
		app.InitChain(abci.RequestInitChain{Validators: vals})

		pv := privVals[i]
		blockDB := dbm.NewMemDB()
		blockStore := store.NewBlockStore(blockDB)

		// one for mempool, one for consensus
		mtx := new(tmsync.Mutex)
		proxyAppConnMem := abcicli.NewLocalClient(mtx, app)
		proxyAppConnCon := abcicli.NewLocalClient(mtx, app)

		mempool := mempl.NewCListMempool(thisConfig.Mempool, proxyAppConnMem, 0)
		mempool.SetLogger(log.TestingLogger().With("module", "mempool"))
		if thisConfig.Consensus.WaitForTxs() {
			mempool.EnableTxsAvailable()
		}

		// mock the evidence pool
		// everyone includes evidence of another double signing
		vIdx := (i + 1) % n

		ev := types.NewMockDuplicateVoteEvidenceWithValidator(1, defaultTestTime, privVals[vIdx], config.ChainID())
		evpool := &statemocks.EvidencePool{}
		evpool.On("CheckEvidence", mock.AnythingOfType("types.EvidenceList")).Return(nil)
		evpool.On("PendingEvidence", mock.AnythingOfType("int64")).Return([]types.Evidence{
			ev}, int64(len(ev.Bytes())))
		evpool.On("Update", mock.AnythingOfType("state.State"), mock.AnythingOfType("types.EvidenceList")).Return()

		evpool2 := sm.EmptyEvidencePool{}

		blockExec := sm.NewBlockExecutor(stateStore, log.TestingLogger(), proxyAppConnCon, mempool, evpool)
		cs := NewState(thisConfig.Consensus, state, blockExec, blockStore, mempool, evpool2)
		cs.SetLogger(log.TestingLogger().With("module", "consensus"))
		cs.SetPrivValidator(pv)

		eventBus := types.NewEventBus()
		eventBus.SetLogger(log.TestingLogger().With("module", "events"))
		err := eventBus.Start()
		require.NoError(t, err)
		cs.SetEventBus(eventBus)

		cs.SetTimeoutTicker(tickerFunc())
		cs.SetLogger(logger.With("validator", i, "module", "consensus"))

		states[i] = cs
	}

	rts := setup(t, states, 100) // buffer must be large enough to not deadlock

	for _, reactor := range rts.reactors {
		state := reactor.conS.GetState()
		reactor.SwitchToConsensus(state, false)
	}

	var wg sync.WaitGroup
	for _, sub := range rts.subs {
		wg.Add(1)

		// We expect for each validator that is the proposer to propose one piece of
		// evidence.
		go func(s types.Subscription) {
			msg := <-s.Out()
			block := msg.Data().(types.EventDataNewBlock).Block

			require.Len(t, block.Evidence.Evidence, 1)
			wg.Done()
		}(sub)
	}

	wg.Wait()
}

func TestReactorCreatesBlockWhenEmptyBlocksFalse(t *testing.T) {
	configSetup(t)

	n := 4
	states, cleanup := randConsensusState(n, "consensus_reactor_test", newMockTickerFunc(true), newCounter,
		func(c *cfg.Config) {
			c.Consensus.CreateEmptyBlocks = false
		})

	t.Cleanup(func() {
		cleanup()
	})

	rts := setup(t, states, 100) // buffer must be large enough to not deadlock

	for _, reactor := range rts.reactors {
		state := reactor.conS.GetState()
		reactor.SwitchToConsensus(state, false)
	}

	// send a tx
	require.NoError(t, assertMempool(states[3].txNotifier).CheckTx([]byte{1, 2, 3}, nil, mempl.TxInfo{}))

	var wg sync.WaitGroup
	for _, sub := range rts.subs {
		wg.Add(1)

		// wait till everyone makes the first new block
		go func(s types.Subscription) {
			<-s.Out()
			wg.Done()
		}(sub)
	}

	wg.Wait()
}

func TestReactorRecordsVotesAndBlockParts(t *testing.T) {
	configSetup(t)

	n := 4
	states, cleanup := randConsensusState(n, "consensus_reactor_test", newMockTickerFunc(true), newCounter)

	t.Cleanup(func() {
		cleanup()
	})

	rts := setup(t, states, 100) // buffer must be large enough to not deadlock

	for _, reactor := range rts.reactors {
		state := reactor.conS.GetState()
		reactor.SwitchToConsensus(state, false)
	}

	var wg sync.WaitGroup
	for _, sub := range rts.subs {
		wg.Add(1)

		// wait till everyone makes the first new block
		go func(s types.Subscription) {
			<-s.Out()
			wg.Done()
		}(sub)
	}

	wg.Wait()

	// Require at least one node to have sent block parts, but we can't know which
	// peer sent it.
	require.Eventually(
		t,
		func() bool {
			for _, reactor := range rts.reactors {
				for _, ps := range reactor.peers {
					if ps.BlockPartsSent() > 0 {
						return true
					}
				}
			}

			return false
		},
		time.Second,
		10*time.Millisecond,
		"number of block parts sent should've increased",
	)

	nodeID := rts.network.RandomNode().NodeID
	reactor := rts.reactors[nodeID]
	peers := rts.network.Peers(nodeID)

	ps, ok := reactor.GetPeerState(peers[0].NodeID)
	require.True(t, ok)
	require.NotNil(t, ps)
	require.Greater(t, ps.VotesSent(), 0, "number of votes sent should've increased")
}

// func TestReactorVotingPowerChange(t *testing.T) {
// 	configSetup(t)

// 	nVals := 4
// 	css, cleanup := randConsensusState(
// 		nVals,
// 		"consensus_voting_power_changes_test",
// 		newMockTickerFunc(true),
// 		newPersistentKVStore,
// 	)

// 	t.Cleanup(func() {
// 		cleanup()
// 	})

// 	testSuites := make([]*reactorTestSuite, nVals)
// 	for i := range testSuites {
// 		testSuites[i] = setup(t, css[i], 100) // buffer must be large enough to not deadlock
// 	}

// 	// map of active validators
// 	activeVals := make(map[string]struct{})
// 	for i := 0; i < nVals; i++ {
// 		pubKey, err := css[i].privValidator.GetPubKey()
// 		require.NoError(t, err)

// 		addr := pubKey.Address()
// 		activeVals[string(addr)] = struct{}{}
// 	}

// 	var wg sync.WaitGroup
// 	for _, ts := range testSuites {
// 		wg.Add(1)

// 		// wait till everyone makes the first new block
// 		go func(rts *reactorTestSuite) {
// 			<-rts.sub.Out()
// 			wg.Done()
// 		}(ts)
// 	}

// 	wg.Wait()

// 	val1PubKey, err := css[0].privValidator.GetPubKey()
// 	require.NoError(t, err)

// 	val1PubKeyABCI, err := cryptoenc.PubKeyToProto(val1PubKey)
// 	require.NoError(t, err)

// 	updateValidatorTx := kvstore.MakeValSetChangeTx(val1PubKeyABCI, 25)
// 	previousTotalVotingPower := css[0].GetRoundState().LastValidators.TotalVotingPower()

// 	waitForAndValidateBlock(t, nVals, activeVals, blocksSubs, css, updateValidatorTx)
// 	waitForAndValidateBlockWithTx(t, nVals, activeVals, blocksSubs, css, updateValidatorTx)
// 	waitForAndValidateBlock(t, nVals, activeVals, blocksSubs, css)
// 	waitForAndValidateBlock(t, nVals, activeVals, blocksSubs, css)

// 	require.NotEqualf(
// 		t, previousTotalVotingPower, css[0].GetRoundState().LastValidators.TotalVotingPower(),
// 		"expected voting power to change (before: %d, after: %d)",
// 		previousTotalVotingPower,
// 		css[0].GetRoundState().LastValidators.TotalVotingPower(),
// 	)

// 	updateValidatorTx = kvstore.MakeValSetChangeTx(val1PubKeyABCI, 2)
// 	previousTotalVotingPower = css[0].GetRoundState().LastValidators.TotalVotingPower()

// 	waitForAndValidateBlock(t, nVals, activeVals, blocksSubs, css, updateValidatorTx)
// 	waitForAndValidateBlockWithTx(t, nVals, activeVals, blocksSubs, css, updateValidatorTx)
// 	waitForAndValidateBlock(t, nVals, activeVals, blocksSubs, css)
// 	waitForAndValidateBlock(t, nVals, activeVals, blocksSubs, css)

// 	if css[0].GetRoundState().LastValidators.TotalVotingPower() == previousTotalVotingPower {
// 		t.Fatalf(
// 			"expected voting power to change (before: %d, after: %d)",
// 			previousTotalVotingPower, css[0].GetRoundState().LastValidators.TotalVotingPower(),
// 		)
// 	}

// 	updateValidatorTx = kvstore.MakeValSetChangeTx(val1PubKeyABCI, 26)
// 	previousTotalVotingPower = css[0].GetRoundState().LastValidators.TotalVotingPower()

// 	waitForAndValidateBlock(t, nVals, activeVals, blocksSubs, css, updateValidatorTx)
// 	waitForAndValidateBlockWithTx(t, nVals, activeVals, blocksSubs, css, updateValidatorTx)
// 	waitForAndValidateBlock(t, nVals, activeVals, blocksSubs, css)
// 	waitForAndValidateBlock(t, nVals, activeVals, blocksSubs, css)

// 	require.NotEqualf(
// 		t, previousTotalVotingPower, css[0].GetRoundState().LastValidators.TotalVotingPower(),
// 		"expected voting power to change (before: %d, after: %d)",
// 		previousTotalVotingPower,
// 		css[0].GetRoundState().LastValidators.TotalVotingPower(),
// 	)
// }

// ============================================================================
// ============================================================================
// ============================================================================

// func TestReactorValidatorSetChanges(t *testing.T) {
// 	nPeers := 7
// 	nVals := 4
// 	css, _, _, cleanup := randConsensusNetWithPeers(
// 		nVals,
// 		nPeers,
// 		"consensus_val_set_changes_test",
// 		newMockTickerFunc(true),
// 		newPersistentKVStoreWithPath)

// 	defer cleanup()
// 	logger := log.TestingLogger()

// 	reactors, blocksSubs, eventBuses := startConsensusNet(t, css, nPeers)
// 	defer stopConsensusNet(logger, reactors, eventBuses)

// 	// map of active validators
// 	activeVals := make(map[string]struct{})
// 	for i := 0; i < nVals; i++ {
// 		pubKey, err := css[i].privValidator.GetPubKey()
// 		require.NoError(t, err)
// 		activeVals[string(pubKey.Address())] = struct{}{}
// 	}

// 	// wait till everyone makes block 1
// 	timeoutWaitGroup(t, nPeers, func(j int) {
// 		<-blocksSubs[j].Out()
// 	}, css)

// 	//---------------------------------------------------------------------------
// 	logger.Info("---------------------------- Testing adding one validator")

// 	newValidatorPubKey1, err := css[nVals].privValidator.GetPubKey()
// 	assert.NoError(t, err)
// 	valPubKey1ABCI, err := cryptoenc.PubKeyToProto(newValidatorPubKey1)
// 	assert.NoError(t, err)
// 	newValidatorTx1 := kvstore.MakeValSetChangeTx(valPubKey1ABCI, testMinPower)

// 	// wait till everyone makes block 2
// 	// ensure the commit includes all validators
// 	// send newValTx to change vals in block 3
// 	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, css, newValidatorTx1)

// 	// wait till everyone makes block 3.
// 	// it includes the commit for block 2, which is by the original validator set
// 	waitForAndValidateBlockWithTx(t, nPeers, activeVals, blocksSubs, css, newValidatorTx1)

// 	// wait till everyone makes block 4.
// 	// it includes the commit for block 3, which is by the original validator set
// 	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, css)

// 	// the commits for block 4 should be with the updated validator set
// 	activeVals[string(newValidatorPubKey1.Address())] = struct{}{}

// 	// wait till everyone makes block 5
// 	// it includes the commit for block 4, which should have the updated validator set
// 	waitForBlockWithUpdatedValsAndValidateIt(t, nPeers, activeVals, blocksSubs, css)

// 	//---------------------------------------------------------------------------
// 	logger.Info("---------------------------- Testing changing the voting power of one validator")

// 	updateValidatorPubKey1, err := css[nVals].privValidator.GetPubKey()
// 	require.NoError(t, err)
// 	updatePubKey1ABCI, err := cryptoenc.PubKeyToProto(updateValidatorPubKey1)
// 	require.NoError(t, err)
// 	updateValidatorTx1 := kvstore.MakeValSetChangeTx(updatePubKey1ABCI, 25)
// 	previousTotalVotingPower := css[nVals].GetRoundState().LastValidators.TotalVotingPower()

// 	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, css, updateValidatorTx1)
// 	waitForAndValidateBlockWithTx(t, nPeers, activeVals, blocksSubs, css, updateValidatorTx1)
// 	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, css)
// 	waitForBlockWithUpdatedValsAndValidateIt(t, nPeers, activeVals, blocksSubs, css)

// 	if css[nVals].GetRoundState().LastValidators.TotalVotingPower() == previousTotalVotingPower {
// 		t.Errorf(
// 			"expected voting power to change (before: %d, after: %d)",
// 			previousTotalVotingPower,
// 			css[nVals].GetRoundState().LastValidators.TotalVotingPower())
// 	}

// 	//---------------------------------------------------------------------------
// 	logger.Info("---------------------------- Testing adding two validators at once")

// 	newValidatorPubKey2, err := css[nVals+1].privValidator.GetPubKey()
// 	require.NoError(t, err)
// 	newVal2ABCI, err := cryptoenc.PubKeyToProto(newValidatorPubKey2)
// 	require.NoError(t, err)
// 	newValidatorTx2 := kvstore.MakeValSetChangeTx(newVal2ABCI, testMinPower)

// 	newValidatorPubKey3, err := css[nVals+2].privValidator.GetPubKey()
// 	require.NoError(t, err)
// 	newVal3ABCI, err := cryptoenc.PubKeyToProto(newValidatorPubKey3)
// 	require.NoError(t, err)
// 	newValidatorTx3 := kvstore.MakeValSetChangeTx(newVal3ABCI, testMinPower)

// 	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, css, newValidatorTx2, newValidatorTx3)
// 	waitForAndValidateBlockWithTx(t, nPeers, activeVals, blocksSubs, css, newValidatorTx2, newValidatorTx3)
// 	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, css)
// 	activeVals[string(newValidatorPubKey2.Address())] = struct{}{}
// 	activeVals[string(newValidatorPubKey3.Address())] = struct{}{}
// 	waitForBlockWithUpdatedValsAndValidateIt(t, nPeers, activeVals, blocksSubs, css)

// 	//---------------------------------------------------------------------------
// 	logger.Info("---------------------------- Testing removing two validators at once")

// 	removeValidatorTx2 := kvstore.MakeValSetChangeTx(newVal2ABCI, 0)
// 	removeValidatorTx3 := kvstore.MakeValSetChangeTx(newVal3ABCI, 0)

// 	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, css, removeValidatorTx2, removeValidatorTx3)
// 	waitForAndValidateBlockWithTx(t, nPeers, activeVals, blocksSubs, css, removeValidatorTx2, removeValidatorTx3)
// 	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, css)
// 	delete(activeVals, string(newValidatorPubKey2.Address()))
// 	delete(activeVals, string(newValidatorPubKey3.Address()))
// 	waitForBlockWithUpdatedValsAndValidateIt(t, nPeers, activeVals, blocksSubs, css)
// }

// // Check we can make blocks with skip_timeout_commit=false
// func TestReactorWithTimeoutCommit(t *testing.T) {
// 	N := 4
// 	css, cleanup := randConsensusNet(N, "consensus_reactor_with_timeout_commit_test", newMockTickerFunc(false), newCounter)
// 	defer cleanup()
// 	// override default SkipTimeoutCommit == true for tests
// 	for i := 0; i < N; i++ {
// 		css[i].config.SkipTimeoutCommit = false
// 	}

// 	reactors, blocksSubs, eventBuses := startConsensusNet(t, css, N-1)
// 	defer stopConsensusNet(log.TestingLogger(), reactors, eventBuses)

// 	// wait till everyone makes the first new block
// 	timeoutWaitGroup(t, N-1, func(j int) {
// 		<-blocksSubs[j].Out()
// 	}, css)
// }

// func waitForAndValidateBlock(
// 	t *testing.T,
// 	n int,
// 	activeVals map[string]struct{},
// 	blocksSubs []types.Subscription,
// 	css []*State,
// 	txs ...[]byte,
// ) {
// 	timeoutWaitGroup(t, n, func(j int) {
// 		css[j].Logger.Debug("waitForAndValidateBlock")
// 		msg := <-blocksSubs[j].Out()
// 		newBlock := msg.Data().(types.EventDataNewBlock).Block
// 		css[j].Logger.Debug("waitForAndValidateBlock: Got block", "height", newBlock.Height)
// 		err := validateBlock(newBlock, activeVals)
// 		assert.Nil(t, err)
// 		for _, tx := range txs {
// 			err := assertMempool(css[j].txNotifier).CheckTx(tx, nil, mempl.TxInfo{})
// 			assert.Nil(t, err)
// 		}
// 	}, css)
// }

// func waitForAndValidateBlockWithTx(
// 	t *testing.T,
// 	n int,
// 	activeVals map[string]struct{},
// 	blocksSubs []types.Subscription,
// 	css []*State,
// 	txs ...[]byte,
// ) {
// 	timeoutWaitGroup(t, n, func(j int) {
// 		ntxs := 0
// 	BLOCK_TX_LOOP:
// 		for {
// 			css[j].Logger.Debug("waitForAndValidateBlockWithTx", "ntxs", ntxs)
// 			msg := <-blocksSubs[j].Out()
// 			newBlock := msg.Data().(types.EventDataNewBlock).Block
// 			css[j].Logger.Debug("waitForAndValidateBlockWithTx: Got block", "height", newBlock.Height)
// 			err := validateBlock(newBlock, activeVals)
// 			assert.Nil(t, err)

// 			// check that txs match the txs we're waiting for.
// 			// note they could be spread over multiple blocks,
// 			// but they should be in order.
// 			for _, tx := range newBlock.Data.Txs {
// 				assert.EqualValues(t, txs[ntxs], tx)
// 				ntxs++
// 			}

// 			if ntxs == len(txs) {
// 				break BLOCK_TX_LOOP
// 			}
// 		}

// 	}, css)
// }

// func waitForBlockWithUpdatedValsAndValidateIt(
// 	t *testing.T,
// 	n int,
// 	updatedVals map[string]struct{},
// 	blocksSubs []types.Subscription,
// 	css []*State,
// ) {
// 	timeoutWaitGroup(t, n, func(j int) {

// 		var newBlock *types.Block
// 	LOOP:
// 		for {
// 			css[j].Logger.Debug("waitForBlockWithUpdatedValsAndValidateIt")
// 			msg := <-blocksSubs[j].Out()
// 			newBlock = msg.Data().(types.EventDataNewBlock).Block
// 			if newBlock.LastCommit.Size() == len(updatedVals) {
// 				css[j].Logger.Debug("waitForBlockWithUpdatedValsAndValidateIt: Got block", "height", newBlock.Height)
// 				break LOOP
// 			} else {
// 				css[j].Logger.Debug(
// 					"waitForBlockWithUpdatedValsAndValidateIt: Got block with no new validators. Skipping",
// 					"height",
// 					newBlock.Height)
// 			}
// 		}

// 		err := validateBlock(newBlock, updatedVals)
// 		assert.Nil(t, err)
// 	}, css)
// }

// // expects high synchrony!
// func validateBlock(block *types.Block, activeVals map[string]struct{}) error {
// 	if block.LastCommit.Size() != len(activeVals) {
// 		return fmt.Errorf(
// 			"commit size doesn't match number of active validators. Got %d, expected %d",
// 			block.LastCommit.Size(),
// 			len(activeVals))
// 	}

// 	for _, commitSig := range block.LastCommit.Signatures {
// 		if _, ok := activeVals[string(commitSig.ValidatorAddress)]; !ok {
// 			return fmt.Errorf("found vote for inactive validator %X", commitSig.ValidatorAddress)
// 		}
// 	}
// 	return nil
// }
