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

	abciclientmocks "github.com/tendermint/tendermint/abci/client/mocks"
	abci "github.com/tendermint/tendermint/abci/types"
	abcimocks "github.com/tendermint/tendermint/abci/types/mocks"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/internal/test"
	"github.com/tendermint/tendermint/libs/log"
	mpmocks "github.com/tendermint/tendermint/mempool/mocks"
	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
	"github.com/tendermint/tendermint/proxy"
	pmocks "github.com/tendermint/tendermint/proxy/mocks"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/state/mocks"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
	"github.com/tendermint/tendermint/version"
)

var (
	chainID             = "execution_chain"
	testPartSize uint32 = 65536
)

func TestApplyBlock(t *testing.T) {
	app := &testApp{}
	cc := proxy.NewLocalClientCreator(app)
	proxyApp := proxy.NewAppConns(cc, proxy.NopMetrics())
	err := proxyApp.Start()
	require.Nil(t, err)
	defer proxyApp.Stop() //nolint:errcheck // ignore for tests

	state, stateDB, _ := makeState(1, 1)
	stateStore := sm.NewStore(stateDB, sm.StoreOptions{
		DiscardABCIResponses: false,
	})
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
		mock.Anything).Return(nil)
	blockExec := sm.NewBlockExecutor(stateStore, log.TestingLogger(), proxyApp.Consensus(),
		mp, sm.EmptyEvidencePool{}, blockStore)

	block := makeBlock(state, 1, new(types.Commit))
	bps, err := block.MakePartSet(testPartSize)
	require.NoError(t, err)
	blockID := types.BlockID{Hash: block.Hash(), PartSetHeader: bps.Header()}

	state, err = blockExec.ApplyBlock(state, blockID, block)
	require.Nil(t, err)

	// TODO check state and mempool
	assert.EqualValues(t, 1, state.Version.Consensus.App, "App version wasn't updated")
}

// TestFinalizeBlockDecidedLastCommit ensures we correctly send the
// DecidedLastCommit to the application. The test ensures that the
// DecidedLastCommit properly reflects which validators signed the preceding
// block.
func TestFinalizeBlockDecidedLastCommit(t *testing.T) {
	app := &testApp{}
	cc := proxy.NewLocalClientCreator(app)
	proxyApp := proxy.NewAppConns(cc, proxy.NopMetrics())
	err := proxyApp.Start()
	require.NoError(t, err)
	defer proxyApp.Stop() //nolint:errcheck // ignore for tests

	state, stateDB, privVals := makeState(7, 1)
	stateStore := sm.NewStore(stateDB, sm.StoreOptions{
		DiscardABCIResponses: false,
	})
	absentSig := types.NewCommitSigAbsent()

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
			evpool.On("Update", mock.Anything, mock.Anything).Return()
			evpool.On("CheckEvidence", mock.Anything).Return(nil)
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

			eventBus := types.NewEventBus()
			require.NoError(t, eventBus.Start())

			blockExec := sm.NewBlockExecutor(stateStore, log.NewNopLogger(), proxyApp.Consensus(), mp, evpool, blockStore)
			state, _, lastCommit, err := makeAndCommitGoodBlock(state, 1, new(types.Commit), state.NextValidators.Validators[0].Address, blockExec, privVals, nil)
			require.NoError(t, err)

			for idx, isAbsent := range tc.absentCommitSigs {
				if isAbsent {
					lastCommit.Signatures[idx] = absentSig
				}
			}

			// block for height 2
			block := makeBlock(state, 2, lastCommit)
			bps, err := block.MakePartSet(testPartSize)
			require.NoError(t, err)
			blockID := types.BlockID{Hash: block.Hash(), PartSetHeader: bps.Header()}
			_, err = blockExec.ApplyBlock(state, blockID, block)
			require.NoError(t, err)

			// -> app receives a list of validators with a bool indicating if they signed
			for i, v := range app.CommitVotes {
				_, absent := tc.absentCommitSigs[i]
				assert.Equal(t, !absent, v.SignedLastBlock)
			}
		})
	}
}

// TestFinalizeBlockValidators ensures we send absent validators list.
func TestFinalizeBlockValidators(t *testing.T) {
	app := &testApp{}
	cc := proxy.NewLocalClientCreator(app)
	proxyApp := proxy.NewAppConns(cc, proxy.NopMetrics())
	err := proxyApp.Start()
	require.NoError(t, err)
	defer proxyApp.Stop() //nolint:errcheck // no need to check error again

	state, stateDB, _ := makeState(2, 2)
	stateStore := sm.NewStore(stateDB, sm.StoreOptions{
		DiscardABCIResponses: false,
	})

	prevHash := state.LastBlockID.Hash
	prevParts := types.PartSetHeader{}
	prevBlockID := types.BlockID{Hash: prevHash, PartSetHeader: prevParts}

	var (
		now        = tmtime.Now()
		commitSig0 = types.NewCommitSigForBlock(
			[]byte("Signature1"),
			state.Validators.Validators[0].Address,
			now,
		)
		commitSig1 = types.NewCommitSigForBlock(
			[]byte("Signature2"),
			state.Validators.Validators[1].Address,
			now,
		)
		absentSig = types.NewCommitSigAbsent()
	)

	testCases := []struct {
		desc                     string
		lastCommitSigs           []types.CommitSig
		expectedAbsentValidators []int
	}{
		{"none absent", []types.CommitSig{commitSig0, commitSig1}, []int{}},
		{"one absent", []types.CommitSig{commitSig0, absentSig}, []int{1}},
		{"multiple absent", []types.CommitSig{absentSig, absentSig}, []int{0, 1}},
	}

	for _, tc := range testCases {
		lastCommit := types.NewExtendedCommit(1, 0, prevBlockID, tc.lastCommitSigs)

		// block for height 2
		block := makeBlock(state, 2, lastCommit.StripExtensions())

		_, err = sm.ExecCommitBlock(proxyApp.Consensus(), block, log.TestingLogger(), stateStore, 1)
		require.Nil(t, err, tc.desc)

		// -> app receives a list of validators with a bool indicating if they signed
		ctr := 0
		for i, v := range app.CommitVotes {
			if ctr < len(tc.expectedAbsentValidators) &&
				tc.expectedAbsentValidators[ctr] == i {

				assert.False(t, v.SignedLastBlock)
				ctr++
			} else {
				assert.True(t, v.SignedLastBlock)
			}
		}
	}
}

// TestFinalizeBlockMisbehavior ensures we send misbehavior list.
func TestFinalizeBlockMisbehavior(t *testing.T) {
	app := &testApp{}
	cc := proxy.NewLocalClientCreator(app)
	proxyApp := proxy.NewAppConns(cc, proxy.NopMetrics())
	err := proxyApp.Start()
	require.NoError(t, err)
	defer proxyApp.Stop() //nolint:errcheck // ignore for tests

	state, stateDB, privVals := makeState(1, 1)
	stateStore := sm.NewStore(stateDB, sm.StoreOptions{
		DiscardABCIResponses: false,
	})

	defaultEvidenceTime := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	privVal := privVals[state.Validators.Validators[0].Address.String()]
	blockID := makeBlockID([]byte("headerhash"), 1000, []byte("partshash"))
	header := &types.Header{
		Version:            tmversion.Consensus{Block: version.BlockProtocol, App: 1},
		ChainID:            state.ChainID,
		Height:             10,
		Time:               defaultEvidenceTime,
		LastBlockID:        blockID,
		LastCommitHash:     crypto.CRandBytes(tmhash.Size),
		DataHash:           crypto.CRandBytes(tmhash.Size),
		ValidatorsHash:     state.Validators.Hash(),
		NextValidatorsHash: state.Validators.Hash(),
		ConsensusHash:      crypto.CRandBytes(tmhash.Size),
		AppHash:            crypto.CRandBytes(tmhash.Size),
		LastResultsHash:    crypto.CRandBytes(tmhash.Size),
		EvidenceHash:       crypto.CRandBytes(tmhash.Size),
		ProposerAddress:    crypto.CRandBytes(crypto.AddressSize),
	}

	// we don't need to worry about validating the evidence as long as they pass validate basic
	dve, err := types.NewMockDuplicateVoteEvidenceWithValidator(3, defaultEvidenceTime, privVal, state.ChainID)
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
	evpool.On("Update", mock.AnythingOfType("state.State"), mock.AnythingOfType("types.EvidenceList")).Return()
	evpool.On("CheckEvidence", mock.AnythingOfType("types.EvidenceList")).Return(nil)
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

	blockStore := store.NewBlockStore(dbm.NewMemDB())

	blockExec := sm.NewBlockExecutor(stateStore, log.TestingLogger(), proxyApp.Consensus(),
		mp, evpool, blockStore)

	block := makeBlock(state, 1, new(types.Commit))
	block.Evidence = types.EvidenceData{Evidence: ev}
	block.Header.EvidenceHash = block.Evidence.Hash()
	bps, err := block.MakePartSet(testPartSize)
	require.NoError(t, err)

	blockID = types.BlockID{Hash: block.Hash(), PartSetHeader: bps.Header()}

	_, err = blockExec.ApplyBlock(state, blockID, block)
	require.NoError(t, err)

	// TODO check state and mempool
	assert.Equal(t, abciMb, app.Misbehavior)
}

func TestProcessProposal(t *testing.T) {
	const height = 2
	txs := test.MakeNTxs(height, 10)

	logger := log.NewNopLogger()
	app := &abcimocks.Application{}
	app.On("ProcessProposal", mock.Anything, mock.Anything).Return(&abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_ACCEPT}, nil)

	cc := proxy.NewLocalClientCreator(app)
	proxyApp := proxy.NewAppConns(cc, proxy.NopMetrics())
	err := proxyApp.Start()
	require.NoError(t, err)
	defer proxyApp.Stop() //nolint:errcheck // ignore for tests

	state, stateDB, privVals := makeState(1, height)
	stateStore := sm.NewStore(stateDB, sm.StoreOptions{
		DiscardABCIResponses: false,
	})
	blockStore := store.NewBlockStore(dbm.NewMemDB())
	eventBus := types.NewEventBus()
	err = eventBus.Start()
	require.NoError(t, err)

	blockExec := sm.NewBlockExecutor(
		stateStore,
		logger,
		proxyApp.Consensus(),
		new(mpmocks.Mempool),
		sm.EmptyEvidencePool{},
		blockStore,
	)

	block0 := makeBlock(state, height-1, new(types.Commit))
	lastCommitSig := []types.CommitSig{}
	partSet, err := block0.MakePartSet(types.BlockPartSizeBytes)
	require.NoError(t, err)
	blockID := types.BlockID{Hash: block0.Hash(), PartSetHeader: partSet.Header()}
	voteInfos := []abci.VoteInfo{}
	for _, privVal := range privVals {
		pk, err := privVal.GetPubKey()
		require.NoError(t, err)
		idx, _ := state.Validators.GetByAddress(pk.Address())
		vote, err := test.MakeVote(privVal, block0.Header.ChainID, idx, height-1, 0, 2, blockID, time.Now())
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

	block1 := makeBlock(state, height, &types.Commit{
		Height:     height - 1,
		Signatures: lastCommitSig,
	})

	block1.Txs = txs

	expectedRpp := &abci.RequestProcessProposal{
		Txs:         block1.Txs.ToSliceOfBytes(),
		Hash:        block1.Hash(),
		Height:      block1.Header.Height,
		Time:        block1.Header.Time,
		Misbehavior: block1.Evidence.Evidence.ToABCI(),
		ProposedLastCommit: abci.CommitInfo{
			Round: 0,
			Votes: voteInfos,
		},
		NextValidatorsHash: block1.NextValidatorsHash,
		ProposerAddress:    block1.ProposerAddress,
	}

	acceptBlock, err := blockExec.ProcessProposal(block1, state)
	require.NoError(t, err)
	require.True(t, acceptBlock)
	app.AssertExpectations(t)
	app.AssertCalled(t, "ProcessProposal", context.TODO(), expectedRpp)
}

func TestValidateValidatorUpdates(t *testing.T) {
	pubkey1 := ed25519.GenPrivKey().PubKey()
	pubkey2 := ed25519.GenPrivKey().PubKey()
	pk1, err := cryptoenc.PubKeyToProto(pubkey1)
	assert.NoError(t, err)
	pk2, err := cryptoenc.PubKeyToProto(pubkey2)
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

	pk, err := cryptoenc.PubKeyToProto(pubkey1)
	require.NoError(t, err)
	pk2, err := cryptoenc.PubKeyToProto(pubkey2)
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
	app := &testApp{}
	cc := proxy.NewLocalClientCreator(app)
	proxyApp := proxy.NewAppConns(cc, proxy.NopMetrics())
	err := proxyApp.Start()
	require.NoError(t, err)
	defer proxyApp.Stop() //nolint:errcheck // ignore for tests

	state, stateDB, _ := makeState(1, 1)
	stateStore := sm.NewStore(stateDB, sm.StoreOptions{
		DiscardABCIResponses: false,
	})
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

	blockStore := store.NewBlockStore(dbm.NewMemDB())
	blockExec := sm.NewBlockExecutor(
		stateStore,
		log.TestingLogger(),
		proxyApp.Consensus(),
		mp,
		sm.EmptyEvidencePool{},
		blockStore,
	)

	eventBus := types.NewEventBus()
	err = eventBus.Start()
	require.NoError(t, err)
	defer eventBus.Stop() //nolint:errcheck // ignore for tests

	blockExec.SetEventBus(eventBus)

	updatesSub, err := eventBus.Subscribe(
		context.Background(),
		"TestEndBlockValidatorUpdates",
		types.EventQueryValidatorSetUpdates,
	)
	require.NoError(t, err)

	block := makeBlock(state, 1, new(types.Commit))
	bps, err := block.MakePartSet(testPartSize)
	require.NoError(t, err)
	blockID := types.BlockID{Hash: block.Hash(), PartSetHeader: bps.Header()}

	pubkey := ed25519.GenPrivKey().PubKey()
	pk, err := cryptoenc.PubKeyToProto(pubkey)
	require.NoError(t, err)
	app.ValidatorUpdates = []abci.ValidatorUpdate{
		{PubKey: pk, Power: 10},
	}

	state, err = blockExec.ApplyBlock(state, blockID, block)
	require.NoError(t, err)
	// test new validator was added to NextValidators
	if assert.Equal(t, state.Validators.Size()+1, state.NextValidators.Size()) {
		idx, _ := state.NextValidators.GetByAddress(pubkey.Address())
		if idx < 0 {
			t.Fatalf("can't find address %v in the set %v", pubkey.Address(), state.NextValidators)
		}
	}

	// test we threw an event
	select {
	case msg := <-updatesSub.Out():
		event, ok := msg.Data().(types.EventDataValidatorSetUpdates)
		require.True(t, ok, "Expected event of type EventDataValidatorSetUpdates, got %T", msg.Data())
		if assert.NotEmpty(t, event.ValidatorUpdates) {
			assert.Equal(t, pubkey, event.ValidatorUpdates[0].PubKey)
			assert.EqualValues(t, 10, event.ValidatorUpdates[0].VotingPower)
		}
	case <-updatesSub.Cancelled():
		t.Fatalf("updatesSub was canceled (reason: %v)", updatesSub.Err())
	case <-time.After(1 * time.Second):
		t.Fatal("Did not receive EventValidatorSetUpdates within 1 sec.")
	}
}

// TestFinalizeBlockValidatorUpdatesResultingInEmptySet checks that processing validator updates that
// would result in empty set causes no panic, an error is raised and NextValidators is not updated
func TestFinalizeBlockValidatorUpdatesResultingInEmptySet(t *testing.T) {
	app := &testApp{}
	cc := proxy.NewLocalClientCreator(app)
	proxyApp := proxy.NewAppConns(cc, proxy.NopMetrics())
	err := proxyApp.Start()
	require.NoError(t, err)
	defer proxyApp.Stop() //nolint:errcheck // ignore for tests

	state, stateDB, _ := makeState(1, 1)
	stateStore := sm.NewStore(stateDB, sm.StoreOptions{
		DiscardABCIResponses: false,
	})
	blockStore := store.NewBlockStore(dbm.NewMemDB())
	blockExec := sm.NewBlockExecutor(
		stateStore,
		log.TestingLogger(),
		proxyApp.Consensus(),
		new(mpmocks.Mempool),
		sm.EmptyEvidencePool{},
		blockStore,
	)

	block := makeBlock(state, 1, new(types.Commit))
	bps, err := block.MakePartSet(testPartSize)
	require.NoError(t, err)
	blockID := types.BlockID{Hash: block.Hash(), PartSetHeader: bps.Header()}

	vp, err := cryptoenc.PubKeyToProto(state.Validators.Validators[0].PubKey)
	require.NoError(t, err)
	// Remove the only validator
	app.ValidatorUpdates = []abci.ValidatorUpdate{
		{PubKey: vp, Power: 0},
	}

	assert.NotPanics(t, func() { state, err = blockExec.ApplyBlock(state, blockID, block) })
	assert.Error(t, err)
	assert.NotEmpty(t, state.NextValidators.Validators)
}

func TestEmptyPrepareProposal(t *testing.T) {
	const height = 2

	app := &abci.BaseApplication{}
	cc := proxy.NewLocalClientCreator(app)
	proxyApp := proxy.NewAppConns(cc, proxy.NopMetrics())
	err := proxyApp.Start()
	require.NoError(t, err)
	defer proxyApp.Stop() //nolint:errcheck // ignore for tests

	state, stateDB, privVals := makeState(1, height)
	stateStore := sm.NewStore(stateDB, sm.StoreOptions{
		DiscardABCIResponses: false,
	})
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

	blockStore := store.NewBlockStore(dbm.NewMemDB())
	blockExec := sm.NewBlockExecutor(
		stateStore,
		log.TestingLogger(),
		proxyApp.Consensus(),
		mp,
		sm.EmptyEvidencePool{},
		blockStore,
	)
	pa, _ := state.Validators.GetByIndex(0)
	commit, _, err := makeValidCommit(height, types.BlockID{}, state.Validators, privVals)
	require.NoError(t, err)
	_, err = blockExec.CreateProposalBlock(height, state, commit, pa)
	require.NoError(t, err)
}

// TestPrepareProposalTxsAllIncluded tests that any transactions included in
// the prepare proposal response are included in the block.
func TestPrepareProposalTxsAllIncluded(t *testing.T) {
	const height = 2

	state, stateDB, privVals := makeState(1, height)
	stateStore := sm.NewStore(stateDB, sm.StoreOptions{
		DiscardABCIResponses: false,
	})

	evpool := &mocks.EvidencePool{}
	evpool.On("PendingEvidence", mock.Anything).Return([]types.Evidence{}, int64(0))

	txs := test.MakeNTxs(height, 10)
	mp := &mpmocks.Mempool{}
	mp.On("ReapMaxBytesMaxGas", mock.Anything, mock.Anything).Return(txs[2:])

	app := &abcimocks.Application{}
	app.On("PrepareProposal", mock.Anything, mock.Anything).Return(&abci.ResponsePrepareProposal{
		Txs: txs.ToSliceOfBytes(),
	}, nil)
	cc := proxy.NewLocalClientCreator(app)
	proxyApp := proxy.NewAppConns(cc, proxy.NopMetrics())
	err := proxyApp.Start()
	require.NoError(t, err)
	defer proxyApp.Stop() //nolint:errcheck // ignore for tests

	blockStore := store.NewBlockStore(dbm.NewMemDB())
	blockExec := sm.NewBlockExecutor(
		stateStore,
		log.TestingLogger(),
		proxyApp.Consensus(),
		mp,
		evpool,
		blockStore,
	)
	pa, _ := state.Validators.GetByIndex(0)
	commit, _, err := makeValidCommit(height, types.BlockID{}, state.Validators, privVals)
	require.NoError(t, err)
	block, err := blockExec.CreateProposalBlock(height, state, commit, pa)
	require.NoError(t, err)

	for i, tx := range block.Data.Txs {
		require.Equal(t, txs[i], tx)
	}

	mp.AssertExpectations(t)
}

// TestPrepareProposalReorderTxs tests that CreateBlock produces a block with transactions
// in the order matching the order they are returned from PrepareProposal.
func TestPrepareProposalReorderTxs(t *testing.T) {
	const height = 2

	state, stateDB, privVals := makeState(1, height)
	stateStore := sm.NewStore(stateDB, sm.StoreOptions{
		DiscardABCIResponses: false,
	})

	evpool := &mocks.EvidencePool{}
	evpool.On("PendingEvidence", mock.Anything).Return([]types.Evidence{}, int64(0))

	txs := test.MakeNTxs(height, 10)
	mp := &mpmocks.Mempool{}
	mp.On("ReapMaxBytesMaxGas", mock.Anything, mock.Anything).Return(txs)

	txs = txs[2:]
	txs = append(txs[len(txs)/2:], txs[:len(txs)/2]...)

	app := &abcimocks.Application{}
	app.On("PrepareProposal", mock.Anything, mock.Anything).Return(&abci.ResponsePrepareProposal{
		Txs: txs.ToSliceOfBytes(),
	}, nil)

	cc := proxy.NewLocalClientCreator(app)
	proxyApp := proxy.NewAppConns(cc, proxy.NopMetrics())
	err := proxyApp.Start()
	require.NoError(t, err)
	defer proxyApp.Stop() //nolint:errcheck // ignore for tests

	blockStore := store.NewBlockStore(dbm.NewMemDB())
	blockExec := sm.NewBlockExecutor(
		stateStore,
		log.TestingLogger(),
		proxyApp.Consensus(),
		mp,
		evpool,
		blockStore,
	)
	pa, _ := state.Validators.GetByIndex(0)
	commit, _, err := makeValidCommit(height, types.BlockID{}, state.Validators, privVals)
	require.NoError(t, err)
	block, err := blockExec.CreateProposalBlock(height, state, commit, pa)
	require.NoError(t, err)
	for i, tx := range block.Data.Txs {
		require.Equal(t, txs[i], tx)
	}

	mp.AssertExpectations(t)

}

// TestPrepareProposalErrorOnTooManyTxs tests that the block creation logic returns
// an error if the ResponsePrepareProposal returned from the application is invalid.
func TestPrepareProposalErrorOnTooManyTxs(t *testing.T) {
	const height = 2

	state, stateDB, privVals := makeState(1, height)
	// limit max block size
	state.ConsensusParams.Block.MaxBytes = 60 * 1024
	stateStore := sm.NewStore(stateDB, sm.StoreOptions{
		DiscardABCIResponses: false,
	})

	evpool := &mocks.EvidencePool{}
	evpool.On("PendingEvidence", mock.Anything).Return([]types.Evidence{}, int64(0))

	const nValidators = 1
	var bytesPerTx int64 = 3
	maxDataBytes := types.MaxDataBytes(state.ConsensusParams.Block.MaxBytes, 0, nValidators)
	txs := test.MakeNTxs(height, maxDataBytes/bytesPerTx+2) // +2 so that tx don't fit
	mp := &mpmocks.Mempool{}
	mp.On("ReapMaxBytesMaxGas", mock.Anything, mock.Anything).Return(txs)

	app := &abcimocks.Application{}
	app.On("PrepareProposal", mock.Anything, mock.Anything).Return(&abci.ResponsePrepareProposal{
		Txs: txs.ToSliceOfBytes(),
	}, nil)

	cc := proxy.NewLocalClientCreator(app)
	proxyApp := proxy.NewAppConns(cc, proxy.NopMetrics())
	err := proxyApp.Start()
	require.NoError(t, err)
	defer proxyApp.Stop() //nolint:errcheck // ignore for tests

	blockStore := store.NewBlockStore(dbm.NewMemDB())
	blockExec := sm.NewBlockExecutor(
		stateStore,
		log.NewNopLogger(),
		proxyApp.Consensus(),
		mp,
		evpool,
		blockStore,
	)
	pa, _ := state.Validators.GetByIndex(0)
	commit, _, err := makeValidCommit(height, types.BlockID{}, state.Validators, privVals)
	require.NoError(t, err)
	block, err := blockExec.CreateProposalBlock(height, state, commit, pa)
	require.Nil(t, block)
	require.ErrorContains(t, err, "transaction data size exceeds maximum")

	mp.AssertExpectations(t)
}

// TestPrepareProposalErrorOnPrepareProposalError tests when the client returns an error
// upon calling PrepareProposal on it.
func TestPrepareProposalErrorOnPrepareProposalError(t *testing.T) {
	const height = 2

	state, stateDB, privVals := makeState(1, height)
	stateStore := sm.NewStore(stateDB, sm.StoreOptions{
		DiscardABCIResponses: false,
	})

	evpool := &mocks.EvidencePool{}
	evpool.On("PendingEvidence", mock.Anything).Return([]types.Evidence{}, int64(0))

	txs := test.MakeNTxs(height, 10)
	mp := &mpmocks.Mempool{}
	mp.On("ReapMaxBytesMaxGas", mock.Anything, mock.Anything).Return(txs)

	cm := &abciclientmocks.Client{}
	cm.On("SetLogger", mock.Anything).Return()
	cm.On("Start").Return(nil)
	cm.On("Quit").Return(nil)
	cm.On("PrepareProposal", mock.Anything, mock.Anything).Return(nil, errors.New("an injected error")).Once()
	cm.On("Stop").Return(nil)
	cc := &pmocks.ClientCreator{}
	cc.On("NewABCIClient").Return(cm, nil)
	proxyApp := proxy.NewAppConns(cc, proxy.NopMetrics())
	err := proxyApp.Start()
	require.NoError(t, err)
	defer proxyApp.Stop() //nolint:errcheck // ignore for tests

	blockStore := store.NewBlockStore(dbm.NewMemDB())
	blockExec := sm.NewBlockExecutor(
		stateStore,
		log.NewNopLogger(),
		proxyApp.Consensus(),
		mp,
		evpool,
		blockStore,
	)
	pa, _ := state.Validators.GetByIndex(0)
	commit, _, err := makeValidCommit(height, types.BlockID{}, state.Validators, privVals)
	require.NoError(t, err)
	block, err := blockExec.CreateProposalBlock(height, state, commit, pa)
	require.Nil(t, block)
	require.ErrorContains(t, err, "an injected error")

	mp.AssertExpectations(t)
}

func makeBlockID(hash []byte, partSetSize uint32, partSetHash []byte) types.BlockID {
	var (
		h   = make([]byte, tmhash.Size)
		psH = make([]byte, tmhash.Size)
	)
	copy(h, hash)
	copy(psH, partSetHash)q
	return types.BlockID{
		Hash: h,
		PartSetHeader: types.PartSetHeader{
			Total: partSetSize,
			Hash:  psH,
		},
	}
}
