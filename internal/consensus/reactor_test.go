package consensus

import (
	"context"
	"errors"
	"fmt"
	"os"
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
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/dash"
	"github.com/tendermint/tendermint/dash/llmq"
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
	stateChannels       map[types.NodeID]p2p.Channel
	dataChannels        map[types.NodeID]p2p.Channel
	voteChannels        map[types.NodeID]p2p.Channel
	voteSetBitsChannels map[types.NodeID]p2p.Channel
}

func (rts *reactorTestSuite) switchToConsensus(ctx context.Context) {
	for nodeID, reactor := range rts.reactors {
		state := reactor.state.GetState()
		sCtx := dash.ContextWithProTxHash(ctx, rts.states[nodeID].privValidatorProTxHash)
		reactor.SwitchToConsensus(sCtx, state, false)
	}
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

	privProTxHashes := make([]crypto.ProTxHash, len(states))
	for i, state := range states {
		privProTxHashes[i] = state.privValidatorProTxHash
	}
	rts := &reactorTestSuite{
		network:       p2ptest.MakeNetwork(ctx, t, p2ptest.NetworkOptions{NumNodes: numNodes, ProTxHashes: privProTxHashes}),
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
		return func(ctx context.Context, desc *p2p.ChannelDescriptor) (p2p.Channel, error) {
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

	for i := 0; i < numNodes; i++ {
		state := states[i]
		sCtx := dash.ContextWithProTxHash(ctx, states[i].privValidatorProTxHash)
		node := rts.network.NodeByProTxHash(state.privValidatorProTxHash)
		require.NotNil(t, node)
		nodeID := node.NodeID
		reactor := NewReactor(
			state.logger.With("node", nodeID),
			state,
			chCreator(nodeID),
			func(ctx context.Context) *p2p.PeerUpdates { return node.MakePeerUpdates(ctx, t) },
			state.eventBus,
			true,
			NopMetrics(),
		)

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

		require.NoError(t, reactor.Start(sCtx))
		require.True(t, reactor.IsRunning())
		t.Cleanup(reactor.Wait)
	}

	require.Len(t, rts.reactors, numNodes)

	// start the in-memory network and connect all peers with each other
	rts.network.Start(ctx, t)

	t.Cleanup(leaktest.Check(t))

	return rts
}

func waitForAndValidateBlock(
	bctx context.Context,
	t *testing.T,
	n int,
	blocksSubs []eventbus.Subscription,

) []*types.Block {
	t.Helper()

	blocks := make([]*types.Block, n)

	ctx, cancel := context.WithCancel(bctx)
	defer cancel()

	fn := func(j int) {
		msg, err := blocksSubs[j].Next(ctx)
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return
		case errors.Is(err, context.Canceled):
			return
		case err != nil:
			cancel() // terminate other workers
			require.NoError(t, err)
			return
		}
		newBlock := msg.Data().(types.EventDataNewBlock).Block
		require.NoError(t, newBlock.ValidateBasic())
		blocks[j] = newBlock
	}

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(j int) {
			defer wg.Done()
			fn(j)
		}(i)
	}

	wg.Wait()
	if err := ctx.Err(); errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("encountered timeout")
	}
	return blocks
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

	n := 2
	states, cleanup := makeConsensusState(ctx, t,
		cfg, n, "consensus_reactor_test",
		newMockTickerFunc(true))
	t.Cleanup(cleanup)

	rts := setup(ctx, t, n, states, 100) // buffer must be large enough to not deadlock

	rts.switchToConsensus(ctx)

	var wg sync.WaitGroup
	errCh := make(chan error, len(rts.subs))

	for _, sub := range rts.subs {
		wg.Add(1)

		// wait till everyone makes the first new block
		go func(s eventbus.Subscription) {
			defer wg.Done()
			_, err := s.Next(ctx)
			switch {
			case errors.Is(err, context.DeadlineExceeded):
				return
			case errors.Is(err, context.Canceled):
				return
			case err != nil:
				errCh <- err
				cancel() // terminate other workers
				return
			}
		}(sub)
	}

	wg.Wait()
	if err := ctx.Err(); errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("encountered timeout")
	}
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	default:
	}

	errCh = make(chan error, len(rts.blocksyncSubs))
	for _, sub := range rts.blocksyncSubs {
		wg.Add(1)

		// wait till everyone makes the consensus switch
		go func(s eventbus.Subscription) {
			defer wg.Done()
			msg, err := s.Next(ctx)
			switch {
			case errors.Is(err, context.DeadlineExceeded):
				return
			case errors.Is(err, context.Canceled):
				return
			case err != nil:
				errCh <- err
				cancel() // terminate other workers
				return
			}
			ensureBlockSyncStatus(t, msg, true, 0)
		}(sub)
	}

	wg.Wait()
	if err := ctx.Err(); errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("encountered timeout")
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	default:
	}
}

func TestReactorWithEvidence(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	cfg := configSetup(t)

	n := 4
	testName := "consensus_reactor_test"
	tickerFunc := newTickerFunc()

	consParams := factory.ConsensusParams()

	// if this parameter is not increased, then with a high probability the code will be stuck on proposal step
	// due to a timeout handler performs before than validators will be ready for the message
	consParams.Timeout.Propose = 1 * time.Second

	genDoc, privVals := factory.RandGenesisDoc(cfg, n, 1, consParams)
	states := make([]*State, n)
	logger := consensusLogger(t)

	for i := 0; i < n; i++ {
		stateDB := dbm.NewMemDB() // each state needs its own db
		stateStore := sm.NewStore(stateDB)
		state, err := sm.MakeGenesisState(genDoc)
		require.NoError(t, err)
		require.NoError(t, stateStore.Save(state))
		thisConfig, err := ResetConfig(t.TempDir(), fmt.Sprintf("%s_%d", testName, i))
		require.NoError(t, err)

		defer os.RemoveAll(thisConfig.RootDir)

		app := kvstore.NewApplication()
		vals := types.TM2PB.ValidatorUpdates(state.Validators)
		_, err = app.InitChain(ctx, &abci.RequestInitChain{ValidatorSet: &vals})
		require.NoError(t, err)

		pv := privVals[i]
		blockDB := dbm.NewMemDB()
		blockStore := store.NewBlockStore(blockDB)

		// one for mempool, one for consensus
		proxyAppConnMem := abciclient.NewLocalClient(logger, app)
		proxyAppConnCon := abciclient.NewLocalClient(logger, app)

		mempool := mempool.NewTxMempool(
			log.NewNopLogger().With("module", "mempool"),
			thisConfig.Mempool,
			proxyAppConnMem,
		)

		if thisConfig.Consensus.WaitForTxs() {
			mempool.EnableTxsAvailable()
		}

		// mock the evidence pool
		// everyone includes evidence of another double signing
		vIdx := (i + 1) % n

		ev, err := types.NewMockDuplicateVoteEvidenceWithValidator(ctx, 1, defaultTestTime, privVals[vIdx], cfg.ChainID(), state.Validators.QuorumType, state.Validators.QuorumHash)
		require.NoError(t, err)
		evpool := &statemocks.EvidencePool{}
		evpool.On("CheckEvidence", mock.Anything, mock.AnythingOfType("types.EvidenceList")).Return(nil)
		evpool.On("PendingEvidence", mock.AnythingOfType("int64")).Return([]types.Evidence{
			ev}, int64(len(ev.Bytes())))
		evpool.On("Update", mock.Anything, mock.AnythingOfType("state.State"), mock.AnythingOfType("types.EvidenceList")).Return()

		evpool2 := sm.EmptyEvidencePool{}

		eventBus := eventbus.NewDefault(log.NewNopLogger().With("module", "events"))
		require.NoError(t, eventBus.Start(ctx))

		blockExec := sm.NewBlockExecutor(stateStore, log.NewNopLogger(), proxyAppConnCon, mempool, evpool, blockStore, eventBus, sm.NopMetrics())

		cs, err := NewState(logger.With("validator", i, "module", "consensus"),
			thisConfig.Consensus, stateStore, blockExec, blockStore, mempool, evpool2, eventBus)
		require.NoError(t, err)
		cs.SetPrivValidator(ctx, pv)

		cs.SetTimeoutTicker(tickerFunc())

		states[i] = cs
	}

	rts := setup(ctx, t, n, states, 100) // buffer must be large enough to not deadlock

	rts.switchToConsensus(ctx)

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

	n := 2
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

	rts.switchToConsensus(ctx)

	// send a tx
	require.NoError(
		t,
		assertMempool(t, states[1].txNotifier).CheckTx(
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

	n := 2
	states, cleanup := makeConsensusState(ctx, t,
		cfg, n, "consensus_reactor_test",
		newMockTickerFunc(true))
	t.Cleanup(cleanup)

	rts := setup(ctx, t, n, states, 100) // buffer must be large enough to not deadlock

	rts.switchToConsensus(ctx)

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

func TestReactorValidatorSetChanges(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cfg := configSetup(t)

	nPeers := 7
	nVals := 4

	updates := []validatorUpdate{
		{height: 5, count: 1, operation: addValsOp},
		{height: 10, count: 2, operation: addValsOp},
		{height: 17, count: 2, operation: removeValsOp},
	}
	gen := consensusNetGen{
		cfg:              cfg,
		nPeers:           nPeers,
		nVals:            nVals,
		appFunc:          newKVStoreFunc(),
		validatorUpdates: updates,
	}
	states, _, _, validatorSetUpdates := gen.generate(ctx, t)

	var (
		endHeight         int64
		allowedValidators = make([]map[string]struct{}, 0, len(updates)+1)
		heights           = []int64{0}
	)
	for _, item := range updates {
		heights = append(heights, item.height)
	}
	for _, height := range heights {
		validatorSetUpdate, ok := validatorSetUpdates[height]
		require.True(t, ok)
		allowedValidators = append(allowedValidators, makeProTxHashMap(validatorSetUpdate.ProTxHashes()))
	}
	endHeight = heights[len(heights)-1] + int64(len(validatorSetUpdates[heights[len(heights)-1]].ValidatorUpdates))
	heights = append(heights, endHeight)

	rts := setup(ctx, t, nPeers, states, 100) // buffer must be large enough to not deadlock

	rts.switchToConsensus(ctx)

	var wg sync.WaitGroup
	for _, sub := range rts.subs {
		wg.Add(1)

		// wait till everyone makes the first new block
		go func(s eventbus.Subscription) {
			defer wg.Done()
			_, err := s.Next(ctx)
			switch {
			case err == nil:
			case errors.Is(err, context.DeadlineExceeded):
			default:
				t.Log(err)
				cancel()
			}
		}(sub)
	}

	wg.Wait()

	blocksSubs := make([]eventbus.Subscription, 0, len(rts.subs))
	for _, sub := range rts.subs {
		blocksSubs = append(blocksSubs, sub)
	}
	var height int64 = 2
	blocksByHeights := make(map[int64][]*types.Block)
	for ; height <= endHeight; height++ {
		blocks := waitForAndValidateBlock(ctx, t, nPeers, blocksSubs)
		blocksByHeights[height] = blocks
		validate(t, states)
	}
	var i int64 = 2
	for _, h := range heights[1:] {
		vals := allowedValidators[0]
		allowedValidators = allowedValidators[1:]
		for ; i <= h; i++ {
			for _, block := range blocksByHeights[i] {
				_, ok := vals[block.ProposerProTxHash.ShortString()]
				require.True(t, ok)
			}
		}
	}
	i = 2
	vh := heights[0]
	for _, h := range heights[1:] {
		for ; i < h+2; i++ {
			for _, block := range blocksByHeights[i] {
				require.Equalf(t, validatorSetUpdates[vh].QuorumHash, block.LastCommit.QuorumHash.Bytes(),
					"LastCommit at %d has unexpected quorum-hash. want %X, got %X",
					block.LastCommit.Height,
					validatorSetUpdates[vh].QuorumHash,
					block.LastCommit.QuorumHash.Bytes(),
				)
			}
		}
		vh = h
	}
}

func makeProTxHashMap(proTxHashes []crypto.ProTxHash) map[string]struct{} {
	res := make(map[string]struct{})
	for _, proTxHash := range proTxHashes {
		res[proTxHash.ShortString()] = struct{}{}
	}
	return res
}

type validatorSetUpdateStore interface {
	AddValidatorSetUpdate(vsu abci.ValidatorSetUpdate, height int64)
}

const (
	addValsOp    = "add"
	removeValsOp = "remove"
)

type validatorUpdater struct {
	lastProTxHashes []crypto.ProTxHash
	stateIndexMap   map[string]int
	states          []*State
	stores          []validatorSetUpdateStore
}

func newValidatorUpdater(states []*State, stores []validatorSetUpdateStore, nVals int) (*validatorUpdater, error) {
	updater := validatorUpdater{
		lastProTxHashes: make([]crypto.ProTxHash, nVals),
		states:          states,
		stores:          stores,
		stateIndexMap:   make(map[string]int),
	}
	var (
		proTxHash crypto.ProTxHash
		err       error
	)
	for i, state := range states {
		if i < nVals {
			updater.lastProTxHashes[i], err = states[i].privValidator.GetProTxHash(context.Background())
			if err != nil {
				return nil, err
			}
		}
		proTxHash, err = state.privValidator.GetProTxHash(context.Background())
		if err != nil {
			return nil, err
		}
		updater.stateIndexMap[proTxHash.String()] = i
	}
	return &updater, nil
}

func (u *validatorUpdater) execOperation(
	ctx context.Context,
	operation string,
	height int64,
	count int,
) (*quorumData, error) {
	switch operation {
	case addValsOp:
		return u.addValidatorsAt(ctx, height, count)
	case removeValsOp:
		return u.removeValidatorsAt(ctx, height, count)
	}
	return nil, fmt.Errorf("unknown operation %s", operation)
}

func (u *validatorUpdater) addValidatorsAt(ctx context.Context, height int64, count int) (*quorumData, error) {
	proTxHashes := u.lastProTxHashes
	l := len(proTxHashes)
	// add new newProTxHashes
	for i := l; i < l+count; i++ {
		proTxHash, err := u.states[i].privValidator.GetProTxHash(ctx)
		if err != nil {
			return nil, err
		}
		proTxHashes = append(proTxHashes, proTxHash)
	}
	qd, err := generatePrivValUpdate(proTxHashes)
	if err != nil {
		return nil, err
	}
	u.updateStatePrivVals(ctx, qd, height)
	u.updateValidatorSetUpdateStore(qd.validatorSetUpdate, height)
	return qd, nil
}

func (u *validatorUpdater) removeValidatorsAt(ctx context.Context, height int64, count int) (*quorumData, error) {
	l := len(u.lastProTxHashes)
	if count >= l {
		return nil, fmt.Errorf("you can not remove all validators")
	}
	var newProTxHashes []crypto.ProTxHash
	for i := 0; i < l-count; i++ {
		proTxHash, err := u.states[i].privValidator.GetProTxHash(ctx)
		if err != nil {
			return nil, err
		}
		newProTxHashes = append(newProTxHashes, proTxHash)
	}
	qd, err := generatePrivValUpdate(newProTxHashes)
	if err != nil {
		return nil, err
	}
	u.updateStatePrivVals(ctx, qd, height)
	u.updateValidatorSetUpdateStore(qd.validatorSetUpdate, height)
	return qd, nil
}

func (u *validatorUpdater) updateStatePrivVals(ctx context.Context, data *quorumData, height int64) {
	iter := data.Iter()
	for iter.Next() {
		proTxHash, qks := iter.Value()
		j := u.stateIndexMap[proTxHash.String()]
		priVal := u.states[j].PrivValidator()
		priVal.UpdatePrivateKey(
			ctx,
			qks.PrivKey,
			data.quorumHash,
			data.ThresholdPubKey,
			height,
		)
	}
	u.lastProTxHashes = data.ProTxHashes
}

func (u *validatorUpdater) updateValidatorSetUpdateStore(vsu abci.ValidatorSetUpdate, height int64) {
	for _, s := range u.stores {
		s.AddValidatorSetUpdate(vsu, height)
	}
}

func generatePrivValUpdate(proTxHashes []crypto.ProTxHash) (*quorumData, error) {
	// generate LLMQ data
	ld, err := llmq.Generate(proTxHashes)
	if err != nil {
		return nil, err
	}
	qd := quorumData{Data: *ld, quorumHash: crypto.RandQuorumHash()}
	vsu, err := abci.LLMQToValidatorSetProto(*ld, abci.WithQuorumHash(qd.quorumHash))
	if err != nil {
		return nil, err
	}
	qd.validatorSetUpdate = *vsu
	return &qd, nil
}

func validate(t *testing.T, states []*State) {
	currHeight, currValidators := states[0].GetValidatorSet()
	currValidatorCount := currValidators.Size()
	for validatorID, state := range states {
		height, validators := state.GetValidatorSet()
		assert.Equal(t, currHeight, height, "validator_id=%d", validatorID)
		assert.Equal(t, currValidatorCount, len(validators.Validators), "validator_id=%d", validatorID)
		assert.True(t, currValidators.Equals(validators), "validator_id=%d", validatorID)
	}
}
