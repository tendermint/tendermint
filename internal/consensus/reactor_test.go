package consensus

import (
	"bytes"
	"context"
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
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
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

	ctx, cancel := context.WithCancel(context.Background())

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

		blocksSub, err := state.eventBus.Subscribe(ctx, testSubscriber, types.EventQueryNewBlock, size)
		require.NoError(t, err)

		fsSub, err := state.eventBus.Subscribe(ctx, testSubscriber, types.EventQueryBlockSyncStatus, size)
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
		for _, r := range rts.reactors {
			require.NoError(t, r.eventBus.Stop())
			require.NoError(t, r.Stop())
			require.False(t, r.IsRunning())
		}

		leaktest.Check(t)
		cancel()
	})

	return rts
}

func validateBlock(block *types.Block, activeVals map[string]struct{}) error {
	if _, ok := activeVals[block.ProposerProTxHash.String()]; !ok {
		return fmt.Errorf("found vote for inactive validator %X", block.ProposerProTxHash)
	}
	return block.ValidateBasic()
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
	quorumHash crypto.QuorumHash,
	blocksSubs []types.Subscription,
	states []*State,
) {

	fn := func(j int) {
		var newBlock *types.Block

	LOOP:
		for {
			msg := <-blocksSubs[j].Out()
			newBlock = msg.Data().(types.EventDataNewBlock).Block
			if bytes.Equal(newBlock.LastCommit.QuorumHash, quorumHash) {
				break LOOP
			}
			states[j].Logger.Info(
				"waitForBlockWithUpdatedValsAndValidateIt: Got block with no new validators. Skipping",
				"height",
				newBlock.Height,
				"lastCommitQuorum", newBlock.LastCommit.QuorumHash, "ActualQuorum", quorumHash,
			)
		}

		require.NoError(t, newBlock.ValidateBasic())
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

	genDoc, privVals := factory.RandGenesisDoc(cfg, n, 1)
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
		app.InitChain(abci.RequestInitChain{ValidatorSet: &vals})

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
		ev, _ := types.NewMockDuplicateVoteEvidenceWithValidator(1, defaultTestTime, privVals[vIdx], cfg.ChainID(), state.Validators.QuorumType, state.Validators.QuorumHash)
		evpool := &statemocks.EvidencePool{}
		evpool.On("CheckEvidence", mock.AnythingOfType("types.EvidenceList")).Return(nil)
		evpool.On("PendingEvidence", mock.AnythingOfType("int64")).Return([]types.Evidence{
			ev}, int64(len(ev.Bytes())))
		evpool.On("Update", mock.AnythingOfType("state.State"), mock.AnythingOfType("types.EvidenceList")).Return()

		evpool2 := sm.EmptyEvidencePool{}

		blockExec := sm.NewBlockExecutor(stateStore, log.TestingLogger(), proxyAppConnCon, proxyAppConnCon, mempool, evpool, blockStore, nil)
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

	rts := setup(t, nPeers, states, 100) // buffer must be large enough to not deadlock

	for _, reactor := range rts.reactors {
		state := reactor.state.GetState()
		reactor.SwitchToConsensus(state, false)
	}

	// map of active validators
	activeVals := make(map[string]struct{})
	for i := 0; i < nVals; i++ {
		proTxHash, err := states[i].privValidator.GetProTxHash(context.Background())
		require.NoError(t, err)

		activeVals[proTxHash.String()] = struct{}{}
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

	valsUpdater, err := newValidatorUpdater(states, nVals)
	require.NoError(t, err)

	// add one validator to a validator set
	addOneVal, err := valsUpdater.addValidatorsAt(5, 1)
	require.NoError(t, err)

	// add two validators to the validator set
	addTwoVals, err := valsUpdater.addValidatorsAt(10, 2)
	require.NoError(t, err)

	// remove two validators from the validator set
	removeTwoVals, err := valsUpdater.removeValidatorsAt(15, 2)
	require.NoError(t, err)

	// wait till everyone makes block 2
	// ensure the commit includes all validators
	// send newValTx to change vals in block 3
	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, states, addOneVal.txs...)

	// wait till everyone makes block 3.
	// it includes the commit for block 2, which is by the original validator set
	waitForAndValidateBlockWithTx(t, nPeers, activeVals, blocksSubs, states, addOneVal.txs...)

	// wait till everyone makes block 4.
	// it includes the commit for block 3, which is by the original validator set
	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, states)

	// the commits for block 4 should be with the updated validator set
	activeVals = makeProTxHashMap(addOneVal.newProTxHashes)

	// wait till everyone makes block 5
	// it includes the commit for block 4, which should have the updated validator set
	waitForBlockWithUpdatedValsAndValidateIt(t, nPeers, addOneVal.quorumHash, blocksSubs, states)

	validate(t, states)

	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, states, addTwoVals.txs...)
	waitForAndValidateBlockWithTx(t, nPeers, activeVals, blocksSubs, states, addTwoVals.txs...)
	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, states)

	// the commits for block 8 should be with the updated validator set
	activeVals = makeProTxHashMap(addTwoVals.newProTxHashes)

	waitForBlockWithUpdatedValsAndValidateIt(t, nPeers, addTwoVals.quorumHash, blocksSubs, states)

	validate(t, states)

	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, states, removeTwoVals.txs...)
	waitForAndValidateBlockWithTx(t, nPeers, activeVals, blocksSubs, states, removeTwoVals.txs...)
	waitForAndValidateBlock(t, nPeers, activeVals, blocksSubs, states)

	waitForBlockWithUpdatedValsAndValidateIt(t, nPeers, removeTwoVals.quorumHash, blocksSubs, states)

	validate(t, states)
}

func makeProTxHashMap(proTxHashes []crypto.ProTxHash) map[string]struct{} {
	res := make(map[string]struct{})
	for _, proTxHash := range proTxHashes {
		res[proTxHash.String()] = struct{}{}
	}
	return res
}

type privValUpdate struct {
	quorumHash      crypto.QuorumHash
	thresholdPubKey crypto.PubKey
	newProTxHashes  []crypto.ProTxHash
	privKeys        []crypto.PrivKey
	txs             [][]byte
}

type validatorUpdater struct {
	lastProTxHashes []crypto.ProTxHash
	stateIndexMap   map[string]int
	states          []*State
}

func newValidatorUpdater(states []*State, nVals int) (*validatorUpdater, error) {
	updater := validatorUpdater{
		lastProTxHashes: make([]crypto.ProTxHash, nVals),
		states:          states,
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

func (u *validatorUpdater) addValidatorsAt(height int64, count int) (*privValUpdate, error) {
	proTxHashes := u.lastProTxHashes
	l := len(proTxHashes)
	// add new newProTxHashes
	for i := l; i < l+count; i++ {
		proTxHash, err := u.states[i].privValidator.GetProTxHash(context.Background())
		if err != nil {
			return nil, err
		}
		proTxHashes = append(proTxHashes, proTxHash)
	}
	res, err := generatePrivValUpdate(proTxHashes)
	if err != nil {
		return nil, err
	}
	u.updateStatePrivVals(res, height)
	return res, nil
}

func (u *validatorUpdater) removeValidatorsAt(height int64, count int) (*privValUpdate, error) {
	l := len(u.lastProTxHashes)
	if count >= l {
		return nil, fmt.Errorf("you can not remove all validators")
	}
	var newProTxHashes []crypto.ProTxHash
	for i := 0; i < l-count; i++ {
		proTxHash, err := u.states[i].privValidator.GetProTxHash(context.Background())
		if err != nil {
			return nil, err
		}
		newProTxHashes = append(newProTxHashes, proTxHash)
	}
	priValUpdate, err := generatePrivValUpdate(newProTxHashes)
	if err != nil {
		return nil, err
	}
	u.updateStatePrivVals(priValUpdate, height)
	return priValUpdate, nil
}

func (u *validatorUpdater) updateStatePrivVals(privValUpdate *privValUpdate, height int64) {
	for i, proTxHash := range privValUpdate.newProTxHashes {
		j := u.stateIndexMap[proTxHash.String()]
		u.states[j].privValidator.UpdatePrivateKey(
			context.Background(),
			privValUpdate.privKeys[i],
			privValUpdate.quorumHash,
			privValUpdate.thresholdPubKey,
			height,
		)
	}
	u.lastProTxHashes = privValUpdate.newProTxHashes
}

func generatePrivValUpdate(proTxHashes []crypto.ProTxHash) (*privValUpdate, error) {
	privVal := privValUpdate{
		quorumHash: crypto.RandQuorumHash(),
	}
	// generate LLMQ data
	proTxHashes, privKeys, thresholdPubKey := bls12381.CreatePrivLLMQDataOnProTxHashesDefaultThreshold(proTxHashes)

	// make transactions for every validator
	for i, proTxHash := range proTxHashes {
		protoPubKey, err := encoding.PubKeyToProto(privKeys[i].PubKey())
		if err != nil {
			return nil, err
		}
		privVal.txs = append(privVal.txs, kvstore.MakeValSetChangeTx(proTxHash.Bytes(), &protoPubKey, types.DefaultDashVotingPower))
	}
	privVal.txs = append(privVal.txs, kvstore.MakeQuorumHashTx(privVal.quorumHash))
	protoThresholdPubKey, err := encoding.PubKeyToProto(thresholdPubKey)
	if err != nil {
		return nil, err
	}
	privVal.txs = append(privVal.txs, kvstore.MakeThresholdPublicKeyChangeTx(protoThresholdPubKey))
	privVal.privKeys = privKeys
	privVal.newProTxHashes = proTxHashes
	privVal.thresholdPubKey = thresholdPubKey
	return &privVal, nil
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
