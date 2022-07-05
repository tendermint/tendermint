package state_test

import (
	"context"
	"errors"
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
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/encoding"
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
	"github.com/tendermint/tendermint/version"
)

var (
	chainID             = "execution_chain"
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
	blockExec := sm.NewBlockExecutor(stateStore, logger, proxyApp, mp, sm.EmptyEvidencePool{}, blockStore, eventBus, sm.NopMetrics())

	block := sf.MakeBlock(state, 1, new(types.Commit))
	bps, err := block.MakePartSet(testPartSize)
	require.NoError(t, err)
	blockID := types.BlockID{Hash: block.Hash(), PartSetHeader: bps.Header()}

	state, err = blockExec.ApplyBlock(ctx, state, blockID, block)
	require.NoError(t, err)

	// TODO check state and mempool
	assert.EqualValues(t, 1, state.Version.Consensus.App, "App version wasn't updated")
}

// TestFinalizeBlockDecidedLastCommit ensures we correctly send the
// DecidedLastCommit to the application. The test ensures that the
// DecidedLastCommit properly reflects which validators signed the preceding
// block.
func TestFinalizeBlockDecidedLastCommit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()
	app := &testApp{}
	cc := abciclient.NewLocalClient(logger, app)
	appClient := proxy.New(cc, logger, proxy.NopMetrics())

	err := appClient.Start(ctx)
	require.NoError(t, err)

	state, stateDB, privVals := makeState(t, 7, 1)
	stateStore := sm.NewStore(stateDB)
	absentSig := types.NewExtendedCommitSigAbsent()

	testCases := []struct {
		name             string
		absentCommitSigs map[int]bool
	}{
		{"none absent", map[int]bool{}},
		{"one absent", map[int]bool{1: true}},
		{"multiple absent", map[int]bool{1: true, 3: true}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			blockStore := store.NewBlockStore(dbm.NewMemDB())
			evpool := &mocks.EvidencePool{}
			evpool.On("PendingEvidence", mock.Anything).Return([]types.Evidence{}, 0)
			evpool.On("Update", ctx, mock.Anything, mock.Anything).Return()
			evpool.On("CheckEvidence", ctx, mock.Anything).Return(nil)
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

			blockExec := sm.NewBlockExecutor(stateStore, log.NewNopLogger(), appClient, mp, evpool, blockStore, eventBus, sm.NopMetrics())
			state, _, lastCommit := makeAndCommitGoodBlock(ctx, t, state, 1, new(types.Commit), state.NextValidators.Validators[0].Address, blockExec, privVals, nil)

			for idx, isAbsent := range tc.absentCommitSigs {
				if isAbsent {
					lastCommit.ExtendedSignatures[idx] = absentSig
				}
			}

			// block for height 2
			block := sf.MakeBlock(state, 2, lastCommit.ToCommit())
			bps, err := block.MakePartSet(testPartSize)
			require.NoError(t, err)
			blockID := types.BlockID{Hash: block.Hash(), PartSetHeader: bps.Header()}
			_, err = blockExec.ApplyBlock(ctx, state, blockID, block)
			require.NoError(t, err)

			// -> app receives a list of validators with a bool indicating if they signed
			for i, v := range app.CommitVotes {
				_, absent := tc.absentCommitSigs[i]
				assert.Equal(t, !absent, v.SignedLastBlock)
			}
		})
	}
}

// TestFinalizeBlockMisbehavior ensures we send misbehavior list.
func TestFinalizeBlockMisbehavior(t *testing.T) {
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

	defaultEvidenceTime := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	privVal := privVals[state.Validators.Validators[0].Address.String()]
	blockID := makeBlockID([]byte("headerhash"), 1000, []byte("partshash"))
	header := &types.Header{
		Version:            version.Consensus{Block: version.BlockProtocol, App: 1},
		ChainID:            state.ChainID,
		Height:             10,
		Time:               defaultEvidenceTime,
		LastBlockID:        blockID,
		LastCommitHash:     crypto.CRandBytes(crypto.HashSize),
		DataHash:           crypto.CRandBytes(crypto.HashSize),
		ValidatorsHash:     state.Validators.Hash(),
		NextValidatorsHash: state.Validators.Hash(),
		ConsensusHash:      crypto.CRandBytes(crypto.HashSize),
		AppHash:            crypto.CRandBytes(crypto.HashSize),
		LastResultsHash:    crypto.CRandBytes(crypto.HashSize),
		EvidenceHash:       crypto.CRandBytes(crypto.HashSize),
		ProposerAddress:    crypto.CRandBytes(crypto.AddressSize),
	}

	// we don't need to worry about validating the evidence as long as they pass validate basic
	dve, err := types.NewMockDuplicateVoteEvidenceWithValidator(ctx, 3, defaultEvidenceTime, privVal, state.ChainID)
	require.NoError(t, err)
	dve.ValidatorPower = 1000
	lcae := &types.LightClientAttackEvidence{
		ConflictingBlock: &types.LightBlock{
			SignedHeader: &types.SignedHeader{
				Header: header,
				Commit: &types.Commit{
					Height:  10,
					BlockID: makeBlockID(header.Hash(), 100, []byte("partshash")),
					Signatures: []types.CommitSig{{
						BlockIDFlag:      types.BlockIDFlagNil,
						ValidatorAddress: crypto.AddressHash([]byte("validator_address")),
						Timestamp:        defaultEvidenceTime,
						Signature:        crypto.CRandBytes(types.MaxSignatureSize)}},
				},
			},
			ValidatorSet: state.Validators,
		},
		CommonHeight:        8,
		ByzantineValidators: []*types.Validator{state.Validators.Validators[0]},
		TotalVotingPower:    12,
		Timestamp:           defaultEvidenceTime,
	}

	ev := []types.Evidence{dve, lcae}

	abciMb := []abci.Misbehavior{
		{
			Type:             abci.MisbehaviorType_DUPLICATE_VOTE,
			Height:           3,
			Time:             defaultEvidenceTime,
			Validator:        types.TM2PB.Validator(state.Validators.Validators[0]),
			TotalVotingPower: 10,
		},
		{
			Type:             abci.MisbehaviorType_LIGHT_CLIENT_ATTACK,
			Height:           8,
			Time:             defaultEvidenceTime,
			Validator:        types.TM2PB.Validator(state.Validators.Validators[0]),
			TotalVotingPower: 12,
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

	block := sf.MakeBlock(state, 1, new(types.Commit))
	block.Evidence = ev
	block.Header.EvidenceHash = block.Evidence.Hash()
	bps, err := block.MakePartSet(testPartSize)
	require.NoError(t, err)

	blockID = types.BlockID{Hash: block.Hash(), PartSetHeader: bps.Header()}

	_, err = blockExec.ApplyBlock(ctx, state, blockID, block)
	require.NoError(t, err)

	// TODO check state and mempool
	assert.Equal(t, abciMb, app.Misbehavior)
}

func TestProcessProposal(t *testing.T) {
	const height = 2
	txs := factory.MakeNTxs(height, 10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := abcimocks.NewApplication(t)
	logger := log.NewNopLogger()
	cc := abciclient.NewLocalClient(logger, app)
	proxyApp := proxy.New(cc, logger, proxy.NopMetrics())
	err := proxyApp.Start(ctx)
	require.NoError(t, err)

	state, stateDB, privVals := makeState(t, 1, height)
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

	block0 := sf.MakeBlock(state, height-1, new(types.Commit))
	lastCommitSig := []types.CommitSig{}
	partSet, err := block0.MakePartSet(types.BlockPartSizeBytes)
	require.NoError(t, err)
	blockID := types.BlockID{Hash: block0.Hash(), PartSetHeader: partSet.Header()}
	voteInfos := []abci.VoteInfo{}
	for _, privVal := range privVals {
		vote, err := factory.MakeVote(ctx, privVal, block0.Header.ChainID, 0, 0, 0, 2, blockID, time.Now())
		require.NoError(t, err)
		pk, err := privVal.GetPubKey(ctx)
		require.NoError(t, err)
		addr := pk.Address()
		voteInfos = append(voteInfos,
			abci.VoteInfo{
				SignedLastBlock: true,
				Validator: abci.Validator{
					Address: addr,
					Power:   1000,
				},
			})
		lastCommitSig = append(lastCommitSig, vote.CommitSig())
	}

	block1 := sf.MakeBlock(state, height, &types.Commit{
		Height:     height - 1,
		Signatures: lastCommitSig,
	})
	block1.Txs = txs

	expectedRpp := &abci.RequestProcessProposal{
		Txs:         block1.Txs.ToSliceOfBytes(),
		Hash:        block1.Hash(),
		Height:      block1.Header.Height,
		Time:        block1.Header.Time,
		Misbehavior: block1.Evidence.ToABCI(),
		ProposedLastCommit: abci.CommitInfo{
			Round: 0,
			Votes: voteInfos,
		},
		NextValidatorsHash: block1.NextValidatorsHash,
		ProposerAddress:    block1.ProposerAddress,
	}

	app.On("ProcessProposal", mock.Anything, mock.Anything).Return(&abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}, nil)
	acceptBlock, err := blockExec.ProcessProposal(ctx, block1, state)
	require.NoError(t, err)
	require.True(t, acceptBlock)
	app.AssertExpectations(t)
	app.AssertCalled(t, "ProcessProposal", ctx, expectedRpp)
}

func TestValidateValidatorUpdates(t *testing.T) {
	pubkey1 := ed25519.GenPrivKey().PubKey()
	pubkey2 := ed25519.GenPrivKey().PubKey()
	pk1, err := encoding.PubKeyToProto(pubkey1)
	assert.NoError(t, err)
	pk2, err := encoding.PubKeyToProto(pubkey2)
	assert.NoError(t, err)

	defaultValidatorParams := types.ValidatorParams{PubKeyTypes: []string{types.ABCIPubKeyTypeEd25519}}

	testCases := []struct {
		name string

		abciUpdates     []abci.ValidatorUpdate
		validatorParams types.ValidatorParams

		shouldErr bool
	}{
		{
			"adding a validator is OK",
			[]abci.ValidatorUpdate{{PubKey: pk2, Power: 20}},
			defaultValidatorParams,
			false,
		},
		{
			"updating a validator is OK",
			[]abci.ValidatorUpdate{{PubKey: pk1, Power: 20}},
			defaultValidatorParams,
			false,
		},
		{
			"removing a validator is OK",
			[]abci.ValidatorUpdate{{PubKey: pk2, Power: 0}},
			defaultValidatorParams,
			false,
		},
		{
			"adding a validator with negative power results in error",
			[]abci.ValidatorUpdate{{PubKey: pk2, Power: -100}},
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
	pubkey1 := ed25519.GenPrivKey().PubKey()
	val1 := types.NewValidator(pubkey1, 10)
	pubkey2 := ed25519.GenPrivKey().PubKey()
	val2 := types.NewValidator(pubkey2, 20)

	pk, err := encoding.PubKeyToProto(pubkey1)
	require.NoError(t, err)
	pk2, err := encoding.PubKeyToProto(pubkey2)
	require.NoError(t, err)

	testCases := []struct {
		name string

		currentSet  *types.ValidatorSet
		abciUpdates []abci.ValidatorUpdate

		resultingSet *types.ValidatorSet
		shouldErr    bool
	}{
		{
			"adding a validator is OK",
			types.NewValidatorSet([]*types.Validator{val1}),
			[]abci.ValidatorUpdate{{PubKey: pk2, Power: 20}},
			types.NewValidatorSet([]*types.Validator{val1, val2}),
			false,
		},
		{
			"updating a validator is OK",
			types.NewValidatorSet([]*types.Validator{val1}),
			[]abci.ValidatorUpdate{{PubKey: pk, Power: 20}},
			types.NewValidatorSet([]*types.Validator{types.NewValidator(pubkey1, 20)}),
			false,
		},
		{
			"removing a validator is OK",
			types.NewValidatorSet([]*types.Validator{val1, val2}),
			[]abci.ValidatorUpdate{{PubKey: pk2, Power: 0}},
			types.NewValidatorSet([]*types.Validator{val1}),
			false,
		},
		{
			"removing a non-existing validator results in error",
			types.NewValidatorSet([]*types.Validator{val1}),
			[]abci.ValidatorUpdate{{PubKey: pk2, Power: 0}},
			types.NewValidatorSet([]*types.Validator{val1}),
			true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			updates, err := types.PB2TM.ValidatorUpdates(tc.abciUpdates)
			assert.NoError(t, err)
			err = tc.currentSet.UpdateWithChangeSet(updates)
			if tc.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				require.Equal(t, tc.resultingSet.Size(), tc.currentSet.Size())

				assert.Equal(t, tc.resultingSet.TotalVotingPower(), tc.currentSet.TotalVotingPower())

				assert.Equal(t, tc.resultingSet.Validators[0].Address, tc.currentSet.Validators[0].Address)
				if tc.resultingSet.Size() > 1 {
					assert.Equal(t, tc.resultingSet.Validators[1].Address, tc.currentSet.Validators[1].Address)
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

	block := sf.MakeBlock(state, 1, new(types.Commit))
	bps, err := block.MakePartSet(testPartSize)
	require.NoError(t, err)
	blockID := types.BlockID{Hash: block.Hash(), PartSetHeader: bps.Header()}

	pubkey := ed25519.GenPrivKey().PubKey()
	pk, err := encoding.PubKeyToProto(pubkey)
	require.NoError(t, err)
	app.ValidatorUpdates = []abci.ValidatorUpdate{
		{PubKey: pk, Power: 10},
	}

	state, err = blockExec.ApplyBlock(ctx, state, blockID, block)
	require.NoError(t, err)
	// test new validator was added to NextValidators
	if assert.Equal(t, state.Validators.Size()+1, state.NextValidators.Size()) {
		idx, _ := state.NextValidators.GetByAddress(pubkey.Address())
		if idx < 0 {
			t.Fatalf("can't find address %v in the set %v", pubkey.Address(), state.NextValidators)
		}
	}

	// test we threw an event
	ctx, cancel = context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	msg, err := updatesSub.Next(ctx)
	require.NoError(t, err)
	event, ok := msg.Data().(types.EventDataValidatorSetUpdates)
	require.True(t, ok, "Expected event of type EventDataValidatorSetUpdates, got %T", msg.Data())
	if assert.NotEmpty(t, event.ValidatorUpdates) {
		assert.Equal(t, pubkey, event.ValidatorUpdates[0].PubKey)
		assert.EqualValues(t, 10, event.ValidatorUpdates[0].VotingPower)
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
	stateStore := sm.NewStore(stateDB)
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

	block := sf.MakeBlock(state, 1, new(types.Commit))
	bps, err := block.MakePartSet(testPartSize)
	require.NoError(t, err)
	blockID := types.BlockID{Hash: block.Hash(), PartSetHeader: bps.Header()}

	vp, err := encoding.PubKeyToProto(state.Validators.Validators[0].PubKey)
	require.NoError(t, err)
	// Remove the only validator
	app.ValidatorUpdates = []abci.ValidatorUpdate{
		{PubKey: vp, Power: 0},
	}

	assert.NotPanics(t, func() { state, err = blockExec.ApplyBlock(ctx, state, blockID, block) })
	assert.Error(t, err)
	assert.NotEmpty(t, state.NextValidators.Validators)
}

func TestEmptyPrepareProposal(t *testing.T) {
	const height = 2
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := log.NewNopLogger()

	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	app := abcimocks.NewApplication(t)
	app.On("PrepareProposal", mock.Anything, mock.Anything).Return(&abci.ResponsePrepareProposal{}, nil)
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
	pa, _ := state.Validators.GetByIndex(0)
	commit, _ := makeValidCommit(ctx, t, height, types.BlockID{}, state.Validators, privVals)
	_, err = blockExec.CreateProposalBlock(ctx, height, state, commit, pa)
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
	pa, _ := state.Validators.GetByIndex(0)
	commit, _ := makeValidCommit(ctx, t, height, types.BlockID{}, state.Validators, privVals)
	block, err := blockExec.CreateProposalBlock(ctx, height, state, commit, pa)
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
	mp := &mpmocks.Mempool{}
	mp.On("ReapMaxBytesMaxGas", mock.Anything, mock.Anything).Return(types.Txs(txs))

	trs := txsToTxRecords(types.Txs(txs))
	trs[0].Action = abci.TxRecord_REMOVED
	trs[1].Action = abci.TxRecord_REMOVED
	mp.On("RemoveTxByKey", mock.Anything).Return(nil).Twice()

	app := abcimocks.NewApplication(t)
	app.On("PrepareProposal", mock.Anything, mock.Anything).Return(&abci.ResponsePrepareProposal{
		TxRecords: trs,
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
	pa, _ := state.Validators.GetByIndex(0)
	commit, _ := makeValidCommit(ctx, t, height, types.BlockID{}, state.Validators, privVals)
	block, err := blockExec.CreateProposalBlock(ctx, height, state, commit, pa)
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
	mp.On("ReapMaxBytesMaxGas", mock.Anything, mock.Anything).Return(types.Txs(txs[2:]))

	trs := txsToTxRecords(types.Txs(txs))
	trs[0].Action = abci.TxRecord_ADDED
	trs[1].Action = abci.TxRecord_ADDED

	app := abcimocks.NewApplication(t)
	app.On("PrepareProposal", mock.Anything, mock.Anything).Return(&abci.ResponsePrepareProposal{
		TxRecords: trs,
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
	pa, _ := state.Validators.GetByIndex(0)
	commit, _ := makeValidCommit(ctx, t, height, types.BlockID{}, state.Validators, privVals)
	block, err := blockExec.CreateProposalBlock(ctx, height, state, commit, pa)
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
	mp.On("ReapMaxBytesMaxGas", mock.Anything, mock.Anything).Return(types.Txs(txs))

	trs := txsToTxRecords(types.Txs(txs))
	trs = trs[2:]
	trs = append(trs[len(trs)/2:], trs[:len(trs)/2]...)

	app := abcimocks.NewApplication(t)
	app.On("PrepareProposal", mock.Anything, mock.Anything).Return(&abci.ResponsePrepareProposal{
		TxRecords: trs,
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
	pa, _ := state.Validators.GetByIndex(0)
	commit, _ := makeValidCommit(ctx, t, height, types.BlockID{}, state.Validators, privVals)
	block, err := blockExec.CreateProposalBlock(ctx, height, state, commit, pa)
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
	maxDataBytes := types.MaxDataBytes(state.ConsensusParams.Block.MaxBytes, 0, nValidators)
	txs := factory.MakeNTxs(height, maxDataBytes/bytesPerTx+2) // +2 so that tx don't fit
	mp := &mpmocks.Mempool{}
	mp.On("ReapMaxBytesMaxGas", mock.Anything, mock.Anything).Return(types.Txs(txs))

	trs := txsToTxRecords(types.Txs(txs))

	app := abcimocks.NewApplication(t)
	app.On("PrepareProposal", mock.Anything, mock.Anything).Return(&abci.ResponsePrepareProposal{
		TxRecords: trs,
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
	pa, _ := state.Validators.GetByIndex(0)
	commit, _ := makeValidCommit(ctx, t, height, types.BlockID{}, state.Validators, privVals)
	block, err := blockExec.CreateProposalBlock(ctx, height, state, commit, pa)
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
	mp.On("ReapMaxBytesMaxGas", mock.Anything, mock.Anything).Return(types.Txs(txs))

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
	pa, _ := state.Validators.GetByIndex(0)
	commit, _ := makeValidCommit(ctx, t, height, types.BlockID{}, state.Validators, privVals)
	block, err := blockExec.CreateProposalBlock(ctx, height, state, commit, pa)
	require.Nil(t, block)
	require.ErrorContains(t, err, "an injected error")

	mp.AssertExpectations(t)
}

// TestCreateProposalBlockPanicOnAbsentVoteExtensions ensures that the CreateProposalBlock
// call correctly panics when the vote extension data is missing from the extended commit
// data that the method receives.
func TestCreateProposalAbsentVoteExtensions(t *testing.T) {
	for _, testCase := range []struct {
		name string

		// The height that is about to be proposed
		height int64

		// The first height during which vote extensions will be required for consensus to proceed.
		extensionEnableHeight int64
		expectPanic           bool
	}{
		{
			name:                  "missing extension data on first required height",
			height:                2,
			extensionEnableHeight: 1,
			expectPanic:           true,
		},
		{
			name:                  "missing extension during before required height",
			height:                2,
			extensionEnableHeight: 2,
			expectPanic:           false,
		},
		{
			name:                  "missing extension data and not required",
			height:                2,
			extensionEnableHeight: 0,
			expectPanic:           false,
		},
		{
			name:                  "missing extension data and required in two heights",
			height:                2,
			extensionEnableHeight: 3,
			expectPanic:           false,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			logger := log.NewNopLogger()

			eventBus := eventbus.NewDefault(logger)
			require.NoError(t, eventBus.Start(ctx))

			app := abcimocks.NewApplication(t)
			if !testCase.expectPanic {
				app.On("PrepareProposal", mock.Anything, mock.Anything).Return(&abci.ResponsePrepareProposal{}, nil)
			}
			cc := abciclient.NewLocalClient(logger, app)
			proxyApp := proxy.New(cc, logger, proxy.NopMetrics())
			err := proxyApp.Start(ctx)
			require.NoError(t, err)

			state, stateDB, privVals := makeState(t, 1, int(testCase.height-1))
			stateStore := sm.NewStore(stateDB)
			state.ConsensusParams.ABCI.VoteExtensionsEnableHeight = testCase.extensionEnableHeight
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
			block := sf.MakeBlock(state, testCase.height, new(types.Commit))
			bps, err := block.MakePartSet(testPartSize)
			require.NoError(t, err)
			blockID := types.BlockID{Hash: block.Hash(), PartSetHeader: bps.Header()}
			pa, _ := state.Validators.GetByIndex(0)
			lastCommit, _ := makeValidCommit(ctx, t, testCase.height-1, blockID, state.Validators, privVals)
			stripSignatures(lastCommit)
			if testCase.expectPanic {
				require.Panics(t, func() {
					blockExec.CreateProposalBlock(ctx, testCase.height, state, lastCommit, pa) //nolint:errcheck
				})
			} else {
				_, err = blockExec.CreateProposalBlock(ctx, testCase.height, state, lastCommit, pa)
				require.NoError(t, err)
			}
		})
	}
}

func stripSignatures(ec *types.ExtendedCommit) {
	for i, commitSig := range ec.ExtendedSignatures {
		commitSig.Extension = nil
		commitSig.ExtensionSignature = nil
		ec.ExtendedSignatures[i] = commitSig
	}
}

func makeBlockID(hash []byte, partSetSize uint32, partSetHash []byte) types.BlockID {
	var (
		h   = make([]byte, crypto.HashSize)
		psH = make([]byte, crypto.HashSize)
	)
	copy(h, hash)
	copy(psH, partSetHash)
	return types.BlockID{
		Hash: h,
		PartSetHeader: types.PartSetHeader{
			Total: partSetSize,
			Hash:  psH,
		},
	}
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
