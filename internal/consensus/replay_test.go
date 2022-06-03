package consensus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
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

func findProposer(ctx context.Context, t *testing.T, validatorStubs []*validatorStub, proTxHash crypto.ProTxHash) *validatorStub {
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
	t.Logf("genesis quorum hash is %X\n", genDoc.QuorumHash)
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

	sqm, err := newStateQuorumManager(css)
	require.NoError(t, err)

	// HEIGHT 2
	hvsu2, err := sqm.addValidator(height, 1)
	require.NoError(t, err)
	height++
	incrementHeight(vss...)

	err = assertMempool(t, css[0].txNotifier).CheckTx(ctx, hvsu2.tx, nil, mempool.TxInfo{})
	assert.Nil(t, err)

	propBlock, _ := css[0].createProposalBlock(ctx) // changeProposer(t, cs1, vs2)
	propBlockParts, err := propBlock.MakePartSet(partSize)
	require.NoError(t, err)
	blockID = types.BlockID{Hash: propBlock.Hash(), PartSetHeader: propBlockParts.Header()}

	proposal := types.NewProposal(vss[1].Height, 1, round, -1, blockID, propBlock.Header.Time)
	p := proposal.ToProto()
	if _, err := vss[1].SignProposal(ctx, cfg.ChainID(), genDoc.QuorumType, genDoc.QuorumHash, p); err != nil {
		t.Fatal("failed to sign bad proposal", err)
	}
	proposal.Signature = p.Signature

	// set the proposal block to state on node 0, this will result in a signed prevote,
	// so we do not need to prevote with it again (hence the vss[1:nVals])
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
	propBlock, _ = css[0].createProposalBlock(ctx) // changeProposer(t, cs1, vs2)
	propBlockParts, err = propBlock.MakePartSet(partSize)
	require.NoError(t, err)
	blockID = types.BlockID{Hash: propBlock.Hash(), PartSetHeader: propBlockParts.Header()}

	proposal = types.NewProposal(vss[2].Height, 1, round, -1, blockID, propBlock.Header.Time)
	p = proposal.ToProto()
	if _, err := vss[2].SignProposal(ctx, cfg.ChainID(), genDoc.QuorumType, genDoc.QuorumHash, p); err != nil {
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
	// 1 new validator comes in here from block 2
	hvsu4, err := sqm.addValidator(height, 2)
	require.NoError(t, err)
	height++
	incrementHeight(vss...)

	err = assertMempool(t, css[0].txNotifier).CheckTx(ctx, hvsu4.tx, nil, mempool.TxInfo{})
	assert.Nil(t, err)

	propBlock, err = css[0].createProposalBlock(ctx) // changeProposer(t, cs1, vs2)
	require.NoError(t, err)
	propBlockParts, err = propBlock.MakePartSet(partSize)
	require.NoError(t, err)
	require.Equal(t, 1, len(propBlock.Txs), "there should be 1 transaction")
	require.Equal(t, hvsu4.tx, []byte(propBlock.Txs[0]))
	blockID = types.BlockID{Hash: propBlock.Hash(), PartSetHeader: propBlockParts.Header()}

	vssProposer := findProposer(ctx, t, vss, css[0].Validators.Proposer.ProTxHash)
	proposal = types.NewProposal(vss[3].Height, 1, round, -1, blockID, propBlock.Header.Time)
	p = proposal.ToProto()
	if _, err := vssProposer.SignProposal(ctx, cfg.ChainID(), genDoc.QuorumType, hvsu2.quorumHash, p); err != nil {
		t.Fatal("failed to sign bad proposal", err)
	}
	proposal.Signature = p.Signature

	// set the proposal block
	if err := css[0].SetProposalAndBlock(ctx, proposal, propBlock, propBlockParts, "some peer"); err != nil {
		t.Fatal(err)
	}
	ensureNewProposal(t, proposalCh, height, round)
	vssForSigning := vss[:nVals+1]
	vssForSigning = sortVValidatorStubsByPower(ctx, t, vssForSigning)

	valIndexFn := func(cssIdx int) int {
		for i, vs := range vssForSigning {
			vsProTxHash, err := vs.GetProTxHash(ctx)
			require.NoError(t, err)

			cssProTxHash, err := css[cssIdx].privValidator.GetProTxHash(ctx)
			require.NoError(t, err)

			if bytes.Equal(vsProTxHash, cssProTxHash) {
				return i
			}
		}
		t.Fatalf("validator css[%d] not found in newVss", cssIdx)
		return -1
	}

	selfIndex := valIndexFn(0)
	require.NotEqual(t, -1, selfIndex)

	// A new validator should come in
	for i := 0; i < nVals+1; i++ {
		if i == selfIndex {
			continue
		}
		signAddVotes(
			ctx, t,
			css[0],
			tmproto.PrecommitType,
			sim.Config.ChainID(),
			blockID,
			vssForSigning[i],
		)
	}
	ensureNewRound(t, newRoundCh, height+1, 0)

	// HEIGHT 5
	height++
	incrementHeight(vss...)
	propBlock, _ = css[0].createProposalBlock(ctx) // changeProposer(t, cs1, vs2)
	propBlockParts, err = propBlock.MakePartSet(partSize)
	require.NoError(t, err)
	blockID = types.BlockID{Hash: propBlock.Hash(), PartSetHeader: propBlockParts.Header()}

	proposal = types.NewProposal(vss[2].Height, 1, round, -1, blockID, propBlock.Header.Time)
	p = proposal.ToProto()
	proposerProTxHash := css[0].RoundState.Validators.GetProposer().ProTxHash
	valIndexFnByProTxHash := func(proTxHash crypto.ProTxHash) int {
		for i, vs := range vss {
			vsProTxHash, err := vs.GetProTxHash(ctx)
			require.NoError(t, err)

			if bytes.Equal(vsProTxHash, proposerProTxHash) {
				return i
			}
		}
		panic(fmt.Sprintf("validator proTxHash %X not found in newVss", proposerProTxHash))
	}
	proposerIndex := valIndexFnByProTxHash(proposerProTxHash)
	if _, err := vss[proposerIndex].SignProposal(ctx, cfg.ChainID(), genDoc.QuorumType, hvsu2.quorumHash, p); err != nil {
		t.Fatal("failed to sign bad proposal", err)
	}
	proposal.Signature = p.Signature

	// set the proposal block
	if err := css[0].SetProposalAndBlock(ctx, proposal, propBlock, propBlockParts, "some peer"); err != nil {
		t.Fatal(err)
	}
	ensureNewProposal(t, proposalCh, height, round)
	for i := 0; i < nVals+1; i++ {
		if i == selfIndex {
			continue
		}
		signAddVotes(
			ctx, t,
			css[0],
			tmproto.PrecommitType,
			sim.Config.ChainID(),
			blockID,
			vssForSigning[i],
		)
	}
	ensureNewRound(t, newRoundCh, height+1, 0)

	// HEIGHT 6
	hvsu6, err := sqm.remValidators(height, 1)
	require.NoError(t, err)
	height++
	incrementHeight(vss...)

	err = assertMempool(t, css[0].txNotifier).CheckTx(ctx, hvsu6.tx, nil, mempool.TxInfo{})
	require.NoError(t, err)

	propBlock, _ = css[0].createProposalBlock(ctx) // changeProposer(t, cs1, vs2)
	propBlockParts, err = propBlock.MakePartSet(partSize)
	require.NoError(t, err)
	blockID = types.BlockID{Hash: propBlock.Hash(), PartSetHeader: propBlockParts.Header()}

	proposal = types.NewProposal(vss[2].Height, 1, round, -1, blockID, propBlock.Header.Time)
	p = proposal.ToProto()
	proposer := css[0].RoundState.Validators.GetProposer()
	proposerProTxHash = proposer.ProTxHash
	proposerPubKey := proposer.PubKey
	valIndexFnByProTxHash = func(proTxHash crypto.ProTxHash) int {
		for i, vs := range vss {
			vsProTxHash, err := vs.GetProTxHash(ctx)
			require.NoError(t, err)

			if bytes.Equal(vsProTxHash, proposerProTxHash) {
				return i
			}
		}
		panic(fmt.Sprintf(
			"validator proTxHash %X not found in newVss",
			proposerProTxHash,
		))
	}
	proposerIndex = valIndexFnByProTxHash(proposerProTxHash)
	validatorsAtProposalHeight := css[0].state.ValidatorsAtHeight(p.Height)

	signID, err :=
		vss[proposerIndex].SignProposal(
			ctx,
			cfg.ChainID(),
			genDoc.QuorumType,
			validatorsAtProposalHeight.QuorumHash,
			p,
		)
	if err != nil {
		t.Fatal("failed to sign bad proposal", err)
	}

	proposerPubKey2, err := vss[proposerIndex].GetPubKey(context.Background(), validatorsAtProposalHeight.QuorumHash)
	if err != nil {
		t.Fatal("failed to get public key")
	}
	proposerProTxHash2, err := vss[proposerIndex].GetProTxHash(context.Background())

	if !bytes.Equal(proposerProTxHash2.Bytes(), proposerProTxHash.Bytes()) {
		t.Fatal("wrong proposer", err)
	}

	if !bytes.Equal(proposerPubKey2.Bytes(), proposerPubKey.Bytes()) {
		t.Fatal("wrong proposer pubKey", err)
	}

	css[0].logger.Debug(
		"signed proposal",
		"height", proposal.Height,
		"round", proposal.Round,
		"proposer", proposerProTxHash.ShortString(),
		"signature", p.Signature,
		"pubkey", proposerPubKey2.Bytes(),
		"quorum type", validatorsAtProposalHeight.QuorumType,
		"quorum hash", validatorsAtProposalHeight.QuorumHash,
		"signID", signID,
	)

	proposal.Signature = p.Signature

	// set the proposal block
	if err := css[0].SetProposalAndBlock(ctx, proposal, propBlock, propBlockParts, "some peer"); err != nil {
		t.Fatal(err)
	}
	ensureNewProposal(t, proposalCh, height, round)

	vssForSigning = vss[0 : nVals+3]
	vssForSigning = sortVValidatorStubsByPower(ctx, t, vssForSigning)

	selfIndex = valIndexFn(0)

	// All validators should be in now
	for i := 0; i < nVals+3; i++ {
		if i == selfIndex {
			continue
		}
		signAddVotes(
			ctx, t,
			css[0],
			tmproto.PrecommitType,
			sim.Config.ChainID(),
			blockID,
			vssForSigning[i],
		)
	}

	ensureNewRound(t, newRoundCh, height+1, 0)

	// HEIGHT 7
	height++
	incrementHeight(vss...)
	propBlock, _ = css[0].createProposalBlock(ctx) // changeProposer(t, cs1, vs2)
	propBlockParts, err = propBlock.MakePartSet(partSize)
	require.NoError(t, err)
	blockID = types.BlockID{Hash: propBlock.Hash(), PartSetHeader: propBlockParts.Header()}

	proposal = types.NewProposal(vss[2].Height, 1, round, -1, blockID, propBlock.Header.Time)
	p = proposal.ToProto()
	proposerProTxHash = css[0].RoundState.Validators.GetProposer().ProTxHash
	proposerIndex = valIndexFnByProTxHash(proposerProTxHash)
	validatorsAtProposalHeight = css[0].state.ValidatorsAtHeight(p.Height)
	if _, err := vss[proposerIndex].SignProposal(
		context.Background(),
		cfg.ChainID(),
		genDoc.QuorumType,
		validatorsAtProposalHeight.QuorumHash,
		p,
	); err != nil {
		t.Fatal("failed to sign bad proposal", err)
	}
	proposal.Signature = p.Signature

	// set the proposal block
	if err := css[0].SetProposalAndBlock(ctx, proposal, propBlock, propBlockParts, "some peer"); err != nil {
		t.Fatal(err)
	}
	selfIndex = valIndexFn(0)
	require.NotEqual(t, -1, selfIndex)
	ensureNewProposal(t, proposalCh, height, round)

	// Still have 7 validators
	for i := 0; i < nVals+3; i++ {
		if i == selfIndex {
			continue
		}
		signAddVotes(
			ctx, t,
			css[0],
			tmproto.PrecommitType,
			sim.Config.ChainID(),
			blockID,
			vssForSigning[i],
		)
	}
	ensureNewRound(t, newRoundCh, height+1, 0)

	// HEIGHT 8
	proTxHashToRemove := hvsu6.ProTxHashes[len(hvsu6.ProTxHashes)-1]
	hvsu8, err := sqm.remValidatorsByProTxHash(height, []crypto.ProTxHash{proTxHashToRemove})
	require.NoError(t, err)
	height++
	incrementHeight(vss...)

	err = assertMempool(t, css[0].txNotifier).CheckTx(context.Background(), hvsu8.tx, nil, mempool.TxInfo{})
	assert.Nil(t, err)

	propBlock, _ = css[0].createProposalBlock(ctx) // changeProposer(t, cs1, vs2)
	propBlockParts, err = propBlock.MakePartSet(partSize)
	require.NoError(t, err)
	blockID = types.BlockID{Hash: propBlock.Hash(), PartSetHeader: propBlockParts.Header()}

	proposal = types.NewProposal(vss[5].Height, 1, round, -1, blockID, propBlock.Header.Time)
	p = proposal.ToProto()
	proposer = css[0].RoundState.Validators.GetProposer()
	proposerProTxHash = proposer.ProTxHash
	proposerPubKey = proposer.PubKey
	proposerIndex = valIndexFnByProTxHash(proposerProTxHash)
	validatorsAtProposalHeight = css[0].state.ValidatorsAtHeight(p.Height)
	signID, err = vss[proposerIndex].SignProposal(
		context.Background(),
		cfg.ChainID(),
		genDoc.QuorumType,
		validatorsAtProposalHeight.QuorumHash,
		p,
	)

	if err != nil {
		t.Fatal("failed to sign bad proposal", err)
	}

	// proposerPubKey2, _ = vss[proposerIndex].GetPubKey(validatorsAtProposalHeight.QuorumHash)

	/*
		if !bytes.Equal(proposerPubKey2.Bytes(), proposerPubKey.Bytes()) {
			//t.Fatal("wrong proposer pubKey", err)
		}*/

	css[0].logger.Debug(
		"signed proposal", "height", proposal.Height, "round", proposal.Round,
		"proposer", proposerProTxHash.ShortString(), "signature", p.Signature,
		"pubkey", proposerPubKey.Bytes(), "quorum type",
		validatorsAtProposalHeight.QuorumType, "quorum hash",
		validatorsAtProposalHeight.QuorumHash, "signID", signID)

	proposal.Signature = p.Signature

	// set the proposal block
	if err := css[0].SetProposalAndBlock(ctx, proposal, propBlock, propBlockParts, "some peer"); err != nil {
		t.Fatal(err)
	}

	ensureNewProposal(t, proposalCh, height, round)
	// Reflect the changes to vss[nVals] at height 3 and resort newVss.
	vssForSigning = vss[0 : nVals+3]
	sortVValidatorStubsByPower(ctx, t, vssForSigning)
	vssForSigning = vssForSigning[0 : nVals+2]
	selfIndex = valIndexFn(0)

	for i := 0; i < nVals+2; i++ {
		if i == selfIndex {
			continue
		}
		signAddVotes(
			ctx, t,
			css[0],
			tmproto.PrecommitType,
			sim.Config.ChainID(),
			blockID,
			vssForSigning[i],
		)
	}
	ensureNewRound(t, newRoundCh, height+1, 0)

	sim.Chain = make([]*types.Block, 0)
	sim.Commits = make([]*types.Commit, 0)
	for i := 1; i <= numBlocks; i++ {
		sim.Chain = append(sim.Chain, css[0].blockStore.LoadBlock(int64(i)))
		sim.Commits = append(sim.Commits, css[0].blockStore.LoadBlockCommit(int64(i)))
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
	var commits []*types.Commit
	var store *mockBlockStore
	var stateDB dbm.DB
	var genesisState sm.State
	var privVal types.PrivValidator

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
		commits = sim.Commits
		store = newMockBlockStore(t, cfg, genesisState.ConsensusParams)
		privVal, err = privval.LoadFilePV(cfg.PrivValidator.KeyFile(), cfg.PrivValidator.StateFile())
		require.NoError(t, err)
	} else { // test single node
		testConfig, err := ResetConfig(t.TempDir(), fmt.Sprintf("%s_%v_s", t.Name(), mode))
		require.NoError(t, err)
		defer func() { _ = os.RemoveAll(testConfig.RootDir) }()
		walBody, err := WALWithNBlocks(ctx, t, logger, numBlocks)
		require.NoError(t, err)
		walFile := tempWALWithData(t, walBody)
		cfg.Consensus.SetWalFile(walFile)

		privVal, err = privval.LoadFilePV(cfg.PrivValidator.KeyFile(), cfg.PrivValidator.StateFile())
		require.NoError(t, err)

		gdoc, err := sm.MakeGenesisDocFromFile(cfg.GenesisFile())
		if err != nil {
			t.Error(err)
		}

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
	firstValidatorProTxHash, _ := state.Validators.GetByIndex(0)
	// run the chain through state.ApplyBlock to build up the tendermint state
	state = buildTMStateFromChain(
		ctx,
		t,
		cfg,
		logger,
		sim.Mempool,
		sim.Evpool,
		stateStore,
		firstValidatorProTxHash,
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
		buildAppStateFromChain(
			ctx, t,
			proxyApp,
			stateStore,
			firstValidatorProTxHash,
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
	stateStore sm.Store,
	mempool mempool.Mempool,
	evpool sm.EvidencePool,
	st sm.State,
	nodeProTxHash crypto.ProTxHash,
	blk *types.Block,
	appClient abciclient.Client,
	blockStore *mockBlockStore,
	eventBus *eventbus.EventBus,
) sm.State {
	testPartSize := types.BlockPartSizeBytes
	blockExec := sm.NewBlockExecutor(
		stateStore,
		log.NewNopLogger(),
		appClient,
		mempool,
		evpool,
		blockStore,
		eventBus,
		sm.NopMetrics(),
	)

	bps, err := blk.MakePartSet(testPartSize)
	require.NoError(t, err)
	blkID := types.BlockID{Hash: blk.Hash(), PartSetHeader: bps.Header()}
	newState, err := blockExec.ApplyBlock(ctx, st, nodeProTxHash, blkID, blk)
	require.NoError(t, err)
	return newState
}

func buildAppStateFromChain(
	ctx context.Context,
	t *testing.T,
	appClient abciclient.Client,
	stateStore sm.Store,
	nodeProTxHash crypto.ProTxHash,
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

	switch mode {
	case 0:
		for i := 0; i < nBlocks; i++ {
			block := chain[i]
			state = applyBlock(ctx, t, stateStore, mempool, evpool, state, nodeProTxHash, block, appClient, blockStore, eventBus)
		}
	case 1, 2, 3:
		for i := 0; i < nBlocks-1; i++ {
			block := chain[i]
			state = applyBlock(ctx, t, stateStore, mempool, evpool, state, nodeProTxHash, block, appClient, blockStore, eventBus)
		}

		if mode == 2 || mode == 3 {
			// update the kvstore height and apphash
			// as if we ran commit but not
			state = applyBlock(ctx, t, stateStore, mempool, evpool, state, nodeProTxHash, chain[nBlocks-1], appClient, blockStore, eventBus)
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
	nodeProTxHash crypto.ProTxHash,
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
		ValidatorSet: &validators,
	})
	require.NoError(t, err)

	require.NoError(t, stateStore.Save(state))

	eventBus := eventbus.NewDefault(logger)
	require.NoError(t, eventBus.Start(ctx))

	switch mode {
	case 0:
		// sync right up
		for _, block := range chain {
			state = applyBlock(ctx, t, stateStore, mempool, evpool, state, nodeProTxHash, block, proxyApp, blockStore, eventBus)
		}

	case 1, 2, 3:
		// sync up to the penultimate as if we stored the block.
		// whether we commit or not depends on the appHash
		for _, block := range chain[:len(chain)-1] {
			state = applyBlock(ctx, t, stateStore, mempool, evpool, state, nodeProTxHash, block, proxyApp, blockStore, eventBus)
		}

		// apply the final block to a state copy so we can
		// get the right next appHash but keep the state back
		applyBlock(ctx, t, stateStore, mempool, evpool, state, nodeProTxHash, chain[len(chain)-1], proxyApp, blockStore, eventBus)
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

func (app *badApp) Commit(context.Context) (*abci.ResponseCommit, error) {
	app.height++
	if app.onlyLastHashIsWrong {
		if app.height == app.numBlocks {
			return &abci.ResponseCommit{Data: tmrand.Bytes(32)}, nil
		}
		return &abci.ResponseCommit{Data: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
			0, 0, 0, 0, 0, 0, 0, 0, app.height}}, nil
	} else if app.allHashesAreWrong {
		return &abci.ResponseCommit{Data: tmrand.Bytes(32)}, nil
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
				// previous block, needed to detemine StateID
				var stateID types.StateID
				if len(blocks) >= 1 {
					prevBlock := blocks[len(blocks)-1]
					stateID = types.StateID{Height: prevBlock.Height, LastAppHash: prevBlock.AppHash}
				} else {
					stateID = types.StateID{Height: genDoc.InitialHeight, LastAppHash: genDoc.AppHash}
				}

				thisBlockCommit = types.NewCommit(p.Height, p.Round,
					p.BlockID, stateID,
					&types.QuorumVoteSigns{
						ThresholdVoteSigns: types.ThresholdVoteSigns{
							BlockSign:    p.BlockSignature,
							StateSign:    p.StateSignature,
							VoteExtSigns: types.VoteExtSigns2BytesSlices(p.VoteExtensions, true),
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
