package consensus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dashevo/dashd-go/btcjson"
	"github.com/fortytw2/leaktest"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/internal/eventbus"
	"github.com/tendermint/tendermint/internal/mempool"
	"github.com/tendermint/tendermint/internal/proxy"
	"github.com/tendermint/tendermint/internal/pubsub"
	sm "github.com/tendermint/tendermint/internal/state"
	sf "github.com/tendermint/tendermint/internal/state/test/factory"
	"github.com/tendermint/tendermint/internal/store"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/libs/log"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/privval"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

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

func startNewStateAndWaitForBlock(ctx context.Context, t *testing.T, consensusReplayConfig *config.Config,
	lastBlockHeight int64, blockDB dbm.DB, stateStore sm.Store) {
	logger := log.NewNopLogger()
	state, err := sm.MakeGenesisStateFromFile(consensusReplayConfig.GenesisFile())
	require.NoError(t, err)
	privValidator := loadPrivValidator(t, consensusReplayConfig)
	blockStore := store.NewBlockStore(dbm.NewMemDB())
	cs := newStateWithConfigAndBlockStore(
		ctx,
		t,
		logger,
		consensusReplayConfig,
		state,
		privValidator,
		kvstore.NewApplication(),
		blockStore,
	)

	bytes, err := os.ReadFile(cs.config.WalFile())
	require.NoError(t, err)
	require.NotNil(t, bytes)

	require.NoError(t, cs.Start(ctx))
	defer func() {
		cs.Stop()
	}()
	t.Cleanup(cs.Wait)
	// This is just a signal that we haven't halted; its not something contained
	// in the WAL itself. Assuming the consensus state is running, replay of any
	// WAL, including the empty one, should eventually be followed by a new
	// block, or else something is wrong.
	newBlockSub, err := cs.eventBus.SubscribeWithArgs(ctx, pubsub.SubscribeArgs{
		ClientID: testSubscriber,
		Query:    types.EventQueryNewBlock,
	})
	require.NoError(t, err)
	ctxto, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	_, err = newBlockSub.Next(ctxto)
	if errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("Timed out waiting for new block (see trace above)")
	} else if err != nil {
		t.Fatal("newBlockSub was canceled")
	}
}

func sendTxs(ctx context.Context, t *testing.T, cs *State) {
	t.Helper()
	for i := 0; i < 256; i++ {
		select {
		case <-ctx.Done():
			return
		default:
			tx := []byte{byte(i)}

			require.NoError(t, assertMempool(t, cs.txNotifier).CheckTx(ctx, tx, nil, mempool.TxInfo{}))

			i++
		}
	}
}

// TestWALCrash uses crashing WAL to test we can recover from any WAL failure.
func TestWALCrash(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	testCases := []struct {
		name         string
		initFn       func(dbm.DB, *State, context.Context)
		heightToStop int64
	}{
		{"empty block",
			func(stateDB dbm.DB, cs *State, ctx context.Context) {},
			1},
		{"many non-empty blocks",
			func(stateDB dbm.DB, cs *State, ctx context.Context) {
				go sendTxs(ctx, t, cs)
			},
			3},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			consensusReplayConfig, err := ResetConfig(t.TempDir(), tc.name)
			require.NoError(t, err)
			crashWALandCheckLiveness(ctx, t, consensusReplayConfig, tc.initFn, tc.heightToStop)
		})
	}
}

func crashWALandCheckLiveness(rctx context.Context, t *testing.T, consensusReplayConfig *config.Config,
	initFn func(dbm.DB, *State, context.Context), heightToStop int64) {
	walPanicked := make(chan error)
	crashingWal := &crashingWAL{panicCh: walPanicked, heightToStop: heightToStop}

	i := 1
LOOP:
	for {
		// create consensus state from a clean slate
		logger := log.NewTestingLogger(t)
		blockDB := dbm.NewMemDB()
		stateDB := dbm.NewMemDB()
		stateStore := sm.NewStore(stateDB)
		blockStore := store.NewBlockStore(blockDB)
		state, err := sm.MakeGenesisStateFromFile(consensusReplayConfig.GenesisFile())
		require.NoError(t, err)

		// default timeout value 30ms is not enough, if this parameter is not increased,
		// then with a high probability the code will be stuck on proposal step
		// due to a timeout handler performs before than validators will be ready for the message
		state.ConsensusParams.Timeout.Propose = 1 * time.Second

		privValidator := loadPrivValidator(t, consensusReplayConfig)
		cs := newStateWithConfigAndBlockStore(
			rctx,
			t,
			logger,
			consensusReplayConfig,
			state,
			privValidator,
			kvstore.NewApplication(),
			blockStore,
		)

		// start sending transactions
		ctx, cancel := context.WithCancel(rctx)
		initFn(stateDB, cs, ctx)

		// clean up WAL file from the previous iteration
		walFile := cs.config.WalFile()
		os.Remove(walFile)

		// set crashing WAL
		csWal, err := cs.OpenWAL(ctx, walFile)
		require.NoError(t, err)
		crashingWal.next = csWal

		// reset the message counter
		crashingWal.msgIndex = 1
		cs.wal = crashingWal

		// start consensus state
		err = cs.Start(ctx)
		require.NoError(t, err)

		i++

		select {
		case <-rctx.Done():
			t.Fatal("context canceled before test completed")
		case err := <-walPanicked:
			// make sure we can make blocks after a crash
			startNewStateAndWaitForBlock(ctx, t, consensusReplayConfig, cs.Height, blockDB, stateStore)

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

func (w *crashingWAL) SearchForEndHeight(
	height int64,
	options *WALSearchOptions) (rd io.ReadCloser, found bool, err error) {
	return w.next.SearchForEndHeight(height, options)
}

func (w *crashingWAL) Start(ctx context.Context) error { return w.next.Start(ctx) }
func (w *crashingWAL) Stop()                           { w.next.Stop() }
func (w *crashingWAL) Wait()                           { w.next.Wait() }

//------------------------------------------------------------------------------------------
type simulatorTestSuite struct {
	GenesisState sm.State
	Config       *config.Config
	Chain        []*types.Block
	Commits      []*types.Commit
	CleanupFunc  cleanupFunc

	Mempool mempool.Mempool
	Evpool  sm.EvidencePool

	ValidatorSetUpdates map[int64]abci.ValidatorSetUpdate
	HeightTxs           []types.Txs
}

const (
	numBlocks = 6
)

//---------------------------------------
// Test handshake/replay

// 0 - all synced up
// 1 - saved block but app and state are behind
// 2 - save block and committed but state is behind
// 3 - save block and committed with truncated block store and state behind
var modes = []uint{0, 1, 2, 3}

func findValByProTxHash(ctx context.Context, t *testing.T, validatorStubs []*validatorStub, proTxHash crypto.ProTxHash) *validatorStub {
	for _, validatorStub := range validatorStubs {
		valProTxHash, err := validatorStub.GetProTxHash(ctx)
		require.NoError(t, err)
		if bytes.Equal(valProTxHash, proTxHash) {
			return validatorStub
		}
	}
	t.Error("validator not found")
	return nil
}

func findStateByProTxHash(t *testing.T, css []*State, proTxHash crypto.ProTxHash) *State {
	for _, consensusState := range css {
		if proTxHash.Equal(consensusState.privValidatorProTxHash) {
			return consensusState
		}
	}
	t.Error("validator state not found for proTxHash", proTxHash)
	return nil
}

// This is actually not a test, it's for storing validator change tx data for testHandshakeReplay
func setupSimulator(ctx context.Context, t *testing.T) *simulatorTestSuite {
	// t.Helper()
	cfg := configSetup(t)

	sim := &simulatorTestSuite{
		Mempool:   emptyMempool{},
		Evpool:    sm.EmptyEvidencePool{},
		HeightTxs: []types.Txs{{}, {types.Tx("key1=val")}, {}, {types.Tx("key2=val")}, {}, {types.Tx("key3=val")}, {}},
	}

	nPeers := 7
	nVals := 4

	gen := consensusNetGen{
		cfg:       cfg,
		nPeers:    nPeers,
		nVals:     nVals,
		tickerFun: newMockTickerFunc(true),
		appFunc:   newKVStoreFunc(),
		validatorUpdates: []validatorUpdate{
			{height: 2, count: 1, operation: "add"},
			{height: 4, count: 2, operation: "add"},
			{height: 6, count: 1, operation: "remove"},
		},
	}
	css, genDoc, cfg, valSetUpdates := gen.generate(ctx, t)
	sim.ValidatorSetUpdates = valSetUpdates

	t.Logf("genesis quorum hash is %X\n", genDoc.QuorumHash)
	sim.Config = cfg

	var err error
	sim.GenesisState, err = sm.MakeGenesisState(genDoc)
	require.NoError(t, err)

	newRoundCh := subscribe(ctx, t, css[0].eventBus, types.EventQueryNewRound)
	proposalCh := subscribe(ctx, t, css[0].eventBus, types.EventQueryCompleteProposal)

	vss := make([]*validatorStub, nPeers)
	for i := 0; i < nPeers; i++ {
		vss[i] = newValidatorStub(css[i].privValidator, int32(i), 0)
	}
	height, round := css[0].Height, css[0].Round

	// start the machine; note height should be equal to InitialHeight here,
	// so we don't need to increment it
	startTestRound(ctx, css[0], height, round)
	incrementHeight(vss...)
	ensureNewRound(t, newRoundCh, height, 0)
	ensureNewProposal(t, proposalCh, height, round)
	rs := css[0].GetRoundState()

	// Stop auto proposing blocks, as this could lead to issues based on the
	// randomness of proposer selection
	css[0].config.DontAutoPropose = true

	blockID := types.BlockID{Hash: rs.ProposalBlock.Hash(), PartSetHeader: rs.ProposalBlockParts.Header()}
	signAddVotes(ctx, t, css[0], tmproto.PrecommitType, sim.Config.ChainID(), blockID, vss[1:nVals]...)

	ensureNewRound(t, newRoundCh, height+1, 0)

	var vssForSigning []*validatorStub
	for _, txs := range sim.HeightTxs[1:] {
		height++
		incrementHeight(vss...)
		vals := findSuitableValidatorSetUpdates(height, valSetUpdates).ValidatorUpdates
		require.Len(t, css[0].Validators.Validators, len(vals))
		for _, tx := range txs {
			err = assertMempool(t, css[0].txNotifier).CheckTx(ctx, tx, nil, mempool.TxInfo{})
			assert.Nil(t, err)
		}
		vssForSigning = determineActiveValidators(ctx, t, vss, css[0].Validators)
		blockID = createSignSendProposal(ctx, t, css, vss, cfg.ChainID(), txs.ToSliceOfBytes())
		ensureNewProposal(t, proposalCh, height, round)
		require.True(t, css[0].Validators.HasPublicKeys)
		signAddVotes(ctx, t, css[0], tmproto.PrecommitType, sim.Config.ChainID(), blockID, vssForSigning...)
		ensureNewRound(t, newRoundCh, height+1, 0)
	}
	sim.Chain = make([]*types.Block, 0)
	sim.Commits = make([]*types.Commit, 0)
	for i := 1; i <= numBlocks; i++ {
		sim.Chain = append(sim.Chain, css[0].blockStore.LoadBlock(int64(i)))
		sim.Commits = append(sim.Commits, css[0].blockStore.LoadBlockCommit(int64(i)))
	}
	return sim
}

func findSuitableValidatorSetUpdates(height int64, validatorSetUpdates map[int64]abci.ValidatorSetUpdate) abci.ValidatorSetUpdate {
	var closestHeight int64
	for h := range validatorSetUpdates {
		if h < height && h >= closestHeight {
			closestHeight = h
		}
	}
	return validatorSetUpdates[closestHeight]
}

func determineActiveValidators(ctx context.Context,
	t *testing.T,
	vss []*validatorStub,
	validatorSet *types.ValidatorSet,
) []*validatorStub {
	proTxHashMap := make(map[string]struct{})
	for _, v := range validatorSet.Validators {
		proTxHashMap[v.ProTxHash.String()] = struct{}{}
	}
	vssForSigning := make([]*validatorStub, 0, len(proTxHashMap))
	for _, v := range vss {
		proTxHash, _ := v.GetProTxHash(ctx)
		_, ok := proTxHashMap[proTxHash.String()]
		if ok {
			vssForSigning = append(vssForSigning, v)
		}
	}
	return sortVValidatorStubsByPower(ctx, t, vssForSigning)
}

func createSignSendProposal(ctx context.Context,
	t *testing.T,
	css []*State,
	vss []*validatorStub,
	chainID string,
	assertTxs [][]byte,
) types.BlockID {
	const (
		partSize = types.BlockPartSizeBytes
	)

	quorumType := css[0].Validators.QuorumType
	quorumHash := css[0].Validators.QuorumHash
	height := css[0].RoundState.Height
	round := css[0].RoundState.Round

	proposer := css[0].Validators.GetProposer()
	proposerVs := findValByProTxHash(ctx, t, vss, proposer.ProTxHash)
	proposerCs := findStateByProTxHash(t, css, proposer.ProTxHash)

	assertProposerState(ctx, t, proposerCs, proposerVs, quorumHash)

	// _, propCS := findConsensusStateByProTxHash(ctx, t, css, proposer.ProTxHash)
	// We create proposal block on css[0] because we use it there when pushing TXs
	propBlock, _ := css[0].CreateProposalBlock(ctx)
	propBlockParts, err := propBlock.MakePartSet(partSize)
	require.NoError(t, err)
	blockID := types.BlockID{Hash: propBlock.Hash(), PartSetHeader: propBlockParts.Header()}
	if assertTxs != nil {
		require.Equal(t, len(assertTxs), len(propBlock.Txs), "height %d", height)
		for _, tx := range assertTxs {
			require.Contains(t, propBlock.Txs, types.Tx(tx))
		}
	}

	proposal := types.NewProposal(height, 1, round, -1, blockID, propBlock.Header.Time)
	p := proposal.ToProto()

	var signID tmbytes.HexBytes
	if signID, err = proposerVs.SignProposal(ctx, chainID, quorumType, quorumHash, p); err != nil {
		t.Fatal("failed to sign bad proposal", err)
	}
	proposal.Signature = p.Signature

	// set the proposal block to state on node 0, this will result in a signed prevote,
	// so we do not need to prevote with it again (hence the vss[1:nVals])
	if err := css[0].SetProposalAndBlock(ctx, proposal, propBlock, propBlockParts, "some peer"); err != nil {
		t.Fatal(err)
	}

	pubkey, err := proposerVs.GetPubKey(ctx, quorumHash)
	assert.NoError(t, err)
	css[0].logger.Debug(
		"signed and pushed proposal",
		"height", proposal.Height,
		"round", proposal.Round,
		"proposer", proposer.ProTxHash.ShortString(),
		"signature", p.Signature,
		"pubkey", pubkey.HexString(),
		"quorum type", quorumType,
		"quorum hash", quorumHash,
		"signID", signID,
	)

	return blockID
}

func assertProposerState(ctx context.Context, t *testing.T, csProposer *State, vsProposer *validatorStub, quorumHash tmbytes.HexBytes) {
	t.Helper()

	vsProposerPubKey, err := vsProposer.GetPubKey(context.Background(), quorumHash)
	assert.NoError(t, err, "read vsProposer pubkey")
	vsProposerProTxHash, err := vsProposer.GetProTxHash(context.Background())
	assert.NoError(t, err, "read vsProposer proTxHash")

	csProposerPubKey, err := csProposer.privValidator.GetPubKey(ctx, quorumHash)
	require.NoError(t, err)

	assert.True(t, csProposer.privValidatorProTxHash.Equal(vsProposerProTxHash), "proTxHash match")

	assert.Equal(t, vsProposerPubKey.Bytes(), csProposerPubKey.Bytes(), "pubkey match")

}

// Sync from scratch
func TestHandshakeReplayAll(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sim := setupSimulator(ctx, t)

	t.Cleanup(leaktest.Check(t))

	for _, m := range modes {
		testHandshakeReplay(ctx, t, sim, 0, m, false)
	}
	for _, m := range modes {
		testHandshakeReplay(ctx, t, sim, 0, m, true)
	}
}

// Sync many, not from scratch
func TestHandshakeReplaySome(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sim := setupSimulator(ctx, t)

	t.Cleanup(leaktest.Check(t))

	for _, m := range modes {
		testHandshakeReplay(ctx, t, sim, 2, m, false)
	}
	for _, m := range modes {
		testHandshakeReplay(ctx, t, sim, 2, m, true)
	}
}

// Sync from lagging by one
func TestHandshakeReplayOne(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sim := setupSimulator(ctx, t)

	for _, m := range modes {
		t.Run(fmt.Sprintf("L1_M%d", m), func(t *testing.T) {
			testHandshakeReplay(ctx, t, sim, numBlocks-1, m, false)
		})
	}
	for _, m := range modes {
		t.Run(fmt.Sprintf("L2_M%d", m), func(t *testing.T) {
			testHandshakeReplay(ctx, t, sim, numBlocks-1, m, true)
		})
	}
}

// Sync from caught up
func TestHandshakeReplayNone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sim := setupSimulator(ctx, t)

	t.Cleanup(leaktest.Check(t))

	for _, m := range modes {
		t.Run(fmt.Sprintf("L1_M%d", m), func(t *testing.T) {
			testHandshakeReplay(ctx, t, sim, numBlocks, m, false)
		})
	}
	for _, m := range modes {
		t.Run(fmt.Sprintf("L2_M%d", m), func(t *testing.T) {
			testHandshakeReplay(ctx, t, sim, numBlocks, m, true)
		})
	}
}

func tempWALWithData(t *testing.T, data []byte) string {
	t.Helper()

	walFile, err := os.CreateTemp(t.TempDir(), "wal")
	require.NoError(t, err, "failed to create temp WAL file")
	t.Cleanup(func() { _ = os.RemoveAll(walFile.Name()) })

	_, err = walFile.Write(data)
	require.NoError(t, err, "failed to  write to temp WAL file")

	require.NoError(t, walFile.Close(), "failed to close temp WAL file")
	return walFile.Name()
}

// Make some blocks. Start a fresh app and apply nBlocks blocks.
// Then restart the app and sync it up with the remaining blocks
func testHandshakeReplay(
	rctx context.Context,
	t *testing.T,
	sim *simulatorTestSuite,
	nBlocks int,
	mode uint,
	testValidatorsChange bool,
) {
	var store *mockBlockStore
	var stateDB dbm.DB
	var genesisState sm.State
	var privVal types.PrivValidator

	ctx, cancel := context.WithCancel(rctx)
	t.Cleanup(cancel)

	cfg := sim.Config
	testName := strings.ReplaceAll(t.Name(), "/", "_")

	logger := log.NewNopLogger()

	privVal = privval.MustLoadOrGenFilePVFromConfig(cfg)

	testConfig, err := ResetConfig(t.TempDir(), fmt.Sprintf("%s_s", testName))
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(testConfig.RootDir) }()

	genesisState = sim.GenesisState
	stateDB = dbm.NewMemDB()
	chain := append([]*types.Block{}, sim.Chain...) // copy chain
	commits := sim.Commits
	store = newMockBlockStore(t, cfg, genesisState.ConsensusParams)

	opts := []func(app *kvstore.Application){
		kvstore.WithValidatorSetUpdates(sim.ValidatorSetUpdates),
	}

	if !testValidatorsChange {
		// test single node
		opts = nil
		ng := nodeGen{
			cfg:     getConfig(t),
			logger:  logger,
			mempool: &mockMempool{calls: sim.HeightTxs},
		}
		node := ng.Generate(ctx, t)

		walBody, err := WALWithNBlocks(ctx, t, logger, node, numBlocks)
		require.NoError(t, err)
		walFile := tempWALWithData(t, walBody)
		cfg.Consensus.SetWalFile(walFile)

		gdoc, err := sm.MakeGenesisDocFromFile(cfg.GenesisFile())
		require.NoError(t, err)

		wal, err := NewWAL(ctx, logger, walFile)
		require.NoError(t, err)
		err = wal.Start(ctx)
		require.NoError(t, err)
		t.Cleanup(func() { cancel(); wal.Wait() })
		chain, commits = makeBlockchainFromWAL(t, wal, gdoc)
		pubKey, err := privVal.GetPubKey(ctx, gdoc.QuorumHash)
		require.NoError(t, err)
		stateDB, genesisState, store = stateAndStore(t, cfg, pubKey, kvstore.ProtocolVersion)
	}
	proTxHash, err := privVal.GetProTxHash(ctx)
	require.NoError(t, err)

	stateStore := sm.NewStore(stateDB)
	store.chain = chain
	store.commits = commits

	state := genesisState.Copy()
	// run the chain through state.ApplyBlock to build up the tendermint state
	state = buildTMStateFromChain(
		ctx,
		t,
		logger,
		sim.Mempool,
		sim.Evpool,
		kvstore.NewApplication(opts...),
		stateStore,
		state,
		chain,
		mode,
		store,
	)
	latestAppHash := state.AppHash

	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	client := abciclient.NewLocalClient(logger, kvstore.NewApplication(opts...))
	if nBlocks > 0 {
		// run nBlocks against a new client to build up the app state.
		// use a throwaway tendermint state
		proxyApp := proxy.New(client, logger, proxy.NopMetrics())
		stateStore := sm.NewStore(dbm.NewMemDB())
		err := stateStore.Save(genesisState)
		require.NoError(t, err)
		buildAppStateFromChain(
			ctx, t,
			proxyApp,
			stateStore,
			sim.Mempool,
			sim.Evpool,
			genesisState,
			chain,
			eventBus,
			nBlocks,
			mode,
			store,
		)
	}

	// Prune block store if requested
	expectError := false
	if mode == 3 {
		pruned, err := store.PruneBlocks(2)
		require.NoError(t, err)
		require.EqualValues(t, 1, pruned)
		expectError = int64(nBlocks) < 2
	}

	// now start the app using the handshake - it should sync
	genDoc, err := sm.MakeGenesisDocFromFile(cfg.GenesisFile())
	require.NoError(t, err)
	handshaker := NewHandshaker(
		logger,
		stateStore,
		state,
		store,
		eventBus,
		genDoc,
		proTxHash,
		cfg.Consensus.AppHashSize,
	)
	proxyApp := proxy.New(client, logger, proxy.NopMetrics())
	require.NoError(t, proxyApp.Start(ctx), "Error starting proxy app connections")
	require.True(t, proxyApp.IsRunning())
	require.NotNil(t, proxyApp)
	t.Cleanup(func() { cancel(); proxyApp.Wait() })

	_, err = handshaker.Handshake(ctx, proxyApp)
	if expectError {
		require.Error(t, err)
		return
	}
	require.NoError(t, err, "Error on abci handshake")

	// get the latest app hash from the app
	res, err := proxyApp.Info(ctx, &abci.RequestInfo{Version: ""})
	require.NoError(t, err)

	// the app hash should be synced up
	require.Equalf(t, latestAppHash.Bytes(), res.LastBlockAppHash,
		"Expected app hashes to match after handshake/replay. got %X, expected %X",
		res.LastBlockAppHash,
		latestAppHash,
	)

	expectedBlocksToSync := numBlocks - nBlocks
	if nBlocks > 0 && mode == 1 {
		expectedBlocksToSync++
	}

	if handshaker.NBlocks() != expectedBlocksToSync {
		t.Fatalf(
			"Expected handshake to sync %d blocks, got %d",
			expectedBlocksToSync,
			handshaker.NBlocks(),
		)
	}
}

func applyBlock(
	ctx context.Context,
	t *testing.T,
	blockExec *sm.BlockExecutor,
	st sm.State,
	blk *types.Block,
) sm.State {
	testPartSize := types.BlockPartSizeBytes
	bps, err := blk.MakePartSet(testPartSize)
	require.NoError(t, err)
	blkID := types.BlockID{Hash: blk.Hash(), PartSetHeader: bps.Header()}
	newState, err := blockExec.ApplyBlock(ctx, st, blkID, blk)
	require.NoError(t, err)
	return newState
}

func buildAppStateFromChain(
	ctx context.Context,
	t *testing.T,
	appClient abciclient.Client,
	stateStore sm.Store,
	mempool mempool.Mempool,
	evpool sm.EvidencePool,
	state sm.State,
	chain []*types.Block,
	eventBus *eventbus.EventBus,
	nBlocks int,
	mode uint,
	blockStore *mockBlockStore,
) {
	t.Helper()
	// start a new app without handshake, play nBlocks blocks
	require.NoError(t, appClient.Start(ctx))

	state.Version.Consensus.App = kvstore.ProtocolVersion // simulate handshake, receive app version
	validators := types.TM2PB.ValidatorUpdates(state.Validators)
	_, err := appClient.InitChain(ctx, &abci.RequestInitChain{
		ValidatorSet: &validators,
	})
	require.NoError(t, err)

	require.NoError(t, stateStore.Save(state)) // save height 1's validatorsInfo

	blockExec := sm.NewBlockExecutor(
		stateStore,
		consensusLogger(t),
		appClient,
		mempool,
		evpool,
		blockStore,
		eventBus,
		sm.NopMetrics(),
	)

	switch mode {
	case 0:
		for i := 0; i < nBlocks; i++ {
			state = applyBlock(ctx, t, blockExec, state, chain[i])
		}
	case 1, 2, 3:
		for i := 0; i < nBlocks-1; i++ {
			state = applyBlock(ctx, t, blockExec, state, chain[i])
		}

		if mode == 2 || mode == 3 {
			// update the kvstore height and apphash
			// as if we ran commit but not
			state = applyBlock(ctx, t, blockExec, state, chain[nBlocks-1])
		}
	default:
		require.Fail(t, "unknown mode %v", mode)
	}

}

func buildTMStateFromChain(
	ctx context.Context,
	t *testing.T,
	logger log.Logger,
	mempool mempool.Mempool,
	evpool sm.EvidencePool,
	app abci.Application,
	stateStore sm.Store,
	state sm.State,
	chain []*types.Block,
	mode uint,
	blockStore *mockBlockStore,
) sm.State {
	t.Helper()

	// run the whole chain against this client to build up the tendermint state
	client := abciclient.NewLocalClient(logger, app)

	proxyApp := proxy.New(client, logger, proxy.NopMetrics())
	require.NoError(t, proxyApp.Start(ctx))

	state.Version.Consensus.App = kvstore.ProtocolVersion // simulate handshake, receive app version
	validators := types.TM2PB.ValidatorUpdates(state.Validators)
	_, err := proxyApp.InitChain(ctx, &abci.RequestInitChain{
		ValidatorSet: &validators,
	})
	require.NoError(t, err)

	require.NoError(t, stateStore.Save(state))

	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	blockExec := sm.NewBlockExecutor(
		stateStore,
		log.NewNopLogger(),
		proxyApp,
		mempool,
		evpool,
		blockStore,
		eventBus,
		sm.NopMetrics(),
	)

	switch mode {
	case 0:
		// sync right up
		for _, block := range chain {
			state = applyBlock(ctx, t, blockExec, state, block)
		}

	case 1, 2, 3:
		// sync up to the penultimate as if we stored the block.
		// whether we commit or not depends on the appHash
		for _, block := range chain[:len(chain)-1] {
			state = applyBlock(ctx, t, blockExec, state, block)
		}

		// apply the final block to a state copy so we can
		// get the right next appHash but keep the state back
		state = applyBlock(ctx, t, blockExec, state, chain[len(chain)-1])
	default:
		require.Fail(t, "unknown mode %v", mode)
	}

	return state
}

func TestHandshakeErrorsIfAppReturnsWrongAppHash(t *testing.T) {
	// 1. Initialize tendermint and commit 3 blocks with the following app hashes:
	//		- 0x01
	//		- 0x02
	//		- 0x03

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := ResetConfig(t.TempDir(), "handshake_test_")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(cfg.RootDir) })
	privVal, err := privval.LoadFilePV(cfg.PrivValidator.KeyFile(), cfg.PrivValidator.StateFile())
	require.NoError(t, err)
	const appVersion = 0x0
	quorumHash, err := privVal.GetFirstQuorumHash(ctx)
	require.NoError(t, err)
	pubKey, err := privVal.GetPubKey(ctx, quorumHash)
	require.NoError(t, err)
	proTxHash, err := privVal.GetProTxHash(ctx)
	require.NoError(t, err)
	stateDB, state, store := stateAndStore(t, cfg, pubKey, appVersion)
	stateStore := sm.NewStore(stateDB)
	genDoc, err := sm.MakeGenesisDocFromFile(cfg.GenesisFile())
	require.NoError(t, err)
	state.LastValidators = state.Validators.Copy()
	// mode = 0 for committing all the blocks
	blocks := sf.MakeBlocks(ctx, t, 3, &state, privVal, 1)

	store.chain = blocks

	logger := log.NewNopLogger()

	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	// 2. Tendermint must panic if app returns wrong hash for the first block
	//		- RANDOM HASH
	//		- 0x02
	//		- 0x03
	{
		app := &badApp{numBlocks: 3, allHashesAreWrong: true}
		client := abciclient.NewLocalClient(logger, app)
		proxyApp := proxy.New(client, logger, proxy.NopMetrics())
		err := proxyApp.Start(ctx)
		require.NoError(t, err)
		t.Cleanup(func() { cancel(); proxyApp.Wait() })

		h := NewHandshaker(
			logger,
			stateStore,
			state,
			store,
			eventBus,
			genDoc,
			proTxHash,
			cfg.Consensus.AppHashSize,
		)
		_, err = h.Handshake(ctx, proxyApp)
		assert.Error(t, err)
		t.Log(err)
	}

	// 3. Tendermint must panic if app returns wrong hash for the last block
	//		- 0x01
	//		- 0x02
	//		- RANDOM HASH
	{
		app := &badApp{numBlocks: 3, onlyLastHashIsWrong: true}
		client := abciclient.NewLocalClient(logger, app)
		proxyApp := proxy.New(client, logger, proxy.NopMetrics())
		err := proxyApp.Start(ctx)
		require.NoError(t, err)
		t.Cleanup(func() { cancel(); proxyApp.Wait() })

		h := NewHandshaker(
			logger,
			stateStore,
			state, store,
			eventBus,
			genDoc,
			proTxHash,
			cfg.Consensus.AppHashSize,
		)
		_, err = h.Handshake(ctx, proxyApp)
		require.Error(t, err)
	}
}

type badApp struct {
	abci.BaseApplication
	numBlocks           byte
	height              byte
	allHashesAreWrong   bool
	onlyLastHashIsWrong bool
}

func (app *badApp) ProcessProposal(_ context.Context, _ *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
	app.height++
	if app.onlyLastHashIsWrong {
		if app.height == app.numBlocks {
			return &abci.ResponseProcessProposal{AppHash: tmrand.Bytes(32)}, nil
		}
		appHash := make([]byte, crypto.DefaultAppHashSize)
		appHash[crypto.DefaultAppHashSize-1] = app.height
		return &abci.ResponseProcessProposal{AppHash: appHash}, nil
	} else if app.allHashesAreWrong {
		return &abci.ResponseProcessProposal{AppHash: tmrand.Bytes(32)}, nil
	}
	panic("either allHashesAreWrong or onlyLastHashIsWrong must be set")
}

//--------------------------
// utils for making blocks

func makeBlockchainFromWAL(t *testing.T, wal WAL, genDoc *types.GenesisDoc) ([]*types.Block, []*types.Commit) {
	t.Helper()
	var height int64

	// Search for height marker
	gr, found, err := wal.SearchForEndHeight(height, &WALSearchOptions{})
	require.NoError(t, err)
	require.True(t, found, "wal does not contain height %d", height)
	defer gr.Close()

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
		}
		require.NoError(t, err)

		piece := readPieceFromWAL(msg)
		if piece == nil {
			continue
		}

		switch p := piece.(type) {
		case EndHeightMessage:
			// if its not the first one, we have a full block
			if thisBlockParts != nil {
				var pbb = new(tmproto.Block)
				bz, err := io.ReadAll(thisBlockParts.GetReader())
				require.NoError(t, err)

				require.NoError(t, proto.Unmarshal(bz, pbb))

				block, err := types.BlockFromProto(pbb)
				require.NoError(t, err)

				require.Equal(t, block.Height, height+1,
					"read bad block from wal. got height %d, expected %d", block.Height, height+1)

				commitHeight := thisBlockCommit.Height
				require.Equal(t, commitHeight, height+1,
					"commit doesnt match. got height %d, expected %d", commitHeight, height+1)

				blocks = append(blocks, block)
				commits = append(commits, thisBlockCommit)
				height++
			}
		case *types.PartSetHeader:
			thisBlockParts = types.NewPartSetFromHeader(*p)
		case *types.Part:
			_, err := thisBlockParts.AddPart(p)
			require.NoError(t, err)
		case *types.Vote:
			if p.Type == tmproto.PrecommitType {
				thisBlockCommit = types.NewCommit(p.Height, p.Round,
					p.BlockID, p.StateID(),
					&types.CommitSigns{
						QuorumSigns: types.QuorumSigns{
							BlockSign:      p.BlockSignature,
							StateSign:      p.StateSignature,
							ExtensionSigns: types.MakeThresholdExtensionSigns(p.VoteExtensions),
						},
						QuorumHash: crypto.RandQuorumHash(),
					},
				)
			}
		}
	}
	// grab the last block too
	bz, err := io.ReadAll(thisBlockParts.GetReader())
	require.NoError(t, err)

	var pbb = new(tmproto.Block)
	require.NoError(t, proto.Unmarshal(bz, pbb))

	block, err := types.BlockFromProto(pbb)
	require.NoError(t, err)

	require.Equal(t, block.Height, height+1, "read bad block from wal. got height %d, expected %d", block.Height, height+1)
	commitHeight := thisBlockCommit.Height
	require.Equal(t, commitHeight, height+1, "commit does not match. got height %d, expected %d", commitHeight, height+1)

	blocks = append(blocks, block)
	commits = append(commits, thisBlockCommit)
	return blocks, commits
}

func readPieceFromWAL(msg *TimedWALMessage) interface{} {
	// for logging
	switch m := msg.Msg.(type) {
	case msgInfo:
		switch msg := m.Msg.(type) {
		case *ProposalMessage:
			return &msg.Proposal.BlockID.PartSetHeader
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
func stateAndStore(
	t *testing.T,
	cfg *config.Config,
	pubKey crypto.PubKey,
	appVersion uint64,
) (dbm.DB, sm.State, *mockBlockStore) {
	stateDB := dbm.NewMemDB()
	stateStore := sm.NewStore(stateDB)
	state, err := sm.MakeGenesisStateFromFile(cfg.GenesisFile())
	require.NoError(t, err)
	state.Version.Consensus.App = appVersion
	store := newMockBlockStore(t, cfg, state.ConsensusParams)
	require.NoError(t, stateStore.Save(state))

	return stateDB, state, store
}

//----------------------------------
// mock block store

type mockBlockStore struct {
	cfg     *config.Config
	params  types.ConsensusParams
	chain   []*types.Block
	commits []*types.Commit
	base    int64
	t       *testing.T

	coreChainLockedHeight uint32
}

var _ sm.BlockStore = &mockBlockStore{}

// TODO: NewBlockStore(db.NewMemDB) ...
func newMockBlockStore(t *testing.T, cfg *config.Config, params types.ConsensusParams) *mockBlockStore {
	return &mockBlockStore{
		cfg:    cfg,
		params: params,
		t:      t,

		coreChainLockedHeight: 1,
	}
}

func (bs *mockBlockStore) Height() int64                 { return int64(len(bs.chain)) }
func (bs *mockBlockStore) CoreChainLockedHeight() uint32 { return bs.coreChainLockedHeight }
func (bs *mockBlockStore) Base() int64                   { return bs.base }

func (bs *mockBlockStore) Size() int64                         { return bs.Height() - bs.Base() + 1 }
func (bs *mockBlockStore) LoadBaseMeta() *types.BlockMeta      { return bs.LoadBlockMeta(bs.base) }
func (bs *mockBlockStore) LoadBlock(height int64) *types.Block { return bs.chain[height-1] }
func (bs *mockBlockStore) LoadBlockByHash(hash []byte) *types.Block {
	return bs.chain[int64(len(bs.chain))-1]
}
func (bs *mockBlockStore) LoadBlockMetaByHash(hash []byte) *types.BlockMeta { return nil }
func (bs *mockBlockStore) LoadBlockMeta(height int64) *types.BlockMeta {
	block := bs.chain[height-1]
	bps, err := block.MakePartSet(types.BlockPartSizeBytes)
	require.NoError(bs.t, err)
	return &types.BlockMeta{
		BlockID: types.BlockID{Hash: block.Hash(), PartSetHeader: bps.Header()},
		Header:  block.Header,
	}
}
func (bs *mockBlockStore) LoadBlockPart(height int64, index int) *types.Part { return nil }
func (bs *mockBlockStore) SaveBlock(
	block *types.Block,
	blockParts *types.PartSet,
	seenCommit *types.Commit,
) {
}

func (bs *mockBlockStore) LoadBlockCommit(height int64) *types.Commit {
	return bs.commits[height-1]
}
func (bs *mockBlockStore) LoadSeenCommit() *types.Commit {
	return bs.commits[len(bs.commits)-1]
}
func (bs *mockBlockStore) LoadSeenCommitAt(height int64) *types.Commit {
	return bs.commits[height-1]
}

func (bs *mockBlockStore) PruneBlocks(height int64) (uint64, error) {
	pruned := uint64(0)
	for i := int64(0); i < height-1; i++ {
		bs.chain[i] = nil
		bs.commits[i] = nil
		pruned++
	}
	bs.base = height
	return pruned, nil
}

//---------------------------------------
// Test handshake/init chain

func TestHandshakeUpdatesValidators(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := ResetConfig(t.TempDir(), "handshake_test_")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(cfg.RootDir) })

	privVal, err := privval.LoadFilePV(cfg.PrivValidator.KeyFile(), cfg.PrivValidator.StateFile())
	require.NoError(t, err)

	logger := log.NewNopLogger()
	val, _ := randValidator()
	randQuorumHash, err := privVal.GetFirstQuorumHash(ctx)
	require.NoError(t, err)
	vals := types.NewValidatorSet(
		[]*types.Validator{val},
		val.PubKey,
		btcjson.LLMQType_5_60,
		randQuorumHash,
		true,
	)
	abciValidatorSetUpdates := types.TM2PB.ValidatorUpdates(vals)
	app := &initChainApp{vals: &abciValidatorSetUpdates}
	client := abciclient.NewLocalClient(logger, app)

	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	pubKey, err := privVal.GetPubKey(ctx, randQuorumHash)
	require.NoError(t, err)
	proTxHash, err := privVal.GetProTxHash(ctx)
	require.NoError(t, err)
	stateDB, state, store := stateAndStore(t, cfg, pubKey, 0x0)
	stateStore := sm.NewStore(stateDB)

	oldValProTxHash := state.Validators.Validators[0].ProTxHash

	// now start the app using the handshake - it should sync
	genDoc, err := sm.MakeGenesisDocFromFile(cfg.GenesisFile())
	require.NoError(t, err)

	handshaker := NewHandshaker(
		logger,
		stateStore,
		state,
		store,
		eventBus,
		genDoc,
		proTxHash,
		cfg.Consensus.AppHashSize,
	)
	proxyApp := proxy.New(client, logger, proxy.NopMetrics())
	require.NoError(t, proxyApp.Start(ctx), "Error starting proxy app connections")

	_, err = handshaker.Handshake(ctx, proxyApp)
	require.NoError(t, err, "error on abci handshake")

	// reload the state, check the validator set was updated
	state, err = stateStore.Load()
	require.NoError(t, err)

	newValProTxHash := state.Validators.Validators[0].ProTxHash
	expectValProTxHash := val.ProTxHash
	assert.NotEqual(t, oldValProTxHash, newValProTxHash)
	assert.Equal(t, newValProTxHash, expectValProTxHash)
}

func TestHandshakeInitialCoreLockHeight(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const InitialCoreHeight uint32 = 12345
	logger := log.NewNopLogger()
	conf, err := ResetConfig(t.TempDir(), "handshake_test_initial_core_lock_height")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(conf.RootDir) })

	privVal, err := privval.LoadFilePV(conf.PrivValidator.KeyFile(), conf.PrivValidator.StateFile())
	require.NoError(t, err)

	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	randQuorumHash, err := privVal.GetFirstQuorumHash(ctx)
	require.NoError(t, err)

	app := &initChainApp{initialCoreHeight: InitialCoreHeight}
	client := abciclient.NewLocalClient(logger, app)

	pubKey, err := privVal.GetPubKey(ctx, randQuorumHash)
	require.NoError(t, err)
	proTxHash, err := privVal.GetProTxHash(ctx)
	require.NoError(t, err)
	stateDB, state, store := stateAndStore(t, conf, pubKey, 0x0)
	stateStore := sm.NewStore(stateDB)

	// now start the app using the handshake - it should sync
	genDoc, _ := sm.MakeGenesisDocFromFile(conf.GenesisFile())
	handshaker := NewHandshaker(
		logger,
		stateStore,
		state,
		store,
		eventBus,
		genDoc,
		proTxHash,
		conf.Consensus.AppHashSize,
	)

	proxyApp := proxy.New(client, logger, proxy.NopMetrics())
	require.NoError(t, proxyApp.Start(ctx), "Error starting proxy app connections")
	_, err = handshaker.Handshake(ctx, proxyApp)
	require.NoError(t, err, "error on abci handshake")

	// reload the state, check the validator set was updated
	state, err = stateStore.Load()
	require.NoError(t, err)
	assert.Equal(t, InitialCoreHeight, state.LastCoreChainLockedBlockHeight)
	assert.Equal(t, InitialCoreHeight, handshaker.initialState.LastCoreChainLockedBlockHeight)
}

// returns the vals on InitChain
type initChainApp struct {
	abci.BaseApplication
	vals              *abci.ValidatorSetUpdate
	initialCoreHeight uint32
}

func (ica *initChainApp) InitChain(_ context.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
	resp := abci.ResponseInitChain{
		InitialCoreHeight: ica.initialCoreHeight,
	}
	if ica.vals != nil {
		resp.ValidatorSetUpdate = *ica.vals
	}
	return &resp, nil
}

func randValidator() (*types.Validator, types.PrivValidator) {
	quorumHash := crypto.RandQuorumHash()
	privVal := types.NewMockPVForQuorum(quorumHash)
	proTxHash, _ := privVal.GetProTxHash(context.Background())
	pubKey, _ := privVal.GetPubKey(context.Background(), quorumHash)
	val := types.NewValidatorDefaultVotingPower(pubKey, proTxHash)
	return val, privVal
}
