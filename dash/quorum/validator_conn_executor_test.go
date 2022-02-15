package quorum

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abciclient "github.com/tendermint/tendermint/abci/client"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/dash/quorum/mock"
	"github.com/tendermint/tendermint/dash/quorum/selectpeers"
	mmock "github.com/tendermint/tendermint/internal/mempool/mock"
	"github.com/tendermint/tendermint/internal/p2p"
	"github.com/tendermint/tendermint/internal/proxy"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/store"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

const (
	mySeedID uint16 = math.MaxUint16 - 1
)

var (
	chainID             = "execution_chain"
	testPartSize uint32 = 65536
	nTxsPerBlock        = 10
)

type validatorUpdate struct {
	validators      []*types.Validator
	expectedHistory []mock.HistoryEvent
}
type testCase struct {
	me               *types.Validator
	validatorUpdates []validatorUpdate
}

// TestValidatorConnExecutorNotValidator checks what happens if current node is not a validator.
// Expected: nothing happens
func TestValidatorConnExecutor_NotValidator(t *testing.T) {

	me := mock.NewValidator(mySeedID)
	tc := testCase{
		me: me,
		validatorUpdates: []validatorUpdate{
			0: {
				validators: []*types.Validator{
					mock.NewValidator(1),
					mock.NewValidator(2),
					mock.NewValidator(3),
				},
				expectedHistory: []mock.HistoryEvent{},
			}},
	}
	executeTestCase(t, tc)
}

// TestValidatorConnExecutor_WrongAddress checks behavior in case of several issues in the address.
// Expected behavior: invalid address is dialed. Previous addresses are disconnected.
func TestValidatorConnExecutor_WrongAddress(t *testing.T) {
	me := mock.NewValidator(mySeedID)
	nodeID := mock.NewNodeID(1000)
	addr1, err := types.ParseValidatorAddress("http://" + nodeID + "@www.domain-that-does-not-exist.com:80")
	require.NoError(t, err)

	val1 := mock.NewValidator(100)
	val1.NodeAddress = addr1

	valsWithoutAddress := make([]*types.Validator, 5)
	for i := 0; i < len(valsWithoutAddress); i++ {
		valsWithoutAddress[i] = mock.NewValidator(uint16(200 + i))
		valsWithoutAddress[i].NodeAddress = types.ValidatorAddress{}
	}

	tc := testCase{
		me: me,
		validatorUpdates: []validatorUpdate{
			0: {
				validators: []*types.Validator{
					me,
					mock.NewValidator(1),
					mock.NewValidator(2),
					mock.NewValidator(3),
					mock.NewValidator(4),
				},
				expectedHistory: []mock.HistoryEvent{
					{Operation: mock.OpDial},
					{Operation: mock.OpDial},
				},
			},
			1: { // val1 has invalid address, but we still pass it to dial, so it will be here
				validators: []*types.Validator{
					me,
					val1,
				},
				expectedHistory: []mock.HistoryEvent{
					{Operation: mock.OpStop},
					{Operation: mock.OpStop},
					{Operation: mock.OpDial, Params: []string{string(val1.NodeAddress.NodeID)}},
				},
			},
			2: { // disconnect val1and dial 2 new validators (skipping invalid one)
				validators: []*types.Validator{
					me,
					valsWithoutAddress[0],
					mock.NewValidator(1),
					mock.NewValidator(2),
					mock.NewValidator(3),
					mock.NewValidator(4),
					mock.NewValidator(5),
				},
				expectedHistory: []mock.HistoryEvent{
					{Operation: mock.OpStop, Params: []string{string(val1.NodeAddress.NodeID)}},
					{Operation: mock.OpDial, Params: []string{
						mock.NewNodeID(2),
						mock.NewNodeID(5),
					}},
					{Operation: mock.OpDial, Params: []string{
						mock.NewNodeID(2),
						mock.NewNodeID(5),
					}},
				},
			},
			3: { // this should disconnect everyone because none of the validators has correct address
				validators: append([]*types.Validator{me}, valsWithoutAddress...),
				expectedHistory: []mock.HistoryEvent{
					{Operation: mock.OpStop},
					{Operation: mock.OpStop},
				},
			},
		}}

	executeTestCase(t, tc)
}

// TestValidatorConnExecutor_Myself checks what happens if we want to connect to ourselves.
// Expected: connections from previous update are stopped, no new connection established.
func TestValidatorConnExecutor_Myself(t *testing.T) {

	me := mock.NewValidator(mySeedID)

	tc := testCase{
		me: me,
		validatorUpdates: []validatorUpdate{
			0: {
				validators: []*types.Validator{ // new set should have validator 1, 2 and 3
					me,
					mock.NewValidator(1),
					mock.NewValidator(2),
					mock.NewValidator(3),
				},
				expectedHistory: []mock.HistoryEvent{
					{
						Operation: mock.OpDial,
						Params:    []string{mock.NewNodeID(1), mock.NewNodeID(2), mock.NewNodeID(3)},
					},
					{
						Operation: mock.OpDial,
						Params:    []string{mock.NewNodeID(1), mock.NewNodeID(2), mock.NewNodeID(3)},
					},
					{
						Operation: mock.OpDial,
						Params:    []string{mock.NewNodeID(1), mock.NewNodeID(2), mock.NewNodeID(3)},
					},
				},
			},
			1: {
				validators: []*types.Validator{ // new set should have validator 2 and 5
					me,
					mock.NewValidator(1),
					mock.NewValidator(2),
					mock.NewValidator(3),
					mock.NewValidator(4),
					mock.NewValidator(5),
				},
				expectedHistory: []mock.HistoryEvent{
					{
						Operation: mock.OpStop,
						Params:    []string{mock.NewNodeID(1)},
					},
					{Operation: mock.OpStop, Params: []string{mock.NewNodeID(3)}},
					{Operation: mock.OpDial, Params: []string{mock.NewNodeID(5)}},
					// {Operation: mock.OpDial, Params: []string{mock.NewNodeID(2), mock.NewNodeID(5)}},
				},
			},
			2: {
				validators: []*types.Validator{me},
				expectedHistory: []mock.HistoryEvent{
					{
						Operation: mock.OpStop,
						Params:    []string{mock.NewNodeID(2), mock.NewNodeID(5)},
					},
					{
						Operation: mock.OpStop,
						Params:    []string{mock.NewNodeID(2), mock.NewNodeID(5)},
					},
				},
			},
		},
	}
	executeTestCase(t, tc)
}

// TestValidatorConnExecutor_EmptyVSet checks what will happen if the ABCI App provides an empty validator set.
// Expected: nothing happens
func TestValidatorConnExecutor_EmptyVSet(t *testing.T) {
	me := mock.NewValidator(mySeedID)
	tc := testCase{
		me: me,
		validatorUpdates: []validatorUpdate{
			0: {
				validators: []*types.Validator{me,
					mock.NewValidator(1),
					mock.NewValidator(2),
					mock.NewValidator(3),
					mock.NewValidator(4),
					mock.NewValidator(5),
				},
				expectedHistory: []mock.HistoryEvent{
					{
						Operation: mock.OpDial,
						Params:    []string{mock.NewNodeID(2), mock.NewNodeID(5)},
					},
					{
						Operation: mock.OpDial,
						Params:    []string{mock.NewNodeID(2), mock.NewNodeID(5)},
					},
				},
			},
			1: {},
		},
	}
	executeTestCase(t, tc)
}

// TestValidatorConnExecutor_ValidatorUpdatesSequence checks sequence of multiple validators switched
func TestValidatorConnExecutor_ValidatorUpdatesSequence(t *testing.T) {
	me := mock.NewValidator(mySeedID)
	tc := testCase{
		me: me,
		validatorUpdates: []validatorUpdate{
			0: {
				validators: []*types.Validator{me,
					mock.NewValidator(1),
					mock.NewValidator(2),
					mock.NewValidator(3),
					mock.NewValidator(4),
				},
				expectedHistory: []mock.HistoryEvent{
					{Operation: mock.OpDial, Params: []string{mock.NewNodeID(1), mock.NewNodeID(2)}},
					{Operation: mock.OpDial, Params: []string{mock.NewNodeID(1), mock.NewNodeID(2)}},
				},
			},
			1: {
				validators: []*types.Validator{
					me,
					mock.NewValidator(2),
					mock.NewValidator(3),
					mock.NewValidator(4),
					mock.NewValidator(5),
				},
				expectedHistory: []mock.HistoryEvent{
					{
						Operation: mock.OpStop,
						Params:    []string{mock.NewNodeID(1)},
					},
					{
						Operation: mock.OpDial,
						Params:    []string{mock.NewNodeID(5)},
					},
				},
			},
			2: { // the same validator set as above, nothing should happen
				validators: []*types.Validator{
					me,
					mock.NewValidator(2),
					mock.NewValidator(3),
					mock.NewValidator(4),
					mock.NewValidator(5),
				},
				expectedHistory: []mock.HistoryEvent{},
			},
			3: { // only 1 validator (except myself), we should stop other validators
				validators: []*types.Validator{
					me,
					mock.NewValidator(1),
				},
				expectedHistory: []mock.HistoryEvent{
					0: {
						Operation: mock.OpStop,
						Params:    []string{mock.NewNodeID(2), mock.NewNodeID(5)},
					},
					1: {
						Operation: mock.OpStop,
						Params:    []string{mock.NewNodeID(2), mock.NewNodeID(5)},
					},
					2: {
						Operation: mock.OpDial,
						Params:    []string{mock.NewNodeID(1)},
					},
				},
			},
			4: { // everything stops
				validators: []*types.Validator{me},
				expectedHistory: []mock.HistoryEvent{
					0: {Operation: mock.OpStop, Params: []string{mock.NewNodeID(1)}},
				},
			},
			5: { // 20 validators; nothing dialed in previous round, so we just dial new validators
				validators: append(mock.NewValidators(20), me),
				expectedHistory: []mock.HistoryEvent{
					{
						Operation: mock.OpDial,
						Params:    []string{mock.NewNodeID(2), mock.NewNodeID(6), mock.NewNodeID(14), mock.NewNodeID(16)},
					},
					{
						Operation: mock.OpDial,
						Params:    []string{mock.NewNodeID(2), mock.NewNodeID(6), mock.NewNodeID(14), mock.NewNodeID(16)},
					},
					{
						Operation: mock.OpDial,
						Params:    []string{mock.NewNodeID(2), mock.NewNodeID(6), mock.NewNodeID(14), mock.NewNodeID(16)},
					},
					{
						Operation: mock.OpDial,
						Params:    []string{mock.NewNodeID(2), mock.NewNodeID(6), mock.NewNodeID(14), mock.NewNodeID(16)},
					},
				},
			},
		},
	}

	executeTestCase(t, tc)
}

// TestEndBlock verifies if ValidatorConnExecutor is called correctly during processing of EndBlock
// message from the ABCI app.
func TestEndBlock(t *testing.T) {
	const timeout = 3 * time.Second // how long we'll wait for connection
	app := newTestApp()

	clientCreator := abciclient.NewLocalCreator(app)
	require.NotNil(t, clientCreator)
	proxyApp := proxy.NewAppConns(clientCreator)
	require.NotNil(t, proxyApp)

	err := proxyApp.Start()
	require.Nil(t, err)
	defer proxyApp.Stop() //nolint:errcheck // ignore for tests

	state, stateDB, _ := makeState(3, 1)
	nodeProTxHash := state.Validators.Validators[0].ProTxHash
	stateStore := sm.NewStore(stateDB)
	blockStore := store.NewBlockStore(dbm.NewMemDB())

	blockExec := sm.NewBlockExecutor(
		stateStore,
		log.TestingLogger(),
		proxyApp.Consensus(),
		proxyApp.Query(),
		mmock.Mempool{},
		sm.EmptyEvidencePool{},
		blockStore,
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

	block := makeBlock(state, 1, new(types.Commit))
	blockID := types.BlockID{
		Hash:          block.Hash(),
		PartSetHeader: block.MakePartSet(testPartSize).Header(),
	}

	vals := state.Validators
	proTxHashes := vals.GetProTxHashes()
	addProTxHashes := make([]tmbytes.HexBytes, 0, 100)
	for i := 0; i < 100; i++ {
		addProTxHash := crypto.RandProTxHash()
		addProTxHashes = append(addProTxHashes, addProTxHash)
	}
	proTxHashes = append(proTxHashes, addProTxHashes...)
	newVals, _ := types.GenerateValidatorSet(types.NewValSetParam(proTxHashes))

	// Ensure new validators have some IP addresses set
	for _, validator := range newVals.Validators {
		validator.NodeAddress = types.RandValidatorAddress()
	}

	// setup ValidatorConnExecutor
	sw := mock.NewDashDialer()
	proTxHash := newVals.Validators[0].ProTxHash
	vc, err := NewValidatorConnExecutor(proTxHash, eventBus, sw)
	require.NoError(t, err)
	err = vc.Start()
	require.NoError(t, err)
	defer func() { err := vc.Stop(); require.NoError(t, err) }()

	app.ValidatorSetUpdates[1] = newVals.ABCIEquivalentValidatorUpdates()

	state, err = blockExec.ApplyBlock(state, nodeProTxHash, blockID, block)
	require.Nil(t, err)
	// test new validator was added to NextValidators
	require.Equal(t, state.Validators.Size()+100, state.NextValidators.Size())
	nextValidatorsProTxHashes := mock.ValidatorsProTxHashes(state.NextValidators.Validators)
	for _, addProTxHash := range addProTxHashes {
		assert.Contains(t, nextValidatorsProTxHashes, addProTxHash)
	}

	// test we threw an event
	select {
	case msg := <-updatesSub.Out():
		event, ok := msg.Data().(types.EventDataValidatorSetUpdate)
		require.True(
			t,
			ok,
			"Expected event of type EventDataValidatorSetUpdates, got %T",
			msg.Data(),
		)
		if assert.NotEmpty(t, event.ValidatorSetUpdates) {
			for _, addProTxHash := range addProTxHashes {
				assert.Contains(t, mock.ValidatorsProTxHashes(event.ValidatorSetUpdates), addProTxHash)
			}
			assert.EqualValues(
				t,
				types.DefaultDashVotingPower,
				event.ValidatorSetUpdates[1].VotingPower,
			)
			assert.NotEmpty(t, event.QuorumHash)
		}
	case <-updatesSub.Canceled():
		t.Fatalf("updatesSub was canceled (reason: %v)", updatesSub.Err())
	case <-time.After(1 * time.Second):
		t.Fatal("Did not receive EventValidatorSetUpdates within 1 sec.")
	}

	// ensure some history got generated inside the Switch; we expect 1 dial event
	select {
	case msg := <-sw.HistoryChan:
		t.Logf("Got message: %s %+v", msg.Operation, msg.Params)
		assert.EqualValues(t, mock.OpDial, msg.Operation)
	case <-time.After(timeout):
		t.Error("Timed out waiting for a history message")
		t.FailNow()
	}
}

// ****** utility functions ****** //

// executeTestCase feeds validator update messages into the event bus
// and ensures operations executed on mock Dash Connection Manager (history records)
// match `expectedHistory`, that is:
// * operation in history record is the same as in `expectedHistory`
// * params in history record are a subset of params in `expectedHistory`
func executeTestCase(t *testing.T, tc testCase) {
	// const TIMEOUT = 100 * time.Millisecond
	const TIMEOUT = 5 * time.Second

	eventBus, sw, vc := setup(t, tc.me)
	defer cleanup(t, eventBus, sw, vc)

	for updateID, update := range tc.validatorUpdates {
		updateEvent := types.EventDataValidatorSetUpdate{
			ValidatorSetUpdates: update.validators,
			QuorumHash:          mock.NewQuorumHash(1000),
		}
		err := eventBus.PublishEventValidatorSetUpdates(updateEvent)
		assert.NoError(t, err)

		// checks
		for checkID, check := range update.expectedHistory {
			select {
			case msg := <-sw.HistoryChan:
				// t.Logf("History event: %+v", msg)
				assert.EqualValues(
					t, check.Operation, msg.Operation,
					"Update %d: wrong operation %s in expected event %d",
					updateID, check.Operation, checkID,
				)
				allowedParams := check.Params
				// if params are nil, we default to all validator addresses; use []string{} to allow no addresses
				if allowedParams == nil {
					allowedParams = allowedParamsDefaults(t, tc, updateID, check, updateEvent.QuorumHash)
				}
				for _, param := range msg.Params {
					// Params of the call need to "contains" only these values as:
					// * we don't dial again already connected validators, and
					// * we randomly select a few validators from new validator set
					assert.Contains(
						t, allowedParams, param,
						"Update %d: wrong params in expected event %d, op %s",
						updateID, checkID, check.Operation,
					)
				}

				// assert.EqualValues(t, check.Params, msg.Params, "check %d", i)
			case <-time.After(TIMEOUT):
				t.Logf("Update %d: timed out waiting for history event %d: %+v", updateID, checkID, check)
				t.FailNow()
			}
		}

		// ensure no new history message arrives, eg. there are no additional operations done on the switch
		select {
		case msg := <-sw.HistoryChan:
			t.Errorf("unexpected history event for update=%d: %+v", updateID, msg)
		case <-time.After(50 * time.Millisecond):
			// this is correct - we time out
		}
	}

}

func allowedParamsDefaults(
	t *testing.T,
	tc testCase,
	updateID int,
	check mock.HistoryEvent,
	quorumHash tmbytes.HexBytes) []string {

	var (
		validators []*types.Validator
	)

	switch check.Operation {
	case mock.OpDial:
		validators = tc.validatorUpdates[updateID].validators
	case mock.OpStop:
		if updateID > 0 {
			validators = tc.validatorUpdates[updateID-1].validators
		}
	}

	selector := selectpeers.NewDIP6ValidatorSelector(quorumHash)
	allowedValidators, err := selector.SelectValidators(validators, tc.me)
	require.NoError(t, err)
	nodeIDs := newValidatorMap(allowedValidators).NodeIDs()
	return nodeIDs
}

// setup creates ValidatorConnExecutor and some dependencies.
// Use `defer cleanup()` to free the resources.
func setup(
	t *testing.T,
	me *types.Validator,
) (eventBus *types.EventBus, sw *mock.DashDialer, vc *ValidatorConnExecutor) {
	eventBus = types.NewEventBus()
	err := eventBus.Start()
	require.NoError(t, err)

	sw = mock.NewDashDialer()

	proTxHash := me.ProTxHash
	vc, err = NewValidatorConnExecutor(proTxHash, eventBus, sw, WithLogger(log.TestingLogger()))
	require.NoError(t, err)
	err = vc.Start()
	require.NoError(t, err)

	return eventBus, sw, vc
}

// cleanup frees some resources allocated for tests
func cleanup(t *testing.T, bus *types.EventBus, dialer p2p.DashDialer, vc *ValidatorConnExecutor) {
	assert.NoError(t, bus.Stop())
	assert.NoError(t, vc.Stop())
}

// SOME UTILS //

// make some bogus txs
func makeTxs(height int64) (txs []types.Tx) {
	for i := 0; i < nTxsPerBlock; i++ {
		txs = append(txs, types.Tx([]byte{byte(height), byte(i)}))
	}
	return txs
}

func makeState(nVals int, height int64) (sm.State, dbm.DB, map[string]types.PrivValidator) {
	privValsByProTxHash := make(map[string]types.PrivValidator, nVals)
	valSet, privVals := types.RandValidatorSet(nVals)
	for i := 0; i < nVals; i++ {
		validator := valSet.Validators[i]
		proTxHash := validator.ProTxHash
		privValsByProTxHash[proTxHash.String()] = privVals[i]
	}
	genVals := types.MakeGenesisValsFromValidatorSet(valSet)
	s, _ := sm.MakeGenesisState(&types.GenesisDoc{
		ChainID:            chainID,
		Validators:         genVals,
		ThresholdPublicKey: valSet.ThresholdPublicKey,
		QuorumHash:         valSet.QuorumHash,
		AppHash:            nil,
	})

	stateDB := dbm.NewMemDB()
	stateStore := sm.NewStore(stateDB)
	if err := stateStore.Save(s); err != nil {
		panic(err)
	}

	for i := int64(1); i < height; i++ {
		s.LastBlockHeight++
		s.LastValidators = s.Validators.Copy()
		if err := stateStore.Save(s); err != nil {
			panic(err)
		}
	}

	return s, stateDB, privValsByProTxHash
}

func makeBlock(state sm.State, height int64, commit *types.Commit) *types.Block {
	block, _ := state.MakeBlock(height, nil, makeTxs(state.LastBlockHeight),
		commit, nil, state.Validators.GetProposer().ProTxHash, 0)
	return block
}

// TEST APP //

// testApp which changes validators according to updates defined in testApp.ValidatorSetUpdates
type testApp struct {
	abci.BaseApplication

	ByzantineValidators []abci.Evidence
	ValidatorSetUpdates map[int64]*abci.ValidatorSetUpdate
}

func newTestApp() *testApp {
	return &testApp{
		ByzantineValidators: []abci.Evidence{},
		ValidatorSetUpdates: map[int64]*abci.ValidatorSetUpdate{},
	}
}

var _ abci.Application = (*testApp)(nil)

func (app *testApp) Info(req abci.RequestInfo) (resInfo abci.ResponseInfo) {
	return abci.ResponseInfo{}
}

func (app *testApp) BeginBlock(req abci.RequestBeginBlock) abci.ResponseBeginBlock {
	app.ByzantineValidators = req.ByzantineValidators
	return abci.ResponseBeginBlock{}
}

func (app *testApp) EndBlock(req abci.RequestEndBlock) abci.ResponseEndBlock {
	return abci.ResponseEndBlock{
		ValidatorSetUpdate: app.ValidatorSetUpdates[req.Height],
		ConsensusParamUpdates: &tmproto.ConsensusParams{
			Version: &tmproto.VersionParams{
				AppVersion: 1}}}
}

func (app *testApp) DeliverTx(req abci.RequestDeliverTx) abci.ResponseDeliverTx {
	return abci.ResponseDeliverTx{Events: []abci.Event{}}
}

func (app *testApp) CheckTx(req abci.RequestCheckTx) abci.ResponseCheckTx {
	return abci.ResponseCheckTx{}
}

func (app *testApp) Commit() abci.ResponseCommit {
	return abci.ResponseCommit{RetainHeight: 1}
}

func (app *testApp) Query(reqQuery abci.RequestQuery) (resQuery abci.ResponseQuery) {
	return
}
