package consensus

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/internal/eventbus"
	"github.com/tendermint/tendermint/internal/mempool"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/internal/p2p/p2ptest"
	tmpubsub "github.com/tendermint/tendermint/internal/pubsub"
	sm "github.com/tendermint/tendermint/internal/state"
	statemocks "github.com/tendermint/tendermint/internal/state/mocks"
	"github.com/tendermint/tendermint/internal/store"
	"github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/libs/log"
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
	subs                map[types.NodeID]eventbus.Subscription
	blocksyncSubs       map[types.NodeID]eventbus.Subscription
	stateChannels       map[types.NodeID]*p2p.Channel
	dataChannels        map[types.NodeID]*p2p.Channel
	voteChannels        map[types.NodeID]*p2p.Channel
	voteSetBitsChannels map[types.NodeID]*p2p.Channel
}

func chDesc(chID p2p.ChannelID, size int) *p2p.ChannelDescriptor {
	return &p2p.ChannelDescriptor{
		ID:                 chID,
		MessageType:        new(tmcons.Message),
		RecvBufferCapacity: size,
	}
}

func setup(
	ctx context.Context,
	t *testing.T,
	numNodes int,
	states []*State,
	size int,
) *reactorTestSuite {
	t.Helper()

	rts := &reactorTestSuite{
		network:       p2ptest.MakeNetwork(ctx, t, p2ptest.NetworkOptions{NumNodes: numNodes}),
		states:        make(map[types.NodeID]*State),
		reactors:      make(map[types.NodeID]*Reactor, numNodes),
		subs:          make(map[types.NodeID]eventbus.Subscription, numNodes),
		blocksyncSubs: make(map[types.NodeID]eventbus.Subscription, numNodes),
	}

	rts.stateChannels = rts.network.MakeChannelsNoCleanup(ctx, t, chDesc(StateChannel, size))
	rts.dataChannels = rts.network.MakeChannelsNoCleanup(ctx, t, chDesc(DataChannel, size))
	rts.voteChannels = rts.network.MakeChannelsNoCleanup(ctx, t, chDesc(VoteChannel, size))
	rts.voteSetBitsChannels = rts.network.MakeChannelsNoCleanup(ctx, t, chDesc(VoteSetBitsChannel, size))

	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	chCreator := func(nodeID types.NodeID) p2p.ChannelCreator {
		return func(ctx context.Context, desc *p2p.ChannelDescriptor) (*p2p.Channel, error) {
			switch desc.ID {
			case StateChannel:
				return rts.stateChannels[nodeID], nil
			case DataChannel:
				return rts.dataChannels[nodeID], nil
			case VoteChannel:
				return rts.voteChannels[nodeID], nil
			case VoteSetBitsChannel:
				return rts.voteSetBitsChannels[nodeID], nil
			default:
				return nil, fmt.Errorf("invalid channel; %v", desc.ID)
			}
		}
	}

	i := 0
	for nodeID, node := range rts.network.Nodes {
		state := states[i]

		reactor, err := NewReactor(ctx,
			state.logger.With("node", nodeID),
			state,
			chCreator(nodeID),
			node.MakePeerUpdates(ctx, t),
			true,
			NopMetrics(),
		)
		require.NoError(t, err)

		reactor.SetEventBus(state.eventBus)

		blocksSub, err := state.eventBus.SubscribeWithArgs(ctx, tmpubsub.SubscribeArgs{
			ClientID: testSubscriber,
			Query:    types.EventQueryNewBlock,
			Limit:    size,
		})
		require.NoError(t, err)

		fsSub, err := state.eventBus.SubscribeWithArgs(ctx, tmpubsub.SubscribeArgs{
			ClientID: testSubscriber,
			Query:    types.EventQueryBlockSyncStatus,
			Limit:    size,
		})
		require.NoError(t, err)

		rts.states[nodeID] = state
		rts.subs[nodeID] = blocksSub
		rts.reactors[nodeID] = reactor
		rts.blocksyncSubs[nodeID] = fsSub

		// simulate handle initChain in handshake
		if state.state.LastBlockHeight == 0 {
			require.NoError(t, state.blockExec.Store().Save(state.state))
		}

		require.NoError(t, reactor.Start(ctx))
		require.True(t, reactor.IsRunning())
		t.Cleanup(reactor.Wait)

		i++
	}

	require.Len(t, rts.reactors, numNodes)

	// start the in-memory network and connect all peers with each other
	rts.network.Start(ctx, t)

	t.Cleanup(leaktest.Check(t))

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
	bctx context.Context,
	t *testing.T,
	n int,
	activeVals map[string]struct{},
	blocksSubs []eventbus.Subscription,
	states []*State,
	txs ...[]byte,
) {
	t.Helper()

	ctx, cancel := context.WithCancel(bctx)
	defer cancel()
	fn := func(j int) {
		msg, err := blocksSubs[j].Next(ctx)
		if !assert.NoError(t, err) {
			cancel()
			return
		}

		newBlock := msg.Data().(types.EventDataNewBlock).Block
		require.NoError(t, validateBlock(newBlock, activeVals))

		for _, tx := range txs {
			err := assertMempool(t, states[j].txNotifier).CheckTx(ctx, tx, nil, mempool.TxInfo{})
			if errors.Is(err, types.ErrTxInCache) {
				continue
			}
			require.NoError(t, err)
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(j int) {
			defer wg.Done()
			fn(j)
		}(i)
	}

	wg.Wait()
}

func waitForAndValidateBlockWithTx(
	bctx context.Context,
	t *testing.T,
	n int,
	activeVals map[string]struct{},
	blocksSubs []eventbus.Subscription,
	states []*State,
	txs ...[]byte,
) {
	t.Helper()

	ctx, cancel := context.WithCancel(bctx)
	defer cancel()
	fn := func(j int) {
		ntxs := 0
		for {
			msg, err := blocksSubs[j].Next(ctx)
			if !assert.NoError(t, err) {
				cancel()
				return
			}

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
				break
			}
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(j int) {
			defer wg.Done()
			fn(j)
		}(i)
	}

	wg.Wait()
}

func waitForBlockWithUpdatedValsAndValidateIt(
	bctx context.Context,
	t *testing.T,
	n int,
	updatedVals map[string]struct{},
	blocksSubs []eventbus.Subscription,
	css []*State,
) {
	t.Helper()
	ctx, cancel := context.WithCancel(bctx)
	defer cancel()

	fn := func(j int) {
		var newBlock *types.Block

		for {
			msg, err := blocksSubs[j].Next(ctx)
			if !assert.NoError(t, err) {
				cancel()
				return
			}

			newBlock = msg.Data().(types.EventDataNewBlock).Block
			if newBlock.LastCommit.Size() == len(updatedVals) {
				break
			}
		}

		require.NoError(t, validateBlock(newBlock, updatedVals))
	}

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(j int) {
			defer wg.Done()
			fn(j)
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	cfg := configSetup(t)

	n := 4
	states, cleanup := makeConsensusState(ctx, t,
		cfg, n, "consensus_reactor_test",
		newMockTickerFunc(true))
	t.Cleanup(cleanup)

	rts := setup(ctx, t, n, states, 100) // buffer must be large enough to not deadlock

	for _, reactor := range rts.reactors {
		state := reactor.state.GetState()
		reactor.SwitchToConsensus(ctx, state, false)
	}

	var wg sync.WaitGroup
	for _, sub := range rts.subs {
		wg.Add(1)

		// wait till everyone makes the first new block
		go func(s eventbus.Subscription) {
			defer wg.Done()
			_, err := s.Next(ctx)
			if !assert.NoError(t, err) {
				cancel()
			}
		}(sub)
	}

	wg.Wait()

	for _, sub := range rts.blocksyncSubs {
		wg.Add(1)

		// wait till everyone makes the consensus switch
		go func(s eventbus.Subscription) {
			defer wg.Done()
			msg, err := s.Next(ctx)
			if !assert.NoError(t, err) {
				cancel()
				return
			}
			ensureBlockSyncStatus(t, msg, true, 0)
		}(sub)
	}

	wg.Wait()
}

func TestReactorWithEvidence(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	cfg := configSetup(t)

	n := 4
	testName := "consensus_reactor_test"
	tickerFunc := newMockTickerFunc(true)

	valSet, privVals := factory.ValidatorSet(ctx, t, n, 30)
	genDoc := factory.GenesisDoc(cfg, time.Now(), valSet.Validators, nil)
	states := make([]*State, n)
	logger := consensusLogger()

	for i := 0; i < n; i++ {
		stateDB := dbm.NewMemDB() // each state needs its own db
		stateStore := sm.NewStore(stateDB)
		state, err := sm.MakeGenesisState(genDoc)
		require.NoError(t, err)
		thisConfig, err := ResetConfig(t.TempDir(), fmt.Sprintf("%s_%d", testName, i))
		require.NoError(t, err)

		defer os.RemoveAll(thisConfig.RootDir)

		ensureDir(t, path.Dir(thisConfig.Consensus.WalFile()), 0700) // dir for wal
		app := kvstore.NewApplication()
		vals := types.TM2PB.ValidatorUpdates(state.Validators)
		app.InitChain(abci.RequestInitChain{Validators: vals})

		pv := privVals[i]
		blockDB := dbm.NewMemDB()
		blockStore := store.NewBlockStore(blockDB)

		// one for mempool, one for consensus
		proxyAppConnMem := abciclient.NewLocalClient(logger, app)
		proxyAppConnCon := abciclient.NewLocalClient(logger, app)

		mempool := mempool.NewTxMempool(
			log.TestingLogger().With("module", "mempool"),
			thisConfig.Mempool,
			proxyAppConnMem,
			0,
		)

		if thisConfig.Consensus.WaitForTxs() {
			mempool.EnableTxsAvailable()
		}

		// mock the evidence pool
		// everyone includes evidence of another double signing
		vIdx := (i + 1) % n

		ev, err := types.NewMockDuplicateVoteEvidenceWithValidator(ctx, 1, defaultTestTime, privVals[vIdx], cfg.ChainID())
		require.NoError(t, err)
		evpool := &statemocks.EvidencePool{}
		evpool.On("CheckEvidence", ctx, mock.AnythingOfType("types.EvidenceList")).Return(nil)
		evpool.On("PendingEvidence", mock.AnythingOfType("int64")).Return([]types.Evidence{
			ev}, int64(len(ev.Bytes())))
		evpool.On("Update", ctx, mock.AnythingOfType("state.State"), mock.AnythingOfType("types.EvidenceList")).Return()

		evpool2 := sm.EmptyEvidencePool{}

		blockExec := sm.NewBlockExecutor(stateStore, log.TestingLogger(), proxyAppConnCon, mempool, evpool, blockStore)

		cs := NewState(ctx, logger.With("validator", i, "module", "consensus"),
			thisConfig.Consensus, state, blockExec, blockStore, mempool, evpool2)
		cs.SetPrivValidator(ctx, pv)

		eventBus := eventbus.NewDefault(log.TestingLogger().With("module", "events"))
		require.NoError(t, eventBus.Start(ctx))
		cs.SetEventBus(eventBus)

		cs.SetTimeoutTicker(tickerFunc())

		states[i] = cs
	}

	rts := setup(ctx, t, n, states, 100) // buffer must be large enough to not deadlock

	for _, reactor := range rts.reactors {
		state := reactor.state.GetState()
		reactor.SwitchToConsensus(ctx, state, false)
	}

	var wg sync.WaitGroup
	for _, sub := range rts.subs {
		wg.Add(1)

		// We expect for each validator that is the proposer to propose one piece of
		// evidence.
		go func(s eventbus.Subscription) {
			defer wg.Done()
			msg, err := s.Next(ctx)
			if !assert.NoError(t, err) {
				cancel()
				return
			}

			block := msg.Data().(types.EventDataNewBlock).Block
			require.Len(t, block.Evidence, 1)
		}(sub)
	}

	wg.Wait()
}

func TestReactorCreatesBlockWhenEmptyBlocksFalse(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	cfg := configSetup(t)

	n := 4
	states, cleanup := makeConsensusState(ctx,
		t,
		cfg,
		n,
		"consensus_reactor_test",
		newMockTickerFunc(true),
		func(c *config.Config) {
			c.Consensus.CreateEmptyBlocks = false
		},
	)

	t.Cleanup(cleanup)

	rts := setup(ctx, t, n, states, 100) // buffer must be large enough to not deadlock

	for _, reactor := range rts.reactors {
		state := reactor.state.GetState()
		reactor.SwitchToConsensus(ctx, state, false)
	}

	// send a tx
	require.NoError(
		t,
		assertMempool(t, states[3].txNotifier).CheckTx(
			ctx,
			[]byte{1, 2, 3},
			nil,
			mempool.TxInfo{},
		),
	)

	var wg sync.WaitGroup
	for _, sub := range rts.subs {
		wg.Add(1)

		// wait till everyone makes the first new block
		go func(s eventbus.Subscription) {
			defer wg.Done()
			_, err := s.Next(ctx)
			if !assert.NoError(t, err) {
				cancel()
			}
		}(sub)
	}

	wg.Wait()
}

func TestReactorRecordsVotesAndBlockParts(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	cfg := configSetup(t)

	n := 4
	states, cleanup := makeConsensusState(ctx, t,
		cfg, n, "consensus_reactor_test",
		newMockTickerFunc(true))
	t.Cleanup(cleanup)

	rts := setup(ctx, t, n, states, 100) // buffer must be large enough to not deadlock

	for _, reactor := range rts.reactors {
		state := reactor.state.GetState()
		reactor.SwitchToConsensus(ctx, state, false)
	}

	var wg sync.WaitGroup
	for _, sub := range rts.subs {
		wg.Add(1)

		// wait till everyone makes the first new block
		go func(s eventbus.Subscription) {
			defer wg.Done()
			_, err := s.Next(ctx)
			if !assert.NoError(t, err) {
				cancel()
			}
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	cfg := configSetup(t)

	n := 4
	states, cleanup := makeConsensusState(ctx,
		t,
		cfg,
		n,
		"consensus_voting_power_changes_test",
		newMockTickerFunc(true),
	)

	t.Cleanup(cleanup)

	rts := setup(ctx, t, n, states, 100) // buffer must be large enough to not deadlock

	for _, reactor := range rts.reactors {
		state := reactor.state.GetState()
		reactor.SwitchToConsensus(ctx, state, false)
	}

	// map of active validators
	activeVals := make(map[string]struct{})
	for i := 0; i < n; i++ {
		pubKey, err := states[i].privValidator.GetPubKey(ctx)
		require.NoError(t, err)

		addr := pubKey.Address()
		activeVals[string(addr)] = struct{}{}
	}

	var wg sync.WaitGroup
	for _, sub := range rts.subs {
		wg.Add(1)

		// wait till everyone makes the first new block
		go func(s eventbus.Subscription) {
			defer wg.Done()
			_, err := s.Next(ctx)
			if !assert.NoError(t, err) {
				cancel()
			}
		}(sub)
	}

	wg.Wait()

	blocksSubs := []eventbus.Subscription{}
	for _, sub := range rts.subs {
		blocksSubs = append(blocksSubs, sub)
	}

	val1PubKey, err := states[0].privValidator.GetPubKey(ctx)
	require.NoError(t, err)

	val1PubKeyABCI, err := encoding.PubKeyToProto(val1PubKey)
	require.NoError(t, err)

	updateValidatorTx := kvstore.MakeValSetChangeTx(val1PubKeyABCI, 25)
	previousTotalVotingPower := states[0].GetRoundState().LastValidators.TotalVotingPower()

	waitForAndValidateBlock(ctx, t, n, activeVals, blocksSubs, states, updateValidatorTx)
	waitForAndValidateBlockWithTx(ctx, t, n, activeVals, blocksSubs, states, updateValidatorTx)
	waitForAndValidateBlock(ctx, t, n, activeVals, blocksSubs, states)
	waitForAndValidateBlock(ctx, t, n, activeVals, blocksSubs, states)

	require.NotEqualf(
		t, previousTotalVotingPower, states[0].GetRoundState().LastValidators.TotalVotingPower(),
		"expected voting power to change (before: %d, after: %d)",
		previousTotalVotingPower,
		states[0].GetRoundState().LastValidators.TotalVotingPower(),
	)

	updateValidatorTx = kvstore.MakeValSetChangeTx(val1PubKeyABCI, 2)
	previousTotalVotingPower = states[0].GetRoundState().LastValidators.TotalVotingPower()

	waitForAndValidateBlock(ctx, t, n, activeVals, blocksSubs, states, updateValidatorTx)
	waitForAndValidateBlockWithTx(ctx, t, n, activeVals, blocksSubs, states, updateValidatorTx)
	waitForAndValidateBlock(ctx, t, n, activeVals, blocksSubs, states)
	waitForAndValidateBlock(ctx, t, n, activeVals, blocksSubs, states)

	require.NotEqualf(
		t, states[0].GetRoundState().LastValidators.TotalVotingPower(), previousTotalVotingPower,
		"expected voting power to change (before: %d, after: %d)",
		previousTotalVotingPower, states[0].GetRoundState().LastValidators.TotalVotingPower(),
	)

	updateValidatorTx = kvstore.MakeValSetChangeTx(val1PubKeyABCI, 26)
	previousTotalVotingPower = states[0].GetRoundState().LastValidators.TotalVotingPower()

	waitForAndValidateBlock(ctx, t, n, activeVals, blocksSubs, states, updateValidatorTx)
	waitForAndValidateBlockWithTx(ctx, t, n, activeVals, blocksSubs, states, updateValidatorTx)
	waitForAndValidateBlock(ctx, t, n, activeVals, blocksSubs, states)
	waitForAndValidateBlock(ctx, t, n, activeVals, blocksSubs, states)

	require.NotEqualf(
		t, previousTotalVotingPower, states[0].GetRoundState().LastValidators.TotalVotingPower(),
		"expected voting power to change (before: %d, after: %d)",
		previousTotalVotingPower,
		states[0].GetRoundState().LastValidators.TotalVotingPower(),
	)
}

func TestReactorValidatorSetChanges(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	cfg := configSetup(t)

	nPeers := 7
	nVals := 4
	states, _, _, cleanup := randConsensusNetWithPeers(
		ctx,
		t,
		cfg,
		nVals,
		nPeers,
		"consensus_val_set_changes_test",
		newMockTickerFunc(true),
		newEpehemeralKVStore,
	)
	t.Cleanup(cleanup)

	rts := setup(ctx, t, nPeers, states, 100) // buffer must be large enough to not deadlock

	for _, reactor := range rts.reactors {
		state := reactor.state.GetState()
		reactor.SwitchToConsensus(ctx, state, false)
	}

	// map of active validators
	activeVals := make(map[string]struct{})
	for i := 0; i < nVals; i++ {
		pubKey, err := states[i].privValidator.GetPubKey(ctx)
		require.NoError(t, err)

		activeVals[string(pubKey.Address())] = struct{}{}
	}

	var wg sync.WaitGroup
	for _, sub := range rts.subs {
		wg.Add(1)

		// wait till everyone makes the first new block
		go func(s eventbus.Subscription) {
			defer wg.Done()
			_, err := s.Next(ctx)
			if !assert.NoError(t, err) {
				cancel()
			}
		}(sub)
	}

	wg.Wait()

	newValidatorPubKey1, err := states[nVals].privValidator.GetPubKey(ctx)
	require.NoError(t, err)

	valPubKey1ABCI, err := encoding.PubKeyToProto(newValidatorPubKey1)
	require.NoError(t, err)

	newValidatorTx1 := kvstore.MakeValSetChangeTx(valPubKey1ABCI, testMinPower)

	blocksSubs := []eventbus.Subscription{}
	for _, sub := range rts.subs {
		blocksSubs = append(blocksSubs, sub)
	}

	// wait till everyone makes block 2
	// ensure the commit includes all validators
	// send newValTx to change vals in block 3
	waitForAndValidateBlock(ctx, t, nPeers, activeVals, blocksSubs, states, newValidatorTx1)

	// wait till everyone makes block 3.
	// it includes the commit for block 2, which is by the original validator set
	waitForAndValidateBlockWithTx(ctx, t, nPeers, activeVals, blocksSubs, states, newValidatorTx1)

	// wait till everyone makes block 4.
	// it includes the commit for block 3, which is by the original validator set
	waitForAndValidateBlock(ctx, t, nPeers, activeVals, blocksSubs, states)

	// the commits for block 4 should be with the updated validator set
	activeVals[string(newValidatorPubKey1.Address())] = struct{}{}

	// wait till everyone makes block 5
	// it includes the commit for block 4, which should have the updated validator set
	waitForBlockWithUpdatedValsAndValidateIt(ctx, t, nPeers, activeVals, blocksSubs, states)

	updateValidatorPubKey1, err := states[nVals].privValidator.GetPubKey(ctx)
	require.NoError(t, err)

	updatePubKey1ABCI, err := encoding.PubKeyToProto(updateValidatorPubKey1)
	require.NoError(t, err)

	updateValidatorTx1 := kvstore.MakeValSetChangeTx(updatePubKey1ABCI, 25)
	previousTotalVotingPower := states[nVals].GetRoundState().LastValidators.TotalVotingPower()

	waitForAndValidateBlock(ctx, t, nPeers, activeVals, blocksSubs, states, updateValidatorTx1)
	waitForAndValidateBlockWithTx(ctx, t, nPeers, activeVals, blocksSubs, states, updateValidatorTx1)
	waitForAndValidateBlock(ctx, t, nPeers, activeVals, blocksSubs, states)
	waitForBlockWithUpdatedValsAndValidateIt(ctx, t, nPeers, activeVals, blocksSubs, states)

	require.NotEqualf(
		t, states[nVals].GetRoundState().LastValidators.TotalVotingPower(), previousTotalVotingPower,
		"expected voting power to change (before: %d, after: %d)",
		previousTotalVotingPower, states[nVals].GetRoundState().LastValidators.TotalVotingPower(),
	)

	newValidatorPubKey2, err := states[nVals+1].privValidator.GetPubKey(ctx)
	require.NoError(t, err)

	newVal2ABCI, err := encoding.PubKeyToProto(newValidatorPubKey2)
	require.NoError(t, err)

	newValidatorTx2 := kvstore.MakeValSetChangeTx(newVal2ABCI, testMinPower)

	newValidatorPubKey3, err := states[nVals+2].privValidator.GetPubKey(ctx)
	require.NoError(t, err)

	newVal3ABCI, err := encoding.PubKeyToProto(newValidatorPubKey3)
	require.NoError(t, err)

	newValidatorTx3 := kvstore.MakeValSetChangeTx(newVal3ABCI, testMinPower)

	waitForAndValidateBlock(ctx, t, nPeers, activeVals, blocksSubs, states, newValidatorTx2, newValidatorTx3)
	waitForAndValidateBlockWithTx(ctx, t, nPeers, activeVals, blocksSubs, states, newValidatorTx2, newValidatorTx3)
	waitForAndValidateBlock(ctx, t, nPeers, activeVals, blocksSubs, states)

	activeVals[string(newValidatorPubKey2.Address())] = struct{}{}
	activeVals[string(newValidatorPubKey3.Address())] = struct{}{}

	waitForBlockWithUpdatedValsAndValidateIt(ctx, t, nPeers, activeVals, blocksSubs, states)

	removeValidatorTx2 := kvstore.MakeValSetChangeTx(newVal2ABCI, 0)
	removeValidatorTx3 := kvstore.MakeValSetChangeTx(newVal3ABCI, 0)

	waitForAndValidateBlock(ctx, t, nPeers, activeVals, blocksSubs, states, removeValidatorTx2, removeValidatorTx3)
	waitForAndValidateBlockWithTx(ctx, t, nPeers, activeVals, blocksSubs, states, removeValidatorTx2, removeValidatorTx3)
	waitForAndValidateBlock(ctx, t, nPeers, activeVals, blocksSubs, states)

	delete(activeVals, string(newValidatorPubKey2.Address()))
	delete(activeVals, string(newValidatorPubKey3.Address()))

	waitForBlockWithUpdatedValsAndValidateIt(ctx, t, nPeers, activeVals, blocksSubs, states)
}
