package evidence_test

import (
	"context"
	"testing"
	"time"

	"github.com/dashevo/dashd-go/btcjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/internal/eventbus"
	"github.com/tendermint/tendermint/internal/evidence"
	"github.com/tendermint/tendermint/internal/evidence/mocks"
	tmpubsub "github.com/tendermint/tendermint/internal/pubsub"
	tmquery "github.com/tendermint/tendermint/internal/pubsub/query"
	sm "github.com/tendermint/tendermint/internal/state"
	smmocks "github.com/tendermint/tendermint/internal/state/mocks"
	"github.com/tendermint/tendermint/internal/store"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/version"
)

const evidenceChainID = "test_chain"

var (
	defaultEvidenceTime           = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	defaultEvidenceMaxBytes int64 = 1000
)

func startPool(t *testing.T, pool *evidence.Pool, store sm.Store) {
	t.Helper()
	state, err := store.Load()
	if err != nil {
		t.Fatalf("cannot load state: %v", err)
	}
	if err := pool.Start(state); err != nil {
		t.Fatalf("cannot start state pool: %v", err)
	}

}

func TestEvidencePoolBasic(t *testing.T) {
	var (
		height     = int64(1)
		stateStore = &smmocks.Store{}
		evidenceDB = dbm.NewMemDB()
		blockStore = &mocks.BlockStore{}
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	valSet, privVals := types.RandValidatorSet(1)
	blockStore.On("LoadBlockMeta", mock.AnythingOfType("int64")).Return(
		&types.BlockMeta{Header: types.Header{Time: defaultEvidenceTime}},
	)
	stateStore.On("LoadValidators", mock.AnythingOfType("int64")).Return(valSet, nil)
	stateStore.On("Load").Return(createState(height+1, valSet), nil)

	logger := log.NewNopLogger()
	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	pool := evidence.NewPool(logger, evidenceDB, stateStore, blockStore, evidence.NopMetrics(), eventBus)
	startPool(t, pool, stateStore)

	// evidence not seen yet:
	evs, size := pool.PendingEvidence(defaultEvidenceMaxBytes)
	require.Equal(t, 0, len(evs))
	require.Zero(t, size)

	ev, err := types.NewMockDuplicateVoteEvidenceWithValidator(ctx, height, defaultEvidenceTime, privVals[0], evidenceChainID,
		valSet.QuorumType, valSet.QuorumHash)
	require.NoError(t, err)
	// good evidence
	evAdded := make(chan struct{})
	go func() {
		<-pool.EvidenceWaitChan()
		close(evAdded)
	}()

	// evidence seen but not yet committed:
	err = pool.AddEvidence(ctx, ev)
	require.NoError(t, err)

	select {
	case <-evAdded:
	case <-time.After(5 * time.Second):
		t.Fatal("evidence was not added to list after 5s")
	}

	next := pool.EvidenceFront()
	require.Equal(t, ev, next.Value.(types.Evidence))

	const evidenceBytes int64 = 640
	evs, size = pool.PendingEvidence(evidenceBytes)
	require.Equal(t, 1, len(evs))
	require.Equal(t, evidenceBytes, size) // check that the size of the single evidence in bytes is correct

	// shouldn't be able to add evidence twice
	err = pool.AddEvidence(ctx, ev)
	require.NoError(t, err)
	evs, _ = pool.PendingEvidence(defaultEvidenceMaxBytes)
	require.Equal(t, 1, len(evs))
}

// Tests inbound evidence for the right time and height
func TestAddExpiredEvidence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		quorumHash          = crypto.RandQuorumHash()
		val                 = types.NewMockPVForQuorum(quorumHash)
		height              = int64(30)
		stateStore          = initializeValidatorState(ctx, t, val, height, btcjson.LLMQType_5_60, quorumHash)
		evidenceDB          = dbm.NewMemDB()
		blockStore          = &mocks.BlockStore{}
		expiredEvidenceTime = time.Date(2018, 1, 1, 0, 0, 0, 0, time.UTC)
		expiredHeight       = int64(2)
	)

	blockStore.On("LoadBlockMeta", mock.AnythingOfType("int64")).Return(func(h int64) *types.BlockMeta {
		if h == height || h == expiredHeight {
			return &types.BlockMeta{Header: types.Header{Time: defaultEvidenceTime}}
		}
		return &types.BlockMeta{Header: types.Header{Time: expiredEvidenceTime}}
	})

	logger := log.NewNopLogger()
	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	pool := evidence.NewPool(logger, evidenceDB, stateStore, blockStore, evidence.NopMetrics(), eventBus)
	startPool(t, pool, stateStore)

	testCases := []struct {
		evHeight      int64
		evTime        time.Time
		expErr        bool
		evDescription string
	}{
		{height, defaultEvidenceTime, false, "valid evidence"},
		{expiredHeight, defaultEvidenceTime, false, "valid evidence (despite old height)"},
		{height - 1, expiredEvidenceTime, false, "valid evidence (despite old time)"},
		{expiredHeight - 1, expiredEvidenceTime, true,
			"evidence from height 1 (created at: 2019-01-01 00:00:00 +0000 UTC) is too old"},
		{height, defaultEvidenceTime.Add(1 * time.Minute), true, "evidence time and block time is different"},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.evDescription, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			vals := pool.State().Validators
			ev, err := types.NewMockDuplicateVoteEvidenceWithValidator(ctx, tc.evHeight, tc.evTime, val, evidenceChainID, vals.QuorumType, vals.QuorumHash)
			require.NoError(t, err)
			err = pool.AddEvidence(ctx, ev)
			if tc.expErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestReportConflictingVotes(t *testing.T) {
	var height int64 = 10

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, pv, _ := defaultTestPool(ctx, t, height)

	quorumHash := pool.State().Validators.QuorumHash

	val := pv.ExtractIntoValidator(ctx, quorumHash)
	ev, err := types.NewMockDuplicateVoteEvidenceWithValidator(ctx, height+1, defaultEvidenceTime, pv, evidenceChainID,
		btcjson.LLMQType_5_60, quorumHash)
	require.NoError(t, err)

	pool.ReportConflictingVotes(ev.VoteA, ev.VoteB)

	// shouldn't be able to submit the same evidence twice
	pool.ReportConflictingVotes(ev.VoteA, ev.VoteB)

	// evidence from consensus should not be added immediately but reside in the consensus buffer
	evList, evSize := pool.PendingEvidence(defaultEvidenceMaxBytes)
	require.Empty(t, evList)
	require.Zero(t, evSize)

	next := pool.EvidenceFront()
	require.Nil(t, next)

	// move to next height and update state and evidence pool
	state := pool.State()
	state.LastBlockHeight++
	state.LastBlockTime = ev.Time()
	state.LastValidators = types.NewValidatorSet([]*types.Validator{val}, val.PubKey, btcjson.LLMQType_5_60,
		quorumHash, true)
	pool.Update(ctx, state, []types.Evidence{})

	// should be able to retrieve evidence from pool
	evList, _ = pool.PendingEvidence(defaultEvidenceMaxBytes)
	require.Equal(t, []types.Evidence{ev}, evList)

	next = pool.EvidenceFront()
	require.NotNil(t, next)
}

func TestEvidencePoolUpdate(t *testing.T) {
	height := int64(21)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, val, _ := defaultTestPool(ctx, t, height)

	state := pool.State()

	// create two lots of old evidence that we expect to be pruned when we update
	prunedEv, err := types.NewMockDuplicateVoteEvidenceWithValidator(ctx,
		1,
		defaultEvidenceTime.Add(1*time.Minute),
		val,
		evidenceChainID,
		state.Validators.QuorumType,
		state.Validators.QuorumHash,
	)
	require.NoError(t, err)

	notPrunedEv, err := types.NewMockDuplicateVoteEvidenceWithValidator(ctx,
		2,
		defaultEvidenceTime.Add(2*time.Minute),
		val,
		evidenceChainID,
		state.Validators.QuorumType,
		state.Validators.QuorumHash,
	)
	require.NoError(t, err)

	require.NoError(t, pool.AddEvidence(ctx, prunedEv))
	require.NoError(t, pool.AddEvidence(ctx, notPrunedEv))

	ev, err := types.NewMockDuplicateVoteEvidenceWithValidator(
		ctx,
		height,
		defaultEvidenceTime.Add(21*time.Minute),
		val,
		evidenceChainID,
		state.Validators.QuorumType,
		state.Validators.QuorumHash,
	)
	require.NoError(t, err)
	lastCommit := makeCommit(height, state.Validators.QuorumHash, val.ProTxHash)

	coreChainLockHeight := state.LastCoreChainLockedBlockHeight
	block := types.MakeBlock(height+1, coreChainLockHeight, nil, []types.Tx{}, lastCommit, []types.Evidence{ev}, 0)

	// update state (partially)
	state.LastBlockHeight = height + 1
	state.LastBlockTime = defaultEvidenceTime.Add(22 * time.Minute)

	evList, _ := pool.PendingEvidence(2 * defaultEvidenceMaxBytes)
	require.Equal(t, 2, len(evList))

	require.Equal(t, uint32(2), pool.Size())

	require.NoError(t, pool.CheckEvidence(ctx, types.EvidenceList{ev}))

	evList, _ = pool.PendingEvidence(3 * defaultEvidenceMaxBytes)
	require.Equal(t, 3, len(evList))

	require.Equal(t, uint32(3), pool.Size())

	pool.Update(ctx, state, block.Evidence)

	// a) Update marks evidence as committed so pending evidence should be empty
	evList, _ = pool.PendingEvidence(defaultEvidenceMaxBytes)
	require.Equal(t, []types.Evidence{notPrunedEv}, evList)

	// b) If we try to check this evidence again it should fail because it has already been committed
	err = pool.CheckEvidence(ctx, types.EvidenceList{ev})
	if assert.Error(t, err) {
		assert.Equal(t, "evidence was already committed", err.(*types.ErrInvalidEvidence).Reason.Error())
	}
}

func TestVerifyPendingEvidencePasses(t *testing.T) {
	var height int64 = 1

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, val, _ := defaultTestPool(ctx, t, height)
	vals := pool.State().Validators
	ev, err := types.NewMockDuplicateVoteEvidenceWithValidator(
		ctx,
		height,
		defaultEvidenceTime.Add(1*time.Minute),
		val,
		evidenceChainID,
		vals.QuorumType,
		vals.QuorumHash,
	)
	require.NoError(t, err)
	require.NoError(t, pool.AddEvidence(ctx, ev))
	require.NoError(t, pool.CheckEvidence(ctx, types.EvidenceList{ev}))
}

func TestVerifyDuplicatedEvidenceFails(t *testing.T) {
	var height int64 = 1

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, val, _ := defaultTestPool(ctx, t, height)
	vals := pool.State().Validators
	ev, err := types.NewMockDuplicateVoteEvidenceWithValidator(
		ctx,
		height,
		defaultEvidenceTime.Add(1*time.Minute),
		val,
		evidenceChainID,
		vals.QuorumType,
		vals.QuorumHash,
	)
	require.NoError(t, err)

	err = pool.CheckEvidence(ctx, types.EvidenceList{ev, ev})
	if assert.Error(t, err) {
		assert.Equal(t, "duplicate evidence", err.(*types.ErrInvalidEvidence).Reason.Error())
	}
}

// Check that we generate events when evidence is added into the evidence pool
func TestEventOnEvidenceValidated(t *testing.T) {
	const height = 1

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, val, eventBus := defaultTestPool(ctx, t, height)

	vals := pool.State().Validators

	ev, err := types.NewMockDuplicateVoteEvidenceWithValidator(
		ctx,
		height,
		defaultEvidenceTime.Add(1*time.Minute),
		val,
		evidenceChainID,
		vals.QuorumType,
		vals.QuorumHash,
	)
	require.NoError(t, err)

	const query = `tm.event='EvidenceValidated'`
	evSub, err := eventBus.SubscribeWithArgs(ctx, tmpubsub.SubscribeArgs{
		ClientID: "test",
		Query:    tmquery.MustCompile(query),
	})
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		defer close(done)
		msg, err := evSub.Next(ctx)
		if ctx.Err() != nil {
			return
		}
		assert.NoError(t, err)

		edt := msg.Data().(types.EventDataEvidenceValidated)
		assert.Equal(t, ev, edt.Evidence)
	}()
	err = pool.AddEvidence(ctx, ev)
	require.NoError(t, err)

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("did not receive a block header after 1 sec.")
	}

}

// Tests that restarting the evidence pool after a potential failure will recover the
// pending evidence and continue to gossip it
func TestRecoverPendingEvidence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	height := int64(10)
	quorumHash := crypto.RandQuorumHash()
	val := types.NewMockPVForQuorum(quorumHash)
	proTxHash := val.ProTxHash
	evidenceDB := dbm.NewMemDB()
	stateStore := initializeValidatorState(ctx, t, val, height, btcjson.LLMQType_5_60, quorumHash)

	state, err := stateStore.Load()
	require.NoError(t, err)

	blockStore, err := initializeBlockStore(dbm.NewMemDB(), state, proTxHash)
	require.NoError(t, err)

	logger := log.NewNopLogger()
	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	// create previous pool and populate it
	pool := evidence.NewPool(logger, evidenceDB, stateStore, blockStore, evidence.NopMetrics(), eventBus)
	startPool(t, pool, stateStore)

	vals := pool.State().Validators

	goodEvidence, err := types.NewMockDuplicateVoteEvidenceWithValidator(
		ctx,
		height,
		defaultEvidenceTime.Add(10*time.Minute),
		val,
		evidenceChainID,
		vals.QuorumType,
		vals.QuorumHash,
	)
	require.NoError(t, err)
	expiredEvidence, err := types.NewMockDuplicateVoteEvidenceWithValidator(
		ctx,
		int64(1),
		defaultEvidenceTime.Add(1*time.Minute),
		val,
		evidenceChainID,
		vals.QuorumType,
		vals.QuorumHash,
	)
	require.NoError(t, err)

	err = pool.AddEvidence(ctx, goodEvidence)
	require.NoError(t, err)
	err = pool.AddEvidence(ctx, expiredEvidence)
	require.NoError(t, err)

	// now recover from the previous pool at a different time
	newStateStore := &smmocks.Store{}
	newStateStore.On("Load").Return(sm.State{
		LastBlockTime:   defaultEvidenceTime.Add(25 * time.Minute),
		LastBlockHeight: height + 15,
		ConsensusParams: types.ConsensusParams{
			Block: types.BlockParams{
				MaxBytes: 22020096,
				MaxGas:   -1,
			},
			Evidence: types.EvidenceParams{
				MaxAgeNumBlocks: 20,
				MaxAgeDuration:  20 * time.Minute,
				MaxBytes:        defaultEvidenceMaxBytes,
			},
		},
	}, nil)

	newPool := evidence.NewPool(logger, evidenceDB, newStateStore, blockStore, evidence.NopMetrics(), nil)
	startPool(t, newPool, newStateStore)
	evList, _ := newPool.PendingEvidence(defaultEvidenceMaxBytes)
	require.Equal(t, 1, len(evList))

	next := newPool.EvidenceFront()
	require.Equal(t, goodEvidence, next.Value.(types.Evidence))
}

func initializeStateFromValidatorSet(t *testing.T, valSet *types.ValidatorSet, height int64) sm.Store {
	stateDB := dbm.NewMemDB()
	stateStore := sm.NewStore(stateDB)
	state := sm.State{
		ChainID:                     evidenceChainID,
		InitialHeight:               1,
		LastBlockHeight:             height,
		LastBlockTime:               defaultEvidenceTime,
		Validators:                  valSet,
		NextValidators:              valSet.CopyIncrementProposerPriority(1),
		LastValidators:              valSet,
		LastHeightValidatorsChanged: 1,
		ConsensusParams: types.ConsensusParams{
			Block: types.BlockParams{
				MaxBytes: 22020096,
				MaxGas:   -1,
			},
			Evidence: types.EvidenceParams{
				MaxAgeNumBlocks: 20,
				MaxAgeDuration:  20 * time.Minute,
				MaxBytes:        1000,
			},
		},
	}

	// save all states up to height
	for i := int64(0); i <= height; i++ {
		state.LastBlockHeight = i
		require.NoError(t, stateStore.Save(state))
	}

	return stateStore
}

func initializeValidatorState(
	ctx context.Context,
	t *testing.T,
	privVal types.PrivValidator,
	height int64,
	quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash,
) sm.Store {
	pubKey, err := privVal.GetPubKey(ctx, quorumHash)
	require.NoError(t, err)
	proTxHash, err := privVal.GetProTxHash(ctx)
	require.NoError(t, err)
	validator := &types.Validator{VotingPower: types.DefaultDashVotingPower, PubKey: pubKey, ProTxHash: proTxHash}

	// create validator set and state
	valSet := &types.ValidatorSet{
		Validators:         []*types.Validator{validator},
		Proposer:           validator,
		ThresholdPublicKey: validator.PubKey,
		QuorumType:         quorumType,
		QuorumHash:         quorumHash,
	}

	return initializeStateFromValidatorSet(t, valSet, height)
}

// initializeBlockStore creates a block storage and populates it w/ a dummy
// block at +height+.
func initializeBlockStore(db dbm.DB, state sm.State, valProTxHash []byte) (*store.BlockStore, error) {
	blockStore := store.NewBlockStore(db)

	for i := int64(1); i <= state.LastBlockHeight; i++ {
		lastCommit := makeCommit(i-1, state.Validators.QuorumHash, valProTxHash)
		block := state.MakeBlock(i, nil, []types.Tx{}, lastCommit, nil, state.Validators.GetProposer().ProTxHash, 0)

		block.Header.Time = defaultEvidenceTime.Add(time.Duration(i) * time.Minute)
		block.Header.Version = version.Consensus{Block: version.BlockProtocol, App: 1}
		const parts = 1
		partSet, err := block.MakePartSet(parts)
		if err != nil {
			return nil, err
		}

		seenCommit := makeCommit(i, state.Validators.QuorumHash, valProTxHash)
		blockStore.SaveBlock(block, partSet, seenCommit)
	}

	return blockStore, nil
}

func makeCommit(height int64, quorumHash []byte, valProTxHash []byte) *types.Commit {
	return types.NewCommit(
		height,
		0,
		types.BlockID{},
		types.StateID{Height: height - 1},
		&types.QuorumVoteSigns{
			ThresholdVoteSigns: types.ThresholdVoteSigns{
				BlockSign: crypto.CRandBytes(types.SignatureSize),
				StateSign: crypto.CRandBytes(types.SignatureSize),
			},
			QuorumHash: quorumHash,
		},
	)
}

func defaultTestPool(ctx context.Context, t *testing.T, height int64) (*evidence.Pool, *types.MockPV, *eventbus.EventBus) {
	t.Helper()
	quorumHash := crypto.RandQuorumHash()
	val := types.NewMockPVForQuorum(quorumHash)

	evidenceDB := dbm.NewMemDB()
	stateStore := initializeValidatorState(ctx, t, val, height, btcjson.LLMQType_5_60, quorumHash)
	state, err := stateStore.Load()
	require.NoError(t, err)
	blockStore, err := initializeBlockStore(dbm.NewMemDB(), state, val.ProTxHash)
	require.NoError(t, err)

	logger := log.NewNopLogger()

	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	pool := evidence.NewPool(logger, evidenceDB, stateStore, blockStore, evidence.NopMetrics(), eventBus)
	startPool(t, pool, stateStore)
	return pool, val, eventBus
}

func createState(height int64, valSet *types.ValidatorSet) sm.State {
	return sm.State{
		ChainID:         evidenceChainID,
		LastBlockHeight: height,
		LastBlockTime:   defaultEvidenceTime,
		Validators:      valSet,
		ConsensusParams: *types.DefaultConsensusParams(),
	}
}
