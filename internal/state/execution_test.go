package state_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abciclient "github.com/tendermint/tendermint/abci/client"
	abciclientmocks "github.com/tendermint/tendermint/abci/client/mocks"
	abci "github.com/tendermint/tendermint/abci/types"
	abcimocks "github.com/tendermint/tendermint/abci/types/mocks"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	"github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/dash"
	"github.com/tendermint/tendermint/internal/eventbus"
	mpmocks "github.com/tendermint/tendermint/internal/mempool/mocks"
	"github.com/tendermint/tendermint/internal/proxy"
	"github.com/tendermint/tendermint/internal/pubsub"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/state/mocks"
	sf "github.com/tendermint/tendermint/internal/state/test/factory"
	"github.com/tendermint/tendermint/internal/store"
	"github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

var (
	testPartSize uint32 = 65536
)

func TestApplyBlock(t *testing.T) {
	app := &testApp{}
	logger := log.NewNopLogger()
	cc := abciclient.NewLocalClient(logger, app)
	proxyApp := proxy.New(cc, logger, proxy.NopMetrics())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	require.NoError(t, proxyApp.Start(ctx))

	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	state, stateDB, _ := makeState(t, 1, 1)
	stateStore := sm.NewStore(stateDB)
	// The state is local, so we just take the first proTxHash
	nodeProTxHash := state.Validators.Validators[0].ProTxHash
	ctx = dash.ContextWithProTxHash(ctx, nodeProTxHash)

	app.ValidatorSetUpdate = state.Validators.ABCIEquivalentValidatorUpdates()

	blockStore := store.NewBlockStore(dbm.NewMemDB())
	mp := &mpmocks.Mempool{}
	mp.On("Lock").Return()
	mp.On("Unlock").Return()
	mp.On("FlushAppConn", mock.Anything).Return(nil)
	mp.On("Update",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(nil)
	blockExec := sm.NewBlockExecutor(
		stateStore,
		logger,
		proxyApp,
		mp,
		sm.EmptyEvidencePool{},
		blockStore,
		eventBus,
		sm.NopMetrics(),
	)

	// Consensus params, version and validators shall be applied in next block.
	consensusParamsBefore := state.ConsensusParams
	validatorsBefore := state.Validators.Hash()

	block, err := sf.MakeBlock(state, 1, new(types.Commit), 1)
	require.NoError(t, err)
	bps, err := block.MakePartSet(testPartSize)
	require.NoError(t, err)
	blockID := types.BlockID{Hash: block.Hash(), PartSetHeader: bps.Header()}

	state, err = blockExec.ApplyBlock(ctx, state, blockID, block)
	require.NoError(t, err)

	// State for next block
	// nextState, err := state.NewStateChangeset(ctx, nil)
	// require.NoError(t, err)
	assert.EqualValues(t, 0, block.Version.App, "App version should not change in current block")
	assert.EqualValues(t, 1, block.ProposedAppVersion, "Block should propose new version")

	assert.Equal(t, consensusParamsBefore.HashConsensusParams(), block.ConsensusHash, "consensus params should change in next block")
	assert.Equal(t, validatorsBefore, block.ValidatorsHash, "validators should change from the next block")

	// TODO check state and mempool
	assert.EqualValues(t, 1, state.Version.Consensus.App, "App version wasn't updated")
}

// TestFinalizeBlockByzantineValidators ensures we send byzantine validators list.
func TestFinalizeBlockByzantineValidators(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := &testApp{}
	logger := log.NewNopLogger()
	cc := abciclient.NewLocalClient(logger, app)
	proxyApp := proxy.New(cc, logger, proxy.NopMetrics())
	err := proxyApp.Start(ctx)
	require.NoError(t, err)

	state, stateDB, privVals := makeState(t, 1, 1)
	stateStore := sm.NewStore(stateDB)
	nodeProTxHash := state.Validators.Validators[0].ProTxHash
	ctx = dash.ContextWithProTxHash(ctx, nodeProTxHash)
	app.ValidatorSetUpdate = state.Validators.ABCIEquivalentValidatorUpdates()

	defaultEvidenceTime := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	privVal := privVals[state.Validators.Validators[0].ProTxHash.String()]

	// we don't need to worry about validating the evidence as long as they pass validate basic
	dve, err := types.NewMockDuplicateVoteEvidenceWithValidator(
		ctx,
		3,
		defaultEvidenceTime,
		privVal,
		state.ChainID,
		state.Validators.QuorumType,
		state.Validators.QuorumHash,
	)
	require.NoError(t, err)
	dve.ValidatorPower = types.DefaultDashVotingPower

	ev := []types.Evidence{dve}

	abciMb := []abci.Misbehavior{
		{
			Type:             abci.MisbehaviorType_DUPLICATE_VOTE,
			Height:           3,
			Time:             defaultEvidenceTime,
			Validator:        types.TM2PB.Validator(state.Validators.Validators[0]),
			TotalVotingPower: types.DefaultDashVotingPower,
		},
	}

	evpool := &mocks.EvidencePool{}
	evpool.On("PendingEvidence", mock.AnythingOfType("int64")).Return(ev, int64(100))
	evpool.On("Update", ctx, mock.AnythingOfType("state.State"), mock.AnythingOfType("types.EvidenceList")).Return()
	evpool.On("CheckEvidence", ctx, mock.AnythingOfType("types.EvidenceList")).Return(nil)
	mp := &mpmocks.Mempool{}
	mp.On("Lock").Return()
	mp.On("Unlock").Return()
	mp.On("FlushAppConn", mock.Anything).Return(nil)
	mp.On("Update",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(nil)

	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	blockStore := store.NewBlockStore(dbm.NewMemDB())

	blockExec := sm.NewBlockExecutor(stateStore, log.NewNopLogger(), proxyApp, mp, evpool, blockStore, eventBus, sm.NopMetrics())

	block, err := sf.MakeBlock(state, 1, new(types.Commit), 1)
	block.SetDashParams(0, nil, block.ProposedAppVersion, nil)
	require.NoError(t, err)
	block.Evidence = ev
	block.Header.EvidenceHash = block.Evidence.Hash()
	bps, err := block.MakePartSet(testPartSize)
	require.NoError(t, err)

	blockID := types.BlockID{Hash: block.Hash(), PartSetHeader: bps.Header()}

	_, err = blockExec.ApplyBlock(ctx, state, blockID, block)
	require.NoError(t, err)

	// TODO check state and mempool
	assert.Equal(t, abciMb, app.Misbehavior)
}

func TestProcessProposal(t *testing.T) {
	const height = 1
	txs := factory.MakeNTxs(height, 10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := abcimocks.NewApplication(t)
	logger := log.NewNopLogger()
	cc := abciclient.NewLocalClient(logger, app)
	proxyApp := proxy.New(cc, logger, proxy.NopMetrics())
	err := proxyApp.Start(ctx)
	require.NoError(t, err)

	state, stateDB, _ := makeState(t, 1, height)
	stateStore := sm.NewStore(stateDB)
	blockStore := store.NewBlockStore(dbm.NewMemDB())

	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	blockExec := sm.NewBlockExecutor(
		stateStore,
		logger,
		proxyApp,
		new(mpmocks.Mempool),
		sm.EmptyEvidencePool{},
		blockStore,
		eventBus,
		sm.NopMetrics(),
	)

	//block0 := sf.MakeBlock(t, state, height-1, new(types.Commit), nil, 0)
	//partSet, err := block0.MakePartSet(types.BlockPartSizeBytes)
	//require.NoError(t, err)
	//blockID := types.BlockID{Hash: block0.Hash(), PartSetHeader: partSet.Header()}
	//stateID := types.RandStateID().WithHeight(height - 1)

	//quorumHash := state.Validators.QuorumHash
	//thBlockSign :=
	//thStateSign
	//voteInfos := []abci.VoteInfo{}
	//for _, privVal := range privVals {
	//	vote, err := factory.MakeVote(ctx, privVal, state.Validators, block0.Header.ChainID, 0, 0, 0, 2, blockID, stateID)
	//	require.NoError(t, err)
	//	proTxHash, err := privVal.GetProTxHash(ctx)
	//	require.NoError(t, err)
	//
	//}

	lastCommit := types.NewCommit(height-1, 0, types.BlockID{}, types.StateID{}, nil)
	block1, err := sf.MakeBlock(state, height, lastCommit, 1)
	require.NoError(t, err)
	block1.Txs = txs
	txResults := factory.ExecTxResults(txs)
	block1.ResultsHash, err = abci.TxResultsHash(txResults)
	require.NoError(t, err)

	version := block1.Version.ToProto()

	expectedRpp := &abci.RequestProcessProposal{
		Txs:         block1.Txs.ToSliceOfBytes(),
		Hash:        block1.Hash(),
		Height:      block1.Header.Height,
		Time:        block1.Header.Time,
		Misbehavior: block1.Evidence.ToABCI(),
		ProposedLastCommit: abci.CommitInfo{
			Round: 0,
			//QuorumHash:
			//BlockSignature:
			//StateSignature:
		},
		NextValidatorsHash: block1.NextValidatorsHash,
		ProposerProTxHash:  block1.ProposerProTxHash,
		Version:            &version,
		ProposedAppVersion: 1,
	}

	app.On("ProcessProposal", mock.Anything, mock.Anything).Return(&abci.ResponseProcessProposal{
		AppHash:   block1.AppHash,
		TxResults: txResults,
		Status:    abci.ResponseProcessProposal_ACCEPT,
	}, nil)
	uncommittedState, err := blockExec.ProcessProposal(ctx, block1, state, true)
	require.NoError(t, err)
	assert.NotZero(t, uncommittedState)
	app.AssertExpectations(t)
	app.AssertCalled(t, "ProcessProposal", ctx, expectedRpp)
}

func TestValidateValidatorUpdates(t *testing.T) {
	pubkey1 := bls12381.GenPrivKey().PubKey()
	pubkey2 := bls12381.GenPrivKey().PubKey()
	pk1, err := encoding.PubKeyToProto(pubkey1)
	assert.NoError(t, err)
	pk2, err := encoding.PubKeyToProto(pubkey2)
	assert.NoError(t, err)

	proTxHash1 := crypto.RandProTxHash()
	proTxHash2 := crypto.RandProTxHash()

	defaultValidatorParams := types.ValidatorParams{
		PubKeyTypes: []string{types.ABCIPubKeyTypeBLS12381},
	}

	addr := types.RandValidatorAddress()

	testCases := []struct {
		name string

		abciUpdates     []abci.ValidatorUpdate
		validatorParams types.ValidatorParams

		shouldErr bool
	}{
		{
			"adding a validator is OK",
			[]abci.ValidatorUpdate{{PubKey: &pk2, Power: 100, ProTxHash: proTxHash2, NodeAddress: addr.String()}},
			defaultValidatorParams,
			false,
		},
		{"adding a validator without address is OK",
			[]abci.ValidatorUpdate{{PubKey: &pk2, Power: 100, ProTxHash: proTxHash2}},
			defaultValidatorParams,
			false,
		},
		{
			"updating a validator is OK",
			[]abci.ValidatorUpdate{{PubKey: &pk1, Power: 100, ProTxHash: proTxHash1, NodeAddress: addr.String()}},
			defaultValidatorParams,
			false,
		},
		{
			"removing a validator is OK",
			[]abci.ValidatorUpdate{{Power: 0, ProTxHash: proTxHash2, NodeAddress: addr.String()}},
			defaultValidatorParams,
			false,
		},
		{
			"adding a validator with negative power results in error",
			[]abci.ValidatorUpdate{{PubKey: &pk2, Power: -100, ProTxHash: proTxHash2, NodeAddress: addr.String()}},
			defaultValidatorParams,
			true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := sm.ValidateValidatorUpdates(tc.abciUpdates, tc.validatorParams)
			if tc.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUpdateValidators(t *testing.T) {
	validatorSet, _ := types.RandValidatorSet(4)
	originalProTxHashes := validatorSet.GetProTxHashes()
	addedProTxHashes := crypto.RandProTxHashes(4)
	combinedProTxHashes := append(originalProTxHashes, addedProTxHashes...) // nolint:gocritic
	combinedValidatorSet, _ := types.GenerateValidatorSet(types.NewValSetParam(combinedProTxHashes))
	regeneratedValidatorSet, _ := types.GenerateValidatorSet(types.NewValSetParam(combinedProTxHashes))
	abciRegeneratedValidatorUpdates := regeneratedValidatorSet.ABCIEquivalentValidatorUpdates()
	removedProTxHashes := combinedValidatorSet.GetProTxHashes()[0 : len(combinedProTxHashes)-2] // these are sorted
	removedValidatorSet, _ := types.GenerateValidatorSet(types.NewValSetParam(
		removedProTxHashes,
	)) // size 6
	abciRemovalValidatorUpdates := removedValidatorSet.ABCIEquivalentValidatorUpdates()
	abciRemovalValidatorUpdates.ValidatorUpdates = append(
		abciRemovalValidatorUpdates.ValidatorUpdates,
		abciRegeneratedValidatorUpdates.ValidatorUpdates[6:]...)
	abciRemovalValidatorUpdates.ValidatorUpdates[6].Power = 0
	abciRemovalValidatorUpdates.ValidatorUpdates[7].Power = 0

	pubkeyRemoval := bls12381.GenPrivKey().PubKey()
	pk, err := encoding.PubKeyToProto(pubkeyRemoval)
	require.NoError(t, err)

	testCases := []struct {
		name string

		currentSet               *types.ValidatorSet
		abciUpdates              *abci.ValidatorSetUpdate
		thresholdPublicKeyUpdate crypto.PubKey

		resultingSet *types.ValidatorSet
		shouldErr    bool
	}{
		{
			"adding a validator set is OK",
			validatorSet,
			combinedValidatorSet.ABCIEquivalentValidatorUpdates(),
			combinedValidatorSet.ThresholdPublicKey,
			combinedValidatorSet,
			false,
		},
		{
			"updating a validator set is OK",
			combinedValidatorSet,
			regeneratedValidatorSet.ABCIEquivalentValidatorUpdates(),
			regeneratedValidatorSet.ThresholdPublicKey,
			regeneratedValidatorSet,
			false,
		},
		{
			"removing a validator is OK",
			regeneratedValidatorSet,
			abciRemovalValidatorUpdates,
			removedValidatorSet.ThresholdPublicKey,
			removedValidatorSet,
			false,
		},
		{
			"removing a non-existing validator results in error",
			removedValidatorSet,
			&abci.ValidatorSetUpdate{
				ValidatorUpdates: []abci.ValidatorUpdate{
					{ProTxHash: crypto.RandProTxHash(), Power: 0},
				},
				ThresholdPublicKey: pk,
				QuorumHash:         removedValidatorSet.QuorumHash,
			},
			removedValidatorSet.ThresholdPublicKey,
			removedValidatorSet,
			true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			updates, _, _, err := types.PB2TM.ValidatorUpdatesFromValidatorSet(tc.abciUpdates)
			assert.NoError(t, err)
			err = tc.currentSet.UpdateWithChangeSet(
				updates,
				tc.thresholdPublicKeyUpdate,
				crypto.RandQuorumHash(),
			)
			if tc.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				require.Equal(t, tc.resultingSet.Size(), tc.currentSet.Size())

				assert.Equal(t, tc.resultingSet.TotalVotingPower(), tc.currentSet.TotalVotingPower())

				assert.Equal(t, tc.resultingSet.Validators[0].ProTxHash, tc.currentSet.Validators[0].ProTxHash)
				if tc.resultingSet.Size() > 1 {
					assert.Equal(t, tc.resultingSet.Validators[1].ProTxHash, tc.currentSet.Validators[1].ProTxHash)
				}
			}
		})
	}
}

// TestFinalizeBlockValidatorUpdates ensures we update validator set and send an event.
func TestFinalizeBlockValidatorUpdates(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := &testApp{}
	logger := log.NewNopLogger()
	cc := abciclient.NewLocalClient(logger, app)
	proxyApp := proxy.New(cc, logger, proxy.NopMetrics())
	err := proxyApp.Start(ctx)
	require.NoError(t, err)

	state, stateDB, _ := makeState(t, 1, 1)
	stateStore := sm.NewStore(stateDB)
	blockStore := store.NewBlockStore(dbm.NewMemDB())
	nodeProTxHash := state.Validators.Validators[0].ProTxHash
	ctx = dash.ContextWithProTxHash(ctx, nodeProTxHash)

	mp := &mpmocks.Mempool{}
	mp.On("Lock").Return()
	mp.On("Unlock").Return()
	mp.On("FlushAppConn", mock.Anything).Return(nil)
	mp.On("Update",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(nil)
	mp.On("ReapMaxBytesMaxGas", mock.Anything, mock.Anything).Return(types.Txs{})

	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	blockExec := sm.NewBlockExecutor(
		stateStore,
		logger,
		proxyApp,
		mp,
		sm.EmptyEvidencePool{},
		blockStore,
		eventBus,
		sm.NopMetrics(),
	)

	updatesSub, err := eventBus.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
		ClientID: "TestFinalizeBlockValidatorUpdates",
		Query:    types.EventQueryValidatorSetUpdates,
	})
	require.NoError(t, err)

	vals := state.Validators
	proTxHashes := vals.GetProTxHashes()

	addProTxHash := crypto.RandProTxHash()

	fmt.Printf("old: %x, new: %x\n", proTxHashes[0], addProTxHash)

	proTxHashes = append(proTxHashes, addProTxHash)
	newVals, _ := types.GenerateValidatorSet(types.NewValSetParam(proTxHashes))
	var pos int
	for i, proTxHash := range newVals.GetProTxHashes() {
		if bytes.Equal(proTxHash.Bytes(), addProTxHash.Bytes()) {
			pos = i
		}
	}

	// Ensure new validators have some node addresses set
	for _, validator := range newVals.Validators {
		if validator.NodeAddress.Zero() {
			validator.NodeAddress = types.RandValidatorAddress()
		}
	}

	app.ValidatorSetUpdate = newVals.ABCIEquivalentValidatorUpdates()

	// state, err = blockExec.ApplyBlock(ctx, state, blockID, block)

	block, uncommittedState, err := blockExec.CreateProposalBlock(
		ctx,
		1,
		state,
		types.NewCommit(state.LastBlockHeight, 0, state.LastBlockID, state.LastStateID, nil),
		proTxHashes[0],
		1,
	)
	require.NoError(t, err)
	blockID, err := block.BlockID()
	require.NoError(t, err)
	state, err = blockExec.FinalizeBlock(ctx, state, uncommittedState, blockID, block)
	require.NoError(t, err)

	require.Nil(t, err)
	// test new validator was added to NextValidators
	if assert.Equal(t, state.LastValidators.Size()+1, state.Validators.Size()) {
		idx, _ := state.Validators.GetByProTxHash(addProTxHash)
		if idx < 0 {
			t.Fatalf("can't find proTxHash %v in the set %v", addProTxHash, state.Validators)
		}
	}

	// test we threw an event
	ctx, cancel = context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	msg, err := updatesSub.Next(ctx)
	require.NoError(t, err)
	event, ok := msg.Data().(types.EventDataValidatorSetUpdate)
	require.True(t, ok, "Expected event of type EventDataValidatorSetUpdate, got %T", msg.Data())
	assert.Len(t, event.QuorumHash, crypto.QuorumHashSize)
	if assert.NotEmpty(t, event.ValidatorSetUpdates) {
		assert.Equal(t, addProTxHash, event.ValidatorSetUpdates[pos].ProTxHash)
		assert.EqualValues(
			t,
			types.DefaultDashVotingPower,
			event.ValidatorSetUpdates[pos].VotingPower,
		)
	}
}

// TestFinalizeBlockValidatorUpdatesResultingInEmptySet checks that processing validator updates that
// would result in empty set causes no panic, an error is raised and NextValidators is not updated
func TestFinalizeBlockValidatorUpdatesResultingInEmptySet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := &testApp{}
	logger := log.NewNopLogger()
	cc := abciclient.NewLocalClient(logger, app)
	proxyApp := proxy.New(cc, logger, proxy.NopMetrics())
	err := proxyApp.Start(ctx)
	require.NoError(t, err)

	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	state, stateDB, _ := makeState(t, 1, 1)
	nodeProTxHash := state.Validators.Validators[0].ProTxHash
	ctx = dash.ContextWithProTxHash(ctx, nodeProTxHash)
	stateStore := sm.NewStore(stateDB)
	proTxHash := state.Validators.Validators[0].ProTxHash
	blockStore := store.NewBlockStore(dbm.NewMemDB())
	blockExec := sm.NewBlockExecutor(
		stateStore,
		log.NewNopLogger(),
		proxyApp,
		new(mpmocks.Mempool),
		sm.EmptyEvidencePool{},
		blockStore,
		eventBus,
		sm.NopMetrics(),
	)

	block, err := sf.MakeBlock(state, 1, new(types.Commit), 1)
	require.NoError(t, err)
	bps, err := block.MakePartSet(testPartSize)
	require.NoError(t, err)
	blockID := types.BlockID{Hash: block.Hash(), PartSetHeader: bps.Header()}

	publicKey, err := encoding.PubKeyToProto(bls12381.GenPrivKey().PubKey())
	require.NoError(t, err)
	// Remove the only validator
	validatorUpdates := []abci.ValidatorUpdate{
		{ProTxHash: proTxHash, Power: 0},
	}
	// the quorum hash needs to be the same
	// because we are providing an update removing a member from a known quorum, not changing the quorum
	app.ValidatorSetUpdate = &abci.ValidatorSetUpdate{
		ValidatorUpdates:   validatorUpdates,
		ThresholdPublicKey: publicKey,
		QuorumHash:         state.Validators.QuorumHash,
	}

	assert.NotPanics(t, func() {
		state, err = blockExec.ApplyBlock(ctx, state, blockID, block)
	})
	assert.NotNil(t, err)
	assert.NotEmpty(t, state.Validators.Validators)
}

func TestEmptyPrepareProposal(t *testing.T) {
	const height = 2
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	app := abcimocks.NewApplication(t)
	app.On("PrepareProposal", mock.Anything, mock.Anything).Return(&abci.ResponsePrepareProposal{
		AppHash: make([]byte, crypto.DefaultAppHashSize),
	}, nil)
	cc := abciclient.NewLocalClient(logger, app)
	proxyApp := proxy.New(cc, logger, proxy.NopMetrics())
	err := proxyApp.Start(ctx)
	require.NoError(t, err)

	state, stateDB, privVals := makeState(t, 1, height)
	stateStore := sm.NewStore(stateDB)
	mp := &mpmocks.Mempool{}
	mp.On("Lock").Return()
	mp.On("Unlock").Return()
	mp.On("FlushAppConn", mock.Anything).Return(nil)
	mp.On("Update",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(nil)
	mp.On("ReapMaxBytesMaxGas", mock.Anything, mock.Anything).Return(types.Txs{})

	blockExec := sm.NewBlockExecutor(
		stateStore,
		logger,
		proxyApp,
		mp,
		sm.EmptyEvidencePool{},
		nil,
		eventBus,
		sm.NopMetrics(),
	)
	proposerProTxHash, _ := state.Validators.GetByIndex(0)
	commit, _ := makeValidCommit(ctx, t, height, types.BlockID{}, types.StateID{}, state.Validators, privVals)
	_, _, err = blockExec.CreateProposalBlock(ctx, height, state, commit, proposerProTxHash, 0)
	require.NoError(t, err)
}

// TestPrepareProposalErrorOnNonExistingRemoved tests that the block creation logic returns
// an error if the ResponsePrepareProposal returned from the application marks
//  a transaction as REMOVED that was not present in the original proposal.
func TestPrepareProposalErrorOnNonExistingRemoved(t *testing.T) {
	const height = 2
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()
	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	state, stateDB, privVals := makeState(t, 1, height)
	stateStore := sm.NewStore(stateDB)

	evpool := &mocks.EvidencePool{}
	evpool.On("PendingEvidence", mock.Anything).Return([]types.Evidence{}, int64(0))

	mp := &mpmocks.Mempool{}
	mp.On("ReapMaxBytesMaxGas", mock.Anything, mock.Anything).Return(types.Txs{})

	app := abcimocks.NewApplication(t)

	// create an invalid ResponsePrepareProposal
	rpp := &abci.ResponsePrepareProposal{
		TxRecords: []*abci.TxRecord{
			{
				Action: abci.TxRecord_REMOVED,
				Tx:     []byte("new tx"),
			},
		},
		TxResults: []*abci.ExecTxResult{{}},
		AppHash:   make([]byte, crypto.DefaultAppHashSize),
	}
	app.On("PrepareProposal", mock.Anything, mock.Anything).Return(rpp, nil)

	cc := abciclient.NewLocalClient(logger, app)
	proxyApp := proxy.New(cc, logger, proxy.NopMetrics())
	err := proxyApp.Start(ctx)
	require.NoError(t, err)

	blockExec := sm.NewBlockExecutor(
		stateStore,
		logger,
		proxyApp,
		mp,
		evpool,
		nil,
		eventBus,
		sm.NopMetrics(),
	)
	proposerProTxHash, _ := state.Validators.GetByIndex(0)
	commit, _ := makeValidCommit(ctx, t, height, types.BlockID{}, types.StateID{}, state.Validators, privVals)
	block, _, err := blockExec.CreateProposalBlock(ctx, height, state, commit, proposerProTxHash, 0)
	require.ErrorContains(t, err, "new transaction incorrectly marked as removed")
	require.Nil(t, block)

	mp.AssertExpectations(t)
}

// TestPrepareProposalRemoveTxs tests that any transactions marked as REMOVED
// are not included in the block produced by CreateProposalBlock. The test also
// ensures that any transactions removed are also removed from the mempool.
func TestPrepareProposalRemoveTxs(t *testing.T) {
	const height = 2
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()
	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	state, stateDB, privVals := makeState(t, 1, height)
	stateStore := sm.NewStore(stateDB)

	evpool := &mocks.EvidencePool{}
	evpool.On("PendingEvidence", mock.Anything).Return([]types.Evidence{}, int64(0))

	txs := factory.MakeNTxs(height, 10)
	txResults := factory.ExecTxResults(txs)
	mp := &mpmocks.Mempool{}
	mp.On("ReapMaxBytesMaxGas", mock.Anything, mock.Anything).Return(txs)

	trs := txsToTxRecords(txs)
	trs[0].Action = abci.TxRecord_REMOVED
	trs[1].Action = abci.TxRecord_REMOVED
	mp.On("RemoveTxByKey", mock.Anything).Return(nil).Twice()

	app := abcimocks.NewApplication(t)
	app.On("PrepareProposal", mock.Anything, mock.Anything).Return(&abci.ResponsePrepareProposal{
		TxRecords: trs,
		TxResults: txResults,
		AppHash:   make([]byte, crypto.DefaultAppHashSize),
	}, nil)

	cc := abciclient.NewLocalClient(logger, app)
	proxyApp := proxy.New(cc, logger, proxy.NopMetrics())
	err := proxyApp.Start(ctx)
	require.NoError(t, err)

	blockExec := sm.NewBlockExecutor(
		stateStore,
		logger,
		proxyApp,
		mp,
		evpool,
		nil,
		eventBus,
		sm.NopMetrics(),
	)
	proTxHash, _ := state.Validators.GetByIndex(0)
	commit, _ := makeValidCommit(ctx, t, height, types.BlockID{}, types.StateID{}, state.Validators, privVals)
	block, _, err := blockExec.CreateProposalBlock(ctx, height, state, commit, proTxHash, 0)
	require.NoError(t, err)
	require.Len(t, block.Data.Txs.ToSliceOfBytes(), len(trs)-2)

	require.Equal(t, -1, block.Data.Txs.Index(types.Tx(trs[0].Tx)))
	require.Equal(t, -1, block.Data.Txs.Index(types.Tx(trs[1].Tx)))

	mp.AssertCalled(t, "RemoveTxByKey", types.Tx(trs[0].Tx).Key())
	mp.AssertCalled(t, "RemoveTxByKey", types.Tx(trs[1].Tx).Key())
	mp.AssertExpectations(t)
}

// TestPrepareProposalAddedTxsIncluded tests that any transactions marked as ADDED
// in the prepare proposal response are included in the block.
func TestPrepareProposalAddedTxsIncluded(t *testing.T) {
	const height = 2
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()
	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	state, stateDB, privVals := makeState(t, 1, height)
	stateStore := sm.NewStore(stateDB)

	evpool := &mocks.EvidencePool{}
	evpool.On("PendingEvidence", mock.Anything).Return([]types.Evidence{}, int64(0))

	txs := factory.MakeNTxs(height, 10)
	mp := &mpmocks.Mempool{}
	mp.On("ReapMaxBytesMaxGas", mock.Anything, mock.Anything).Return(txs[2:])

	trs := txsToTxRecords(txs)
	trs[0].Action = abci.TxRecord_ADDED
	trs[1].Action = abci.TxRecord_ADDED

	txres := factory.ExecTxResults(txs)

	app := abcimocks.NewApplication(t)
	app.On("PrepareProposal", mock.Anything, mock.Anything).Return(&abci.ResponsePrepareProposal{
		AppHash:   make([]byte, crypto.DefaultAppHashSize),
		TxRecords: trs,
		TxResults: txres,
	}, nil)

	cc := abciclient.NewLocalClient(logger, app)
	proxyApp := proxy.New(cc, logger, proxy.NopMetrics())
	err := proxyApp.Start(ctx)
	require.NoError(t, err)

	blockExec := sm.NewBlockExecutor(
		stateStore,
		logger,
		proxyApp,
		mp,
		evpool,
		nil,
		eventBus,
		sm.NopMetrics(),
	)
	proposerProTxHash, _ := state.Validators.GetByIndex(0)
	commit, _ := makeValidCommit(ctx, t, height, types.BlockID{}, types.StateID{}, state.Validators, privVals)
	block, _, err := blockExec.CreateProposalBlock(ctx, height, state, commit, proposerProTxHash, 0)
	require.NoError(t, err)

	require.Equal(t, txs[0], block.Data.Txs[0])
	require.Equal(t, txs[1], block.Data.Txs[1])

	mp.AssertExpectations(t)
}

// TestPrepareProposalReorderTxs tests that CreateBlock produces a block with transactions
// in the order matching the order they are returned from PrepareProposal.
func TestPrepareProposalReorderTxs(t *testing.T) {
	const height = 2
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()
	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	state, stateDB, privVals := makeState(t, 1, height)
	stateStore := sm.NewStore(stateDB)

	evpool := &mocks.EvidencePool{}
	evpool.On("PendingEvidence", mock.Anything).Return([]types.Evidence{}, int64(0))

	txs := factory.MakeNTxs(height, 10)
	mp := &mpmocks.Mempool{}
	mp.On("ReapMaxBytesMaxGas", mock.Anything, mock.Anything).Return(txs)

	trs := txsToTxRecords(txs)
	trs = trs[2:]
	trs = append(trs[len(trs)/2:], trs[:len(trs)/2]...)

	txresults := factory.ExecTxResults(txs)
	txresults = txresults[2:]
	txresults = append(txresults[len(txresults)/2:], txresults[:len(txresults)/2]...)

	app := abcimocks.NewApplication(t)
	app.On("PrepareProposal", mock.Anything, mock.Anything).Return(&abci.ResponsePrepareProposal{
		AppHash:   make([]byte, crypto.DefaultAppHashSize),
		TxRecords: trs,
		TxResults: txresults,
	}, nil)

	cc := abciclient.NewLocalClient(logger, app)
	proxyApp := proxy.New(cc, logger, proxy.NopMetrics())
	err := proxyApp.Start(ctx)
	require.NoError(t, err)

	blockExec := sm.NewBlockExecutor(
		stateStore,
		logger,
		proxyApp,
		mp,
		evpool,
		nil,
		eventBus,
		sm.NopMetrics(),
	)
	proposerProTxHash, _ := state.Validators.GetByIndex(0)
	commit, _ := makeValidCommit(ctx, t, height, types.BlockID{}, types.StateID{}, state.Validators, privVals)
	block, _, err := blockExec.CreateProposalBlock(ctx, height, state, commit, proposerProTxHash, 0)
	require.NoError(t, err)
	for i, tx := range block.Data.Txs {
		require.Equal(t, types.Tx(trs[i].Tx), tx)
	}

	mp.AssertExpectations(t)

}

// TestPrepareProposalErrorOnTooManyTxs tests that the block creation logic returns
// an error if the ResponsePrepareProposal returned from the application is invalid.
func TestPrepareProposalErrorOnTooManyTxs(t *testing.T) {
	const height = 2
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()
	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	state, stateDB, privVals := makeState(t, 1, height)
	// limit max block size
	state.ConsensusParams.Block.MaxBytes = 60 * 1024
	stateStore := sm.NewStore(stateDB)

	evpool := &mocks.EvidencePool{}
	evpool.On("PendingEvidence", mock.Anything).Return([]types.Evidence{}, int64(0))

	const nValidators = 1
	var bytesPerTx int64 = 3
	maxDataBytes := types.MaxDataBytes(state.ConsensusParams.Block.MaxBytes, crypto.BLS12381, 0, nValidators)
	txs := factory.MakeNTxs(height, maxDataBytes/bytesPerTx+2) // +2 so that tx don't fit
	mp := &mpmocks.Mempool{}
	mp.On("ReapMaxBytesMaxGas", mock.Anything, mock.Anything).Return(txs)

	trs := txsToTxRecords(txs)

	app := abcimocks.NewApplication(t)
	app.On("PrepareProposal", mock.Anything, mock.Anything).Return(&abci.ResponsePrepareProposal{
		TxRecords: trs,
		TxResults: factory.ExecTxResults(txs),
		AppHash:   make([]byte, crypto.DefaultAppHashSize),
	}, nil)

	cc := abciclient.NewLocalClient(logger, app)
	proxyApp := proxy.New(cc, logger, proxy.NopMetrics())
	err := proxyApp.Start(ctx)
	require.NoError(t, err)

	blockExec := sm.NewBlockExecutor(
		stateStore,
		logger,
		proxyApp,
		mp,
		evpool,
		nil,
		eventBus,
		sm.NopMetrics(),
	)
	proposerProTxHash, _ := state.Validators.GetByIndex(0)
	commit, _ := makeValidCommit(ctx, t, height, types.BlockID{}, types.StateID{}, state.Validators, privVals)

	block, _, err := blockExec.CreateProposalBlock(ctx, height, state, commit, proposerProTxHash, 0)
	require.ErrorContains(t, err, "transaction data size exceeds maximum")
	require.Nil(t, block, "")

	mp.AssertExpectations(t)
}

// TestPrepareProposalErrorOnPrepareProposalError tests when the client returns an error
// upon calling PrepareProposal on it.
func TestPrepareProposalErrorOnPrepareProposalError(t *testing.T) {
	const height = 2
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()
	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	state, stateDB, privVals := makeState(t, 1, height)
	stateStore := sm.NewStore(stateDB)

	evpool := &mocks.EvidencePool{}
	evpool.On("PendingEvidence", mock.Anything).Return([]types.Evidence{}, int64(0))

	txs := factory.MakeNTxs(height, 10)
	mp := &mpmocks.Mempool{}
	mp.On("ReapMaxBytesMaxGas", mock.Anything, mock.Anything).Return(txs)

	cm := &abciclientmocks.Client{}
	cm.On("IsRunning").Return(true)
	cm.On("Error").Return(nil)
	cm.On("Start", mock.Anything).Return(nil).Once()
	cm.On("Wait").Return(nil).Once()
	cm.On("PrepareProposal", mock.Anything, mock.Anything).Return(nil, errors.New("an injected error")).Once()

	proxyApp := proxy.New(cm, logger, proxy.NopMetrics())
	err := proxyApp.Start(ctx)
	require.NoError(t, err)

	blockExec := sm.NewBlockExecutor(
		stateStore,
		logger,
		proxyApp,
		mp,
		evpool,
		nil,
		eventBus,
		sm.NopMetrics(),
	)
	proTxHash, _ := state.Validators.GetByIndex(0)
	commit, _ := makeValidCommit(ctx, t, height, types.BlockID{}, types.StateID{}, state.Validators, privVals)

	block, _, err := blockExec.CreateProposalBlock(ctx, height, state, commit, proTxHash, 0)
	require.Nil(t, block)
	require.ErrorContains(t, err, "an injected error")

	mp.AssertExpectations(t)
}

func txsToTxRecords(txs []types.Tx) []*abci.TxRecord {
	trs := make([]*abci.TxRecord, len(txs))
	for i, tx := range txs {
		trs[i] = &abci.TxRecord{
			Action: abci.TxRecord_UNMODIFIED,
			Tx:     tx,
		}
	}
	return trs
}
