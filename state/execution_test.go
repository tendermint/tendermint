package state_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/mock"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

var (
	chainID      = "execution_chain"
	testPartSize = 65536
	nTxsPerBlock = 10
)

func TestApplyBlock(t *testing.T) {
	cc := proxy.NewLocalClientCreator(kvstore.NewKVStoreApplication())
	proxyApp := proxy.NewAppConns(cc)
	err := proxyApp.Start()
	require.Nil(t, err)
	defer proxyApp.Stop()

	state, stateDB, _ := makeState(1, 1)

	blockExec := sm.NewBlockExecutor(stateDB, log.TestingLogger(), proxyApp.Consensus(),
		mock.Mempool{}, sm.MockEvidencePool{})

	block := makeBlock(state, 1)
	blockID := types.BlockID{Hash: block.Hash(), PartsHeader: block.MakePartSet(testPartSize).Header()}

	//nolint:ineffassign
	state, err = blockExec.ApplyBlock(state, blockID, block)
	require.Nil(t, err)

	// TODO check state and mempool
}

// TestBeginBlockValidators ensures we send absent validators list.
func TestBeginBlockValidators(t *testing.T) {
	app := &testApp{}
	cc := proxy.NewLocalClientCreator(app)
	proxyApp := proxy.NewAppConns(cc)
	err := proxyApp.Start()
	require.Nil(t, err)
	defer proxyApp.Stop()

	state, stateDB, _ := makeState(2, 2)

	prevHash := state.LastBlockID.Hash
	prevParts := types.PartSetHeader{}
	prevBlockID := types.BlockID{Hash: prevHash, PartsHeader: prevParts}

	now := tmtime.Now()
	commitSig0 := (&types.Vote{ValidatorIndex: 0, Timestamp: now, Type: types.PrecommitType}).CommitSig()
	commitSig1 := (&types.Vote{ValidatorIndex: 1, Timestamp: now}).CommitSig()

	testCases := []struct {
		desc                     string
		lastCommitPrecommits     []*types.CommitSig
		expectedAbsentValidators []int
	}{
		{"none absent", []*types.CommitSig{commitSig0, commitSig1}, []int{}},
		{"one absent", []*types.CommitSig{commitSig0, nil}, []int{1}},
		{"multiple absent", []*types.CommitSig{nil, nil}, []int{0, 1}},
	}

	for _, tc := range testCases {
		lastCommit := types.NewCommit(prevBlockID, tc.lastCommitPrecommits)

		// block for height 2
		block, _ := state.MakeBlock(2, makeTxs(2), lastCommit, nil, state.Validators.GetProposer().Address)

		_, err = sm.ExecCommitBlock(proxyApp.Consensus(), block, log.TestingLogger(), stateDB)
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

// TestBeginBlockByzantineValidators ensures we send byzantine validators list.
func TestBeginBlockByzantineValidators(t *testing.T) {
	app := &testApp{}
	cc := proxy.NewLocalClientCreator(app)
	proxyApp := proxy.NewAppConns(cc)
	err := proxyApp.Start()
	require.Nil(t, err)
	defer proxyApp.Stop()

	state, stateDB, _ := makeState(2, 12)

	prevHash := state.LastBlockID.Hash
	prevParts := types.PartSetHeader{}
	prevBlockID := types.BlockID{Hash: prevHash, PartsHeader: prevParts}

	height1, idx1, val1 := int64(8), 0, state.Validators.Validators[0].Address
	height2, idx2, val2 := int64(3), 1, state.Validators.Validators[1].Address
	ev1 := types.NewMockGoodEvidence(height1, idx1, val1)
	ev2 := types.NewMockGoodEvidence(height2, idx2, val2)

	now := tmtime.Now()
	valSet := state.Validators
	testCases := []struct {
		desc                        string
		evidence                    []types.Evidence
		expectedByzantineValidators []abci.Evidence
	}{
		{"none byzantine", []types.Evidence{}, []abci.Evidence{}},
		{"one byzantine", []types.Evidence{ev1}, []abci.Evidence{types.TM2PB.Evidence(ev1, valSet, now)}},
		{"multiple byzantine", []types.Evidence{ev1, ev2}, []abci.Evidence{
			types.TM2PB.Evidence(ev1, valSet, now),
			types.TM2PB.Evidence(ev2, valSet, now)}},
	}

	commitSig0 := (&types.Vote{ValidatorIndex: 0, Timestamp: now, Type: types.PrecommitType}).CommitSig()
	commitSig1 := (&types.Vote{ValidatorIndex: 1, Timestamp: now}).CommitSig()
	commitSigs := []*types.CommitSig{commitSig0, commitSig1}
	lastCommit := types.NewCommit(prevBlockID, commitSigs)
	for _, tc := range testCases {

		block, _ := state.MakeBlock(10, makeTxs(2), lastCommit, nil, state.Validators.GetProposer().Address)
		block.Time = now
		block.Evidence.Evidence = tc.evidence
		_, err = sm.ExecCommitBlock(proxyApp.Consensus(), block, log.TestingLogger(), stateDB)
		require.Nil(t, err, tc.desc)

		// -> app must receive an index of the byzantine validator
		assert.Equal(t, tc.expectedByzantineValidators, app.ByzantineValidators, tc.desc)
	}
}

func TestValidateValidatorUpdates(t *testing.T) {
	pubkey1 := ed25519.GenPrivKey().PubKey()
	pubkey2 := ed25519.GenPrivKey().PubKey()

	secpKey := secp256k1.GenPrivKey().PubKey()

	defaultValidatorParams := types.ValidatorParams{PubKeyTypes: []string{types.ABCIPubKeyTypeEd25519}}

	testCases := []struct {
		name string

		abciUpdates     []abci.ValidatorUpdate
		validatorParams types.ValidatorParams

		shouldErr bool
	}{
		{
			"adding a validator is OK",

			[]abci.ValidatorUpdate{{PubKey: types.TM2PB.PubKey(pubkey2), Power: 20}},
			defaultValidatorParams,

			false,
		},
		{
			"updating a validator is OK",

			[]abci.ValidatorUpdate{{PubKey: types.TM2PB.PubKey(pubkey1), Power: 20}},
			defaultValidatorParams,

			false,
		},
		{
			"removing a validator is OK",

			[]abci.ValidatorUpdate{{PubKey: types.TM2PB.PubKey(pubkey2), Power: 0}},
			defaultValidatorParams,

			false,
		},
		{
			"adding a validator with negative power results in error",

			[]abci.ValidatorUpdate{{PubKey: types.TM2PB.PubKey(pubkey2), Power: -100}},
			defaultValidatorParams,

			true,
		},
		{
			"adding a validator with pubkey thats not in validator params results in error",

			[]abci.ValidatorUpdate{{PubKey: types.TM2PB.PubKey(secpKey), Power: -100}},
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
			[]abci.ValidatorUpdate{{PubKey: types.TM2PB.PubKey(pubkey2), Power: 20}},

			types.NewValidatorSet([]*types.Validator{val1, val2}),
			false,
		},
		{
			"updating a validator is OK",

			types.NewValidatorSet([]*types.Validator{val1}),
			[]abci.ValidatorUpdate{{PubKey: types.TM2PB.PubKey(pubkey1), Power: 20}},

			types.NewValidatorSet([]*types.Validator{types.NewValidator(pubkey1, 20)}),
			false,
		},
		{
			"removing a validator is OK",

			types.NewValidatorSet([]*types.Validator{val1, val2}),
			[]abci.ValidatorUpdate{{PubKey: types.TM2PB.PubKey(pubkey2), Power: 0}},

			types.NewValidatorSet([]*types.Validator{val1}),
			false,
		},
		{
			"removing a non-existing validator results in error",

			types.NewValidatorSet([]*types.Validator{val1}),
			[]abci.ValidatorUpdate{{PubKey: types.TM2PB.PubKey(pubkey2), Power: 0}},

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

// TestEndBlockValidatorUpdates ensures we update validator set and send an event.
func TestEndBlockValidatorUpdates(t *testing.T) {
	app := &testApp{}
	cc := proxy.NewLocalClientCreator(app)
	proxyApp := proxy.NewAppConns(cc)
	err := proxyApp.Start()
	require.Nil(t, err)
	defer proxyApp.Stop()

	state, stateDB, _ := makeState(1, 1)

	blockExec := sm.NewBlockExecutor(stateDB, log.TestingLogger(), proxyApp.Consensus(), mock.Mempool{}, sm.MockEvidencePool{})

	eventBus := types.NewEventBus()
	err = eventBus.Start()
	require.NoError(t, err)
	defer eventBus.Stop()
	blockExec.SetEventBus(eventBus)

	updatesSub, err := eventBus.Subscribe(context.Background(), "TestEndBlockValidatorUpdates", types.EventQueryValidatorSetUpdates)
	require.NoError(t, err)

	block := makeBlock(state, 1)
	blockID := types.BlockID{Hash: block.Hash(), PartsHeader: block.MakePartSet(testPartSize).Header()}

	pubkey := ed25519.GenPrivKey().PubKey()
	app.ValidatorUpdates = []abci.ValidatorUpdate{
		{PubKey: types.TM2PB.PubKey(pubkey), Power: 10},
	}

	state, err = blockExec.ApplyBlock(state, blockID, block)
	require.Nil(t, err)

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
	defer proxyApp.Stop()

	state, stateDB, _ := makeState(1, 1)
	blockExec := sm.NewBlockExecutor(stateDB, log.TestingLogger(), proxyApp.Consensus(), mock.Mempool{}, sm.MockEvidencePool{})

	block := makeBlock(state, 1)
	blockID := types.BlockID{Hash: block.Hash(), PartsHeader: block.MakePartSet(testPartSize).Header()}

	// Remove the only validator
	app.ValidatorUpdates = []abci.ValidatorUpdate{
		{PubKey: types.TM2PB.PubKey(state.Validators.Validators[0].PubKey), Power: 0},
	}

	assert.NotPanics(t, func() { state, err = blockExec.ApplyBlock(state, blockID, block) })
	assert.NotNil(t, err)
	assert.NotEmpty(t, state.NextValidators.Validators)

}
