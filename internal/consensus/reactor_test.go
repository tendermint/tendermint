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
	dbm "github.com/tendermint/tm-db"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto/encoding"
	tmsync "github.com/tendermint/tendermint/internal/libs/sync"
	"github.com/tendermint/tendermint/internal/mempool"
	mempoolv0 "github.com/tendermint/tendermint/internal/mempool/v0"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/internal/p2p/p2ptest"
	sm "github.com/tendermint/tendermint/internal/state"
	statemocks "github.com/tendermint/tendermint/internal/state/mocks"
	"github.com/tendermint/tendermint/internal/store"
	"github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/libs/log"
	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
	tmcons "github.com/tendermint/tendermint/proto/tendermint/consensus"
	"github.com/tendermint/tendermint/types"
)

var (
	defaultTestTime = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
)

type reactorTestSuite struct {
	network             *p2ptest.Network
	states              map[types.NodeID]*State
	reactors            map[types.NodeID]*Reactor
	subs                map[types.NodeID]types.Subscription
	blocksyncSubs       map[types.NodeID]types.Subscription
	stateChannels       map[types.NodeID]*p2p.Channel
	dataChannels        map[types.NodeID]*p2p.Channel
	voteChannels        map[types.NodeID]*p2p.Channel
	voteSetBitsChannels map[types.NodeID]*p2p.Channel
}

func chDesc(chID p2p.ChannelID) p2p.ChannelDescriptor {
	return p2p.ChannelDescriptor{
		ID: byte(chID),
	}
}

func setup(t *testing.T, numNodes int, states []*State, size int) *reactorTestSuite {
	t.Helper()

	rts := &reactorTestSuite{
		network:       p2ptest.MakeNetwork(t, p2ptest.NetworkOptions{NumNodes: numNodes}),
		states:        make(map[types.NodeID]*State),
		reactors:      make(map[types.NodeID]*Reactor, numNodes),
		subs:          make(map[types.NodeID]types.Subscription, numNodes),
		blocksyncSubs: make(map[types.NodeID]types.Subscription, numNodes),
	}

	rts.stateChannels = rts.network.MakeChannelsNoCleanup(t, chDesc(StateChannel), new(tmcons.Message), size)
	rts.dataChannels = rts.network.MakeChannelsNoCleanup(t, chDesc(DataChannel), new(tmcons.Message), size)
	rts.voteChannels = rts.network.MakeChannelsNoCleanup(t, chDesc(VoteChannel), new(tmcons.Message), size)
	rts.voteSetBitsChannels = rts.network.MakeChannelsNoCleanup(t, chDesc(VoteSetBitsChannel), new(tmcons.Message), size)

	_, cancel := context.WithCancel(context.Background())

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

		blocksSub, err := state.eventBus.Subscribe(context.Background(), testSubscriber, types.EventQueryNewBlock, size)
		require.NoError(t, err)

		fsSub, err := state.eventBus.Subscribe(context.Background(), testSubscriber, types.EventQueryBlockSyncStatus, size)
		require.NoError(t, err)

		rts.states[nodeID] = state
		rts.subs[nodeID] = blocksSub
		rts.reactors[nodeID] = reactor
		rts.blocksyncSubs[nodeID] = fsSub

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
		for nodeID, r := range rts.reactors {
			require.NoError(t, rts.states[nodeID].eventBus.Stop())
			require.NoError(t, r.Stop())
			require.False(t, r.IsRunning())
		}

		leaktest.Check(t)
		cancel()
	})

	return rts
}

func validateBlock(block *types.Block, activeVals map[string]struct{}) error {
	if block.LastCommit.Size() != len(activeVals) {
		return fmt.Errorf(
			"commit size doesn't match number of active validators. Got %d, expected %d",
			block.LastCommit.Size(), len(activeVals),
		)
	}

	for _, commitSig := range block.LastCommit.Signatures {
		if _, ok := activeVals[string(commitSig.ValidatorAddress)]; !ok {
			return fmt.Errorf("found vote for inactive validator %X", commitSig.ValidatorAddress)
		}
	}

	return nil
}

func waitForAndValidateBlock(
	t *testing.T,
	n int,
	activeVals map[string]struct{},
	blocksSubs []types.Subscription,
	states []*State,
	txs ...[]byte,
) {

	fn := func(j int) {
		msg := <-blocksSubs[j].Out()
		newBlock := msg.Data().(types.EventDataNewBlock).Block

		require.NoError(t, validateBlock(newBlock, activeVals))

		for _, tx := range txs {
			require.NoError(t, assertMempool(states[j].txNotifier).CheckTx(context.Background(), tx, nil, mempool.TxInfo{}))
		}
	}

	var wg sync.WaitGroup
	wg.Add(n)

	for i := 0; i < n; i++ {
		go func(j int) {
			fn(j)
			wg.Done()
		}(i)
	}

	wg.Wait()
}

func waitForAndValidateBlockWithTx(
	t *testing.T,
	n int,
	activeVals map[string]struct{},
	blocksSubs []types.Subscription,
	states []*State,
	txs ...[]byte,
) {

	fn := func(j int) {
		ntxs := 0
	BLOCK_TX_LOOP:
		for {
			msg := <-blocksSubs[j].Out()
			newBlock := msg.Data().(types.EventDataNewBlock).Block

			require.NoError(t, validateBlock(newBlock, activeVals))

			// check that txs match the txs we're waiting for.
			// note they could be spread over multiple blocks,
			// but they should be in order.
			for _, tx := range newBlock.Data.Txs {
				require.EqualValues(t, txs[ntxs], tx)
				ntxs++
			}

			if ntxs == len(txs) {
				break BLOCK_TX_LOOP
			}
		}
	}

	var wg sync.WaitGroup
	wg.Add(n)

	for i := 0; i < n; i++ {
		go func(j int) {
			fn(j)
			wg.Done()
		}(i)
	}

	wg.Wait()
}

func waitForBlockWithUpdatedValsAndValidateIt(
	t *testing.T,
	n int,
	updatedVals map[string]struct{},
	blocksSubs []types.Subscription,
	css []*State,
) {

	fn := func(j int) {
		var newBlock *types.Block

	LOOP:
		for {
			msg := <-blocksSubs[j].Out()
			newBlock = msg.Data().(types.EventDataNewBlock).Block
			if newBlock.LastCommit.Size() == len(updatedVals) {
				break LOOP
			}
		}

		require.NoError(t, validateBlock(newBlock, updatedVals))
	}

	var wg sync.WaitGroup
	wg.Add(n)

	for i := 0; i < n; i++ {
		go func(j int) {
			fn(j)
			wg.Done()
		}(i)
	}

	wg.Wait()
}

func ensureBlockSyncStatus(t *testing.T, msg tmpubsub.Message, complete bool, height int64) {
	t.Helper()
	status, ok := msg.Data().(types.EventDataBlockSyncStatus)

	require.True(t, ok)
	require.Equal(t, complete, status.Complete)
	require.Equal(t, height, status.Height)
}

func TestReactorBasic(t *testing.T) {
	cfg := configSetup(t)

	n := 4
	states, cleanup := randConsensusState(t,
		cfg, n, "consensus_reactor_test",
		newMockTickerFunc(true), newKVStore)
	t.Cleanup(cleanup)

	rts := setup(t, n, states, 100) // buffer must be large enough to not deadlock

	for _, reactor := range rts.reactors {
		state := reactor.state.GetState()
		reactor.SwitchToConsensus(state, false)
	}

	var wg sync.WaitGroup
	for _, sub := range rts.subs {
		wg.Add(1)

		// wait till everyone makes the first new block
		go func(s types.Subscription) {
			defer wg.Done()
			<-s.Out()
		}(sub)
	}

	wg.Wait()

	for _, sub := range rts.blocksyncSubs {
		wg.Add(1)

		// wait till everyone makes the consensus switch
		go func(s types.Subscription) {
			defer wg.Done()
			msg := <-s.Out()
			ensureBlockSyncStatus(t, msg, true, 0)
		}(sub)
	}

	wg.Wait()
}

func TestReactorWithEvidence(t *testing.T) {
	cfg := configSetup(t)

	n := 4
	testName := "consensus_reactor_test"
	tickerFunc := newMockTickerFunc(true)
	appFunc := newKVStore

	genDoc, privVals := factory.RandGenesisDoc(cfg, n, false, 30)
	states := make([]*State, n)
	logger := consensusLogger()

	for i := 0; i < n; i++ {
		stateDB := dbm.NewMemDB() // each state needs its own db
		stateStore := sm.NewStore(stateDB)
		state, err := sm.MakeGenesisState(genDoc)
		require.NoError(t, err)
		thisConfig, err := ResetConfig(fmt.Sprintf("%s_%d", testName, i))
		require.NoError(t, err)

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
		proxyAppConnMem := abciclient.NewLocalClient(mtx, app)
		proxyAppConnCon := abciclient.NewLocalClient(mtx, app)

		mempool := mempoolv0.NewCListMempool(thisConfig.Mempool, proxyAppConnMem, 0)
		mempool.SetLogger(log.TestingLogger().With("module", "mempool"))
		if thisConfig.Consensus.WaitForTxs() {
			mempool.EnableTxsAvailable()
		}

		// mock the evidence pool
		// everyone includes evidence of another double signing
		vIdx := (i + 1) % n

		ev := types.NewMockDuplicateVoteEvidenceWithValidator(1, defaultTestTime, privVals[vIdx], cfg.ChainID())
		evpool := &statemocks.EvidencePool{}
		evpool.On("CheckEvidence", mock.AnythingOfType("types.EvidenceList")).Return(nil)
		evpool.On("PendingEvidence", mock.AnythingOfType("int64")).Return([]types.Evidence{
			ev}, int64(len(ev.Bytes())))
		evpool.On("Update", mock.AnythingOfType("state.State"), mock.AnythingOfType("types.EvidenceList")).Return()

		evpool2 := sm.EmptyEvidencePool{}

		blockExec := sm.NewBlockExecutor(stateStore, log.TestingLogger(), proxyAppConnCon, mempool, evpool, blockStore)
		cs := NewState(thisConfig.Consensus, state, blockExec, blockStore, mempool, evpool2)
		cs.SetLogger(log.TestingLogger().With("module", "consensus"))
		cs.SetPrivValidator(pv)

		eventBus := types.NewEventBus()
		eventBus.SetLogger(log.TestingLogger().With("module", "events"))
		err = eventBus.Start()
		require.NoError(t, err)
		cs.SetEventBus(eventBus)

		cs.SetTimeoutTicker(tickerFunc())
		cs.SetLogger(logger.With("validator", i, "module", "consensus"))

		states[i] = cs
	}

	rts := setup(t, n, states, 100) // buffer must be large enough to not deadlock

	for _, reactor := range rts.reactors {
		state := reactor.state.GetState()
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
	cfg := configSetup(t)

	n := 4
	states, cleanup := randConsensusState(
		t,
		cfg,
		n,
		"consensus_reactor_test",
		newMockTickerFunc(true),
		newKVStore,
		func(c *config.Config) {
			c.Consensus.CreateEmptyBlocks = false
		},
	)

	t.Cleanup(cleanup)

	rts := setup(t, n, states, 100) // buffer must be large enough to not deadlock

	for _, reactor := range rts.reactors {
		state := reactor.state.GetState()
		reactor.SwitchToConsensus(state, false)
	}

	// send a tx
	require.NoError(
		t,
		assertMempool(states[3].txNotifier).CheckTx(
			context.Background(),
			[]byte{1, 2, 3},
			nil,
			mempool.TxInfo{},
		),
	)

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
	cfg := configSetup(t)

	n := 4
	states, cleanup := randConsensusState(t,
		cfg, n, "consensus_reactor_test",
		newMockTickerFunc(true), newKVStore)
	t.Cleanup(cleanup)

	rts := setup(t, n, states, 100) // buffer must be large enough to not deadlock

	for _, reactor := range rts.reactors {
		state := reactor.state.GetState()
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

func TestReactorVotingPowerChange(t *testing.T) {
	cfg := configSetup(t)

	n := 4
	states, cleanup := randConsensusState(
		t,
		cfg,
		n,
		"consensus_voting_power_changes_test",
		newMockTickerFunc(true),
		newPersistentKVStore,
	)

	t.Cleanup(cleanup)

	rts := setup(t, n, states, 100) // buffer must be large enough to not deadlock

	for _, reactor := range rts.reactors {
		state := reactor.state.GetState()
		reactor.SwitchToConsensus(state, false)
	}

	// map of active validators
	activeVals := make(map[string]struct{})
	for i := 0; i < n; i++ {
		pubKey, err := states[i].privValidator.GetPubKey(context.Background())
		require.NoError(t, err)

		addr := pubKey.Address()
		activeVals[string(addr)] = struct{}{}
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

	blocksSubs := []types.Subscription{}
	for _, sub := range rts.subs {
		blocksSubs = append(blocksSubs, sub)
	}

	val1PubKey, err := states[0].privValidator.GetPubKey(context.Background())
	require.NoError(t, err)

	val1PubKeyABCI, err := encoding.PubKeyToProto(val1PubKey)
	require.NoError(t, err)

	updateValidatorTx := kvstore.MakeValSetChangeTx(val1PubKeyABCI, 25)
	previousTotalVotingPower := states[0].GetRoundState().LastValidators.TotalVotingPower()

	waitForAndValidateBlock(t, n, activeVals, blocksSubs, states, updateValidatorTx)
	waitForAndValidateBlockWithTx(t, n, activeVals, blocksSubs, states, updateValidatorTx)
	waitForAndValidateBlock(t, n, activeVals, blocksSubs, states)
	waitForAndValidateBlock(t, n, activeVals, blocksSubs, states)

	require.NotEqualf(
		t, previousTotalVotingPower, states[0].GetRoundState().LastValidators.TotalVotingPower(),
		"expected voting power to change (before: %d, after: %d)",
		previousTotalVotingPower,
		states[0].GetRoundState().LastValidators.TotalVotingPower(),
	)

	updateValidatorTx = kvstore.MakeValSetChangeTx(val1PubKeyABCI, 2)
	previousTotalVotingPower = states[0].GetRoundState().LastValidators.TotalVotingPower()

	waitForAndValidateBlock(t, n, activeVals, blocksSubs, states, updateValidatorTx)
	waitForAndValidateBlockWithTx(t, n, activeVals, blocksSubs, states, updateValidatorTx)
	waitForAndValidateBlock(t, n, activeVals, blocksSubs, states)
	waitForAndValidateBlock(t, n, activeVals, blocksSubs, states)

	require.NotEqualf(
		t, states[0].GetRoundState().LastValidators.TotalVotingPower(), previousTotalVotingPower,
		"expected voting power to change (before: %d, after: %d)",
		previousTotalVotingPower, states[0].GetRoundState().LastValidators.TotalVotingPower(),
	)

	updateValidatorTx = kvstore.MakeValSetChangeTx(val1PubKeyABCI, 26)
	previousTotalVotingPower = states[0].GetRoundState().LastValidators.TotalVotingPower()

	waitForAndValidateBlock(t, n, activeVals, blocksSubs, states, updateValidatorTx)
	waitForAndValidateBlockWithTx(t, n, activeVals, blocksSubs, states, updateValidatorTx)
	waitForAndValidateBlock(t, n, activeVals, blocksSubs, states)
	waitForAndValidateBlock(t, n, activeVals, blocksSubs, states)

	require.NotEqualf(
		t, previousTotalVotingPower, states[0].GetRoundState().LastValidators.TotalVotingPower(),
		"expected voting power to change (before: %d, after: %d)",
		previousTotalVotingPower,
		states[0].GetRoundState().LastValidators.TotalVotingPower(),
	)
}

func TestReactorValidatorSetChanges(t *testing.T) {
	cfg := configSetup(t)

	nPeers := 7
	nVals := 4
	states, _, _, cleanup := randConsensusNetWithPeers(
		cfg,
		nVals,
		nPeers,
		"consensus_val_set_changes_test",
		newMockTickerFunc(true),
		newPersistentKVStoreWithPath,
	)
	t.Cleanup(cleanup)

	rts := setup(t, nPeers, states, 1000) // buffer must be large enough to not deadlock

	for _, reactor := range rts.reactors {
		state := reactor.state.GetState()
		reactor.SwitchToConsensus(state, false)
	}

	// map of active validators
	activeVals := make(map[string]struct{})
	for i := 0; i < nVals; i++ {
		pubKey, err := states[i].privValidator.GetPubKey(context.Background())
		require.NoError(t, err)

		activeVals[string(pubKey.Address())] = struct{}{}
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

	newValidatorPubKey1, err := states[nVals].privValidator.GetPubKey(context.Background())
	require.NoError(t, err)

	valPubKey1ABCI, err := encoding.PubKeyToProto(newValidatorPubKey1)
	require.NoError(t, err)

	newValidatorTx1 := kvstore.MakeValSetChangeTx(valPubKey1ABCI, testMinPower)

	blocksSubs := []types.Subscription{}
	for _, sub := range rts.subs {
		blocksSubs = append(blocksSubs, sub)
	}

	// wait till everyone makes block 2
	// ensure the commit includes all validators
	// send newValTx to change vals in block 3
	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, states, newValidatorTx1)

	// wait till everyone makes block 3.
	// it includes the commit for block 2, which is by the original validator set
	waitForAndValidateBlockWithTx(t, nPeers, activeVals, blocksSubs, states, newValidatorTx1)

	// wait till everyone makes block 4.
	// it includes the commit for block 3, which is by the original validator set
	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, states)

	// the commits for block 4 should be with the updated validator set
	activeVals[string(newValidatorPubKey1.Address())] = struct{}{}

	// wait till everyone makes block 5
	// it includes the commit for block 4, which should have the updated validator set
	waitForBlockWithUpdatedValsAndValidateIt(t, nPeers, activeVals, blocksSubs, states)

	updateValidatorPubKey1, err := states[nVals].privValidator.GetPubKey(context.Background())
	require.NoError(t, err)

	updatePubKey1ABCI, err := encoding.PubKeyToProto(updateValidatorPubKey1)
	require.NoError(t, err)

	updateValidatorTx1 := kvstore.MakeValSetChangeTx(updatePubKey1ABCI, 25)
	previousTotalVotingPower := states[nVals].GetRoundState().LastValidators.TotalVotingPower()

	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, states, updateValidatorTx1)
	waitForAndValidateBlockWithTx(t, nPeers, activeVals, blocksSubs, states, updateValidatorTx1)
	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, states)
	waitForBlockWithUpdatedValsAndValidateIt(t, nPeers, activeVals, blocksSubs, states)

	require.NotEqualf(
		t, states[nVals].GetRoundState().LastValidators.TotalVotingPower(), previousTotalVotingPower,
		"expected voting power to change (before: %d, after: %d)",
		previousTotalVotingPower, states[nVals].GetRoundState().LastValidators.TotalVotingPower(),
	)

	newValidatorPubKey2, err := states[nVals+1].privValidator.GetPubKey(context.Background())
	require.NoError(t, err)

	newVal2ABCI, err := encoding.PubKeyToProto(newValidatorPubKey2)
	require.NoError(t, err)

	newValidatorTx2 := kvstore.MakeValSetChangeTx(newVal2ABCI, testMinPower)

	newValidatorPubKey3, err := states[nVals+2].privValidator.GetPubKey(context.Background())
	require.NoError(t, err)

	newVal3ABCI, err := encoding.PubKeyToProto(newValidatorPubKey3)
	require.NoError(t, err)

	newValidatorTx3 := kvstore.MakeValSetChangeTx(newVal3ABCI, testMinPower)

	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, states, newValidatorTx2, newValidatorTx3)
	waitForAndValidateBlockWithTx(t, nPeers, activeVals, blocksSubs, states, newValidatorTx2, newValidatorTx3)
	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, states)

	activeVals[string(newValidatorPubKey2.Address())] = struct{}{}
	activeVals[string(newValidatorPubKey3.Address())] = struct{}{}

	waitForBlockWithUpdatedValsAndValidateIt(t, nPeers, activeVals, blocksSubs, states)

	removeValidatorTx2 := kvstore.MakeValSetChangeTx(newVal2ABCI, 0)
	removeValidatorTx3 := kvstore.MakeValSetChangeTx(newVal3ABCI, 0)

	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, states, removeValidatorTx2, removeValidatorTx3)
	waitForAndValidateBlockWithTx(t, nPeers, activeVals, blocksSubs, states, removeValidatorTx2, removeValidatorTx3)
	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, states)

	delete(activeVals, string(newValidatorPubKey2.Address()))
	delete(activeVals, string(newValidatorPubKey3.Address()))

	waitForBlockWithUpdatedValsAndValidateIt(t, nPeers, activeVals, blocksSubs, states)
}
