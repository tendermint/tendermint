package consensus

import (
	"context"
	"fmt"
	"os"
	"path"
	"runtime"
	"runtime/pprof"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abcicli "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	cfg "github.com/tendermint/tendermint/config"
	cstypes "github.com/tendermint/tendermint/consensus/types"
	"github.com/tendermint/tendermint/crypto/tmhash"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/mock"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

//----------------------------------------------
// in-process testnets

func startConsensusNet(t *testing.T, css []*ConsensusState, n int) (
	[]*ConsensusReactor,
	[]types.Subscription,
	[]*types.EventBus,
) {
	reactors := make([]*ConsensusReactor, n)
	blocksSubs := make([]types.Subscription, 0)
	eventBuses := make([]*types.EventBus, n)
	for i := 0; i < n; i++ {
		/*logger, err := tmflags.ParseLogLevel("consensus:info,*:error", logger, "info")
		if err != nil {	t.Fatal(err)}*/
		reactors[i] = NewConsensusReactor(css[i], true) // so we dont start the consensus states
		reactors[i].SetLogger(css[i].Logger)

		// eventBus is already started with the cs
		eventBuses[i] = css[i].eventBus
		reactors[i].SetEventBus(eventBuses[i])

		blocksSub, err := eventBuses[i].Subscribe(context.Background(), testSubscriber, types.EventQueryNewBlock)
		require.NoError(t, err)
		blocksSubs = append(blocksSubs, blocksSub)

		if css[i].state.LastBlockHeight == 0 { //simulate handle initChain in handshake
			sm.SaveState(css[i].blockExec.DB(), css[i].state)
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
		reactors[i].SwitchToConsensus(s, 0)
	}
	return reactors, blocksSubs, eventBuses
}

func stopConsensusNet(logger log.Logger, reactors []*ConsensusReactor, eventBuses []*types.EventBus) {
	logger.Info("stopConsensusNet", "n", len(reactors))
	for i, r := range reactors {
		logger.Info("stopConsensusNet: Stopping ConsensusReactor", "i", i)
		r.Switch.Stop()
	}
	for i, b := range eventBuses {
		logger.Info("stopConsensusNet: Stopping eventBus", "i", i)
		b.Stop()
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
	types.RegisterMockEvidences(cdc)
	types.RegisterMockEvidences(types.GetCodec())

	nValidators := 4
	testName := "consensus_reactor_test"
	tickerFunc := newMockTickerFunc(true)
	appFunc := newCounter

	// heed the advice from https://www.sandimetz.com/blog/2016/1/20/the-wrong-abstraction
	// to unroll unwieldy abstractions. Here we duplicate the code from:
	// css := randConsensusNet(N, "consensus_reactor_test", newMockTickerFunc(true), newCounter)

	genDoc, privVals := randGenesisDoc(nValidators, false, 30)
	css := make([]*ConsensusState, nValidators)
	logger := consensusLogger()
	for i := 0; i < nValidators; i++ {
		stateDB := dbm.NewMemDB() // each state needs its own db
		state, _ := sm.LoadStateFromDBOrGenesisDoc(stateDB, genDoc)
		thisConfig := ResetConfig(fmt.Sprintf("%s_%d", testName, i))
		defer os.RemoveAll(thisConfig.RootDir)
		ensureDir(path.Dir(thisConfig.Consensus.WalFile()), 0700) // dir for wal
		app := appFunc()
		vals := types.TM2PB.ValidatorUpdates(state.Validators)
		app.InitChain(abci.RequestInitChain{Validators: vals})

		pv := privVals[i]
		// duplicate code from:
		// css[i] = newConsensusStateWithConfig(thisConfig, state, privVals[i], app)

		blockDB := dbm.NewMemDB()
		blockStore := store.NewBlockStore(blockDB)

		// one for mempool, one for consensus
		mtx := new(sync.Mutex)
		proxyAppConnMem := abcicli.NewLocalClient(mtx, app)
		proxyAppConnCon := abcicli.NewLocalClient(mtx, app)

		// Make Mempool
		mempool := mempl.NewCListMempool(thisConfig.Mempool, proxyAppConnMem, 0)
		mempool.SetLogger(log.TestingLogger().With("module", "mempool"))
		if thisConfig.Consensus.WaitForTxs() {
			mempool.EnableTxsAvailable()
		}

		// mock the evidence pool
		// everyone includes evidence of another double signing
		vIdx := (i + 1) % nValidators
		addr := privVals[vIdx].GetPubKey().Address()
		evpool := newMockEvidencePool(addr)

		// Make ConsensusState
		blockExec := sm.NewBlockExecutor(stateDB, log.TestingLogger(), proxyAppConnCon, mempool, evpool)
		cs := NewConsensusState(thisConfig.Consensus, state, blockExec, blockStore, mempool, evpool)
		cs.SetLogger(log.TestingLogger().With("module", "consensus"))
		cs.SetPrivValidator(pv)

		eventBus := types.NewEventBus()
		eventBus.SetLogger(log.TestingLogger().With("module", "events"))
		eventBus.Start()
		cs.SetEventBus(eventBus)

		cs.SetTimeoutTicker(tickerFunc())
		cs.SetLogger(logger.With("validator", i, "module", "consensus"))

		css[i] = cs
	}

	reactors, blocksSubs, eventBuses := startConsensusNet(t, css, nValidators)
	defer stopConsensusNet(log.TestingLogger(), reactors, eventBuses)

	// wait till everyone makes the first new block with no evidence
	timeoutWaitGroup(t, nValidators, func(j int) {
		msg := <-blocksSubs[j].Out()
		block := msg.Data().(types.EventDataNewBlock).Block
		assert.True(t, len(block.Evidence.Evidence) == 0)
	}, css)

	// second block should have evidence
	timeoutWaitGroup(t, nValidators, func(j int) {
		msg := <-blocksSubs[j].Out()
		block := msg.Data().(types.EventDataNewBlock).Block
		assert.True(t, len(block.Evidence.Evidence) > 0)
	}, css)
}

// mock evidence pool returns no evidence for block 1,
// and returnes one piece for all higher blocks. The one piece
// is for a given validator at block 1.
type mockEvidencePool struct {
	height int
	ev     []types.Evidence
}

func newMockEvidencePool(val []byte) *mockEvidencePool {
	return &mockEvidencePool{
		ev: []types.Evidence{types.NewMockGoodEvidence(1, 1, val)},
	}
}

// NOTE: maxBytes is ignored
func (m *mockEvidencePool) PendingEvidence(maxBytes int64) []types.Evidence {
	if m.height > 0 {
		return m.ev
	}
	return nil
}
func (m *mockEvidencePool) AddEvidence(types.Evidence) error { return nil }
func (m *mockEvidencePool) Update(block *types.Block, state sm.State) {
	if m.height > 0 {
		if len(block.Evidence.Evidence) == 0 {
			panic("block has no evidence")
		}
	}
	m.height++
}
func (m *mockEvidencePool) IsCommitted(types.Evidence) bool { return false }

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
	if err := assertMempool(css[3].txNotifier).CheckTx([]byte{1, 2, 3}, nil); err != nil {
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
		peer    = mock.NewPeer(nil)
		msg     = cdc.MustMarshalBinaryBare(&HasVoteMessage{Height: 1, Round: 1, Index: 1, Type: types.PrevoteType})
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
		peer    = mock.NewPeer(nil)
		msg     = cdc.MustMarshalBinaryBare(&HasVoteMessage{Height: 1, Round: 1, Index: 1, Type: types.PrevoteType})
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

//-------------------------------------------------------------
// ensure we can make blocks despite cycling a validator set

func TestReactorVotingPowerChange(t *testing.T) {
	nVals := 4
	logger := log.TestingLogger()
	css, cleanup := randConsensusNet(nVals, "consensus_voting_power_changes_test", newMockTickerFunc(true), newPersistentKVStore)
	defer cleanup()
	reactors, blocksSubs, eventBuses := startConsensusNet(t, css, nVals)
	defer stopConsensusNet(logger, reactors, eventBuses)

	// map of active validators
	activeVals := make(map[string]struct{})
	for i := 0; i < nVals; i++ {
		addr := css[i].privValidator.GetPubKey().Address()
		activeVals[string(addr)] = struct{}{}
	}

	// wait till everyone makes block 1
	timeoutWaitGroup(t, nVals, func(j int) {
		<-blocksSubs[j].Out()
	}, css)

	//---------------------------------------------------------------------------
	logger.Debug("---------------------------- Testing changing the voting power of one validator a few times")

	val1PubKey := css[0].privValidator.GetPubKey()
	val1PubKeyABCI := types.TM2PB.PubKey(val1PubKey)
	updateValidatorTx := kvstore.MakeValSetChangeTx(val1PubKeyABCI, 25)
	previousTotalVotingPower := css[0].GetRoundState().LastValidators.TotalVotingPower()

	waitForAndValidateBlock(t, nVals, activeVals, blocksSubs, css, updateValidatorTx)
	waitForAndValidateBlockWithTx(t, nVals, activeVals, blocksSubs, css, updateValidatorTx)
	waitForAndValidateBlock(t, nVals, activeVals, blocksSubs, css)
	waitForAndValidateBlock(t, nVals, activeVals, blocksSubs, css)

	if css[0].GetRoundState().LastValidators.TotalVotingPower() == previousTotalVotingPower {
		t.Fatalf("expected voting power to change (before: %d, after: %d)", previousTotalVotingPower, css[0].GetRoundState().LastValidators.TotalVotingPower())
	}

	updateValidatorTx = kvstore.MakeValSetChangeTx(val1PubKeyABCI, 2)
	previousTotalVotingPower = css[0].GetRoundState().LastValidators.TotalVotingPower()

	waitForAndValidateBlock(t, nVals, activeVals, blocksSubs, css, updateValidatorTx)
	waitForAndValidateBlockWithTx(t, nVals, activeVals, blocksSubs, css, updateValidatorTx)
	waitForAndValidateBlock(t, nVals, activeVals, blocksSubs, css)
	waitForAndValidateBlock(t, nVals, activeVals, blocksSubs, css)

	if css[0].GetRoundState().LastValidators.TotalVotingPower() == previousTotalVotingPower {
		t.Fatalf("expected voting power to change (before: %d, after: %d)", previousTotalVotingPower, css[0].GetRoundState().LastValidators.TotalVotingPower())
	}

	updateValidatorTx = kvstore.MakeValSetChangeTx(val1PubKeyABCI, 26)
	previousTotalVotingPower = css[0].GetRoundState().LastValidators.TotalVotingPower()

	waitForAndValidateBlock(t, nVals, activeVals, blocksSubs, css, updateValidatorTx)
	waitForAndValidateBlockWithTx(t, nVals, activeVals, blocksSubs, css, updateValidatorTx)
	waitForAndValidateBlock(t, nVals, activeVals, blocksSubs, css)
	waitForAndValidateBlock(t, nVals, activeVals, blocksSubs, css)

	if css[0].GetRoundState().LastValidators.TotalVotingPower() == previousTotalVotingPower {
		t.Fatalf("expected voting power to change (before: %d, after: %d)", previousTotalVotingPower, css[0].GetRoundState().LastValidators.TotalVotingPower())
	}
}

func TestReactorValidatorSetChanges(t *testing.T) {
	nPeers := 7
	nVals := 4
	css, _, _, cleanup := randConsensusNetWithPeers(nVals, nPeers, "consensus_val_set_changes_test", newMockTickerFunc(true), newPersistentKVStoreWithPath)

	defer cleanup()
	logger := log.TestingLogger()

	reactors, blocksSubs, eventBuses := startConsensusNet(t, css, nPeers)
	defer stopConsensusNet(logger, reactors, eventBuses)

	// map of active validators
	activeVals := make(map[string]struct{})
	for i := 0; i < nVals; i++ {
		addr := css[i].privValidator.GetPubKey().Address()
		activeVals[string(addr)] = struct{}{}
	}

	// wait till everyone makes block 1
	timeoutWaitGroup(t, nPeers, func(j int) {
		<-blocksSubs[j].Out()
	}, css)

	//---------------------------------------------------------------------------
	logger.Info("---------------------------- Testing adding one validator")

	newValidatorPubKey1 := css[nVals].privValidator.GetPubKey()
	valPubKey1ABCI := types.TM2PB.PubKey(newValidatorPubKey1)
	newValidatorTx1 := kvstore.MakeValSetChangeTx(valPubKey1ABCI, testMinPower)

	// wait till everyone makes block 2
	// ensure the commit includes all validators
	// send newValTx to change vals in block 3
	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, css, newValidatorTx1)

	// wait till everyone makes block 3.
	// it includes the commit for block 2, which is by the original validator set
	waitForAndValidateBlockWithTx(t, nPeers, activeVals, blocksSubs, css, newValidatorTx1)

	// wait till everyone makes block 4.
	// it includes the commit for block 3, which is by the original validator set
	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, css)

	// the commits for block 4 should be with the updated validator set
	activeVals[string(newValidatorPubKey1.Address())] = struct{}{}

	// wait till everyone makes block 5
	// it includes the commit for block 4, which should have the updated validator set
	waitForBlockWithUpdatedValsAndValidateIt(t, nPeers, activeVals, blocksSubs, css)

	//---------------------------------------------------------------------------
	logger.Info("---------------------------- Testing changing the voting power of one validator")

	updateValidatorPubKey1 := css[nVals].privValidator.GetPubKey()
	updatePubKey1ABCI := types.TM2PB.PubKey(updateValidatorPubKey1)
	updateValidatorTx1 := kvstore.MakeValSetChangeTx(updatePubKey1ABCI, 25)
	previousTotalVotingPower := css[nVals].GetRoundState().LastValidators.TotalVotingPower()

	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, css, updateValidatorTx1)
	waitForAndValidateBlockWithTx(t, nPeers, activeVals, blocksSubs, css, updateValidatorTx1)
	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, css)
	waitForBlockWithUpdatedValsAndValidateIt(t, nPeers, activeVals, blocksSubs, css)

	if css[nVals].GetRoundState().LastValidators.TotalVotingPower() == previousTotalVotingPower {
		t.Errorf("expected voting power to change (before: %d, after: %d)", previousTotalVotingPower, css[nVals].GetRoundState().LastValidators.TotalVotingPower())
	}

	//---------------------------------------------------------------------------
	logger.Info("---------------------------- Testing adding two validators at once")

	newValidatorPubKey2 := css[nVals+1].privValidator.GetPubKey()
	newVal2ABCI := types.TM2PB.PubKey(newValidatorPubKey2)
	newValidatorTx2 := kvstore.MakeValSetChangeTx(newVal2ABCI, testMinPower)

	newValidatorPubKey3 := css[nVals+2].privValidator.GetPubKey()
	newVal3ABCI := types.TM2PB.PubKey(newValidatorPubKey3)
	newValidatorTx3 := kvstore.MakeValSetChangeTx(newVal3ABCI, testMinPower)

	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, css, newValidatorTx2, newValidatorTx3)
	waitForAndValidateBlockWithTx(t, nPeers, activeVals, blocksSubs, css, newValidatorTx2, newValidatorTx3)
	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, css)
	activeVals[string(newValidatorPubKey2.Address())] = struct{}{}
	activeVals[string(newValidatorPubKey3.Address())] = struct{}{}
	waitForBlockWithUpdatedValsAndValidateIt(t, nPeers, activeVals, blocksSubs, css)

	//---------------------------------------------------------------------------
	logger.Info("---------------------------- Testing removing two validators at once")

	removeValidatorTx2 := kvstore.MakeValSetChangeTx(newVal2ABCI, 0)
	removeValidatorTx3 := kvstore.MakeValSetChangeTx(newVal3ABCI, 0)

	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, css, removeValidatorTx2, removeValidatorTx3)
	waitForAndValidateBlockWithTx(t, nPeers, activeVals, blocksSubs, css, removeValidatorTx2, removeValidatorTx3)
	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, css)
	delete(activeVals, string(newValidatorPubKey2.Address()))
	delete(activeVals, string(newValidatorPubKey3.Address()))
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
	css []*ConsensusState,
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
			err := assertMempool(css[j].txNotifier).CheckTx(tx, nil)
			assert.Nil(t, err)
		}
	}, css)
}

func waitForAndValidateBlockWithTx(
	t *testing.T,
	n int,
	activeVals map[string]struct{},
	blocksSubs []types.Subscription,
	css []*ConsensusState,
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
	css []*ConsensusState,
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
				css[j].Logger.Debug("waitForBlockWithUpdatedValsAndValidateIt: Got block with no new validators. Skipping", "height", newBlock.Height)
			}
		}

		err := validateBlock(newBlock, updatedVals)
		assert.Nil(t, err)
	}, css)
}

// expects high synchrony!
func validateBlock(block *types.Block, activeVals map[string]struct{}) error {
	if block.LastCommit.Size() != len(activeVals) {
		return fmt.Errorf("Commit size doesn't match number of active validators. Got %d, expected %d", block.LastCommit.Size(), len(activeVals))
	}

	for _, vote := range block.LastCommit.Precommits {
		if _, ok := activeVals[string(vote.ValidatorAddress)]; !ok {
			return fmt.Errorf("Found vote for unactive validator %X", vote.ValidatorAddress)
		}
	}
	return nil
}

func timeoutWaitGroup(t *testing.T, n int, f func(int), css []*ConsensusState) {
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
	timeout := time.Second * 300

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
		pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
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
	testCases := []struct {
		testName               string
		messageHeight          int64
		messageRound           int
		messageStep            cstypes.RoundStepType
		messageLastCommitRound int
		expectErr              bool
	}{
		{"Valid Message", 0, 0, 0x01, 1, false},
		{"Invalid Message", -1, 0, 0x01, 1, true},
		{"Invalid Message", 0, -1, 0x01, 1, true},
		{"Invalid Message", 0, 0, 0x00, 1, true},
		{"Invalid Message", 0, 0, 0x00, 0, true},
		{"Invalid Message", 1, 0, 0x01, 0, true},
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

			assert.Equal(t, tc.expectErr, message.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestNewValidBlockMessageValidateBasic(t *testing.T) {
	testCases := []struct {
		malleateFn func(*NewValidBlockMessage)
		expErr     string
	}{
		{func(msg *NewValidBlockMessage) {}, ""},
		{func(msg *NewValidBlockMessage) { msg.Height = -1 }, "Negative Height"},
		{func(msg *NewValidBlockMessage) { msg.Round = -1 }, "Negative Round"},
		{
			func(msg *NewValidBlockMessage) { msg.BlockPartsHeader.Total = 2 },
			"BlockParts bit array size 1 not equal to BlockPartsHeader.Total 2",
		},
		{
			func(msg *NewValidBlockMessage) { msg.BlockPartsHeader.Total = 0; msg.BlockParts = cmn.NewBitArray(0) },
			"Empty BlockParts",
		},
		{
			func(msg *NewValidBlockMessage) { msg.BlockParts = cmn.NewBitArray(types.MaxBlockPartsCount + 1) },
			"BlockParts bit array size 1602 not equal to BlockPartsHeader.Total 1",
		},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("#%d", i), func(t *testing.T) {
			msg := &NewValidBlockMessage{
				Height: 1,
				Round:  0,
				BlockPartsHeader: types.PartSetHeader{
					Total: 1,
				},
				BlockParts: cmn.NewBitArray(1),
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
		{func(msg *ProposalPOLMessage) { msg.Height = -1 }, "Negative Height"},
		{func(msg *ProposalPOLMessage) { msg.ProposalPOLRound = -1 }, "Negative ProposalPOLRound"},
		{func(msg *ProposalPOLMessage) { msg.ProposalPOL = cmn.NewBitArray(0) }, "Empty ProposalPOL bit array"},
		{func(msg *ProposalPOLMessage) { msg.ProposalPOL = cmn.NewBitArray(types.MaxVotesCount + 1) },
			"ProposalPOL bit array is too big: 10001, max: 10000"},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("#%d", i), func(t *testing.T) {
			msg := &ProposalPOLMessage{
				Height:           1,
				ProposalPOLRound: 1,
				ProposalPOL:      cmn.NewBitArray(1),
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
		messageRound  int
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
	message.Part.Index = -1

	assert.Equal(t, true, message.ValidateBasic() != nil, "Validate Basic had an unexpected result")
}

func TestHasVoteMessageValidateBasic(t *testing.T) {
	const (
		validSignedMsgType   types.SignedMsgType = 0x01
		invalidSignedMsgType types.SignedMsgType = 0x03
	)

	testCases := []struct {
		testName      string
		messageHeight int64
		messageRound  int
		messageType   types.SignedMsgType
		messageIndex  int
		expectErr     bool
	}{
		{"Valid Message", 0, 0, validSignedMsgType, 0, false},
		{"Invalid Message", -1, 0, validSignedMsgType, 0, true},
		{"Invalid Message", 0, -1, validSignedMsgType, 0, true},
		{"Invalid Message", 0, 0, invalidSignedMsgType, 0, true},
		{"Invalid Message", 0, 0, validSignedMsgType, -1, true},
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
		validSignedMsgType   types.SignedMsgType = 0x01
		invalidSignedMsgType types.SignedMsgType = 0x03
	)

	validBlockID := types.BlockID{}
	invalidBlockID := types.BlockID{
		Hash: cmn.HexBytes{},
		PartsHeader: types.PartSetHeader{
			Total: -1,
			Hash:  cmn.HexBytes{},
		},
	}

	testCases := []struct {
		testName       string
		messageHeight  int64
		messageRound   int
		messageType    types.SignedMsgType
		messageBlockID types.BlockID
		expectErr      bool
	}{
		{"Valid Message", 0, 0, validSignedMsgType, validBlockID, false},
		{"Invalid Message", -1, 0, validSignedMsgType, validBlockID, true},
		{"Invalid Message", 0, -1, validSignedMsgType, validBlockID, true},
		{"Invalid Message", 0, 0, invalidSignedMsgType, validBlockID, true},
		{"Invalid Message", 0, 0, validSignedMsgType, invalidBlockID, true},
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
	testCases := []struct { // nolint: maligned
		malleateFn func(*VoteSetBitsMessage)
		expErr     string
	}{
		{func(msg *VoteSetBitsMessage) {}, ""},
		{func(msg *VoteSetBitsMessage) { msg.Height = -1 }, "Negative Height"},
		{func(msg *VoteSetBitsMessage) { msg.Round = -1 }, "Negative Round"},
		{func(msg *VoteSetBitsMessage) { msg.Type = 0x03 }, "Invalid Type"},
		{func(msg *VoteSetBitsMessage) {
			msg.BlockID = types.BlockID{
				Hash: cmn.HexBytes{},
				PartsHeader: types.PartSetHeader{
					Total: -1,
					Hash:  cmn.HexBytes{},
				},
			}
		}, "Wrong BlockID: Wrong PartsHeader: Negative Total"},
		{func(msg *VoteSetBitsMessage) { msg.Votes = cmn.NewBitArray(types.MaxVotesCount + 1) },
			"Votes bit array is too big: 10001, max: 10000"},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("#%d", i), func(t *testing.T) {
			msg := &VoteSetBitsMessage{
				Height:  1,
				Round:   0,
				Type:    0x01,
				Votes:   cmn.NewBitArray(1),
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
