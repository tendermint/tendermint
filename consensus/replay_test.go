package consensus

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sort"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/mock"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tendermint/version"
	dbm "github.com/tendermint/tm-db"
)

func TestMain(m *testing.M) {
	config = ResetConfig("consensus_reactor_test")
	consensusReplayConfig = ResetConfig("consensus_replay_test")
	configStateTest := ResetConfig("consensus_state_test")
	configMempoolTest := ResetConfig("consensus_mempool_test")
	configByzantineTest := ResetConfig("consensus_byzantine_test")
	code := m.Run()
	os.RemoveAll(config.RootDir)
	os.RemoveAll(consensusReplayConfig.RootDir)
	os.RemoveAll(configStateTest.RootDir)
	os.RemoveAll(configMempoolTest.RootDir)
	os.RemoveAll(configByzantineTest.RootDir)
	os.Exit(code)
}

// These tests ensure we can always recover from failure at any part of the consensus process.
// There are two general failure scenarios: failure during consensus, and failure while applying the block.
// Only the latter interacts with the app and store,
// but the former has to deal with restrictions on re-use of priv_validator keys.
// The `WAL Tests` are for failures during the consensus;
// the `Handshake Tests` are for failures in applying the block.
// With the help of the WAL, we can recover from it all!

//------------------------------------------------------------------------------------------
// WAL Tests

// TODO: It would be better to verify explicitly which states we can recover from without the wal
// and which ones we need the wal for - then we'd also be able to only flush the
// wal writer when we need to, instead of with every message.

func startNewConsensusStateAndWaitForBlock(t *testing.T, consensusReplayConfig *cfg.Config,
	lastBlockHeight int64, blockDB dbm.DB, stateDB dbm.DB) {
	logger := log.TestingLogger()
	state, _ := sm.LoadStateFromDBOrGenesisFile(stateDB, consensusReplayConfig.GenesisFile())
	privValidator := loadPrivValidator(consensusReplayConfig)
	cs := newConsensusStateWithConfigAndBlockStore(consensusReplayConfig, state, privValidator, kvstore.NewKVStoreApplication(), blockDB)
	cs.SetLogger(logger)

	bytes, _ := ioutil.ReadFile(cs.config.WalFile())
	t.Logf("====== WAL: \n\r%X\n", bytes)

	err := cs.Start()
	require.NoError(t, err)
	defer cs.Stop()

	// This is just a signal that we haven't halted; its not something contained
	// in the WAL itself. Assuming the consensus state is running, replay of any
	// WAL, including the empty one, should eventually be followed by a new
	// block, or else something is wrong.
	newBlockSub, err := cs.eventBus.Subscribe(context.Background(), testSubscriber, types.EventQueryNewBlock)
	require.NoError(t, err)
	select {
	case <-newBlockSub.Out():
	case <-newBlockSub.Cancelled():
		t.Fatal("newBlockSub was cancelled")
	case <-time.After(120 * time.Second):
		t.Fatal("Timed out waiting for new block (see trace above)")
	}
}

func sendTxs(ctx context.Context, cs *ConsensusState) {
	for i := 0; i < 256; i++ {
		select {
		case <-ctx.Done():
			return
		default:
			tx := []byte{byte(i)}
			assertMempool(cs.txNotifier).CheckTx(tx, nil)
			i++
		}
	}
}

// TestWALCrash uses crashing WAL to test we can recover from any WAL failure.
func TestWALCrash(t *testing.T) {
	testCases := []struct {
		name         string
		initFn       func(dbm.DB, *ConsensusState, context.Context)
		heightToStop int64
	}{
		{"empty block",
			func(stateDB dbm.DB, cs *ConsensusState, ctx context.Context) {},
			1},
		{"many non-empty blocks",
			func(stateDB dbm.DB, cs *ConsensusState, ctx context.Context) {
				go sendTxs(ctx, cs)
			},
			3},
	}

	for i, tc := range testCases {
		tc := tc
		consensusReplayConfig := ResetConfig(fmt.Sprintf("%s_%d", t.Name(), i))
		t.Run(tc.name, func(t *testing.T) {
			crashWALandCheckLiveness(t, consensusReplayConfig, tc.initFn, tc.heightToStop)
		})
	}
}

func crashWALandCheckLiveness(t *testing.T, consensusReplayConfig *cfg.Config,
	initFn func(dbm.DB, *ConsensusState, context.Context), heightToStop int64) {
	walPanicked := make(chan error)
	crashingWal := &crashingWAL{panicCh: walPanicked, heightToStop: heightToStop}

	i := 1
LOOP:
	for {
		t.Logf("====== LOOP %d\n", i)

		// create consensus state from a clean slate
		logger := log.NewNopLogger()
		blockDB := dbm.NewMemDB()
		stateDB := blockDB
		state, _ := sm.MakeGenesisStateFromFile(consensusReplayConfig.GenesisFile())
		privValidator := loadPrivValidator(consensusReplayConfig)
		cs := newConsensusStateWithConfigAndBlockStore(consensusReplayConfig, state, privValidator, kvstore.NewKVStoreApplication(), blockDB)
		cs.SetLogger(logger)

		// start sending transactions
		ctx, cancel := context.WithCancel(context.Background())
		initFn(stateDB, cs, ctx)

		// clean up WAL file from the previous iteration
		walFile := cs.config.WalFile()
		os.Remove(walFile)

		// set crashing WAL
		csWal, err := cs.OpenWAL(walFile)
		require.NoError(t, err)
		crashingWal.next = csWal
		// reset the message counter
		crashingWal.msgIndex = 1
		cs.wal = crashingWal

		// start consensus state
		err = cs.Start()
		require.NoError(t, err)

		i++

		select {
		case err := <-walPanicked:
			t.Logf("WAL panicked: %v", err)

			// make sure we can make blocks after a crash
			startNewConsensusStateAndWaitForBlock(t, consensusReplayConfig, cs.Height, blockDB, stateDB)

			// stop consensus state and transactions sender (initFn)
			cs.Stop()
			cancel()

			// if we reached the required height, exit
			if _, ok := err.(ReachedHeightToStopError); ok {
				break LOOP
			}
		case <-time.After(10 * time.Second):
			t.Fatal("WAL did not panic for 10 seconds (check the log)")
		}
	}
}

// crashingWAL is a WAL which crashes or rather simulates a crash during Save
// (before and after). It remembers a message for which we last panicked
// (lastPanickedForMsgIndex), so we don't panic for it in subsequent iterations.
type crashingWAL struct {
	next         WAL
	panicCh      chan error
	heightToStop int64

	msgIndex                int // current message index
	lastPanickedForMsgIndex int // last message for which we panicked
}

var _ WAL = &crashingWAL{}

// WALWriteError indicates a WAL crash.
type WALWriteError struct {
	msg string
}

func (e WALWriteError) Error() string {
	return e.msg
}

// ReachedHeightToStopError indicates we've reached the required consensus
// height and may exit.
type ReachedHeightToStopError struct {
	height int64
}

func (e ReachedHeightToStopError) Error() string {
	return fmt.Sprintf("reached height to stop %d", e.height)
}

// Write simulate WAL's crashing by sending an error to the panicCh and then
// exiting the cs.receiveRoutine.
func (w *crashingWAL) Write(m WALMessage) error {
	if endMsg, ok := m.(EndHeightMessage); ok {
		if endMsg.Height == w.heightToStop {
			w.panicCh <- ReachedHeightToStopError{endMsg.Height}
			runtime.Goexit()
			return nil
		}

		return w.next.Write(m)
	}

	if w.msgIndex > w.lastPanickedForMsgIndex {
		w.lastPanickedForMsgIndex = w.msgIndex
		_, file, line, _ := runtime.Caller(1)
		w.panicCh <- WALWriteError{fmt.Sprintf("failed to write %T to WAL (fileline: %s:%d)", m, file, line)}
		runtime.Goexit()
		return nil
	}

	w.msgIndex++
	return w.next.Write(m)
}

func (w *crashingWAL) WriteSync(m WALMessage) error {
	return w.Write(m)
}

func (w *crashingWAL) FlushAndSync() error { return w.next.FlushAndSync() }

func (w *crashingWAL) SearchForEndHeight(height int64, options *WALSearchOptions) (rd io.ReadCloser, found bool, err error) {
	return w.next.SearchForEndHeight(height, options)
}

func (w *crashingWAL) Start() error { return w.next.Start() }
func (w *crashingWAL) Stop() error  { return w.next.Stop() }
func (w *crashingWAL) Wait()        { w.next.Wait() }

//------------------------------------------------------------------------------------------
type testSim struct {
	GenesisState sm.State
	Config       *cfg.Config
	Chain        []*types.Block
	Commits      []*types.Commit
	CleanupFunc  cleanupFunc
}

const (
	numBlocks = 6
)

var (
	mempool = mock.Mempool{}
	evpool  = sm.MockEvidencePool{}

	sim testSim
)

//---------------------------------------
// Test handshake/replay

// 0 - all synced up
// 1 - saved block but app and state are behind
// 2 - save block and committed but state is behind
var modes = []uint{0, 1, 2}

// This is actually not a test, it's for storing validator change tx data for testHandshakeReplay
func TestSimulateValidatorsChange(t *testing.T) {
	nPeers := 7
	nVals := 4
	css, genDoc, config, cleanup := randConsensusNetWithPeers(nVals, nPeers, "replay_test", newMockTickerFunc(true), newPersistentKVStoreWithPath)
	sim.Config = config
	sim.GenesisState, _ = sm.MakeGenesisState(genDoc)
	sim.CleanupFunc = cleanup

	partSize := types.BlockPartSizeBytes

	newRoundCh := subscribe(css[0].eventBus, types.EventQueryNewRound)
	proposalCh := subscribe(css[0].eventBus, types.EventQueryCompleteProposal)

	vss := make([]*validatorStub, nPeers)
	for i := 0; i < nPeers; i++ {
		vss[i] = NewValidatorStub(css[i].privValidator, i)
	}
	height, round := css[0].Height, css[0].Round
	// start the machine
	startTestRound(css[0], height, round)
	incrementHeight(vss...)
	ensureNewRound(newRoundCh, height, 0)
	ensureNewProposal(proposalCh, height, round)
	rs := css[0].GetRoundState()
	signAddVotes(css[0], types.PrecommitType, rs.ProposalBlock.Hash(), rs.ProposalBlockParts.Header(), vss[1:nVals]...)
	ensureNewRound(newRoundCh, height+1, 0)

	//height 2
	height++
	incrementHeight(vss...)
	newValidatorPubKey1 := css[nVals].privValidator.GetPubKey()
	valPubKey1ABCI := types.TM2PB.PubKey(newValidatorPubKey1)
	newValidatorTx1 := kvstore.MakeValSetChangeTx(valPubKey1ABCI, testMinPower)
	err := assertMempool(css[0].txNotifier).CheckTx(newValidatorTx1, nil)
	assert.Nil(t, err)
	propBlock, _ := css[0].createProposalBlock() //changeProposer(t, cs1, vs2)
	propBlockParts := propBlock.MakePartSet(partSize)
	blockID := types.BlockID{Hash: propBlock.Hash(), PartsHeader: propBlockParts.Header()}
	proposal := types.NewProposal(vss[1].Height, round, -1, blockID)
	if err := vss[1].SignProposal(config.ChainID(), proposal); err != nil {
		t.Fatal("failed to sign bad proposal", err)
	}

	// set the proposal block
	if err := css[0].SetProposalAndBlock(proposal, propBlock, propBlockParts, "some peer"); err != nil {
		t.Fatal(err)
	}
	ensureNewProposal(proposalCh, height, round)
	rs = css[0].GetRoundState()
	signAddVotes(css[0], types.PrecommitType, rs.ProposalBlock.Hash(), rs.ProposalBlockParts.Header(), vss[1:nVals]...)
	ensureNewRound(newRoundCh, height+1, 0)

	//height 3
	height++
	incrementHeight(vss...)
	updateValidatorPubKey1 := css[nVals].privValidator.GetPubKey()
	updatePubKey1ABCI := types.TM2PB.PubKey(updateValidatorPubKey1)
	updateValidatorTx1 := kvstore.MakeValSetChangeTx(updatePubKey1ABCI, 25)
	err = assertMempool(css[0].txNotifier).CheckTx(updateValidatorTx1, nil)
	assert.Nil(t, err)
	propBlock, _ = css[0].createProposalBlock() //changeProposer(t, cs1, vs2)
	propBlockParts = propBlock.MakePartSet(partSize)
	blockID = types.BlockID{Hash: propBlock.Hash(), PartsHeader: propBlockParts.Header()}
	proposal = types.NewProposal(vss[2].Height, round, -1, blockID)
	if err := vss[2].SignProposal(config.ChainID(), proposal); err != nil {
		t.Fatal("failed to sign bad proposal", err)
	}

	// set the proposal block
	if err := css[0].SetProposalAndBlock(proposal, propBlock, propBlockParts, "some peer"); err != nil {
		t.Fatal(err)
	}
	ensureNewProposal(proposalCh, height, round)
	rs = css[0].GetRoundState()
	signAddVotes(css[0], types.PrecommitType, rs.ProposalBlock.Hash(), rs.ProposalBlockParts.Header(), vss[1:nVals]...)
	ensureNewRound(newRoundCh, height+1, 0)

	//height 4
	height++
	incrementHeight(vss...)
	newValidatorPubKey2 := css[nVals+1].privValidator.GetPubKey()
	newVal2ABCI := types.TM2PB.PubKey(newValidatorPubKey2)
	newValidatorTx2 := kvstore.MakeValSetChangeTx(newVal2ABCI, testMinPower)
	err = assertMempool(css[0].txNotifier).CheckTx(newValidatorTx2, nil)
	assert.Nil(t, err)
	newValidatorPubKey3 := css[nVals+2].privValidator.GetPubKey()
	newVal3ABCI := types.TM2PB.PubKey(newValidatorPubKey3)
	newValidatorTx3 := kvstore.MakeValSetChangeTx(newVal3ABCI, testMinPower)
	err = assertMempool(css[0].txNotifier).CheckTx(newValidatorTx3, nil)
	assert.Nil(t, err)
	propBlock, _ = css[0].createProposalBlock() //changeProposer(t, cs1, vs2)
	propBlockParts = propBlock.MakePartSet(partSize)
	blockID = types.BlockID{Hash: propBlock.Hash(), PartsHeader: propBlockParts.Header()}
	newVss := make([]*validatorStub, nVals+1)
	copy(newVss, vss[:nVals+1])
	sort.Sort(ValidatorStubsByAddress(newVss))
	selfIndex := 0
	for i, vs := range newVss {
		if vs.GetPubKey().Equals(css[0].privValidator.GetPubKey()) {
			selfIndex = i
			break
		}
	}

	proposal = types.NewProposal(vss[3].Height, round, -1, blockID)
	if err := vss[3].SignProposal(config.ChainID(), proposal); err != nil {
		t.Fatal("failed to sign bad proposal", err)
	}

	// set the proposal block
	if err := css[0].SetProposalAndBlock(proposal, propBlock, propBlockParts, "some peer"); err != nil {
		t.Fatal(err)
	}
	ensureNewProposal(proposalCh, height, round)

	removeValidatorTx2 := kvstore.MakeValSetChangeTx(newVal2ABCI, 0)
	err = assertMempool(css[0].txNotifier).CheckTx(removeValidatorTx2, nil)
	assert.Nil(t, err)

	rs = css[0].GetRoundState()
	for i := 0; i < nVals+1; i++ {
		if i == selfIndex {
			continue
		}
		signAddVotes(css[0], types.PrecommitType, rs.ProposalBlock.Hash(), rs.ProposalBlockParts.Header(), newVss[i])
	}

	ensureNewRound(newRoundCh, height+1, 0)

	//height 5
	height++
	incrementHeight(vss...)
	ensureNewProposal(proposalCh, height, round)
	rs = css[0].GetRoundState()
	for i := 0; i < nVals+1; i++ {
		if i == selfIndex {
			continue
		}
		signAddVotes(css[0], types.PrecommitType, rs.ProposalBlock.Hash(), rs.ProposalBlockParts.Header(), newVss[i])
	}
	ensureNewRound(newRoundCh, height+1, 0)

	//height 6
	height++
	incrementHeight(vss...)
	removeValidatorTx3 := kvstore.MakeValSetChangeTx(newVal3ABCI, 0)
	err = assertMempool(css[0].txNotifier).CheckTx(removeValidatorTx3, nil)
	assert.Nil(t, err)
	propBlock, _ = css[0].createProposalBlock() //changeProposer(t, cs1, vs2)
	propBlockParts = propBlock.MakePartSet(partSize)
	blockID = types.BlockID{Hash: propBlock.Hash(), PartsHeader: propBlockParts.Header()}
	newVss = make([]*validatorStub, nVals+3)
	copy(newVss, vss[:nVals+3])
	sort.Sort(ValidatorStubsByAddress(newVss))
	for i, vs := range newVss {
		if vs.GetPubKey().Equals(css[0].privValidator.GetPubKey()) {
			selfIndex = i
			break
		}
	}
	proposal = types.NewProposal(vss[1].Height, round, -1, blockID)
	if err := vss[1].SignProposal(config.ChainID(), proposal); err != nil {
		t.Fatal("failed to sign bad proposal", err)
	}

	// set the proposal block
	if err := css[0].SetProposalAndBlock(proposal, propBlock, propBlockParts, "some peer"); err != nil {
		t.Fatal(err)
	}
	ensureNewProposal(proposalCh, height, round)
	rs = css[0].GetRoundState()
	for i := 0; i < nVals+3; i++ {
		if i == selfIndex {
			continue
		}
		signAddVotes(css[0], types.PrecommitType, rs.ProposalBlock.Hash(), rs.ProposalBlockParts.Header(), newVss[i])
	}
	ensureNewRound(newRoundCh, height+1, 0)

	sim.Chain = make([]*types.Block, 0)
	sim.Commits = make([]*types.Commit, 0)
	for i := 1; i <= numBlocks; i++ {
		sim.Chain = append(sim.Chain, css[0].blockStore.LoadBlock(int64(i)))
		sim.Commits = append(sim.Commits, css[0].blockStore.LoadBlockCommit(int64(i)))
	}
}

// Sync from scratch
func TestHandshakeReplayAll(t *testing.T) {
	for _, m := range modes {
		testHandshakeReplay(t, config, 0, m, false)
	}
	for _, m := range modes {
		testHandshakeReplay(t, config, 0, m, true)
	}
}

// Sync many, not from scratch
func TestHandshakeReplaySome(t *testing.T) {
	for _, m := range modes {
		testHandshakeReplay(t, config, 1, m, false)
	}
	for _, m := range modes {
		testHandshakeReplay(t, config, 1, m, true)
	}
}

// Sync from lagging by one
func TestHandshakeReplayOne(t *testing.T) {
	for _, m := range modes {
		testHandshakeReplay(t, config, numBlocks-1, m, false)
	}
	for _, m := range modes {
		testHandshakeReplay(t, config, numBlocks-1, m, true)
	}
}

// Sync from caught up
func TestHandshakeReplayNone(t *testing.T) {
	for _, m := range modes {
		testHandshakeReplay(t, config, numBlocks, m, false)
	}
	for _, m := range modes {
		testHandshakeReplay(t, config, numBlocks, m, true)
	}
}

// Test mockProxyApp should not panic when app return ABCIResponses with some empty ResponseDeliverTx
func TestMockProxyApp(t *testing.T) {
	sim.CleanupFunc() //clean the test env created in TestSimulateValidatorsChange
	logger := log.TestingLogger()
	var validTxs, invalidTxs = 0, 0
	txIndex := 0

	assert.NotPanics(t, func() {
		abciResWithEmptyDeliverTx := new(sm.ABCIResponses)
		abciResWithEmptyDeliverTx.DeliverTx = make([]*abci.ResponseDeliverTx, 0)
		abciResWithEmptyDeliverTx.DeliverTx = append(abciResWithEmptyDeliverTx.DeliverTx, &abci.ResponseDeliverTx{})

		// called when saveABCIResponses:
		bytes := cdc.MustMarshalBinaryBare(abciResWithEmptyDeliverTx)
		loadedAbciRes := new(sm.ABCIResponses)

		// this also happens sm.LoadABCIResponses
		err := cdc.UnmarshalBinaryBare(bytes, loadedAbciRes)
		require.NoError(t, err)

		mock := newMockProxyApp([]byte("mock_hash"), loadedAbciRes)

		abciRes := new(sm.ABCIResponses)
		abciRes.DeliverTx = make([]*abci.ResponseDeliverTx, len(loadedAbciRes.DeliverTx))
		// Execute transactions and get hash.
		proxyCb := func(req *abci.Request, res *abci.Response) {
			if r, ok := res.Value.(*abci.Response_DeliverTx); ok {
				// TODO: make use of res.Log
				// TODO: make use of this info
				// Blocks may include invalid txs.
				txRes := r.DeliverTx
				if txRes.Code == abci.CodeTypeOK {
					validTxs++
				} else {
					logger.Debug("Invalid tx", "code", txRes.Code, "log", txRes.Log)
					invalidTxs++
				}
				abciRes.DeliverTx[txIndex] = txRes
				txIndex++
			}
		}
		mock.SetResponseCallback(proxyCb)

		someTx := []byte("tx")
		mock.DeliverTxAsync(abci.RequestDeliverTx{Tx: someTx})
	})
	assert.True(t, validTxs == 1)
	assert.True(t, invalidTxs == 0)
}

func tempWALWithData(data []byte) string {
	walFile, err := ioutil.TempFile("", "wal")
	if err != nil {
		panic(fmt.Sprintf("failed to create temp WAL file: %v", err))
	}
	_, err = walFile.Write(data)
	if err != nil {
		panic(fmt.Sprintf("failed to write to temp WAL file: %v", err))
	}
	if err := walFile.Close(); err != nil {
		panic(fmt.Sprintf("failed to close temp WAL file: %v", err))
	}
	return walFile.Name()
}

// Make some blocks. Start a fresh app and apply nBlocks blocks. Then restart the app and sync it up with the remaining blocks
func testHandshakeReplay(t *testing.T, config *cfg.Config, nBlocks int, mode uint, testValidatorsChange bool) {
	var chain []*types.Block
	var commits []*types.Commit
	var store *mockBlockStore
	var stateDB dbm.DB
	var genisisState sm.State
	if testValidatorsChange {
		testConfig := ResetConfig(fmt.Sprintf("%s_%v_m", t.Name(), mode))
		defer os.RemoveAll(testConfig.RootDir)
		stateDB = dbm.NewMemDB()
		genisisState = sim.GenesisState
		config = sim.Config
		chain = sim.Chain
		commits = sim.Commits
		store = newMockBlockStore(config, genisisState.ConsensusParams)
	} else { //test single node
		testConfig := ResetConfig(fmt.Sprintf("%s_%v_s", t.Name(), mode))
		defer os.RemoveAll(testConfig.RootDir)
		walBody, err := WALWithNBlocks(t, numBlocks)
		require.NoError(t, err)
		walFile := tempWALWithData(walBody)
		config.Consensus.SetWalFile(walFile)

		privVal := privval.LoadFilePV(config.PrivValidatorKeyFile(), config.PrivValidatorStateFile())

		wal, err := NewWAL(walFile)
		require.NoError(t, err)
		wal.SetLogger(log.TestingLogger())
		err = wal.Start()
		require.NoError(t, err)
		defer wal.Stop()

		chain, commits, err = makeBlockchainFromWAL(wal)
		require.NoError(t, err)
		stateDB, genisisState, store = stateAndStore(config, privVal.GetPubKey(), kvstore.ProtocolVersion)
	}
	store.chain = chain
	store.commits = commits

	state := genisisState.Copy()
	// run the chain through state.ApplyBlock to build up the tendermint state
	state = buildTMStateFromChain(config, stateDB, state, chain, nBlocks, mode)
	latestAppHash := state.AppHash

	// make a new client creator
	kvstoreApp := kvstore.NewPersistentKVStoreApplication(filepath.Join(config.DBDir(), fmt.Sprintf("replay_test_%d_%d_a", nBlocks, mode)))

	clientCreator2 := proxy.NewLocalClientCreator(kvstoreApp)
	if nBlocks > 0 {
		// run nBlocks against a new client to build up the app state.
		// use a throwaway tendermint state
		proxyApp := proxy.NewAppConns(clientCreator2)
		stateDB1 := dbm.NewMemDB()
		sm.SaveState(stateDB1, genisisState)
		buildAppStateFromChain(proxyApp, stateDB1, genisisState, chain, nBlocks, mode)
	}

	// now start the app using the handshake - it should sync
	genDoc, _ := sm.MakeGenesisDocFromFile(config.GenesisFile())
	handshaker := NewHandshaker(stateDB, state, store, genDoc)
	proxyApp := proxy.NewAppConns(clientCreator2)
	if err := proxyApp.Start(); err != nil {
		t.Fatalf("Error starting proxy app connections: %v", err)
	}
	defer proxyApp.Stop()
	if err := handshaker.Handshake(proxyApp); err != nil {
		t.Fatalf("Error on abci handshake: %v", err)
	}

	// get the latest app hash from the app
	res, err := proxyApp.Query().InfoSync(abci.RequestInfo{Version: ""})
	if err != nil {
		t.Fatal(err)
	}

	// the app hash should be synced up
	if !bytes.Equal(latestAppHash, res.LastBlockAppHash) {
		t.Fatalf("Expected app hashes to match after handshake/replay. got %X, expected %X", res.LastBlockAppHash, latestAppHash)
	}

	expectedBlocksToSync := numBlocks - nBlocks
	if nBlocks == numBlocks && mode > 0 {
		expectedBlocksToSync++
	} else if nBlocks > 0 && mode == 1 {
		expectedBlocksToSync++
	}

	if handshaker.NBlocks() != expectedBlocksToSync {
		t.Fatalf("Expected handshake to sync %d blocks, got %d", expectedBlocksToSync, handshaker.NBlocks())
	}
}

func applyBlock(stateDB dbm.DB, st sm.State, blk *types.Block, proxyApp proxy.AppConns) sm.State {
	testPartSize := types.BlockPartSizeBytes
	blockExec := sm.NewBlockExecutor(stateDB, log.TestingLogger(), proxyApp.Consensus(), mempool, evpool)

	blkID := types.BlockID{Hash: blk.Hash(), PartsHeader: blk.MakePartSet(testPartSize).Header()}
	newState, err := blockExec.ApplyBlock(st, blkID, blk)
	if err != nil {
		panic(err)
	}
	return newState
}

func buildAppStateFromChain(proxyApp proxy.AppConns, stateDB dbm.DB,
	state sm.State, chain []*types.Block, nBlocks int, mode uint) {
	// start a new app without handshake, play nBlocks blocks
	if err := proxyApp.Start(); err != nil {
		panic(err)
	}
	defer proxyApp.Stop()

	state.Version.Consensus.App = kvstore.ProtocolVersion //simulate handshake, receive app version
	validators := types.TM2PB.ValidatorUpdates(state.Validators)
	if _, err := proxyApp.Consensus().InitChainSync(abci.RequestInitChain{
		Validators: validators,
	}); err != nil {
		panic(err)
	}
	sm.SaveState(stateDB, state) //save height 1's validatorsInfo

	switch mode {
	case 0:
		for i := 0; i < nBlocks; i++ {
			block := chain[i]
			state = applyBlock(stateDB, state, block, proxyApp)
		}
	case 1, 2:
		for i := 0; i < nBlocks-1; i++ {
			block := chain[i]
			state = applyBlock(stateDB, state, block, proxyApp)
		}

		if mode == 2 {
			// update the kvstore height and apphash
			// as if we ran commit but not
			state = applyBlock(stateDB, state, chain[nBlocks-1], proxyApp)
		}
	}

}

func buildTMStateFromChain(config *cfg.Config, stateDB dbm.DB, state sm.State, chain []*types.Block, nBlocks int, mode uint) sm.State {
	// run the whole chain against this client to build up the tendermint state
	clientCreator := proxy.NewLocalClientCreator(kvstore.NewPersistentKVStoreApplication(filepath.Join(config.DBDir(), fmt.Sprintf("replay_test_%d_%d_t", nBlocks, mode))))
	proxyApp := proxy.NewAppConns(clientCreator)
	if err := proxyApp.Start(); err != nil {
		panic(err)
	}
	defer proxyApp.Stop()

	state.Version.Consensus.App = kvstore.ProtocolVersion //simulate handshake, receive app version
	validators := types.TM2PB.ValidatorUpdates(state.Validators)
	if _, err := proxyApp.Consensus().InitChainSync(abci.RequestInitChain{
		Validators: validators,
	}); err != nil {
		panic(err)
	}
	sm.SaveState(stateDB, state) //save height 1's validatorsInfo

	switch mode {
	case 0:
		// sync right up
		for _, block := range chain {
			state = applyBlock(stateDB, state, block, proxyApp)
		}

	case 1, 2:
		// sync up to the penultimate as if we stored the block.
		// whether we commit or not depends on the appHash
		for _, block := range chain[:len(chain)-1] {
			state = applyBlock(stateDB, state, block, proxyApp)
		}

		// apply the final block to a state copy so we can
		// get the right next appHash but keep the state back
		applyBlock(stateDB, state, chain[len(chain)-1], proxyApp)
	}

	return state
}

func TestHandshakePanicsIfAppReturnsWrongAppHash(t *testing.T) {
	// 1. Initialize tendermint and commit 3 blocks with the following app hashes:
	//		- 0x01
	//		- 0x02
	//		- 0x03
	config := ResetConfig("handshake_test_")
	defer os.RemoveAll(config.RootDir)
	privVal := privval.LoadFilePV(config.PrivValidatorKeyFile(), config.PrivValidatorStateFile())
	const appVersion = 0x0
	stateDB, state, store := stateAndStore(config, privVal.GetPubKey(), appVersion)
	genDoc, _ := sm.MakeGenesisDocFromFile(config.GenesisFile())
	state.LastValidators = state.Validators.Copy()
	// mode = 0 for committing all the blocks
	blocks := makeBlocks(3, &state, privVal)
	store.chain = blocks

	// 2. Tendermint must panic if app returns wrong hash for the first block
	//		- RANDOM HASH
	//		- 0x02
	//		- 0x03
	{
		app := &badApp{numBlocks: 3, allHashesAreWrong: true}
		clientCreator := proxy.NewLocalClientCreator(app)
		proxyApp := proxy.NewAppConns(clientCreator)
		err := proxyApp.Start()
		require.NoError(t, err)
		defer proxyApp.Stop()

		assert.Panics(t, func() {
			h := NewHandshaker(stateDB, state, store, genDoc)
			h.Handshake(proxyApp)
		})
	}

	// 3. Tendermint must panic if app returns wrong hash for the last block
	//		- 0x01
	//		- 0x02
	//		- RANDOM HASH
	{
		app := &badApp{numBlocks: 3, onlyLastHashIsWrong: true}
		clientCreator := proxy.NewLocalClientCreator(app)
		proxyApp := proxy.NewAppConns(clientCreator)
		err := proxyApp.Start()
		require.NoError(t, err)
		defer proxyApp.Stop()

		assert.Panics(t, func() {
			h := NewHandshaker(stateDB, state, store, genDoc)
			h.Handshake(proxyApp)
		})
	}
}

func makeBlocks(n int, state *sm.State, privVal types.PrivValidator) []*types.Block {
	blocks := make([]*types.Block, 0)

	var (
		prevBlock     *types.Block
		prevBlockMeta *types.BlockMeta
	)

	appHeight := byte(0x01)
	for i := 0; i < n; i++ {
		height := int64(i + 1)

		block, parts := makeBlock(*state, prevBlock, prevBlockMeta, privVal, height)
		blocks = append(blocks, block)

		prevBlock = block
		prevBlockMeta = types.NewBlockMeta(block, parts)

		// update state
		state.AppHash = []byte{appHeight}
		appHeight++
		state.LastBlockHeight = height
	}

	return blocks
}

func makeBlock(state sm.State, lastBlock *types.Block, lastBlockMeta *types.BlockMeta,
	privVal types.PrivValidator, height int64) (*types.Block, *types.PartSet) {

	lastCommit := types.NewCommit(types.BlockID{}, nil)
	if height > 1 {
		vote, _ := types.MakeVote(lastBlock.Header.Height, lastBlockMeta.BlockID, state.Validators, privVal, lastBlock.Header.ChainID)
		voteCommitSig := vote.CommitSig()
		lastCommit = types.NewCommit(lastBlockMeta.BlockID, []*types.CommitSig{voteCommitSig})
	}

	return state.MakeBlock(height, []types.Tx{}, lastCommit, nil, state.Validators.GetProposer().Address)
}

type badApp struct {
	abci.BaseApplication
	numBlocks           byte
	height              byte
	allHashesAreWrong   bool
	onlyLastHashIsWrong bool
}

func (app *badApp) Commit() abci.ResponseCommit {
	app.height++
	if app.onlyLastHashIsWrong {
		if app.height == app.numBlocks {
			return abci.ResponseCommit{Data: cmn.RandBytes(8)}
		}
		return abci.ResponseCommit{Data: []byte{app.height}}
	} else if app.allHashesAreWrong {
		return abci.ResponseCommit{Data: cmn.RandBytes(8)}
	}

	panic("either allHashesAreWrong or onlyLastHashIsWrong must be set")
}

//--------------------------
// utils for making blocks

func makeBlockchainFromWAL(wal WAL) ([]*types.Block, []*types.Commit, error) {
	var height int64

	// Search for height marker
	gr, found, err := wal.SearchForEndHeight(height, &WALSearchOptions{})
	if err != nil {
		return nil, nil, err
	}
	if !found {
		return nil, nil, fmt.Errorf("WAL does not contain height %d", height)
	}
	defer gr.Close() // nolint: errcheck

	// log.Notice("Build a blockchain by reading from the WAL")

	var (
		blocks          []*types.Block
		commits         []*types.Commit
		thisBlockParts  *types.PartSet
		thisBlockCommit *types.Commit
	)

	dec := NewWALDecoder(gr)
	for {
		msg, err := dec.Decode()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, nil, err
		}

		piece := readPieceFromWAL(msg)
		if piece == nil {
			continue
		}

		switch p := piece.(type) {
		case EndHeightMessage:
			// if its not the first one, we have a full block
			if thisBlockParts != nil {
				var block = new(types.Block)
				_, err = cdc.UnmarshalBinaryLengthPrefixedReader(thisBlockParts.GetReader(), block, 0)
				if err != nil {
					panic(err)
				}
				if block.Height != height+1 {
					panic(fmt.Sprintf("read bad block from wal. got height %d, expected %d", block.Height, height+1))
				}
				commitHeight := thisBlockCommit.Precommits[0].Height
				if commitHeight != height+1 {
					panic(fmt.Sprintf("commit doesnt match. got height %d, expected %d", commitHeight, height+1))
				}
				blocks = append(blocks, block)
				commits = append(commits, thisBlockCommit)
				height++
			}
		case *types.PartSetHeader:
			thisBlockParts = types.NewPartSetFromHeader(*p)
		case *types.Part:
			_, err := thisBlockParts.AddPart(p)
			if err != nil {
				return nil, nil, err
			}
		case *types.Vote:
			if p.Type == types.PrecommitType {
				commitSigs := []*types.CommitSig{p.CommitSig()}
				thisBlockCommit = types.NewCommit(p.BlockID, commitSigs)
			}
		}
	}
	// grab the last block too
	var block = new(types.Block)
	_, err = cdc.UnmarshalBinaryLengthPrefixedReader(thisBlockParts.GetReader(), block, 0)
	if err != nil {
		panic(err)
	}
	if block.Height != height+1 {
		panic(fmt.Sprintf("read bad block from wal. got height %d, expected %d", block.Height, height+1))
	}
	commitHeight := thisBlockCommit.Precommits[0].Height
	if commitHeight != height+1 {
		panic(fmt.Sprintf("commit doesnt match. got height %d, expected %d", commitHeight, height+1))
	}
	blocks = append(blocks, block)
	commits = append(commits, thisBlockCommit)
	return blocks, commits, nil
}

func readPieceFromWAL(msg *TimedWALMessage) interface{} {
	// for logging
	switch m := msg.Msg.(type) {
	case msgInfo:
		switch msg := m.Msg.(type) {
		case *ProposalMessage:
			return &msg.Proposal.BlockID.PartsHeader
		case *BlockPartMessage:
			return msg.Part
		case *VoteMessage:
			return msg.Vote
		}
	case EndHeightMessage:
		return m
	}

	return nil
}

// fresh state and mock store
func stateAndStore(config *cfg.Config, pubKey crypto.PubKey, appVersion version.Protocol) (dbm.DB, sm.State, *mockBlockStore) {
	stateDB := dbm.NewMemDB()
	state, _ := sm.MakeGenesisStateFromFile(config.GenesisFile())
	state.Version.Consensus.App = appVersion
	store := newMockBlockStore(config, state.ConsensusParams)
	sm.SaveState(stateDB, state)
	return stateDB, state, store
}

//----------------------------------
// mock block store

type mockBlockStore struct {
	config  *cfg.Config
	params  types.ConsensusParams
	chain   []*types.Block
	commits []*types.Commit
}

// TODO: NewBlockStore(db.NewMemDB) ...
func newMockBlockStore(config *cfg.Config, params types.ConsensusParams) *mockBlockStore {
	return &mockBlockStore{config, params, nil, nil}
}

func (bs *mockBlockStore) Height() int64                       { return int64(len(bs.chain)) }
func (bs *mockBlockStore) LoadBlock(height int64) *types.Block { return bs.chain[height-1] }
func (bs *mockBlockStore) LoadBlockMeta(height int64) *types.BlockMeta {
	block := bs.chain[height-1]
	return &types.BlockMeta{
		BlockID: types.BlockID{Hash: block.Hash(), PartsHeader: block.MakePartSet(types.BlockPartSizeBytes).Header()},
		Header:  block.Header,
	}
}
func (bs *mockBlockStore) LoadBlockPart(height int64, index int) *types.Part { return nil }
func (bs *mockBlockStore) SaveBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit) {
}
func (bs *mockBlockStore) LoadBlockCommit(height int64) *types.Commit {
	return bs.commits[height-1]
}
func (bs *mockBlockStore) LoadSeenCommit(height int64) *types.Commit {
	return bs.commits[height-1]
}

//---------------------------------------
// Test handshake/init chain

func TestHandshakeUpdatesValidators(t *testing.T) {
	val, _ := types.RandValidator(true, 10)
	vals := types.NewValidatorSet([]*types.Validator{val})
	app := &initChainApp{vals: types.TM2PB.ValidatorUpdates(vals)}
	clientCreator := proxy.NewLocalClientCreator(app)

	config := ResetConfig("handshake_test_")
	defer os.RemoveAll(config.RootDir)
	privVal := privval.LoadFilePV(config.PrivValidatorKeyFile(), config.PrivValidatorStateFile())
	stateDB, state, store := stateAndStore(config, privVal.GetPubKey(), 0x0)

	oldValAddr := state.Validators.Validators[0].Address

	// now start the app using the handshake - it should sync
	genDoc, _ := sm.MakeGenesisDocFromFile(config.GenesisFile())
	handshaker := NewHandshaker(stateDB, state, store, genDoc)
	proxyApp := proxy.NewAppConns(clientCreator)
	if err := proxyApp.Start(); err != nil {
		t.Fatalf("Error starting proxy app connections: %v", err)
	}
	defer proxyApp.Stop()
	if err := handshaker.Handshake(proxyApp); err != nil {
		t.Fatalf("Error on abci handshake: %v", err)
	}

	// reload the state, check the validator set was updated
	state = sm.LoadState(stateDB)

	newValAddr := state.Validators.Validators[0].Address
	expectValAddr := val.Address
	assert.NotEqual(t, oldValAddr, newValAddr)
	assert.Equal(t, newValAddr, expectValAddr)
}

// returns the vals on InitChain
type initChainApp struct {
	abci.BaseApplication
	vals []abci.ValidatorUpdate
}

func (ica *initChainApp) InitChain(req abci.RequestInitChain) abci.ResponseInitChain {
	return abci.ResponseInitChain{
		Validators: ica.vals,
	}
}
