//nolint:lll
package consensus

import (
	"context"
	"fmt"
	"github.com/dashevo/dashd-go/btcjson"
	"os"
	"path"
	"runtime"
	"runtime/pprof"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	abcicli "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	cfg "github.com/tendermint/tendermint/config"
	cstypes "github.com/tendermint/tendermint/consensus/types"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/libs/bits"
	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/libs/log"
	tmsync "github.com/tendermint/tendermint/libs/sync"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	p2pmock "github.com/tendermint/tendermint/p2p/mock"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	sm "github.com/tendermint/tendermint/state"
	statemocks "github.com/tendermint/tendermint/state/mocks"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
)

//----------------------------------------------
// in-process testnets

var defaultTestTime = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)

func startConsensusNet(t *testing.T, css []*State, n int) (
	[]*Reactor,
	[]types.Subscription,
	[]*types.EventBus,
) {
	reactors := make([]*Reactor, n)
	blocksSubs := make([]types.Subscription, 0)
	eventBuses := make([]*types.EventBus, n)
	for i := 0; i < n; i++ {
		/*logger, err := tmflags.ParseLogLevel("consensus:info,*:error", logger, "info")
		if err != nil {	t.Fatal(err)}*/
		reactors[i] = NewReactor(css[i], true) // so we dont start the consensus states
		reactors[i].SetLogger(css[i].Logger)

		// eventBus is already started with the cs
		eventBuses[i] = css[i].eventBus
		reactors[i].SetEventBus(eventBuses[i])

		blocksSub, err := eventBuses[i].Subscribe(context.Background(), testSubscriber, types.EventQueryNewBlock)
		require.NoError(t, err)
		blocksSubs = append(blocksSubs, blocksSub)

		if css[i].state.LastBlockHeight == 0 { // simulate handle initChain in handshake
			if err := css[i].blockExec.Store().Save(css[i].state); err != nil {
				t.Error(err)
			}

		}
	}
	// make connected switches and start all reactors
	p2p.MakeConnectedSwitches(config.P2P, n, func(i int, s *p2p.Switch) *p2p.Switch {
		s.AddReactor("CONSENSUS", reactors[i])
		s.SetLogger(reactors[i].conS.Logger.With("module", "p2p"))
		return s
	}, p2p.Connect2Switches)

	// now that everyone is connected,  start the state machines
	// If we started the state machines before everyone was connected,
	// we'd block when the cs fires NewBlockEvent and the peers are trying to start their reactors
	// TODO: is this still true with new pubsub?
	for i := 0; i < n; i++ {
		s := reactors[i].conS.GetState()
		reactors[i].SwitchToConsensus(s, false)
	}
	return reactors, blocksSubs, eventBuses
}

func stopConsensusNet(logger log.Logger, reactors []*Reactor, eventBuses []*types.EventBus) {
	logger.Info("stopConsensusNet", "n", len(reactors))
	for i, r := range reactors {
		logger.Info("stopConsensusNet: Stopping Reactor", "i", i)
		if err := r.Switch.Stop(); err != nil {
			logger.Error("error trying to stop switch", "error", err)
		}
	}
	for i, b := range eventBuses {
		logger.Info("stopConsensusNet: Stopping eventBus", "i", i)
		if err := b.Stop(); err != nil {
			logger.Error("error trying to stop eventbus", "error", err)
		}
	}
	logger.Info("stopConsensusNet: DONE", "n", len(reactors))
}

// Ensure a testnet makes blocks
func TestReactorBasic(t *testing.T) {
	N := 4
	css, cleanup := randConsensusNet(N, "consensus_reactor_test", newMockTickerFunc(true), newCounter)
	defer cleanup()
	reactors, blocksSubs, eventBuses := startConsensusNet(t, css, N)
	defer stopConsensusNet(log.TestingLogger(), reactors, eventBuses)
	// wait till everyone makes the first new block
	timeoutWaitGroup(t, N, func(j int) {
		<-blocksSubs[j].Out()
	}, css)
}

// Ensure we can process blocks with evidence
func TestReactorWithEvidence(t *testing.T) {
	nValidators := 4
	testName := "consensus_reactor_test"
	tickerFunc := newMockTickerFunc(true)
	appFunc := newCounter

	// heed the advice from https://www.sandimetz.com/blog/2016/1/20/the-wrong-abstraction
	// to unroll unwieldy abstractions. Here we duplicate the code from:
	// css := randConsensusNet(N, "consensus_reactor_test", newMockTickerFunc(true), newCounter)

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
		app.InitChain(abci.RequestInitChain{ValidatorSet: vals})

		pv := privVals[i]
		// duplicate code from:
		// css[i] = newStateWithConfig(thisConfig, state, privVals[i], app)

		blockDB := dbm.NewMemDB()
		blockStore := store.NewBlockStore(blockDB)

		// one for mempool, one for consensus
		mtx := new(tmsync.Mutex)
		proxyAppConnMem := abcicli.NewLocalClient(mtx, app)
		proxyAppConnCon := abcicli.NewLocalClient(mtx, app)
		proxyAppConnQry := abcicli.NewLocalClient(mtx, app)

		// Make Mempool
		mempool := mempl.NewCListMempool(thisConfig.Mempool, proxyAppConnMem, 0)
		mempool.SetLogger(log.TestingLogger().With("module", "mempool"))
		if thisConfig.Consensus.WaitForTxs() {
			mempool.EnableTxsAvailable()
		}

		// mock the evidence pool
		// everyone includes evidence of another double signing
		vIdx := (i + 1) % nValidators
		ev := types.NewMockDuplicateVoteEvidenceWithValidator(1, defaultTestTime, privVals[vIdx], config.ChainID(),
			btcjson.LLMQType_5_60, state.Validators.QuorumHash)
		evpool := &statemocks.EvidencePool{}
		evpool.On("CheckEvidence", mock.AnythingOfType("types.EvidenceList")).Return(nil)
		evpool.On("PendingEvidence", mock.AnythingOfType("int64")).Return([]types.Evidence{
			ev}, int64(len(ev.Bytes())))
		evpool.On("Update", mock.AnythingOfType("state.State"), mock.AnythingOfType("types.EvidenceList")).Return()

		evpool2 := sm.EmptyEvidencePool{}

		// Make State
		blockExec := sm.NewBlockExecutor(stateStore, log.TestingLogger(), proxyAppConnCon, proxyAppConnQry, mempool, evpool, nil)
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

	reactors, blocksSubs, eventBuses := startConsensusNet(t, css, nValidators)
	defer stopConsensusNet(log.TestingLogger(), reactors, eventBuses)

	// we expect for each validator that is the proposer to propose one piece of evidence.
	for i := 0; i < nValidators; i++ {
		timeoutWaitGroup(t, nValidators, func(j int) {
			msg := <-blocksSubs[j].Out()
			block := msg.Data().(types.EventDataNewBlock).Block
			assert.Len(t, block.Evidence.Evidence, 1)
		}, css)
	}
}

//------------------------------------

// Ensure a testnet makes blocks when there are txs
func TestReactorCreatesBlockWhenEmptyBlocksFalse(t *testing.T) {
	N := 4
	css, cleanup := randConsensusNet(N, "consensus_reactor_test", newMockTickerFunc(true), newCounter,
		func(c *cfg.Config) {
			c.Consensus.CreateEmptyBlocks = false
		})
	defer cleanup()
	reactors, blocksSubs, eventBuses := startConsensusNet(t, css, N)
	defer stopConsensusNet(log.TestingLogger(), reactors, eventBuses)

	// send a tx
	if err := assertMempool(css[3].txNotifier).CheckTx([]byte{1, 2, 3}, nil, mempl.TxInfo{}); err != nil {
		t.Error(err)
	}

	// wait till everyone makes the first new block
	timeoutWaitGroup(t, N, func(j int) {
		<-blocksSubs[j].Out()
	}, css)
}

func TestReactorReceiveDoesNotPanicIfAddPeerHasntBeenCalledYet(t *testing.T) {
	N := 1
	css, cleanup := randConsensusNet(N, "consensus_reactor_test", newMockTickerFunc(true), newCounter)
	defer cleanup()
	reactors, _, eventBuses := startConsensusNet(t, css, N)
	defer stopConsensusNet(log.TestingLogger(), reactors, eventBuses)

	var (
		reactor = reactors[0]
		peer    = p2pmock.NewPeer(nil)
		msg     = MustEncode(&HasVoteMessage{Height: 1,
			Round: 1, Index: 1, Type: tmproto.PrevoteType})
	)

	reactor.InitPeer(peer)

	// simulate switch calling Receive before AddPeer
	assert.NotPanics(t, func() {
		reactor.Receive(StateChannel, peer, msg)
		reactor.AddPeer(peer)
	})
}

func TestReactorReceivePanicsIfInitPeerHasntBeenCalledYet(t *testing.T) {
	N := 1
	css, cleanup := randConsensusNet(N, "consensus_reactor_test", newMockTickerFunc(true), newCounter)
	defer cleanup()
	reactors, _, eventBuses := startConsensusNet(t, css, N)
	defer stopConsensusNet(log.TestingLogger(), reactors, eventBuses)

	var (
		reactor = reactors[0]
		peer    = p2pmock.NewPeer(nil)
		msg     = MustEncode(&HasVoteMessage{Height: 1,
			Round: 1, Index: 1, Type: tmproto.PrevoteType})
	)

	// we should call InitPeer here

	// simulate switch calling Receive before AddPeer
	assert.Panics(t, func() {
		reactor.Receive(StateChannel, peer, msg)
	})
}

// Test we record stats about votes and block parts from other peers.
func TestReactorRecordsVotesAndBlockParts(t *testing.T) {
	N := 4
	css, cleanup := randConsensusNet(N, "consensus_reactor_test", newMockTickerFunc(true), newCounter)
	defer cleanup()
	reactors, blocksSubs, eventBuses := startConsensusNet(t, css, N)
	defer stopConsensusNet(log.TestingLogger(), reactors, eventBuses)

	// wait till everyone makes the first new block
	timeoutWaitGroup(t, N, func(j int) {
		<-blocksSubs[j].Out()
	}, css)

	// Get peer
	peer := reactors[1].Switch.Peers().List()[0]
	// Get peer state
	ps := peer.Get(types.PeerStateKey).(*PeerState)

	assert.Equal(t, true, ps.VotesSent() > 0, "number of votes sent should have increased")
	assert.Equal(t, true, ps.BlockPartsSent() > 0, "number of votes sent should have increased")
}

func TestReactorValidatorSetChanges(t *testing.T) {
	nPeers := 7
	nVals := 4
	css, _, _, cleanup := randConsensusNetWithPeers(
		nVals,
		nPeers,
		"consensus_val_set_changes_test",
		newMockTickerFunc(true),
		newPersistentKVStoreWithPath)

	defer cleanup()
	logger := log.TestingLogger()

	reactors, blocksSubs, eventBuses := startConsensusNet(t, css, nPeers)
	defer stopConsensusNet(logger, reactors, eventBuses)

	// map of active validators
	activeVals := make(map[string]struct{})
	for i := 0; i < nVals; i++ {
		proTxHash, err := css[i].privValidator.GetProTxHash()
		require.NoError(t, err)
		activeVals[string(proTxHash)] = struct{}{}
	}

	// wait till everyone makes block 1
	timeoutWaitGroup(t, nPeers, func(j int) {
		<-blocksSubs[j].Out()
	}, css)

	//---------------------------------------------------------------------------
	logger.Info("---------------------------- Testing adding one validator")

	// css contains all peers, the first 4 here are validators, so we will take the 5th peer and add it as a validator

	updatedValidators, newValidatorProTxHashes, newThresholdPublicKey, quorumHash := updateConsensusNetAddNewValidators(css, 2, 1, true)

	updateTransactions := make([][]byte, len(updatedValidators)+2)
	for i := 0; i < len(updatedValidators); i++ {
		// start by adding all validator transactions
		abciPubKey, err := cryptoenc.PubKeyToProto(updatedValidators[i].PubKey)
		require.NoError(t, err)
		updateTransactions[i] = kvstore.MakeValSetChangeTx(updatedValidators[i].ProTxHash, abciPubKey, testMinPower)
	}
	abciThresholdPubKey, err := cryptoenc.PubKeyToProto(newThresholdPublicKey)
	require.NoError(t, err)
	updateTransactions[len(updatedValidators)] = kvstore.MakeThresholdPublicKeyChangeTx(abciThresholdPubKey)

	updateTransactions[len(updatedValidators)+1] = kvstore.MakeQuorumHashTx(quorumHash)

	// wait till everyone makes block 2
	// ensure the commit includes all validators
	// send newValTx to change vals in block 3
	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, css, updateTransactions...)

	// wait till everyone makes block 3.
	// it includes the commit for block 2, which is by the original validator set
	waitForAndValidateBlockWithTx(t, nPeers, activeVals, blocksSubs, css, updateTransactions...)

	// wait till everyone makes block 4.
	// it includes the commit for block 3, which is by the original validator set
	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, css)

	// the commits for block 4 should be with the updated validator set
	activeVals[string(newValidatorProTxHashes[0])] = struct{}{}

	// wait till everyone makes block 6
	// it includes the commit for block 4, which should have the updated validator set
	waitForBlockWithUpdatedValsAndValidateIt(t, nPeers, activeVals, blocksSubs, css)

	//---------------------------------------------------------------------------
	logger.Info("---------------------------- Testing adding two validators at once")

	updatedValidators, newValidatorProTxHashes, newThresholdPublicKey, quorumHash = updateConsensusNetAddNewValidators(css, 7, 2, true)

	updateTransactions2 := make([][]byte, len(updatedValidators)+2)
	for i := 0; i < len(updatedValidators); i++ {
		// start by adding all validator transactions
		abciPubKey, err := cryptoenc.PubKeyToProto(updatedValidators[i].PubKey)
		require.NoError(t, err)
		updateTransactions2[i] = kvstore.MakeValSetChangeTx(updatedValidators[i].ProTxHash, abciPubKey, testMinPower)
	}
	abciThresholdPubKey, err = cryptoenc.PubKeyToProto(newThresholdPublicKey)
	require.NoError(t, err)
	updateTransactions2[len(updatedValidators)] = kvstore.MakeThresholdPublicKeyChangeTx(abciThresholdPubKey)

	updateTransactions2[len(updatedValidators)+1] = kvstore.MakeQuorumHashTx(quorumHash)

	// block 7
	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, css, updateTransactions2...)
	// block 8
	waitForAndValidateBlockWithTx(t, nPeers, activeVals, blocksSubs, css, updateTransactions2...)
	// block 9
	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, css)
	activeVals[string(newValidatorProTxHashes[0])] = struct{}{}
	activeVals[string(newValidatorProTxHashes[1])] = struct{}{}
	// block 11
	waitForBlockWithUpdatedValsAndValidateIt(t, nPeers, activeVals, blocksSubs, css)

	//---------------------------------------------------------------------------
	logger.Info("---------------------------- Testing removing two validators at once")

	// since the quorum hash is changing, we do not need to set the removed validators

	updatedValidators, removedValidators, newThresholdPublicKey, newQuorumHash := updateConsensusNetRemoveValidators(css, 12, 2, true)

	updateTransactions3 := make([][]byte, len(updatedValidators)+2)
	for i := 0; i < len(updatedValidators); i++ {
		// start by adding all validator transactions
		abciPubKey, err := cryptoenc.PubKeyToProto(updatedValidators[i].PubKey)
		require.NoError(t, err)
		updateTransactions3[i] = kvstore.MakeValSetChangeTx(updatedValidators[i].ProTxHash, abciPubKey, testMinPower)
	}
	abciThresholdPubKey, err = cryptoenc.PubKeyToProto(newThresholdPublicKey)
	require.NoError(t, err)
	updateTransactions3[len(updatedValidators)] = kvstore.MakeThresholdPublicKeyChangeTx(abciThresholdPubKey)

	updateTransactions3[len(updatedValidators)+1] = kvstore.MakeQuorumHashTx(newQuorumHash)

	// block 12
	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, css, updateTransactions3...)
	// block 13
	waitForAndValidateBlockWithTx(t, nPeers, activeVals, blocksSubs, css, updateTransactions3...)
	// block 14
	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, css)
	delete(activeVals, string(removedValidators[0].ProTxHash))
	delete(activeVals, string(removedValidators[1].ProTxHash))
	// block 16
	waitForBlockWithUpdatedValsAndValidateIt(t, nPeers, activeVals, blocksSubs, css)
}

// Check we can make blocks with skip_timeout_commit=false
func TestReactorWithTimeoutCommit(t *testing.T) {
	N := 4
	css, cleanup := randConsensusNet(N, "consensus_reactor_with_timeout_commit_test", newMockTickerFunc(false), newCounter)
	defer cleanup()
	// override default SkipTimeoutCommit == true for tests
	for i := 0; i < N; i++ {
		css[i].config.SkipTimeoutCommit = false
	}

	reactors, blocksSubs, eventBuses := startConsensusNet(t, css, N-1)
	defer stopConsensusNet(log.TestingLogger(), reactors, eventBuses)

	// wait till everyone makes the first new block
	timeoutWaitGroup(t, N-1, func(j int) {
		<-blocksSubs[j].Out()
	}, css)
}

func waitForAndValidateBlock(
	t *testing.T,
	n int,
	activeVals map[string]struct{},
	blocksSubs []types.Subscription,
	css []*State,
	txs ...[]byte,
) {
	timeoutWaitGroup(t, n, func(j int) {
		css[j].Logger.Debug("waitForAndValidateBlock")
		msg := <-blocksSubs[j].Out()
		newBlock := msg.Data().(types.EventDataNewBlock).Block
		css[j].Logger.Debug("waitForAndValidateBlock: Got block", "height", newBlock.Height)
		err := validateBlock(newBlock, activeVals)
		assert.Nil(t, err)
		for _, tx := range txs {
			err := assertMempool(css[j].txNotifier).CheckTx(tx, nil, mempl.TxInfo{})
			assert.Nil(t, err)
		}
	}, css)
}

func waitForAndValidateBlockWithTx(
	t *testing.T,
	n int,
	activeVals map[string]struct{},
	blocksSubs []types.Subscription,
	css []*State,
	txs ...[]byte,
) {
	timeoutWaitGroup(t, n, func(j int) {
		ntxs := 0
	BLOCK_TX_LOOP:
		for {
			css[j].Logger.Debug("waitForAndValidateBlockWithTx", "ntxs", ntxs)
			msg := <-blocksSubs[j].Out()
			newBlock := msg.Data().(types.EventDataNewBlock).Block
			css[j].Logger.Debug("waitForAndValidateBlockWithTx: Got block", "height", newBlock.Height)
			err := validateBlock(newBlock, activeVals)
			assert.Nil(t, err)

			// check that txs match the txs we're waiting for.
			// note they could be spread over multiple blocks,
			// but they should be in order.
			for _, tx := range newBlock.Data.Txs {
				assert.EqualValues(t, txs[ntxs], tx)
				ntxs++
			}

			if ntxs == len(txs) {
				break BLOCK_TX_LOOP
			}
		}

	}, css)
}

func waitForBlockWithUpdatedValsAndValidateIt(
	t *testing.T,
	n int,
	updatedVals map[string]struct{},
	blocksSubs []types.Subscription,
	css []*State,
) {
	timeoutWaitGroup(t, n, func(j int) {

		var newBlock *types.Block
	LOOP:
		for {
			css[j].Logger.Debug("waitForBlockWithUpdatedValsAndValidateIt")
			msg := <-blocksSubs[j].Out()
			newBlock = msg.Data().(types.EventDataNewBlock).Block
			if newBlock.LastCommit.Size() == len(updatedVals) {
				css[j].Logger.Debug("waitForBlockWithUpdatedValsAndValidateIt: Got block", "height", newBlock.Height)
				break LOOP
			} else {
				css[j].Logger.Debug(
					"waitForBlockWithUpdatedValsAndValidateIt: Got block with no new validators. Skipping",
					"height",
					newBlock.Height)
			}
		}

		err := validateBlock(newBlock, updatedVals)
		assert.Nil(t, err)
	}, css)
}

// expects high synchrony!
func validateBlock(block *types.Block, activeVals map[string]struct{}) error {
	if block.LastCommit.Size() != len(activeVals) {
		return fmt.Errorf(
			"commit size doesn't match number of active validators. Got %d, expected %d",
			block.LastCommit.Size(),
			len(activeVals))
	}

	for _, commitSig := range block.LastCommit.Signatures {
		if _, ok := activeVals[string(commitSig.ValidatorProTxHash)]; !ok {
			return fmt.Errorf("found vote for inactive validator %X", commitSig.ValidatorProTxHash)
		}
	}
	return nil
}

func timeoutWaitGroup(t *testing.T, n int, f func(int), css []*State) {
	wg := new(sync.WaitGroup)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(j int) {
			f(j)
			wg.Done()
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// we're running many nodes in-process, possibly in in a virtual machine,
	// and spewing debug messages - making a block could take a while,
	timeout := time.Second * 120

	select {
	case <-done:
	case <-time.After(timeout):
		for i, cs := range css {
			t.Log("#################")
			t.Log("Validator", i)
			t.Log(cs.GetRoundState())
			t.Log("")
		}
		os.Stdout.Write([]byte("pprof.Lookup('goroutine'):\n"))
		err := pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
		require.NoError(t, err)
		capture()
		panic("Timed out waiting for all validators to commit a block")
	}
}

func capture() {
	trace := make([]byte, 10240000)
	count := runtime.Stack(trace, true)
	fmt.Printf("Stack of %d bytes: %s\n", count, trace)
}

//-------------------------------------------------------------
// Ensure basic validation of structs is functioning

func TestNewRoundStepMessageValidateBasic(t *testing.T) {
	testCases := []struct { // nolint: maligned
		expectErr              bool
		messageRound           int32
		messageLastCommitRound int32
		messageHeight          int64
		testName               string
		messageStep            cstypes.RoundStepType
	}{
		{false, 0, 0, 0, "Valid Message", cstypes.RoundStepNewHeight},
		{true, -1, 0, 0, "Negative round", cstypes.RoundStepNewHeight},
		{true, 0, 0, -1, "Negative height", cstypes.RoundStepNewHeight},
		{true, 0, 0, 0, "Invalid Step", cstypes.RoundStepApplyCommit + 1},
		// The following cases will be handled by ValidateHeight
		{false, 0, 0, 1, "H == 1 but LCR != -1 ", cstypes.RoundStepNewHeight},
		{false, 0, -1, 2, "H > 1 but LCR < 0", cstypes.RoundStepNewHeight},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			message := NewRoundStepMessage{
				Height:          tc.messageHeight,
				Round:           tc.messageRound,
				Step:            tc.messageStep,
				LastCommitRound: tc.messageLastCommitRound,
			}

			err := message.ValidateBasic()
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNewRoundStepMessageValidateHeight(t *testing.T) {
	initialHeight := int64(10)
	testCases := []struct { // nolint: maligned
		expectErr              bool
		messageLastCommitRound int32
		messageHeight          int64
		testName               string
	}{
		{false, 0, 11, "Valid Message"},
		{true, 0, -1, "Negative height"},
		{true, 0, 0, "Zero height"},
		{true, 0, 10, "Initial height but LCR != -1 "},
		{true, -1, 11, "Normal height but LCR < 0"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			message := NewRoundStepMessage{
				Height:          tc.messageHeight,
				Round:           0,
				Step:            cstypes.RoundStepNewHeight,
				LastCommitRound: tc.messageLastCommitRound,
			}

			err := message.ValidateHeight(initialHeight)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNewValidBlockMessageValidateBasic(t *testing.T) {
	testCases := []struct {
		malleateFn func(*NewValidBlockMessage)
		expErr     string
	}{
		{func(msg *NewValidBlockMessage) {}, ""},
		{func(msg *NewValidBlockMessage) { msg.Height = -1 }, "negative Height"},
		{func(msg *NewValidBlockMessage) { msg.Round = -1 }, "negative Round"},
		{
			func(msg *NewValidBlockMessage) { msg.BlockPartSetHeader.Total = 2 },
			"blockParts bit array size 1 not equal to BlockPartSetHeader.Total 2",
		},
		{
			func(msg *NewValidBlockMessage) {
				msg.BlockPartSetHeader.Total = 0
				msg.BlockParts = bits.NewBitArray(0)
			},
			"empty blockParts",
		},
		{
			func(msg *NewValidBlockMessage) { msg.BlockParts = bits.NewBitArray(int(types.MaxBlockPartsCount) + 1) },
			"blockParts bit array size 1602 not equal to BlockPartSetHeader.Total 1",
		},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("#%d", i), func(t *testing.T) {
			msg := &NewValidBlockMessage{
				Height: 1,
				Round:  0,
				BlockPartSetHeader: types.PartSetHeader{
					Total: 1,
				},
				BlockParts: bits.NewBitArray(1),
			}

			tc.malleateFn(msg)
			err := msg.ValidateBasic()
			if tc.expErr != "" && assert.Error(t, err) {
				assert.Contains(t, err.Error(), tc.expErr)
			}
		})
	}
}

func TestProposalPOLMessageValidateBasic(t *testing.T) {
	testCases := []struct {
		malleateFn func(*ProposalPOLMessage)
		expErr     string
	}{
		{func(msg *ProposalPOLMessage) {}, ""},
		{func(msg *ProposalPOLMessage) { msg.Height = -1 }, "negative Height"},
		{func(msg *ProposalPOLMessage) { msg.ProposalPOLRound = -1 }, "negative ProposalPOLRound"},
		{func(msg *ProposalPOLMessage) { msg.ProposalPOL = bits.NewBitArray(0) }, "empty ProposalPOL bit array"},
		{func(msg *ProposalPOLMessage) { msg.ProposalPOL = bits.NewBitArray(types.MaxVotesCount + 1) },
			"proposalPOL bit array is too big: 10001, max: 10000"},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("#%d", i), func(t *testing.T) {
			msg := &ProposalPOLMessage{
				Height:           1,
				ProposalPOLRound: 1,
				ProposalPOL:      bits.NewBitArray(1),
			}

			tc.malleateFn(msg)
			err := msg.ValidateBasic()
			if tc.expErr != "" && assert.Error(t, err) {
				assert.Contains(t, err.Error(), tc.expErr)
			}
		})
	}
}

func TestBlockPartMessageValidateBasic(t *testing.T) {
	testPart := new(types.Part)
	testPart.Proof.LeafHash = tmhash.Sum([]byte("leaf"))
	testCases := []struct {
		testName      string
		messageHeight int64
		messageRound  int32
		messagePart   *types.Part
		expectErr     bool
	}{
		{"Valid Message", 0, 0, testPart, false},
		{"Invalid Message", -1, 0, testPart, true},
		{"Invalid Message", 0, -1, testPart, true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			message := BlockPartMessage{
				Height: tc.messageHeight,
				Round:  tc.messageRound,
				Part:   tc.messagePart,
			}

			assert.Equal(t, tc.expectErr, message.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}

	message := BlockPartMessage{Height: 0, Round: 0, Part: new(types.Part)}
	message.Part.Index = 1

	assert.Equal(t, true, message.ValidateBasic() != nil, "Validate Basic had an unexpected result")
}

func TestHasVoteMessageValidateBasic(t *testing.T) {
	const (
		validSignedMsgType   tmproto.SignedMsgType = 0x01
		invalidSignedMsgType tmproto.SignedMsgType = 0x03
	)

	testCases := []struct { // nolint: maligned
		expectErr     bool
		messageRound  int32
		messageIndex  int32
		messageHeight int64
		testName      string
		messageType   tmproto.SignedMsgType
	}{
		{false, 0, 0, 0, "Valid Message", validSignedMsgType},
		{true, -1, 0, 0, "Invalid Message", validSignedMsgType},
		{true, 0, -1, 0, "Invalid Message", validSignedMsgType},
		{true, 0, 0, 0, "Invalid Message", invalidSignedMsgType},
		{true, 0, 0, -1, "Invalid Message", validSignedMsgType},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			message := HasVoteMessage{
				Height: tc.messageHeight,
				Round:  tc.messageRound,
				Type:   tc.messageType,
				Index:  tc.messageIndex,
			}

			assert.Equal(t, tc.expectErr, message.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestVoteSetMaj23MessageValidateBasic(t *testing.T) {
	const (
		validSignedMsgType   tmproto.SignedMsgType = 0x01
		invalidSignedMsgType tmproto.SignedMsgType = 0x03
	)

	validBlockID := types.BlockID{}
	invalidBlockID := types.BlockID{
		Hash: bytes.HexBytes{},
		PartSetHeader: types.PartSetHeader{
			Total: 1,
			Hash:  []byte{0},
		},
	}

	testCases := []struct { // nolint: maligned
		expectErr      bool
		messageRound   int32
		messageHeight  int64
		testName       string
		messageType    tmproto.SignedMsgType
		messageBlockID types.BlockID
	}{
		{false, 0, 0, "Valid Message", validSignedMsgType, validBlockID},
		{true, -1, 0, "Invalid Message", validSignedMsgType, validBlockID},
		{true, 0, -1, "Invalid Message", validSignedMsgType, validBlockID},
		{true, 0, 0, "Invalid Message", invalidSignedMsgType, validBlockID},
		{true, 0, 0, "Invalid Message", validSignedMsgType, invalidBlockID},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			message := VoteSetMaj23Message{
				Height:  tc.messageHeight,
				Round:   tc.messageRound,
				Type:    tc.messageType,
				BlockID: tc.messageBlockID,
			}

			assert.Equal(t, tc.expectErr, message.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestVoteSetBitsMessageValidateBasic(t *testing.T) {
	testCases := []struct {
		malleateFn func(*VoteSetBitsMessage)
		expErr     string
	}{
		{func(msg *VoteSetBitsMessage) {}, ""},
		{func(msg *VoteSetBitsMessage) { msg.Height = -1 }, "negative Height"},
		{func(msg *VoteSetBitsMessage) { msg.Type = 0x03 }, "invalid Type"},
		{func(msg *VoteSetBitsMessage) {
			msg.BlockID = types.BlockID{
				Hash: bytes.HexBytes{},
				PartSetHeader: types.PartSetHeader{
					Total: 1,
					Hash:  []byte{0},
				},
			}
		}, "wrong BlockID: wrong PartSetHeader: wrong Hash:"},
		{func(msg *VoteSetBitsMessage) { msg.Votes = bits.NewBitArray(types.MaxVotesCount + 1) },
			"votes bit array is too big: 10001, max: 10000"},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("#%d", i), func(t *testing.T) {
			msg := &VoteSetBitsMessage{
				Height:  1,
				Round:   0,
				Type:    0x01,
				Votes:   bits.NewBitArray(1),
				BlockID: types.BlockID{},
			}

			tc.malleateFn(msg)
			err := msg.ValidateBasic()
			if tc.expErr != "" && assert.Error(t, err) {
				assert.Contains(t, err.Error(), tc.expErr)
			}
		})
	}
}
