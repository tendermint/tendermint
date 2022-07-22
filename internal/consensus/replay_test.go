package consensus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"runtime"
	"testing"
	"time"

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
	"github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/internal/eventbus"
	"github.com/tendermint/tendermint/internal/mempool"
	"github.com/tendermint/tendermint/internal/proxy"
	"github.com/tendermint/tendermint/internal/pubsub"
	sm "github.com/tendermint/tendermint/internal/state"
	sf "github.com/tendermint/tendermint/internal/state/test/factory"
	"github.com/tendermint/tendermint/internal/store"
	"github.com/tendermint/tendermint/internal/test/factory"
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
		logger := log.NewNopLogger()
		blockDB := dbm.NewMemDB()
		stateDB := dbm.NewMemDB()
		stateStore := sm.NewStore(stateDB)
		blockStore := store.NewBlockStore(blockDB)
		state, err := sm.MakeGenesisStateFromFile(consensusReplayConfig.GenesisFile())
		require.NoError(t, err)
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
	ExtCommits   []*types.ExtendedCommit
	CleanupFunc  cleanupFunc

	Mempool mempool.Mempool
	Evpool  sm.EvidencePool
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

// This is actually not a test, it's for storing validator change tx data for testHandshakeReplay
func setupSimulator(ctx context.Context, t *testing.T) *simulatorTestSuite {
	t.Helper()
	cfg := configSetup(t)

	sim := &simulatorTestSuite{
		Mempool: emptyMempool{},
		Evpool:  sm.EmptyEvidencePool{},
	}

	nPeers := 7
	nVals := 4

	css, genDoc, cfg, cleanup := randConsensusNetWithPeers(
		ctx,
		t,
		cfg,
		nVals,
		nPeers,
		"replay_test",
		newMockTickerFunc(true),
		newEpehemeralKVStore)
	sim.Config = cfg
	defer func() { t.Cleanup(cleanup) }()

	var err error
	sim.GenesisState, err = sm.MakeGenesisState(genDoc)
	require.NoError(t, err)

	partSize := types.BlockPartSizeBytes

	newRoundCh := subscribe(ctx, t, css[0].eventBus, types.EventQueryNewRound)
	proposalCh := subscribe(ctx, t, css[0].eventBus, types.EventQueryCompleteProposal)

	vss := make([]*validatorStub, nPeers)
	for i := 0; i < nPeers; i++ {
		vss[i] = newValidatorStub(css[i].privValidator, int32(i))
	}
	height, round := css[0].Height, css[0].Round

	// start the machine
	startTestRound(ctx, css[0], height, round)
	incrementHeight(vss...)
	ensureNewRound(t, newRoundCh, height, 0)
	ensureNewProposal(t, proposalCh, height, round)
	rs := css[0].GetRoundState()

	signAddVotes(ctx, t, css[0], tmproto.PrecommitType, sim.Config.ChainID(),
		types.BlockID{Hash: rs.ProposalBlock.Hash(), PartSetHeader: rs.ProposalBlockParts.Header()},
		vss[1:nVals]...)

	ensureNewRound(t, newRoundCh, height+1, 0)

	// HEIGHT 2
	height++
	incrementHeight(vss...)
	newValidatorPubKey1, err := css[nVals].privValidator.GetPubKey(ctx)
	require.NoError(t, err)
	valPubKey1ABCI, err := encoding.PubKeyToProto(newValidatorPubKey1)
	require.NoError(t, err)
	newValidatorTx1 := kvstore.MakeValSetChangeTx(valPubKey1ABCI, testMinPower)
	err = assertMempool(t, css[0].txNotifier).CheckTx(ctx, newValidatorTx1, nil, mempool.TxInfo{})
	assert.NoError(t, err)
	propBlock, err := css[0].createProposalBlock(ctx) // changeProposer(t, cs1, vs2)
	require.NoError(t, err)
	propBlockParts, err := propBlock.MakePartSet(partSize)
	require.NoError(t, err)
	blockID := types.BlockID{Hash: propBlock.Hash(), PartSetHeader: propBlockParts.Header()}

	proposal := types.NewProposal(vss[1].Height, round, -1, blockID, propBlock.Header.Time)
	p := proposal.ToProto()
	if err := vss[1].SignProposal(ctx, cfg.ChainID(), p); err != nil {
		t.Fatal("failed to sign bad proposal", err)
	}
	proposal.Signature = p.Signature

	// set the proposal block
	if err := css[0].SetProposalAndBlock(ctx, proposal, propBlock, propBlockParts, "some peer"); err != nil {
		t.Fatal(err)
	}
	ensureNewProposal(t, proposalCh, height, round)
	rs = css[0].GetRoundState()
	signAddVotes(ctx, t, css[0], tmproto.PrecommitType, sim.Config.ChainID(),
		types.BlockID{Hash: rs.ProposalBlock.Hash(), PartSetHeader: rs.ProposalBlockParts.Header()},
		vss[1:nVals]...)
	ensureNewRound(t, newRoundCh, height+1, 0)

	// HEIGHT 3
	height++
	incrementHeight(vss...)
	updateValidatorPubKey1, err := css[nVals].privValidator.GetPubKey(ctx)
	require.NoError(t, err)
	updatePubKey1ABCI, err := encoding.PubKeyToProto(updateValidatorPubKey1)
	require.NoError(t, err)
	updateValidatorTx1 := kvstore.MakeValSetChangeTx(updatePubKey1ABCI, 25)
	err = assertMempool(t, css[0].txNotifier).CheckTx(ctx, updateValidatorTx1, nil, mempool.TxInfo{})
	assert.NoError(t, err)
	propBlock, err = css[0].createProposalBlock(ctx) // changeProposer(t, cs1, vs2)
	require.NoError(t, err)
	propBlockParts, err = propBlock.MakePartSet(partSize)
	require.NoError(t, err)
	blockID = types.BlockID{Hash: propBlock.Hash(), PartSetHeader: propBlockParts.Header()}

	proposal = types.NewProposal(vss[2].Height, round, -1, blockID, propBlock.Header.Time)
	p = proposal.ToProto()
	if err := vss[2].SignProposal(ctx, cfg.ChainID(), p); err != nil {
		t.Fatal("failed to sign bad proposal", err)
	}
	proposal.Signature = p.Signature

	// set the proposal block
	if err := css[0].SetProposalAndBlock(ctx, proposal, propBlock, propBlockParts, "some peer"); err != nil {
		t.Fatal(err)
	}
	ensureNewProposal(t, proposalCh, height, round)
	rs = css[0].GetRoundState()
	signAddVotes(ctx, t, css[0], tmproto.PrecommitType, sim.Config.ChainID(),
		types.BlockID{Hash: rs.ProposalBlock.Hash(), PartSetHeader: rs.ProposalBlockParts.Header()},
		vss[1:nVals]...)
	ensureNewRound(t, newRoundCh, height+1, 0)

	// HEIGHT 4
	height++
	incrementHeight(vss...)
	newValidatorPubKey2, err := css[nVals+1].privValidator.GetPubKey(ctx)
	require.NoError(t, err)
	newVal2ABCI, err := encoding.PubKeyToProto(newValidatorPubKey2)
	require.NoError(t, err)
	newValidatorTx2 := kvstore.MakeValSetChangeTx(newVal2ABCI, testMinPower)
	err = assertMempool(t, css[0].txNotifier).CheckTx(ctx, newValidatorTx2, nil, mempool.TxInfo{})
	assert.NoError(t, err)
	newValidatorPubKey3, err := css[nVals+2].privValidator.GetPubKey(ctx)
	require.NoError(t, err)
	newVal3ABCI, err := encoding.PubKeyToProto(newValidatorPubKey3)
	require.NoError(t, err)
	newValidatorTx3 := kvstore.MakeValSetChangeTx(newVal3ABCI, testMinPower)
	err = assertMempool(t, css[0].txNotifier).CheckTx(ctx, newValidatorTx3, nil, mempool.TxInfo{})
	assert.NoError(t, err)
	propBlock, err = css[0].createProposalBlock(ctx) // changeProposer(t, cs1, vs2)
	require.NoError(t, err)
	propBlockParts, err = propBlock.MakePartSet(partSize)
	require.NoError(t, err)
	blockID = types.BlockID{Hash: propBlock.Hash(), PartSetHeader: propBlockParts.Header()}
	newVss := make([]*validatorStub, nVals+1)
	copy(newVss, vss[:nVals+1])
	newVss = sortVValidatorStubsByPower(ctx, t, newVss)

	valIndexFn := func(cssIdx int) int {
		for i, vs := range newVss {
			vsPubKey, err := vs.GetPubKey(ctx)
			require.NoError(t, err)

			cssPubKey, err := css[cssIdx].privValidator.GetPubKey(ctx)
			require.NoError(t, err)

			if vsPubKey.Equals(cssPubKey) {
				return i
			}
		}
		t.Fatalf("validator css[%d] not found in newVss", cssIdx)
		return -1
	}

	selfIndex := valIndexFn(0)
	require.NotEqual(t, -1, selfIndex)

	proposal = types.NewProposal(vss[3].Height, round, -1, blockID, propBlock.Header.Time)
	p = proposal.ToProto()
	if err := vss[3].SignProposal(ctx, cfg.ChainID(), p); err != nil {
		t.Fatal("failed to sign bad proposal", err)
	}
	proposal.Signature = p.Signature

	// set the proposal block
	if err := css[0].SetProposalAndBlock(ctx, proposal, propBlock, propBlockParts, "some peer"); err != nil {
		t.Fatal(err)
	}
	ensureNewProposal(t, proposalCh, height, round)

	removeValidatorTx2 := kvstore.MakeValSetChangeTx(newVal2ABCI, 0)
	err = assertMempool(t, css[0].txNotifier).CheckTx(ctx, removeValidatorTx2, nil, mempool.TxInfo{})
	assert.NoError(t, err)

	rs = css[0].GetRoundState()
	for i := 0; i < nVals+1; i++ {
		if i == selfIndex {
			continue
		}
		signAddVotes(ctx, t, css[0],
			tmproto.PrecommitType, sim.Config.ChainID(),
			types.BlockID{Hash: rs.ProposalBlock.Hash(), PartSetHeader: rs.ProposalBlockParts.Header()},
			newVss[i])
	}
	ensureNewRound(t, newRoundCh, height+1, 0)

	// HEIGHT 5
	height++
	incrementHeight(vss...)
	// Reflect the changes to vss[nVals] at height 3 and resort newVss.
	newVssIdx := valIndexFn(nVals)
	require.NotEqual(t, -1, newVssIdx)

	newVss[newVssIdx].VotingPower = 25
	newVss = sortVValidatorStubsByPower(ctx, t, newVss)

	selfIndex = valIndexFn(0)
	require.NotEqual(t, -1, selfIndex)
	ensureNewProposal(t, proposalCh, height, round)
	rs = css[0].GetRoundState()
	for i := 0; i < nVals+1; i++ {
		if i == selfIndex {
			continue
		}
		signAddVotes(ctx, t, css[0],
			tmproto.PrecommitType, sim.Config.ChainID(),
			types.BlockID{Hash: rs.ProposalBlock.Hash(), PartSetHeader: rs.ProposalBlockParts.Header()},
			newVss[i])
	}
	ensureNewRound(t, newRoundCh, height+1, 0)

	// HEIGHT 6
	height++
	incrementHeight(vss...)
	removeValidatorTx3 := kvstore.MakeValSetChangeTx(newVal3ABCI, 0)
	err = assertMempool(t, css[0].txNotifier).CheckTx(ctx, removeValidatorTx3, nil, mempool.TxInfo{})
	assert.NoError(t, err)
	propBlock, err = css[0].createProposalBlock(ctx) // changeProposer(t, cs1, vs2)
	require.NoError(t, err)
	propBlockParts, err = propBlock.MakePartSet(partSize)
	require.NoError(t, err)
	blockID = types.BlockID{Hash: propBlock.Hash(), PartSetHeader: propBlockParts.Header()}
	newVss = make([]*validatorStub, nVals+3)
	copy(newVss, vss[:nVals+3])
	newVss = sortVValidatorStubsByPower(ctx, t, newVss)

	selfIndex = valIndexFn(0)
	require.NotEqual(t, -1, selfIndex)
	proposal = types.NewProposal(vss[1].Height, round, -1, blockID, propBlock.Header.Time)
	p = proposal.ToProto()
	if err := vss[1].SignProposal(ctx, cfg.ChainID(), p); err != nil {
		t.Fatal("failed to sign bad proposal", err)
	}
	proposal.Signature = p.Signature

	// set the proposal block
	if err := css[0].SetProposalAndBlock(ctx, proposal, propBlock, propBlockParts, "some peer"); err != nil {
		t.Fatal(err)
	}
	ensureNewProposal(t, proposalCh, height, round)
	rs = css[0].GetRoundState()
	for i := 0; i < nVals+3; i++ {
		if i == selfIndex {
			continue
		}
		signAddVotes(ctx, t, css[0],
			tmproto.PrecommitType, sim.Config.ChainID(),
			types.BlockID{Hash: rs.ProposalBlock.Hash(), PartSetHeader: rs.ProposalBlockParts.Header()},
			newVss[i])
	}
	ensureNewRound(t, newRoundCh, height+1, 0)

	sim.Chain = []*types.Block{}
	sim.ExtCommits = []*types.ExtendedCommit{}
	for i := 1; i <= numBlocks; i++ {
		sim.Chain = append(sim.Chain, css[0].blockStore.LoadBlock(int64(i)))
		sim.ExtCommits = append(sim.ExtCommits, css[0].blockStore.LoadBlockExtendedCommit(int64(i)))
	}

	return sim
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
		testHandshakeReplay(ctx, t, sim, numBlocks-1, m, false)
	}
	for _, m := range modes {
		testHandshakeReplay(ctx, t, sim, numBlocks-1, m, true)
	}
}

// Sync from caught up
func TestHandshakeReplayNone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sim := setupSimulator(ctx, t)

	t.Cleanup(leaktest.Check(t))

	for _, m := range modes {
		testHandshakeReplay(ctx, t, sim, numBlocks, m, false)
	}
	for _, m := range modes {
		testHandshakeReplay(ctx, t, sim, numBlocks, m, true)
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
	var chain []*types.Block
	var extCommits []*types.ExtendedCommit
	var store *mockBlockStore
	var stateDB dbm.DB
	var genesisState sm.State

	ctx, cancel := context.WithCancel(rctx)
	t.Cleanup(cancel)

	cfg := sim.Config

	logger := log.NewNopLogger()
	if testValidatorsChange {
		testConfig, err := ResetConfig(t.TempDir(), fmt.Sprintf("%s_%v_m", t.Name(), mode))
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(testConfig.RootDir) }()
		stateDB = dbm.NewMemDB()

		genesisState = sim.GenesisState
		cfg = sim.Config
		chain = append([]*types.Block{}, sim.Chain...) // copy chain
		extCommits = sim.ExtCommits
		store = newMockBlockStore(t, cfg, genesisState.ConsensusParams)
	} else { // test single node
		testConfig, err := ResetConfig(t.TempDir(), fmt.Sprintf("%s_%v_s", t.Name(), mode))
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(testConfig.RootDir) }()
		walBody, err := WALWithNBlocks(ctx, t, logger, numBlocks)
		require.NoError(t, err)
		walFile := tempWALWithData(t, walBody)
		cfg.Consensus.SetWalFile(walFile)

		privVal, err := privval.LoadFilePV(cfg.PrivValidator.KeyFile(), cfg.PrivValidator.StateFile())
		require.NoError(t, err)

		wal, err := NewWAL(ctx, logger, walFile)
		require.NoError(t, err)
		err = wal.Start(ctx)
		require.NoError(t, err)
		t.Cleanup(func() { cancel(); wal.Wait() })
		chain, extCommits = makeBlockchainFromWAL(t, wal)
		pubKey, err := privVal.GetPubKey(ctx)
		require.NoError(t, err)
		stateDB, genesisState, store = stateAndStore(t, cfg, pubKey, kvstore.ProtocolVersion)

	}
	stateStore := sm.NewStore(stateDB)
	store.chain = chain
	store.extCommits = extCommits

	state := genesisState.Copy()
	// run the chain through state.ApplyBlock to build up the tendermint state
	state = buildTMStateFromChain(
		ctx,
		t,
		cfg,
		logger,
		sim.Mempool,
		sim.Evpool,
		stateStore,
		state,
		chain,
		nBlocks,
		mode,
		store,
	)
	latestAppHash := state.AppHash

	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	client := abciclient.NewLocalClient(logger, kvstore.NewApplication())
	if nBlocks > 0 {
		// run nBlocks against a new client to build up the app state.
		// use a throwaway tendermint state
		proxyApp := proxy.New(client, logger, proxy.NopMetrics())
		stateDB1 := dbm.NewMemDB()
		stateStore := sm.NewStore(stateDB1)
		err := stateStore.Save(genesisState)
		require.NoError(t, err)
		buildAppStateFromChain(ctx, t, proxyApp, stateStore, sim.Mempool, sim.Evpool, genesisState, chain, eventBus, nBlocks, mode, store)
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
	handshaker := NewHandshaker(logger, stateStore, state, store, eventBus, genDoc)
	proxyApp := proxy.New(client, logger, proxy.NopMetrics())
	require.NoError(t, proxyApp.Start(ctx), "Error starting proxy app connections")
	require.True(t, proxyApp.IsRunning())
	require.NotNil(t, proxyApp)
	t.Cleanup(func() { cancel(); proxyApp.Wait() })

	err = handshaker.Handshake(ctx, proxyApp)
	if expectError {
		require.Error(t, err)
		return
	}
	require.NoError(t, err, "Error on abci handshake")

	// get the latest app hash from the app
	res, err := proxyApp.Info(ctx, &abci.RequestInfo{Version: ""})
	if err != nil {
		t.Fatal(err)
	}

	// the app hash should be synced up
	if !bytes.Equal(latestAppHash, res.LastBlockAppHash) {
		t.Fatalf(
			"Expected app hashes to match after handshake/replay. got %X, expected %X",
			res.LastBlockAppHash,
			latestAppHash)
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

func applyBlock(
	ctx context.Context,
	t *testing.T,
	stateStore sm.Store,
	mempool mempool.Mempool,
	evpool sm.EvidencePool,
	st sm.State,
	blk *types.Block,
	appClient abciclient.Client,
	blockStore *mockBlockStore,
	eventBus *eventbus.EventBus,
) sm.State {
	testPartSize := types.BlockPartSizeBytes
	blockExec := sm.NewBlockExecutor(stateStore, log.NewNopLogger(), appClient, mempool, evpool, blockStore, eventBus, sm.NopMetrics())

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
		Validators: validators,
	})
	require.NoError(t, err)

	require.NoError(t, stateStore.Save(state)) // save height 1's validatorsInfo

	switch mode {
	case 0:
		for i := 0; i < nBlocks; i++ {
			block := chain[i]
			state = applyBlock(ctx, t, stateStore, mempool, evpool, state, block, appClient, blockStore, eventBus)
		}
	case 1, 2, 3:
		for i := 0; i < nBlocks-1; i++ {
			block := chain[i]
			state = applyBlock(ctx, t, stateStore, mempool, evpool, state, block, appClient, blockStore, eventBus)
		}

		if mode == 2 || mode == 3 {
			// update the kvstore height and apphash
			// as if we ran commit but not
			state = applyBlock(ctx, t, stateStore, mempool, evpool, state, chain[nBlocks-1], appClient, blockStore, eventBus)
		}
	default:
		require.Fail(t, "unknown mode %v", mode)
	}

}

func buildTMStateFromChain(
	ctx context.Context,
	t *testing.T,
	cfg *config.Config,
	logger log.Logger,
	mempool mempool.Mempool,
	evpool sm.EvidencePool,
	stateStore sm.Store,
	state sm.State,
	chain []*types.Block,
	nBlocks int,
	mode uint,
	blockStore *mockBlockStore,
) sm.State {
	t.Helper()

	// run the whole chain against this client to build up the tendermint state
	client := abciclient.NewLocalClient(logger, kvstore.NewApplication())

	proxyApp := proxy.New(client, logger, proxy.NopMetrics())
	require.NoError(t, proxyApp.Start(ctx))

	state.Version.Consensus.App = kvstore.ProtocolVersion // simulate handshake, receive app version
	validators := types.TM2PB.ValidatorUpdates(state.Validators)
	_, err := proxyApp.InitChain(ctx, &abci.RequestInitChain{
		Validators: validators,
	})
	require.NoError(t, err)

	require.NoError(t, stateStore.Save(state))

	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	switch mode {
	case 0:
		// sync right up
		for _, block := range chain {
			state = applyBlock(ctx, t, stateStore, mempool, evpool, state, block, proxyApp, blockStore, eventBus)
		}

	case 1, 2, 3:
		// sync up to the penultimate as if we stored the block.
		// whether we commit or not depends on the appHash
		for _, block := range chain[:len(chain)-1] {
			state = applyBlock(ctx, t, stateStore, mempool, evpool, state, block, proxyApp, blockStore, eventBus)
		}

		// apply the final block to a state copy so we can
		// get the right next appHash but keep the state back
		applyBlock(ctx, t, stateStore, mempool, evpool, state, chain[len(chain)-1], proxyApp, blockStore, eventBus)
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
	pubKey, err := privVal.GetPubKey(ctx)
	require.NoError(t, err)
	stateDB, state, store := stateAndStore(t, cfg, pubKey, appVersion)
	stateStore := sm.NewStore(stateDB)
	genDoc, err := sm.MakeGenesisDocFromFile(cfg.GenesisFile())
	require.NoError(t, err)
	state.LastValidators = state.Validators.Copy()
	// mode = 0 for committing all the blocks
	blocks := sf.MakeBlocks(ctx, t, 3, &state, privVal)

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

		h := NewHandshaker(logger, stateStore, state, store, eventBus, genDoc)
		assert.Error(t, h.Handshake(ctx, proxyApp))
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

		h := NewHandshaker(logger, stateStore, state, store, eventBus, genDoc)
		require.Error(t, h.Handshake(ctx, proxyApp))
	}
}

type badApp struct {
	abci.BaseApplication
	numBlocks           byte
	height              byte
	allHashesAreWrong   bool
	onlyLastHashIsWrong bool
}

func (app *badApp) FinalizeBlock(_ context.Context, _ *abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error) {
	app.height++
	if app.onlyLastHashIsWrong {
		if app.height == app.numBlocks {
			return &abci.ResponseFinalizeBlock{AppHash: tmrand.Bytes(8)}, nil
		}
		return &abci.ResponseFinalizeBlock{AppHash: []byte{app.height}}, nil
	} else if app.allHashesAreWrong {
		return &abci.ResponseFinalizeBlock{AppHash: tmrand.Bytes(8)}, nil
	}

	panic("either allHashesAreWrong or onlyLastHashIsWrong must be set")
}

//--------------------------
// utils for making blocks

func makeBlockchainFromWAL(t *testing.T, wal WAL) ([]*types.Block, []*types.ExtendedCommit) {
	t.Helper()
	var height int64

	// Search for height marker
	gr, found, err := wal.SearchForEndHeight(height, &WALSearchOptions{})
	require.NoError(t, err)
	require.True(t, found, "wal does not contain height %d", height)
	defer gr.Close()

	// log.Notice("Build a blockchain by reading from the WAL")

	var (
		blocks             []*types.Block
		extCommits         []*types.ExtendedCommit
		thisBlockParts     *types.PartSet
		thisBlockExtCommit *types.ExtendedCommit
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

				commitHeight := thisBlockExtCommit.Height
				require.Equal(t, commitHeight, height+1,
					"commit doesnt match. got height %d, expected %d", commitHeight, height+1)

				blocks = append(blocks, block)
				extCommits = append(extCommits, thisBlockExtCommit)
				height++
			}
		case *types.PartSetHeader:
			thisBlockParts = types.NewPartSetFromHeader(*p)
		case *types.Part:
			_, err := thisBlockParts.AddPart(p)
			require.NoError(t, err)
		case *types.Vote:
			if p.Type == tmproto.PrecommitType {
				thisBlockExtCommit = &types.ExtendedCommit{
					Height:             p.Height,
					Round:              p.Round,
					BlockID:            p.BlockID,
					ExtendedSignatures: []types.ExtendedCommitSig{p.ExtendedCommitSig()},
				}
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
	commitHeight := thisBlockExtCommit.Height
	require.Equal(t, commitHeight, height+1, "commit does not match. got height %d, expected %d", commitHeight, height+1)

	blocks = append(blocks, block)
	extCommits = append(extCommits, thisBlockExtCommit)
	return blocks, extCommits
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
	cfg        *config.Config
	params     types.ConsensusParams
	chain      []*types.Block
	extCommits []*types.ExtendedCommit
	base       int64
	t          *testing.T
}

var _ sm.BlockStore = &mockBlockStore{}

// TODO: NewBlockStore(db.NewMemDB) ...
func newMockBlockStore(t *testing.T, cfg *config.Config, params types.ConsensusParams) *mockBlockStore {
	return &mockBlockStore{
		cfg:    cfg,
		params: params,
		t:      t,
	}
}

func (bs *mockBlockStore) Height() int64                       { return int64(len(bs.chain)) }
func (bs *mockBlockStore) Base() int64                         { return bs.base }
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
func (bs *mockBlockStore) SaveBlockWithExtendedCommit(block *types.Block, blockParts *types.PartSet, seenCommit *types.ExtendedCommit) {
}
func (bs *mockBlockStore) SaveBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit) {
}

func (bs *mockBlockStore) LoadBlockCommit(height int64) *types.Commit {
	return bs.extCommits[height-1].ToCommit()
}
func (bs *mockBlockStore) LoadSeenCommit() *types.Commit {
	return bs.extCommits[len(bs.extCommits)-1].ToCommit()
}
func (bs *mockBlockStore) LoadBlockExtendedCommit(height int64) *types.ExtendedCommit {
	return bs.extCommits[height-1]
}

func (bs *mockBlockStore) PruneBlocks(height int64) (uint64, error) {
	pruned := uint64(0)
	for i := int64(0); i < height-1; i++ {
		bs.chain[i] = nil
		bs.extCommits[i] = nil
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

	logger := log.NewNopLogger()
	votePower := 10 + int64(rand.Uint32())
	val, _, err := factory.Validator(ctx, votePower)
	require.NoError(t, err)
	vals := types.NewValidatorSet([]*types.Validator{val})
	app := &initChainApp{vals: types.TM2PB.ValidatorUpdates(vals)}
	client := abciclient.NewLocalClient(logger, app)

	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	cfg, err := ResetConfig(t.TempDir(), "handshake_test_")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(cfg.RootDir) })

	privVal, err := privval.LoadFilePV(cfg.PrivValidator.KeyFile(), cfg.PrivValidator.StateFile())
	require.NoError(t, err)
	pubKey, err := privVal.GetPubKey(ctx)
	require.NoError(t, err)
	stateDB, state, store := stateAndStore(t, cfg, pubKey, 0x0)
	stateStore := sm.NewStore(stateDB)

	oldValAddr := state.Validators.Validators[0].Address

	// now start the app using the handshake - it should sync
	genDoc, err := sm.MakeGenesisDocFromFile(cfg.GenesisFile())
	require.NoError(t, err)

	handshaker := NewHandshaker(logger, stateStore, state, store, eventBus, genDoc)
	proxyApp := proxy.New(client, logger, proxy.NopMetrics())
	require.NoError(t, proxyApp.Start(ctx), "Error starting proxy app connections")

	require.NoError(t, handshaker.Handshake(ctx, proxyApp), "error on abci handshake")

	// reload the state, check the validator set was updated
	state, err = stateStore.Load()
	require.NoError(t, err)

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

func (ica *initChainApp) InitChain(_ context.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
	return &abci.ResponseInitChain{Validators: ica.vals}, nil
}
