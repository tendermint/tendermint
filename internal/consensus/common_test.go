package consensus

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	cstypes "github.com/tendermint/tendermint/internal/consensus/types"
	"github.com/tendermint/tendermint/internal/eventbus"
	"github.com/tendermint/tendermint/internal/mempool"
	tmpubsub "github.com/tendermint/tendermint/internal/pubsub"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/store"
	"github.com/tendermint/tendermint/internal/test/factory"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	tmtime "github.com/tendermint/tendermint/libs/time"
	"github.com/tendermint/tendermint/privval"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

const (
	testSubscriber = "test-client"

	// genesis, chain_id, priv_val
	ensureTimeout = time.Millisecond * 200
)

// A cleanupFunc cleans up any config / test files created for a particular
// test.
type cleanupFunc func()

func configSetup(t *testing.T) *config.Config {
	t.Helper()

	cfg, err := ResetConfig("consensus_reactor_test")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(cfg.RootDir) })

	consensusReplayConfig, err := ResetConfig("consensus_replay_test")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(consensusReplayConfig.RootDir) })

	configStateTest, err := ResetConfig("consensus_state_test")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(configStateTest.RootDir) })

	configMempoolTest, err := ResetConfig("consensus_mempool_test")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(configMempoolTest.RootDir) })

	configByzantineTest, err := ResetConfig("consensus_byzantine_test")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(configByzantineTest.RootDir) })

	return cfg
}

func ensureDir(t *testing.T, dir string, mode os.FileMode) {
	t.Helper()
	require.NoError(t, tmos.EnsureDir(dir, mode))
}

func ResetConfig(name string) (*config.Config, error) {
	return config.ResetTestRoot(name)
}

//-------------------------------------------------------------------------------
// validator stub (a kvstore consensus peer we control)

type validatorStub struct {
	Index  int32 // Validator index. NOTE: we don't assume validator set changes.
	Height int64
	Round  int32
	types.PrivValidator
	VotingPower int64
	lastVote    *types.Vote
}

const testMinPower int64 = 10

func newValidatorStub(privValidator types.PrivValidator, valIndex int32) *validatorStub {
	return &validatorStub{
		Index:         valIndex,
		PrivValidator: privValidator,
		VotingPower:   testMinPower,
	}
}

func (vs *validatorStub) signVote(
	ctx context.Context,
	cfg *config.Config,
	voteType tmproto.SignedMsgType,
	hash []byte,
	header types.PartSetHeader,
) (*types.Vote, error) {

	pubKey, err := vs.PrivValidator.GetPubKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't get pubkey: %w", err)
	}

	vote := &types.Vote{
		ValidatorIndex:   vs.Index,
		ValidatorAddress: pubKey.Address(),
		Height:           vs.Height,
		Round:            vs.Round,
		Timestamp:        tmtime.Now(),
		Type:             voteType,
		BlockID:          types.BlockID{Hash: hash, PartSetHeader: header},
	}
	v := vote.ToProto()
	if err := vs.PrivValidator.SignVote(ctx, cfg.ChainID(), v); err != nil {
		return nil, fmt.Errorf("sign vote failed: %w", err)
	}

	// ref: signVote in FilePV, the vote should use the privious vote info when the sign data is the same.
	if signDataIsEqual(vs.lastVote, v) {
		v.Signature = vs.lastVote.Signature
		v.Timestamp = vs.lastVote.Timestamp
	}

	vote.Signature = v.Signature
	vote.Timestamp = v.Timestamp

	return vote, err
}

// Sign vote for type/hash/header
func signVote(
	ctx context.Context,
	t *testing.T,
	vs *validatorStub,
	cfg *config.Config,
	voteType tmproto.SignedMsgType,
	hash []byte,
	header types.PartSetHeader,
) *types.Vote {

	v, err := vs.signVote(ctx, cfg, voteType, hash, header)
	require.NoError(t, err, "failed to sign vote")

	vs.lastVote = v

	return v
}

func signVotes(
	ctx context.Context,
	t *testing.T,
	cfg *config.Config,
	voteType tmproto.SignedMsgType,
	hash []byte,
	header types.PartSetHeader,
	vss ...*validatorStub,
) []*types.Vote {
	votes := make([]*types.Vote, len(vss))
	for i, vs := range vss {
		votes[i] = signVote(ctx, t, vs, cfg, voteType, hash, header)
	}
	return votes
}

func incrementHeight(vss ...*validatorStub) {
	for _, vs := range vss {
		vs.Height++
	}
}

func incrementRound(vss ...*validatorStub) {
	for _, vs := range vss {
		vs.Round++
	}
}

type ValidatorStubsByPower []*validatorStub

func (vss ValidatorStubsByPower) Len() int {
	return len(vss)
}

func sortVValidatorStubsByPower(ctx context.Context, t *testing.T, vss []*validatorStub) []*validatorStub {
	t.Helper()
	sort.Slice(vss, func(i, j int) bool {
		vssi, err := vss[i].GetPubKey(ctx)
		require.NoError(t, err)

		vssj, err := vss[j].GetPubKey(ctx)
		require.NoError(t, err)

		if vss[i].VotingPower == vss[j].VotingPower {
			return bytes.Compare(vssi.Address(), vssj.Address()) == -1
		}
		return vss[i].VotingPower > vss[j].VotingPower
	})

	for idx, vs := range vss {
		vs.Index = int32(idx)
	}

	return vss
}

//-------------------------------------------------------------------------------
// Functions for transitioning the consensus state

func startTestRound(ctx context.Context, cs *State, height int64, round int32) {
	cs.enterNewRound(ctx, height, round)
	cs.startRoutines(ctx, 0)
}

// Create proposal block from cs1 but sign it with vs.
func decideProposal(
	ctx context.Context,
	t *testing.T,
	cs1 *State,
	vs *validatorStub,
	height int64,
	round int32,
) (proposal *types.Proposal, block *types.Block) {
	t.Helper()

	cs1.mtx.Lock()
	block, blockParts, err := cs1.createProposalBlock()
	require.NoError(t, err)
	validRound := cs1.ValidRound
	chainID := cs1.state.ChainID
	cs1.mtx.Unlock()

	require.NotNil(t, block, "Failed to createProposalBlock. Did you forget to add commit for previous block?")

	// Make proposal
	polRound, propBlockID := validRound, types.BlockID{Hash: block.Hash(), PartSetHeader: blockParts.Header()}
	proposal = types.NewProposal(height, round, polRound, propBlockID)
	p := proposal.ToProto()
	require.NoError(t, vs.SignProposal(ctx, chainID, p))

	proposal.Signature = p.Signature

	return
}

func addVotes(to *State, votes ...*types.Vote) {
	for _, vote := range votes {
		to.peerMsgQueue <- msgInfo{Msg: &VoteMessage{vote}}
	}
}

func signAddVotes(
	ctx context.Context,
	t *testing.T,
	cfg *config.Config,
	to *State,
	voteType tmproto.SignedMsgType,
	hash []byte,
	header types.PartSetHeader,
	vss ...*validatorStub,
) {
	addVotes(to, signVotes(ctx, t, cfg, voteType, hash, header, vss...)...)
}

func validatePrevote(
	ctx context.Context,
	t *testing.T,
	cs *State,
	round int32,
	privVal *validatorStub,
	blockHash []byte,
) {
	t.Helper()

	prevotes := cs.Votes.Prevotes(round)
	pubKey, err := privVal.GetPubKey(ctx)
	require.NoError(t, err)

	address := pubKey.Address()

	vote := prevotes.GetByAddress(address)
	require.NotNil(t, vote, "Failed to find prevote from validator")

	if blockHash == nil {
		require.Nil(t, vote.BlockID.Hash, "Expected prevote to be for nil, got %X", vote.BlockID.Hash)
	} else {
		require.True(t, bytes.Equal(vote.BlockID.Hash, blockHash), "Expected prevote to be for %X, got %X", blockHash, vote.BlockID.Hash)
	}
}

func validateLastPrecommit(ctx context.Context, t *testing.T, cs *State, privVal *validatorStub, blockHash []byte) {
	t.Helper()

	votes := cs.LastCommit
	pv, err := privVal.GetPubKey(ctx)
	require.NoError(t, err)
	address := pv.Address()

	vote := votes.GetByAddress(address)
	require.NotNil(t, vote)

	require.True(t, bytes.Equal(vote.BlockID.Hash, blockHash),
		"Expected precommit to be for %X, got %X", blockHash, vote.BlockID.Hash)
}

func validatePrecommit(
	ctx context.Context,
	t *testing.T,
	cs *State,
	thisRound,
	lockRound int32,
	privVal *validatorStub,
	votedBlockHash,
	lockedBlockHash []byte,
) {
	t.Helper()

	precommits := cs.Votes.Precommits(thisRound)
	pv, err := privVal.GetPubKey(ctx)
	require.NoError(t, err)
	address := pv.Address()

	vote := precommits.GetByAddress(address)
	require.NotNil(t, vote, "Failed to find precommit from validator")

	if votedBlockHash == nil {
		require.Nil(t, vote.BlockID.Hash, "Expected precommit to be for nil")
	} else {
		require.True(t, bytes.Equal(vote.BlockID.Hash, votedBlockHash), "Expected precommit to be for proposal block")
	}

	if lockedBlockHash == nil {
		require.False(t, cs.LockedRound != lockRound || cs.LockedBlock != nil,
			"Expected to be locked on nil at round %d. Got locked at round %d with block %v",
			lockRound,
			cs.LockedRound,
			cs.LockedBlock)
	} else {
		require.False(t, cs.LockedRound != lockRound || !bytes.Equal(cs.LockedBlock.Hash(), lockedBlockHash),
			"Expected block to be locked on round %d, got %d. Got locked block %X, expected %X",
			lockRound,
			cs.LockedRound,
			cs.LockedBlock.Hash(),
			lockedBlockHash)
	}
}

func validatePrevoteAndPrecommit(
	ctx context.Context,
	t *testing.T,
	cs *State,
	thisRound,
	lockRound int32,
	privVal *validatorStub,
	votedBlockHash,
	lockedBlockHash []byte,
) {
	// verify the prevote
	validatePrevote(ctx, t, cs, thisRound, privVal, votedBlockHash)
	// verify precommit
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	validatePrecommit(ctx, t, cs, thisRound, lockRound, privVal, votedBlockHash, lockedBlockHash)
}

func subscribeToVoter(ctx context.Context, t *testing.T, cs *State, addr []byte) <-chan tmpubsub.Message {
	t.Helper()

	ch := make(chan tmpubsub.Message, 1)
	if err := cs.eventBus.Observe(ctx, func(msg tmpubsub.Message) error {
		vote := msg.Data().(types.EventDataVote)
		// we only fire for our own votes
		if bytes.Equal(addr, vote.Vote.ValidatorAddress) {
			ch <- msg
		}
		return nil
	}, types.EventQueryVote); err != nil {
		t.Fatalf("Failed to observe query %v: %v", types.EventQueryVote, err)
	}
	return ch
}

//-------------------------------------------------------------------------------
// consensus states

func newState(
	ctx context.Context,
	t *testing.T,
	logger log.Logger,
	state sm.State,
	pv types.PrivValidator,
	app abci.Application,
) *State {
	t.Helper()

	cfg, err := config.ResetTestRoot("consensus_state_test")
	require.NoError(t, err)

	return newStateWithConfig(ctx, t, logger, cfg, state, pv, app)
}

func newStateWithConfig(
	ctx context.Context,
	t *testing.T,
	logger log.Logger,
	thisConfig *config.Config,
	state sm.State,
	pv types.PrivValidator,
	app abci.Application,
) *State {
	t.Helper()
	return newStateWithConfigAndBlockStore(ctx, t, logger, thisConfig, state, pv, app, store.NewBlockStore(dbm.NewMemDB()))
}

func newStateWithConfigAndBlockStore(
	ctx context.Context,
	t *testing.T,
	logger log.Logger,
	thisConfig *config.Config,
	state sm.State,
	pv types.PrivValidator,
	app abci.Application,
	blockStore *store.BlockStore,
) *State {
	t.Helper()

	// one for mempool, one for consensus
	mtx := new(sync.Mutex)
	proxyAppConnMem := abciclient.NewLocalClient(logger, mtx, app)
	proxyAppConnCon := abciclient.NewLocalClient(logger, mtx, app)

	// Make Mempool

	mempool := mempool.NewTxMempool(
		logger.With("module", "mempool"),
		thisConfig.Mempool,
		proxyAppConnMem,
		0,
	)

	if thisConfig.Consensus.WaitForTxs() {
		mempool.EnableTxsAvailable()
	}

	evpool := sm.EmptyEvidencePool{}

	// Make State
	stateDB := dbm.NewMemDB()
	stateStore := sm.NewStore(stateDB)
	require.NoError(t, stateStore.Save(state))

	blockExec := sm.NewBlockExecutor(stateStore, logger, proxyAppConnCon, mempool, evpool, blockStore)
	cs := NewState(ctx,
		logger.With("module", "consensus"),
		thisConfig.Consensus,
		state,
		blockExec,
		blockStore,
		mempool,
		evpool,
	)
	cs.SetPrivValidator(ctx, pv)

	eventBus := eventbus.NewDefault(logger.With("module", "events"))
	require.NoError(t, eventBus.Start(ctx))

	cs.SetEventBus(eventBus)
	return cs
}

func loadPrivValidator(t *testing.T, cfg *config.Config) *privval.FilePV {
	t.Helper()
	privValidatorKeyFile := cfg.PrivValidator.KeyFile()
	ensureDir(t, filepath.Dir(privValidatorKeyFile), 0700)
	privValidatorStateFile := cfg.PrivValidator.StateFile()
	privValidator, err := privval.LoadOrGenFilePV(privValidatorKeyFile, privValidatorStateFile)
	require.NoError(t, err)
	require.NoError(t, privValidator.Reset())
	return privValidator
}

func randState(
	ctx context.Context,
	t *testing.T,
	cfg *config.Config,
	logger log.Logger,
	nValidators int,
) (*State, []*validatorStub) {
	t.Helper()

	// Get State
	state, privVals := randGenesisState(ctx, t, cfg, nValidators, false, 10)

	vss := make([]*validatorStub, nValidators)

	cs := newState(ctx, t, logger, state, privVals[0], kvstore.NewApplication())

	for i := 0; i < nValidators; i++ {
		vss[i] = newValidatorStub(privVals[i], int32(i))
	}
	// since cs1 starts at 1
	incrementHeight(vss[1:]...)

	return cs, vss
}

//-------------------------------------------------------------------------------

func ensureNoNewEvent(t *testing.T, ch <-chan tmpubsub.Message, timeout time.Duration, errorMessage string) {
	t.Helper()
	select {
	case <-time.After(timeout):
		break
	case <-ch:
		t.Fatal(errorMessage)
	}
}

func ensureNoNewEventOnChannel(t *testing.T, ch <-chan tmpubsub.Message) {
	t.Helper()
	ensureNoNewEvent(t,
		ch,
		ensureTimeout,
		"We should be stuck waiting, not receiving new event on the channel")
}

func ensureNoNewRoundStep(t *testing.T, stepCh <-chan tmpubsub.Message) {
	t.Helper()
	ensureNoNewEvent(
		t,
		stepCh,
		ensureTimeout,
		"We should be stuck waiting, not receiving NewRoundStep event")
}

func ensureNoNewUnlock(t *testing.T, unlockCh <-chan tmpubsub.Message) {
	t.Helper()
	ensureNoNewEvent(t,
		unlockCh,
		ensureTimeout,
		"We should be stuck waiting, not receiving Unlock event")
}

func ensureNoNewTimeout(t *testing.T, stepCh <-chan tmpubsub.Message, timeout int64) {
	t.Helper()
	timeoutDuration := time.Duration(timeout*10) * time.Nanosecond
	ensureNoNewEvent(t,
		stepCh,
		timeoutDuration,
		"We should be stuck waiting, not receiving NewTimeout event")
}

func ensureNewEvent(t *testing.T, ch <-chan tmpubsub.Message, height int64, round int32, timeout time.Duration, errorMessage string) {
	t.Helper()
	select {
	case <-time.After(timeout):
		t.Fatal(errorMessage)
	case msg := <-ch:
		roundStateEvent, ok := msg.Data().(types.EventDataRoundState)
		require.True(t, ok,
			"expected a EventDataRoundState, got %T. Wrong subscription channel?",
			msg.Data())

		require.Equal(t, height, roundStateEvent.Height)
		require.Equal(t, round, roundStateEvent.Round)
		// TODO: We could check also for a step at this point!
	}
}

func ensureNewRound(t *testing.T, roundCh <-chan tmpubsub.Message, height int64, round int32) {
	t.Helper()
	select {
	case <-time.After(ensureTimeout):
		t.Fatal("Timeout expired while waiting for NewRound event")
	case msg := <-roundCh:
		newRoundEvent, ok := msg.Data().(types.EventDataNewRound)
		require.True(t, ok, "expected a EventDataNewRound, got %T. Wrong subscription channel?",
			msg.Data())

		require.Equal(t, height, newRoundEvent.Height)
		require.Equal(t, round, newRoundEvent.Round)
	}
}

func ensureNewTimeout(t *testing.T, timeoutCh <-chan tmpubsub.Message, height int64, round int32, timeout int64) {
	t.Helper()
	timeoutDuration := time.Duration(timeout*10) * time.Nanosecond
	ensureNewEvent(t, timeoutCh, height, round, timeoutDuration,
		"Timeout expired while waiting for NewTimeout event")
}

func ensureNewProposal(t *testing.T, proposalCh <-chan tmpubsub.Message, height int64, round int32) {
	t.Helper()
	select {
	case <-time.After(ensureTimeout):
		t.Fatal("Timeout expired while waiting for NewProposal event")
	case msg := <-proposalCh:
		proposalEvent, ok := msg.Data().(types.EventDataCompleteProposal)
		require.True(t, ok, "expected a EventDataCompleteProposal, got %T. Wrong subscription channel?",
			msg.Data())

		require.Equal(t, height, proposalEvent.Height)
		require.Equal(t, round, proposalEvent.Round)
	}
}

func ensureNewValidBlock(t *testing.T, validBlockCh <-chan tmpubsub.Message, height int64, round int32) {
	t.Helper()
	ensureNewEvent(t, validBlockCh, height, round, ensureTimeout,
		"Timeout expired while waiting for NewValidBlock event")
}

func ensureNewBlock(t *testing.T, blockCh <-chan tmpubsub.Message, height int64) {
	t.Helper()

	select {
	case <-time.After(ensureTimeout):
		t.Fatal("Timeout expired while waiting for NewBlock event")
	case msg := <-blockCh:
		blockEvent, ok := msg.Data().(types.EventDataNewBlock)
		require.True(t, ok, "expected a EventDataNewBlock, got %T. Wrong subscription channel?",
			msg.Data())
		require.Equal(t, height, blockEvent.Block.Height)
	}
}

func ensureNewBlockHeader(t *testing.T, blockCh <-chan tmpubsub.Message, height int64, blockHash tmbytes.HexBytes) {
	t.Helper()
	select {
	case <-time.After(ensureTimeout):
		t.Fatal("Timeout expired while waiting for NewBlockHeader event")
	case msg := <-blockCh:
		blockHeaderEvent, ok := msg.Data().(types.EventDataNewBlockHeader)
		require.True(t, ok, "expected a EventDataNewBlockHeader, got %T. Wrong subscription channel?",
			msg.Data())

		require.Equal(t, height, blockHeaderEvent.Header.Height)
		require.True(t, bytes.Equal(blockHeaderEvent.Header.Hash(), blockHash))
	}
}

func ensureNewUnlock(t *testing.T, unlockCh <-chan tmpubsub.Message, height int64, round int32) {
	t.Helper()
	ensureNewEvent(t, unlockCh, height, round, ensureTimeout,
		"Timeout expired while waiting for NewUnlock event")
}

func ensureProposal(t *testing.T, proposalCh <-chan tmpubsub.Message, height int64, round int32, propID types.BlockID) {
	t.Helper()
	select {
	case <-time.After(ensureTimeout):
		t.Fatal("Timeout expired while waiting for NewProposal event")
	case msg := <-proposalCh:
		proposalEvent, ok := msg.Data().(types.EventDataCompleteProposal)
		require.True(t, ok, "expected a EventDataCompleteProposal, got %T. Wrong subscription channel?",
			msg.Data())
		require.Equal(t, height, proposalEvent.Height)
		require.Equal(t, round, proposalEvent.Round)
		require.True(t, proposalEvent.BlockID.Equals(propID),
			"Proposed block does not match expected block (%v != %v)", proposalEvent.BlockID, propID)
	}
}

func ensurePrecommit(t *testing.T, voteCh <-chan tmpubsub.Message, height int64, round int32) {
	t.Helper()
	ensureVote(t, voteCh, height, round, tmproto.PrecommitType)
}

func ensurePrevote(t *testing.T, voteCh <-chan tmpubsub.Message, height int64, round int32) {
	t.Helper()
	ensureVote(t, voteCh, height, round, tmproto.PrevoteType)
}

func ensureVote(t *testing.T, voteCh <-chan tmpubsub.Message, height int64, round int32, voteType tmproto.SignedMsgType) {
	t.Helper()
	select {
	case <-time.After(ensureTimeout):
		t.Fatal("Timeout expired while waiting for NewVote event")
	case msg := <-voteCh:
		voteEvent, ok := msg.Data().(types.EventDataVote)
		require.True(t, ok, "expected a EventDataVote, got %T. Wrong subscription channel?",
			msg.Data())

		vote := voteEvent.Vote
		require.Equal(t, height, vote.Height)
		require.Equal(t, round, vote.Round)

		require.Equal(t, voteType, vote.Type)
	}
}

func ensurePrecommitTimeout(t *testing.T, ch <-chan tmpubsub.Message) {
	t.Helper()
	select {
	case <-time.After(ensureTimeout):
		t.Fatal("Timeout expired while waiting for the Precommit to Timeout")
	case <-ch:
	}
}

func ensureNewEventOnChannel(t *testing.T, ch <-chan tmpubsub.Message) {
	t.Helper()
	select {
	case <-time.After(ensureTimeout):
		t.Fatal("Timeout expired while waiting for new activity on the channel")
	case <-ch:
	}
}

//-------------------------------------------------------------------------------
// consensus nets

// consensusLogger is a TestingLogger which uses a different
// color for each validator ("validator" key must exist).
func consensusLogger() log.Logger {
	return log.TestingLogger().With("module", "consensus")
}

func randConsensusState(
	ctx context.Context,
	t *testing.T,
	cfg *config.Config,
	nValidators int,
	testName string,
	tickerFunc func() TimeoutTicker,
	appFunc func(t *testing.T, logger log.Logger) abci.Application,
	configOpts ...func(*config.Config),
) ([]*State, cleanupFunc) {

	genDoc, privVals := factory.RandGenesisDoc(ctx, t, cfg, nValidators, false, 30)
	css := make([]*State, nValidators)
	logger := consensusLogger()

	closeFuncs := make([]func() error, 0, nValidators)

	configRootDirs := make([]string, 0, nValidators)

	for i := 0; i < nValidators; i++ {
		blockStore := store.NewBlockStore(dbm.NewMemDB()) // each state needs its own db
		state, err := sm.MakeGenesisState(genDoc)
		require.NoError(t, err)
		thisConfig, err := ResetConfig(fmt.Sprintf("%s_%d", testName, i))
		require.NoError(t, err)

		configRootDirs = append(configRootDirs, thisConfig.RootDir)

		for _, opt := range configOpts {
			opt(thisConfig)
		}

		ensureDir(t, filepath.Dir(thisConfig.Consensus.WalFile()), 0700) // dir for wal

		app := appFunc(t, logger)

		if appCloser, ok := app.(io.Closer); ok {
			closeFuncs = append(closeFuncs, appCloser.Close)
		}

		vals := types.TM2PB.ValidatorUpdates(state.Validators)
		app.InitChain(abci.RequestInitChain{Validators: vals})

		l := logger.With("validator", i, "module", "consensus")
		css[i] = newStateWithConfigAndBlockStore(ctx, t, l, thisConfig, state, privVals[i], app, blockStore)
		css[i].SetTimeoutTicker(tickerFunc())
	}

	return css, func() {
		for _, closer := range closeFuncs {
			_ = closer()
		}
		for _, dir := range configRootDirs {
			os.RemoveAll(dir)
		}
	}
}

// nPeers = nValidators + nNotValidator
func randConsensusNetWithPeers(
	ctx context.Context,
	t *testing.T,
	cfg *config.Config,
	nValidators int,
	nPeers int,
	testName string,
	tickerFunc func() TimeoutTicker,
	appFunc func(log.Logger, string) abci.Application,
) ([]*State, *types.GenesisDoc, *config.Config, cleanupFunc) {
	t.Helper()

	genDoc, privVals := factory.RandGenesisDoc(ctx, t, cfg, nValidators, false, testMinPower)
	css := make([]*State, nPeers)
	logger := consensusLogger()

	var peer0Config *config.Config
	configRootDirs := make([]string, 0, nPeers)
	for i := 0; i < nPeers; i++ {
		state, _ := sm.MakeGenesisState(genDoc)
		thisConfig, err := ResetConfig(fmt.Sprintf("%s_%d", testName, i))
		require.NoError(t, err)

		configRootDirs = append(configRootDirs, thisConfig.RootDir)
		ensureDir(t, filepath.Dir(thisConfig.Consensus.WalFile()), 0700) // dir for wal
		if i == 0 {
			peer0Config = thisConfig
		}
		var privVal types.PrivValidator
		if i < nValidators {
			privVal = privVals[i]
		} else {
			tempKeyFile, err := os.CreateTemp("", "priv_validator_key_")
			require.NoError(t, err)

			tempStateFile, err := os.CreateTemp("", "priv_validator_state_")
			require.NoError(t, err)

			privVal, err = privval.GenFilePV(tempKeyFile.Name(), tempStateFile.Name(), "")
			require.NoError(t, err)
		}

		app := appFunc(logger, filepath.Join(cfg.DBDir(), fmt.Sprintf("%s_%d", testName, i)))
		vals := types.TM2PB.ValidatorUpdates(state.Validators)
		if _, ok := app.(*kvstore.PersistentKVStoreApplication); ok {
			// simulate handshake, receive app version. If don't do this, replay test will fail
			state.Version.Consensus.App = kvstore.ProtocolVersion
		}
		app.InitChain(abci.RequestInitChain{Validators: vals})
		// sm.SaveState(stateDB,state)	//height 1's validatorsInfo already saved in LoadStateFromDBOrGenesisDoc above

		css[i] = newStateWithConfig(ctx, t, logger.With("validator", i, "module", "consensus"), thisConfig, state, privVal, app)
		css[i].SetTimeoutTicker(tickerFunc())
	}
	return css, genDoc, peer0Config, func() {
		for _, dir := range configRootDirs {
			os.RemoveAll(dir)
		}
	}
}

func randGenesisState(
	ctx context.Context,
	t *testing.T,
	cfg *config.Config,
	numValidators int,
	randPower bool,
	minPower int64,
) (sm.State, []types.PrivValidator) {

	genDoc, privValidators := factory.RandGenesisDoc(ctx, t, cfg, numValidators, randPower, minPower)
	s0, err := sm.MakeGenesisState(genDoc)
	require.NoError(t, err)
	return s0, privValidators
}

func newMockTickerFunc(onlyOnce bool) func() TimeoutTicker {
	return func() TimeoutTicker {
		return &mockTicker{
			c:        make(chan timeoutInfo, 10),
			onlyOnce: onlyOnce,
		}
	}
}

// mock ticker only fires on RoundStepNewHeight
// and only once if onlyOnce=true
type mockTicker struct {
	c chan timeoutInfo

	mtx      sync.Mutex
	onlyOnce bool
	fired    bool
}

func (m *mockTicker) Start(context.Context) error {
	return nil
}

func (m *mockTicker) Stop() error {
	return nil
}

func (m *mockTicker) IsRunning() bool { return false }

func (m *mockTicker) ScheduleTimeout(ti timeoutInfo) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	if m.onlyOnce && m.fired {
		return
	}
	if ti.Step == cstypes.RoundStepNewHeight {
		m.c <- ti
		m.fired = true
	}
}

func (m *mockTicker) Chan() <-chan timeoutInfo {
	return m.c
}

func (*mockTicker) SetLogger(log.Logger) {}

func newPersistentKVStore(t *testing.T, logger log.Logger) abci.Application {
	t.Helper()

	dir, err := os.MkdirTemp("", "persistent-kvstore")
	require.NoError(t, err)

	return kvstore.NewPersistentKVStoreApplication(logger, dir)
}

func newKVStore(_ *testing.T, _ log.Logger) abci.Application {
	return kvstore.NewApplication()
}

func newPersistentKVStoreWithPath(logger log.Logger, dbDir string) abci.Application {
	return kvstore.NewPersistentKVStoreApplication(logger, dbDir)
}

func signDataIsEqual(v1 *types.Vote, v2 *tmproto.Vote) bool {
	if v1 == nil || v2 == nil {
		return false
	}

	return v1.Type == v2.Type &&
		bytes.Equal(v1.BlockID.Hash, v2.BlockID.GetHash()) &&
		v1.Height == v2.GetHeight() &&
		v1.Round == v2.Round &&
		bytes.Equal(v1.ValidatorAddress.Bytes(), v2.GetValidatorAddress()) &&
		v1.ValidatorIndex == v2.GetValidatorIndex()
}
