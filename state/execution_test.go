package state

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/ed25519"
	cmn "github.com/tendermint/tendermint/libs/common"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
)

var (
	chainID      = "execution_chain"
	testPartSize = 65536
	nTxsPerBlock = 10
)

func TestApplyBlock(t *testing.T) {
	cc := proxy.NewLocalClientCreator(kvstore.NewKVStoreApplication())
	proxyApp := proxy.NewAppConns(cc, nil)
	err := proxyApp.Start()
	require.Nil(t, err)
	defer proxyApp.Stop()

	state, stateDB := state(1, 1)

	blockExec := NewBlockExecutor(stateDB, log.TestingLogger(), proxyApp.Consensus(),
		MockMempool{}, MockEvidencePool{})

	block := makeBlock(state, 1)
	blockID := types.BlockID{block.Hash(), block.MakePartSet(testPartSize).Header()}

	state, err = blockExec.ApplyBlock(state, blockID, block)
	require.Nil(t, err)

	// TODO check state and mempool
}

// TestBeginBlockValidators ensures we send absent validators list.
func TestBeginBlockValidators(t *testing.T) {
	app := &testApp{}
	cc := proxy.NewLocalClientCreator(app)
	proxyApp := proxy.NewAppConns(cc, nil)
	err := proxyApp.Start()
	require.Nil(t, err)
	defer proxyApp.Stop()

	state, stateDB := state(2, 2)

	prevHash := state.LastBlockID.Hash
	prevParts := types.PartSetHeader{}
	prevBlockID := types.BlockID{prevHash, prevParts}

	now := time.Now().UTC()
	vote0 := &types.CommitSig{Timestamp: now}
	vote1 := &types.CommitSig{Timestamp: now}

	testCases := []struct {
		desc                     string
		lastCommitPrecommits     []*types.CommitSig
		expectedAbsentValidators []int
	}{
		{"none absent", []*types.CommitSig{vote0, vote1}, []int{}},
		{"one absent", []*types.CommitSig{vote0, nil}, []int{1}},
		{"multiple absent", []*types.CommitSig{nil, nil}, []int{0, 1}},
	}

	for _, tc := range testCases {
		lastCommit := &types.Commit{BlockID: prevBlockID, Precommits: tc.lastCommitPrecommits}

		// block for height 2
		block, _ := state.MakeBlock(2, makeTxs(2), lastCommit, nil, state.Validators.GetProposer().Address)
		_, err = ExecCommitBlock(proxyApp.Consensus(), block, log.TestingLogger(), state.Validators, stateDB)
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
	proxyApp := proxy.NewAppConns(cc, nil)
	err := proxyApp.Start()
	require.Nil(t, err)
	defer proxyApp.Stop()

	state, stateDB := state(2, 12)

	prevHash := state.LastBlockID.Hash
	prevParts := types.PartSetHeader{}
	prevBlockID := types.BlockID{prevHash, prevParts}

	height1, idx1, val1 := int64(8), 0, state.Validators.Validators[0].Address
	height2, idx2, val2 := int64(3), 1, state.Validators.Validators[1].Address
	ev1 := types.NewMockGoodEvidence(height1, idx1, val1)
	ev2 := types.NewMockGoodEvidence(height2, idx2, val2)

	now := time.Now()
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

	vote0 := &types.CommitSig{Timestamp: now}
	vote1 := &types.CommitSig{Timestamp: now}
	votes := []*types.CommitSig{vote0, vote1}
	lastCommit := &types.Commit{BlockID: prevBlockID, Precommits: votes}
	for _, tc := range testCases {

		block, _ := state.MakeBlock(10, makeTxs(2), lastCommit, nil, state.Validators.GetProposer().Address)
		block.Time = now
		block.Evidence.Evidence = tc.evidence
		_, err = ExecCommitBlock(proxyApp.Consensus(), block, log.TestingLogger(), state.Validators, stateDB)
		require.Nil(t, err, tc.desc)

		// -> app must receive an index of the byzantine validator
		assert.Equal(t, tc.expectedByzantineValidators, app.ByzantineValidators, tc.desc)
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

		{
			"adding a validator with negative power results in error",

			types.NewValidatorSet([]*types.Validator{val1}),
			[]abci.ValidatorUpdate{{PubKey: types.TM2PB.PubKey(pubkey2), Power: -100}},

			types.NewValidatorSet([]*types.Validator{val1}),
			true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := updateValidators(tc.currentSet, tc.abciUpdates)
			if tc.shouldErr {
				assert.Error(t, err)
			} else {
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
	proxyApp := proxy.NewAppConns(cc, nil)
	err := proxyApp.Start()
	require.Nil(t, err)
	defer proxyApp.Stop()

	state, stateDB := state(1, 1)

	blockExec := NewBlockExecutor(stateDB, log.TestingLogger(), proxyApp.Consensus(),
		MockMempool{}, MockEvidencePool{})
	eventBus := types.NewEventBus()
	err = eventBus.Start()
	require.NoError(t, err)
	defer eventBus.Stop()
	blockExec.SetEventBus(eventBus)

	updatesCh := make(chan interface{}, 1)
	err = eventBus.Subscribe(context.Background(), "TestEndBlockValidatorUpdates", types.EventQueryValidatorSetUpdates, updatesCh)
	require.NoError(t, err)

	block := makeBlock(state, 1)
	blockID := types.BlockID{block.Hash(), block.MakePartSet(testPartSize).Header()}

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
	case e := <-updatesCh:
		event, ok := e.(types.EventDataValidatorSetUpdates)
		require.True(t, ok, "Expected event of type EventDataValidatorSetUpdates, got %T", e)
		if assert.NotEmpty(t, event.ValidatorUpdates) {
			assert.Equal(t, pubkey, event.ValidatorUpdates[0].PubKey)
			assert.EqualValues(t, 10, event.ValidatorUpdates[0].VotingPower)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Did not receive EventValidatorSetUpdates within 1 sec.")
	}
}

//----------------------------------------------------------------------------

// make some bogus txs
func makeTxs(height int64) (txs []types.Tx) {
	for i := 0; i < nTxsPerBlock; i++ {
		txs = append(txs, types.Tx([]byte{byte(height), byte(i)}))
	}
	return txs
}

func state(nVals, height int) (State, dbm.DB) {
	vals := make([]types.GenesisValidator, nVals)
	for i := 0; i < nVals; i++ {
		secret := []byte(fmt.Sprintf("test%d", i))
		pk := ed25519.GenPrivKeyFromSecret(secret)
		vals[i] = types.GenesisValidator{
			pk.PubKey(), 1000, fmt.Sprintf("test%d", i),
		}
	}
	s, _ := MakeGenesisState(&types.GenesisDoc{
		ChainID:    chainID,
		Validators: vals,
		AppHash:    nil,
	})

	// save validators to db for 2 heights
	stateDB := dbm.NewMemDB()
	SaveState(stateDB, s)

	for i := 1; i < height; i++ {
		s.LastBlockHeight++
		SaveState(stateDB, s)
	}
	return s, stateDB
}

func makeBlock(state State, height int64) *types.Block {
	block, _ := state.MakeBlock(height, makeTxs(state.LastBlockHeight), new(types.Commit), nil, state.Validators.GetProposer().Address)
	return block
}

//----------------------------------------------------------------------------

type testApp struct {
	abci.BaseApplication

	CommitVotes         []abci.VoteInfo
	ByzantineValidators []abci.Evidence
	ValidatorUpdates    []abci.ValidatorUpdate
}

var _ abci.Application = (*testApp)(nil)

func (app *testApp) Info(req abci.RequestInfo) (resInfo abci.ResponseInfo) {
	return abci.ResponseInfo{}
}

func (app *testApp) BeginBlock(req abci.RequestBeginBlock) abci.ResponseBeginBlock {
	app.CommitVotes = req.LastCommitInfo.Votes
	app.ByzantineValidators = req.ByzantineValidators
	return abci.ResponseBeginBlock{}
}

func (app *testApp) EndBlock(req abci.RequestEndBlock) abci.ResponseEndBlock {
	return abci.ResponseEndBlock{ValidatorUpdates: app.ValidatorUpdates}
}

func (app *testApp) DeliverTx(tx []byte) abci.ResponseDeliverTx {
	return abci.ResponseDeliverTx{Tags: []cmn.KVPair{}}
}

func (app *testApp) CheckTx(tx []byte) abci.ResponseCheckTx {
	return abci.ResponseCheckTx{}
}

func (app *testApp) Commit() abci.ResponseCommit {
	return abci.ResponseCommit{}
}

func (app *testApp) Query(reqQuery abci.RequestQuery) (resQuery abci.ResponseQuery) {
	return
}
