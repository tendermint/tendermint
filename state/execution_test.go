package state_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	abci "github.com/tendermint/tendermint/abci/types"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/libs/log"
	mmock "github.com/tendermint/tendermint/mempool/mock"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/state/mocks"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/version"
)

var (
	chainID             = "execution_chain"
	testPartSize uint32 = 65536
	nTxsPerBlock        = 10
)

func TestApplyBlock(t *testing.T) {
	app := &testApp{}
	cc := proxy.NewLocalClientCreator(app)
	proxyApp := proxy.NewAppConns(cc)
	err := proxyApp.Start()
	require.Nil(t, err)
	defer proxyApp.Stop() //nolint:errcheck // ignore for tests

	state, stateDB, _ := makeState(1, 1)
	stateStore := sm.NewStore(stateDB)

	proTxHash := state.Validators.Validators[0].ProTxHash
	blockExec := sm.NewBlockExecutor(&proTxHash, stateStore, log.TestingLogger(), proxyApp.Consensus(), proxyApp.Query(),
		mmock.Mempool{}, sm.EmptyEvidencePool{}, nil)

	block := makeBlock(state, 1)
	blockID := types.BlockID{Hash: block.Hash(), PartSetHeader: block.MakePartSet(testPartSize).Header()}

	state, retainHeight, err := blockExec.ApplyBlock(state, blockID, block)
	require.Nil(t, err)
	assert.EqualValues(t, retainHeight, 1)

	// TODO check state and mempool
	assert.EqualValues(t, 1, state.Version.Consensus.App, "App version wasn't updated")
}

// TestBeginBlockByzantineValidators ensures we send byzantine validators list.
func TestBeginBlockByzantineValidators(t *testing.T) {
	app := &testApp{}
	cc := proxy.NewLocalClientCreator(app)
	proxyApp := proxy.NewAppConns(cc)
	err := proxyApp.Start()
	require.Nil(t, err)
	defer proxyApp.Stop() //nolint:errcheck // ignore for tests

	state, stateDB, privVals := makeState(1, 1)
	stateStore := sm.NewStore(stateDB)

	defaultEvidenceTime := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	privVal := privVals[state.Validators.Validators[0].ProTxHash.String()]
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
		ProposerProTxHash:  crypto.RandProTxHash(),
	}

	// we don't need to worry about validating the evidence as long as they pass validate basic
	dve := types.NewMockDuplicateVoteEvidenceWithValidator(3, defaultEvidenceTime, privVal, state.ChainID,
		state.Validators.QuorumType, state.Validators.QuorumHash)
	dve.ValidatorPower = types.DefaultDashVotingPower
	lcae := &types.LightClientAttackEvidence{
		ConflictingBlock: &types.LightBlock{
			SignedHeader: &types.SignedHeader{
				Header: header,
				Commit: types.NewCommit(10, 0, makeBlockID(header.Hash(), 100, []byte("partshash")),
					makeStateID(header.AppHash), crypto.RandQuorumHash(), crypto.CRandBytes(types.SignatureSize),
					crypto.CRandBytes(types.SignatureSize),
				),
			},
			ValidatorSet: state.Validators,
		},
		CommonHeight:        8,
		ByzantineValidators: []*types.Validator{state.Validators.Validators[0]},
		TotalVotingPower:    types.DefaultDashVotingPower,
		Timestamp:           defaultEvidenceTime,
	}

	ev := []types.Evidence{dve, lcae}

	abciEv := []abci.Evidence{
		{
			Type:             abci.EvidenceType_DUPLICATE_VOTE,
			Height:           3,
			Time:             defaultEvidenceTime,
			Validator:        types.TM2PB.Validator(state.Validators.Validators[0]),
			TotalVotingPower: types.DefaultDashVotingPower,
		},
		{
			Type:             abci.EvidenceType_LIGHT_CLIENT_ATTACK,
			Height:           8,
			Time:             defaultEvidenceTime,
			Validator:        types.TM2PB.Validator(state.Validators.Validators[0]),
			TotalVotingPower: types.DefaultDashVotingPower,
		},
	}

	evpool := &mocks.EvidencePool{}
	evpool.On("PendingEvidence", mock.AnythingOfType("int64")).Return(ev, int64(100))
	evpool.On("Update", mock.AnythingOfType("state.State"), mock.AnythingOfType("types.EvidenceList")).Return()
	evpool.On("CheckEvidence", mock.AnythingOfType("types.EvidenceList")).Return(nil)

	proTxHash := state.Validators.Validators[0].ProTxHash
	blockExec := sm.NewBlockExecutor(&proTxHash, stateStore, log.TestingLogger(), proxyApp.Consensus(), proxyApp.Query(),
		mmock.Mempool{}, evpool, nil)

	block := makeBlock(state, 1)
	block.Evidence = types.EvidenceData{Evidence: ev}
	block.Header.EvidenceHash = block.Evidence.Hash()
	blockID = types.BlockID{Hash: block.Hash(), PartSetHeader: block.MakePartSet(testPartSize).Header()}

	state, retainHeight, err := blockExec.ApplyBlock(state, blockID, block)
	require.Nil(t, err)
	assert.EqualValues(t, retainHeight, 1)

	// TODO check state and mempool
	assert.Equal(t, abciEv, app.ByzantineValidators)
}

func TestValidateValidatorUpdates(t *testing.T) {
	pubkey1 := bls12381.GenPrivKey().PubKey()
	pubkey2 := bls12381.GenPrivKey().PubKey()
	pk1, err := cryptoenc.PubKeyToProto(pubkey1)
	assert.NoError(t, err)
	pk2, err := cryptoenc.PubKeyToProto(pubkey2)
	assert.NoError(t, err)

	proTxHash1 := crypto.CRandBytes(32)
	proTxHash2 := crypto.CRandBytes(32)

	defaultValidatorParams := tmproto.ValidatorParams{PubKeyTypes: []string{types.ABCIPubKeyTypeBLS12381}}

	testCases := []struct {
		name string

		abciUpdates     []abci.ValidatorUpdate
		validatorParams tmproto.ValidatorParams

		shouldErr bool
	}{
		{
			"adding a validator is OK",
			[]abci.ValidatorUpdate{{PubKey: pk2, Power: 20, ProTxHash: proTxHash2}},
			defaultValidatorParams,
			false,
		},
		{
			"updating a validator is OK",
			[]abci.ValidatorUpdate{{PubKey: pk1, Power: 20, ProTxHash: proTxHash1}},
			defaultValidatorParams,
			false,
		},
		{
			"removing a validator is OK",
			[]abci.ValidatorUpdate{{PubKey: pk2, Power: 0, ProTxHash: proTxHash2}},
			defaultValidatorParams,
			false,
		},
		{
			"adding a validator with negative power results in error",
			[]abci.ValidatorUpdate{{PubKey: pk2, Power: -100, ProTxHash: proTxHash2}},
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
	validatorSet, _ := types.GenerateValidatorSet(4)
	originalProTxHashes := validatorSet.GetProTxHashes()
	addedProTxHashes := bls12381.CreateProTxHashes(4)
	combinedProTxHashes := append(originalProTxHashes, addedProTxHashes...)
	combinedValidatorSet, _ := types.GenerateValidatorSetUsingProTxHashes(combinedProTxHashes)
	regeneratedValidatorSet, _ := types.GenerateValidatorSetUsingProTxHashes(combinedProTxHashes)
	abciRegeneratedValidatorUpdates := regeneratedValidatorSet.ABCIEquivalentValidatorUpdates()
	removedProTxHashes := combinedValidatorSet.GetProTxHashes()[0 : len(combinedProTxHashes)-2] // these are sorted
	removedValidatorSet, _ := types.GenerateValidatorSetUsingProTxHashes(removedProTxHashes)    // size 6
	abciRemovalValidatorUpdates := removedValidatorSet.ABCIEquivalentValidatorUpdates()
	abciRemovalValidatorUpdates.ValidatorUpdates = append(abciRemovalValidatorUpdates.ValidatorUpdates,
		abciRegeneratedValidatorUpdates.ValidatorUpdates[6:]...)
	abciRemovalValidatorUpdates.ValidatorUpdates[6].Power = 0
	abciRemovalValidatorUpdates.ValidatorUpdates[7].Power = 0

	pubkeyRemoval := bls12381.GenPrivKey().PubKey()
	pk, err := cryptoenc.PubKeyToProto(pubkeyRemoval)
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
				ValidatorUpdates:   []abci.ValidatorUpdate{{ProTxHash: crypto.RandProTxHash(), PubKey: pk, Power: 0}},
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
			err = tc.currentSet.UpdateWithChangeSet(updates, tc.thresholdPublicKeyUpdate)
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

// TestEndBlockValidatorUpdates ensures we update validator set and send an event.
func TestEndBlockValidatorUpdates(t *testing.T) {
	app := &testApp{}
	cc := proxy.NewLocalClientCreator(app)
	proxyApp := proxy.NewAppConns(cc)
	err := proxyApp.Start()
	require.Nil(t, err)
	defer proxyApp.Stop() //nolint:errcheck // ignore for tests

	state, stateDB, _ := makeState(1, 1)
	stateStore := sm.NewStore(stateDB)

	proTxHash := state.Validators.Validators[0].ProTxHash
	blockExec := sm.NewBlockExecutor(
		&proTxHash,
		stateStore,
		log.TestingLogger(),
		proxyApp.Consensus(),
		proxyApp.Query(),
		mmock.Mempool{},
		sm.EmptyEvidencePool{},
		nil,
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

	block := makeBlock(state, 1)
	blockID := types.BlockID{Hash: block.Hash(), PartSetHeader: block.MakePartSet(testPartSize).Header()}

	vals := state.Validators
	proTxHashes := vals.GetProTxHashes()
	addProTxHash := crypto.RandProTxHash()
	proTxHashes = append(proTxHashes, addProTxHash)
	newVals, _ := types.GenerateValidatorSetUsingProTxHashes(proTxHashes)
	var pos int
	for i, proTxHash := range newVals.GetProTxHashes() {
		if bytes.Equal(proTxHash.Bytes(), addProTxHash.Bytes()) {
			pos = i
		}
	}

	app.ValidatorSetUpdate = newVals.ABCIEquivalentValidatorUpdates()

	state, _, err = blockExec.ApplyBlock(state, blockID, block)
	require.Nil(t, err)
	// test new validator was added to NextValidators
	if assert.Equal(t, state.Validators.Size()+1, state.NextValidators.Size()) {
		idx, _ := state.NextValidators.GetByProTxHash(addProTxHash)
		if idx < 0 {
			t.Fatalf("can't find proTxHash %v in the set %v", addProTxHash, state.NextValidators)
		}
	}

	// test we threw an event
	select {
	case msg := <-updatesSub.Out():
		event, ok := msg.Data().(types.EventDataValidatorSetUpdates)
		require.True(t, ok, "Expected event of type EventDataValidatorSetUpdates, got %T", msg.Data())
		if assert.NotEmpty(t, event.ValidatorUpdates) {
			assert.Equal(t, addProTxHash, event.ValidatorUpdates[pos].ProTxHash)
			assert.EqualValues(t, types.DefaultDashVotingPower, event.ValidatorUpdates[1].VotingPower)
		}
	case <-updatesSub.Cancelled():
		t.Fatalf("updatesSub was cancelled (reason: %v)", updatesSub.Err())
	case <-time.After(1 * time.Second):
		t.Fatal("Did not receive EventValidatorSetUpdates within 1 sec.")
	}
}

// TestEndBlockValidatorUpdatesResultingInEmptySet checks that processing validator updates that
// would result in empty set causes no panic, an error is raised and NextValidators is not updated
func TestEndBlockValidatorUpdatesResultingInEmptySet(t *testing.T) {
	app := &testApp{}
	cc := proxy.NewLocalClientCreator(app)
	proxyApp := proxy.NewAppConns(cc)
	err := proxyApp.Start()
	require.Nil(t, err)
	defer proxyApp.Stop() //nolint:errcheck // ignore for tests

	state, stateDB, _ := makeState(1, 1)
	stateStore := sm.NewStore(stateDB)
	proTxHash := state.Validators.Validators[0].ProTxHash
	blockExec := sm.NewBlockExecutor(
		&proTxHash,
		stateStore,
		log.TestingLogger(),
		proxyApp.Consensus(),
		proxyApp.Query(),
		mmock.Mempool{},
		sm.EmptyEvidencePool{},
		nil,
	)

	block := makeBlock(state, 1)
	blockID := types.BlockID{Hash: block.Hash(), PartSetHeader: block.MakePartSet(testPartSize).Header()}

	publicKey, err := cryptoenc.PubKeyToProto(bls12381.GenPrivKey().PubKey())
	require.NoError(t, err)
	// Remove the only validator
	validatorUpdates := []abci.ValidatorUpdate{
		{PubKey: publicKey, ProTxHash: proTxHash, Power: 0},
	}
	// the quorum hash needs to be the same
	// because we are providing an update removing a member from a known quorum, not changing the quorum
	app.ValidatorSetUpdate = &abci.ValidatorSetUpdate{
		ValidatorUpdates:   validatorUpdates,
		ThresholdPublicKey: publicKey,
		QuorumHash:         state.Validators.QuorumHash,
	}

	assert.NotPanics(t, func() { state, _, err = blockExec.ApplyBlock(state, blockID, block) })
	assert.NotNil(t, err)
	assert.NotEmpty(t, state.NextValidators.Validators)
}

func makeBlockID(hash []byte, partSetSize uint32, partSetHash []byte) types.BlockID {
	var (
		h   = make([]byte, tmhash.Size)
		psH = make([]byte, tmhash.Size)
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

func makeStateID(lastAppHash []byte) types.StateID {
	var (
		h = make([]byte, tmhash.Size)
	)
	copy(h, lastAppHash)
	return types.StateID{
		LastAppHash: h,
	}
}
