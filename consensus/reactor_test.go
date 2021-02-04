package consensus

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	abcicli "github.com/tendermint/tendermint/abci/client"
	abci "github.com/tendermint/tendermint/abci/types"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/log"
	tmsync "github.com/tendermint/tendermint/libs/sync"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	tmcons "github.com/tendermint/tendermint/proto/tendermint/consensus"
	sm "github.com/tendermint/tendermint/state"
	statemocks "github.com/tendermint/tendermint/state/mocks"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

var (
	rng             = rand.New(rand.NewSource(time.Now().UnixNano()))
	defaultTestTime = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
)

type reactorTestSuite struct {
	reactor *Reactor
	peerID  p2p.NodeID
	sub     types.Subscription

	stateInCh      chan p2p.Envelope
	stateOutCh     chan p2p.Envelope
	statePeerErrCh chan p2p.PeerError

	dataInCh      chan p2p.Envelope
	dataOutCh     chan p2p.Envelope
	dataPeerErrCh chan p2p.PeerError

	voteInCh      chan p2p.Envelope
	voteOutCh     chan p2p.Envelope
	votePeerErrCh chan p2p.PeerError

	voteSetBitsInCh      chan p2p.Envelope
	voteSetBitsOutCh     chan p2p.Envelope
	voteSetBitsPeerErrCh chan p2p.PeerError

	peerUpdatesCh chan p2p.PeerUpdate
	peerUpdates   *p2p.PeerUpdates
}

func setup(t *testing.T, cs *State, chBuf uint) *reactorTestSuite {
	t.Helper()

	pID := make([]byte, 16)
	_, err := rng.Read(pID)
	require.NoError(t, err)

	peerID, err := p2p.NewNodeID(fmt.Sprintf("%x", pID))
	require.NoError(t, err)

	peerUpdatesCh := make(chan p2p.PeerUpdate, chBuf)

	rts := &reactorTestSuite{
		stateInCh:            make(chan p2p.Envelope, chBuf),
		stateOutCh:           make(chan p2p.Envelope, chBuf),
		statePeerErrCh:       make(chan p2p.PeerError, chBuf),
		dataInCh:             make(chan p2p.Envelope, chBuf),
		dataOutCh:            make(chan p2p.Envelope, chBuf),
		dataPeerErrCh:        make(chan p2p.PeerError, chBuf),
		voteInCh:             make(chan p2p.Envelope, chBuf),
		voteOutCh:            make(chan p2p.Envelope, chBuf),
		votePeerErrCh:        make(chan p2p.PeerError, chBuf),
		voteSetBitsInCh:      make(chan p2p.Envelope, chBuf),
		voteSetBitsOutCh:     make(chan p2p.Envelope, chBuf),
		voteSetBitsPeerErrCh: make(chan p2p.PeerError, chBuf),
		peerUpdatesCh:        peerUpdatesCh,
		peerUpdates:          p2p.NewPeerUpdates(peerUpdatesCh),
		peerID:               peerID,
	}

	stateCh := p2p.NewChannel(
		StateChannel,
		new(tmcons.Message),
		rts.stateInCh,
		rts.stateOutCh,
		rts.statePeerErrCh,
	)

	dataCh := p2p.NewChannel(
		DataChannel,
		new(tmcons.Message),
		rts.dataInCh,
		rts.dataOutCh,
		rts.dataPeerErrCh,
	)

	voteCh := p2p.NewChannel(
		VoteChannel,
		new(tmcons.Message),
		rts.voteInCh,
		rts.voteOutCh,
		rts.votePeerErrCh,
	)

	voteSetBitsCh := p2p.NewChannel(
		VoteSetBitsChannel,
		new(tmcons.Message),
		rts.voteSetBitsInCh,
		rts.voteSetBitsOutCh,
		rts.voteSetBitsPeerErrCh,
	)

	rts.reactor = NewReactor(
		cs.Logger.With("node", rts.peerID),
		cs, stateCh, dataCh, voteCh, voteSetBitsCh, rts.peerUpdates, true,
	)

	rts.reactor.SetEventBus(cs.eventBus)

	blocksSub, err := cs.eventBus.Subscribe(context.Background(), testSubscriber, types.EventQueryNewBlock)
	require.NoError(t, err)
	rts.sub = blocksSub

	// simulate handle initChain in handshake
	if cs.state.LastBlockHeight == 0 {
		require.NoError(t, cs.blockExec.Store().Save(cs.state))
	}

	require.NoError(t, rts.reactor.Start())
	require.True(t, rts.reactor.IsRunning())

	t.Cleanup(func() {
		require.NoError(t, cs.eventBus.Stop())
		require.NoError(t, rts.reactor.Stop())
		require.False(t, rts.reactor.IsRunning())
	})

	return rts
}

func simulateRouter(primary *reactorTestSuite, suites []*reactorTestSuite, dropChErr bool) {
	type channels struct {
		out     chan p2p.Envelope
		in      chan p2p.Envelope
		peerErr chan p2p.PeerError
	}

	// create a mapping for efficient suite lookup by peer ID
	suitesByPeerID := make(map[p2p.NodeID]map[p2p.ChannelID]channels)
	for _, ts := range suites {
		suitesByPeerID[ts.peerID] = map[p2p.ChannelID]channels{
			StateChannel:       {ts.stateOutCh, ts.stateInCh, ts.statePeerErrCh},
			DataChannel:        {ts.dataOutCh, ts.dataInCh, ts.dataPeerErrCh},
			VoteChannel:        {ts.voteOutCh, ts.voteInCh, ts.votePeerErrCh},
			VoteSetBitsChannel: {ts.voteSetBitsOutCh, ts.voteSetBitsInCh, ts.voteSetBitsPeerErrCh},
		}
	}

	cIDs := []p2p.ChannelID{StateChannel, DataChannel, VoteChannel, VoteSetBitsChannel}
	for _, chID := range cIDs {
		// Simulate a router by listening for all outbound envelopes and proxying the
		// envelope to the respective peer (suite).
		go func(c p2p.ChannelID) {
			for envelope := range suitesByPeerID[primary.peerID][c].out {
				if envelope.Broadcast {
					for _, ts := range suites {
						// broadcast to everyone except source
						if ts.peerID != primary.peerID {
							suitesByPeerID[ts.peerID][c].in <- p2p.Envelope{
								From:    primary.peerID,
								To:      ts.peerID,
								Message: envelope.Message,
							}
						}
					}
				} else {
					suitesByPeerID[envelope.To][c].in <- p2p.Envelope{
						From:    primary.peerID,
						To:      envelope.To,
						Message: envelope.Message,
					}
				}
			}
		}(chID)

		go func(c p2p.ChannelID) {
			for pErr := range suitesByPeerID[primary.peerID][c].peerErr {
				if dropChErr {
					primary.reactor.Logger.Debug("dropped peer error", "err", pErr.Err)
				} else {
					primary.peerUpdatesCh <- p2p.PeerUpdate{
						NodeID: pErr.NodeID,
						Status: p2p.PeerStatusDown,
					}
				}
			}
		}(chID)
	}
}

func TestReactorBasic(t *testing.T) {
	configSetup(t)

	n := 4
	css, cleanup := randConsensusState(n, "consensus_reactor_test", newMockTickerFunc(true), newCounter)

	t.Cleanup(func() {
		cleanup()
	})

	testSuites := make([]*reactorTestSuite, n)
	for i := range testSuites {
		testSuites[i] = setup(t, css[i], 100) // buffer must be large enough to not deadlock
	}

	for _, ts := range testSuites {
		simulateRouter(ts, testSuites, true)

		// connect reactor to every other reactor
		for _, tss := range testSuites {
			if ts.peerID != tss.peerID {
				ts.peerUpdatesCh <- p2p.PeerUpdate{
					Status: p2p.PeerStatusUp,
					NodeID: tss.peerID,
				}
			}
		}

		state := ts.reactor.conS.GetState()
		ts.reactor.SwitchToConsensus(state, false)
	}

	var wg sync.WaitGroup
	for _, ts := range testSuites {
		wg.Add(1)

		// wait till everyone makes the first new block
		go func(rts *reactorTestSuite) {
			<-rts.sub.Out()
			wg.Done()
		}(ts)
	}

	wg.Wait()
}

func TestReactorWithEvidence(t *testing.T) {
	configSetup(t)

	nValidators := 4
	testName := "consensus_reactor_test"
	tickerFunc := newMockTickerFunc(true)
	appFunc := newCounter

	genDoc, privVals := randGenesisDoc(nValidators, false, 30)
	css := make([]*State, nValidators)
	logger := consensusLogger()

	for i := 0; i < nValidators; i++ {
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
		vIdx := (i + 1) % nValidators

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

		css[i] = cs
	}

	testSuites := make([]*reactorTestSuite, nValidators)
	for i := range testSuites {
		testSuites[i] = setup(t, css[i], 100) // buffer must be large enough to not deadlock
	}

	for _, ts := range testSuites {
		simulateRouter(ts, testSuites, true)

		// connect reactor to every other reactor
		for _, tss := range testSuites {
			if ts.peerID != tss.peerID {
				ts.peerUpdatesCh <- p2p.PeerUpdate{
					Status: p2p.PeerStatusUp,
					NodeID: tss.peerID,
				}
			}
		}

		state := ts.reactor.conS.GetState()
		ts.reactor.SwitchToConsensus(state, false)
	}

	var wg sync.WaitGroup
	for _, ts := range testSuites {
		wg.Add(1)

		// We expect for each validator that is the proposer to propose one piece of
		// evidence.
		go func(rts *reactorTestSuite) {
			msg := <-rts.sub.Out()
			block := msg.Data().(types.EventDataNewBlock).Block

			require.Len(t, block.Evidence.Evidence, 1)
			wg.Done()
		}(ts)
	}

	wg.Wait()
}

func TestReactorCreatesBlockWhenEmptyBlocksFalse(t *testing.T) {
	configSetup(t)

	n := 4
	css, cleanup := randConsensusState(n, "consensus_reactor_test", newMockTickerFunc(true), newCounter,
		func(c *cfg.Config) {
			c.Consensus.CreateEmptyBlocks = false
		})

	t.Cleanup(func() {
		cleanup()
	})

	testSuites := make([]*reactorTestSuite, n)
	for i := range testSuites {
		testSuites[i] = setup(t, css[i], 100) // buffer must be large enough to not deadlock
	}

	for _, ts := range testSuites {
		simulateRouter(ts, testSuites, true)

		// connect reactor to every other reactor
		for _, tss := range testSuites {
			if ts.peerID != tss.peerID {
				ts.peerUpdatesCh <- p2p.PeerUpdate{
					Status: p2p.PeerStatusUp,
					NodeID: tss.peerID,
				}
			}
		}

		state := ts.reactor.conS.GetState()
		ts.reactor.SwitchToConsensus(state, false)
	}

	// send a tx
	require.NoError(t, assertMempool(css[3].txNotifier).CheckTx([]byte{1, 2, 3}, nil, mempl.TxInfo{}))

	var wg sync.WaitGroup
	for _, ts := range testSuites {
		wg.Add(1)

		// wait till everyone makes the first new block
		go func(rts *reactorTestSuite) {
			<-rts.sub.Out()
			wg.Done()
		}(ts)
	}

	wg.Wait()
}

func TestReactorRecordsVotesAndBlockParts(t *testing.T) {
	configSetup(t)

	n := 4
	css, cleanup := randConsensusState(n, "consensus_reactor_test", newMockTickerFunc(true), newCounter)

	t.Cleanup(func() {
		cleanup()
	})

	testSuites := make([]*reactorTestSuite, n)
	for i := range testSuites {
		testSuites[i] = setup(t, css[i], 100) // buffer must be large enough to not deadlock
	}

	for _, ts := range testSuites {
		simulateRouter(ts, testSuites, true)

		// connect reactor to every other reactor
		for _, tss := range testSuites {
			if ts.peerID != tss.peerID {
				ts.peerUpdatesCh <- p2p.PeerUpdate{
					Status: p2p.PeerStatusUp,
					NodeID: tss.peerID,
				}
			}
		}

		state := ts.reactor.conS.GetState()
		ts.reactor.SwitchToConsensus(state, false)
	}

	var wg sync.WaitGroup
	for _, ts := range testSuites {
		wg.Add(1)

		// wait till everyone makes the first new block
		go func(rts *reactorTestSuite) {
			<-rts.sub.Out()
			wg.Done()
		}(ts)
	}

	wg.Wait()

	var ps *PeerState

	require.Eventually(t, func() bool {
		var ok bool
		ps, ok = testSuites[1].reactor.GetPeerState(testSuites[0].peerID)
		return ok
	}, time.Second, 10*time.Millisecond)

	require.Equal(t, true, ps.VotesSent() > 0, "number of votes sent should have increased")
	require.Equal(t, true, ps.BlockPartsSent() > 0, "number of votes sent should have increased")
}

// ============================================================================
// ============================================================================
// ============================================================================

// //-------------------------------------------------------------
// // ensure we can make blocks despite cycling a validator set

// func TestReactorVotingPowerChange(t *testing.T) {
// 	nVals := 4
// 	logger := log.TestingLogger()
// 	css, cleanup := randConsensusNet(
// 		nVals,
// 		"consensus_voting_power_changes_test",
// 		newMockTickerFunc(true),
// 		newPersistentKVStore)
// 	defer cleanup()
// 	reactors, blocksSubs, eventBuses := startConsensusNet(t, css, nVals)
// 	defer stopConsensusNet(logger, reactors, eventBuses)

// 	// map of active validators
// 	activeVals := make(map[string]struct{})
// 	for i := 0; i < nVals; i++ {
// 		pubKey, err := css[i].privValidator.GetPubKey()
// 		require.NoError(t, err)
// 		addr := pubKey.Address()
// 		activeVals[string(addr)] = struct{}{}
// 	}

// 	// wait till everyone makes block 1
// 	timeoutWaitGroup(t, nVals, func(j int) {
// 		<-blocksSubs[j].Out()
// 	}, css)

// 	//---------------------------------------------------------------------------
// 	logger.Debug("---------------------------- Testing changing the voting power of one validator a few times")

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

// 	if css[0].GetRoundState().LastValidators.TotalVotingPower() == previousTotalVotingPower {
// 		t.Fatalf(
// 			"expected voting power to change (before: %d, after: %d)",
// 			previousTotalVotingPower,
// 			css[0].GetRoundState().LastValidators.TotalVotingPower())
// 	}

// 	updateValidatorTx = kvstore.MakeValSetChangeTx(val1PubKeyABCI, 2)
// 	previousTotalVotingPower = css[0].GetRoundState().LastValidators.TotalVotingPower()

// 	waitForAndValidateBlock(t, nVals, activeVals, blocksSubs, css, updateValidatorTx)
// 	waitForAndValidateBlockWithTx(t, nVals, activeVals, blocksSubs, css, updateValidatorTx)
// 	waitForAndValidateBlock(t, nVals, activeVals, blocksSubs, css)
// 	waitForAndValidateBlock(t, nVals, activeVals, blocksSubs, css)

// 	if css[0].GetRoundState().LastValidators.TotalVotingPower() == previousTotalVotingPower {
// 		t.Fatalf(
// 			"expected voting power to change (before: %d, after: %d)",
// 			previousTotalVotingPower,
// 			css[0].GetRoundState().LastValidators.TotalVotingPower())
// 	}

// 	updateValidatorTx = kvstore.MakeValSetChangeTx(val1PubKeyABCI, 26)
// 	previousTotalVotingPower = css[0].GetRoundState().LastValidators.TotalVotingPower()

// 	waitForAndValidateBlock(t, nVals, activeVals, blocksSubs, css, updateValidatorTx)
// 	waitForAndValidateBlockWithTx(t, nVals, activeVals, blocksSubs, css, updateValidatorTx)
// 	waitForAndValidateBlock(t, nVals, activeVals, blocksSubs, css)
// 	waitForAndValidateBlock(t, nVals, activeVals, blocksSubs, css)

// 	if css[0].GetRoundState().LastValidators.TotalVotingPower() == previousTotalVotingPower {
// 		t.Fatalf(
// 			"expected voting power to change (before: %d, after: %d)",
// 			previousTotalVotingPower,
// 			css[0].GetRoundState().LastValidators.TotalVotingPower())
// 	}
// }

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
