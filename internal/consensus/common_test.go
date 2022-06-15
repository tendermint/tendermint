//nolint: lll
package consensus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/dashevo/dashd-go/btcjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	abciclient "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/dash/llmq"
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
	ensureTimeout  = time.Millisecond * 800
)

// A cleanupFunc cleans up any config / test files created for a particular
// test.
type cleanupFunc func()

func configSetup(t *testing.T) *config.Config {
	t.Helper()

	cfg, err := ResetConfig(t.TempDir(), "consensus_reactor_test")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(cfg.RootDir) })
	walDir := filepath.Dir(cfg.Consensus.WalFile())
	ensureDir(t, walDir, 0700)

	return cfg
}

func ensureDir(t *testing.T, dir string, mode os.FileMode) {
	t.Helper()
	require.NoError(t, tmos.EnsureDir(dir, mode))
}

func ResetConfig(dir, name string) (*config.Config, error) {
	return config.ResetTestRoot(dir, name)
}

//-------------------------------------------------------------------------------
// validator stub (a kvstore consensus peer we control)

type validatorStub struct {
	Index  int32 // Validator index. NOTE: we don't assume validator set changes.
	Height int64
	Round  int32
	clock  tmtime.Source
	types.PrivValidator
	VotingPower int64
	lastVote    *types.Vote
}

const testMinPower int64 = types.DefaultDashVotingPower

func newValidatorStub(privValidator types.PrivValidator, valIndex int32, initialHeight int64) *validatorStub {
	return &validatorStub{
		Index:         valIndex,
		PrivValidator: privValidator,
		VotingPower:   testMinPower,
		clock:         tmtime.DefaultSource{},
		Height:        initialHeight,
	}
}

func (vs *validatorStub) signVote(
	ctx context.Context,
	voteType tmproto.SignedMsgType,
	chainID string,
	blockID types.BlockID,
	lastAppHash []byte,
	quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash,
	voteExtensions []types.VoteExtension) (*types.Vote, error) {

	proTxHash, err := vs.PrivValidator.GetProTxHash(ctx)
	if err != nil {
		return nil, fmt.Errorf("can't get proTxHash: %w", err)
	}

	vote := &types.Vote{
		Type:               voteType,
		Height:             vs.Height,
		Round:              vs.Round,
		BlockID:            blockID,
		ValidatorProTxHash: proTxHash,
		ValidatorIndex:     vs.Index,
		VoteExtensions:     exts,
	}

	stateID := types.StateID{
		Height:      vote.Height - 1,
		LastAppHash: lastAppHash,
	}
	v := vote.ToProto()

	if err := vs.PrivValidator.SignVote(ctx, chainID, quorumType, quorumHash, v, stateID, nil); err != nil {
		return nil, fmt.Errorf("sign vote failed: %w", err)
	}

	// ref: signVote in FilePV, the vote should use the previous vote info when the sign data is the same.
	if signDataIsEqual(vs.lastVote, v) {
		v.BlockSignature = vs.lastVote.BlockSignature
		v.StateSignature = vs.lastVote.StateSignature
		for i, ext := range vs.lastVote.VoteExtensions {
			v.VoteExtensions[i].Signature = ext.Signature
		}
	}

	vote.BlockSignature = v.BlockSignature
	vote.StateSignature = v.StateSignature
	for i, ext := range v.VoteExtensions {
		vote.VoteExtensions[i].Signature = ext.Signature
	}

	return vote, err
}

// Sign vote for type/hash/header
func signVote(
	ctx context.Context,
	t *testing.T,
	vs *validatorStub,
	voteType tmproto.SignedMsgType,
	chainID string,
	blockID types.BlockID,
	lastAppHash []byte,
	quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash) *types.Vote {

	var exts []types.VoteExtension
	if voteType == tmproto.PrecommitType {
		exts = []types.VoteExtension{{
			Type:      tmproto.VoteExtensionType_DEFAULT,
			Extension: []byte("extension"),
		}}
	}
	v, err := vs.signVote(ctx, voteType, chainID, blockID, lastAppHash, quorumType, quorumHash, exts)
	require.NoError(t, err, "failed to sign vote")

	vs.lastVote = v

	return v
}

func signVotes(
	ctx context.Context,
	t *testing.T,
	voteType tmproto.SignedMsgType,
	chainID string,
	blockID types.BlockID,
	lastAppHash []byte,
	quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash,
	vss ...*validatorStub,
) []*types.Vote {
	votes := make([]*types.Vote, len(vss))
	for i, vs := range vss {
		votes[i] = signVote(ctx, t, vs, voteType, chainID, blockID, lastAppHash, quorumType, quorumHash)
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

func sortVValidatorStubsByPower(ctx context.Context, t *testing.T, vss []*validatorStub) []*validatorStub {
	t.Helper()
	sort.Slice(vss, func(i, j int) bool {
		vssi, err := vss[i].GetProTxHash(ctx)
		require.NoError(t, err)

		vssj, err := vss[j].GetProTxHash(ctx)
		require.NoError(t, err)

		if vss[i].VotingPower == vss[j].VotingPower {
			return bytes.Compare(vssi.Bytes(), vssj.Bytes()) == -1
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
	block, err := cs1.createProposalBlock(ctx)
	require.NoError(t, err)
	blockParts, err := block.MakePartSet(types.BlockPartSizeBytes)
	require.NoError(t, err)
	validRound := cs1.ValidRound
	chainID := cs1.state.ChainID

	validatorsAtProposalHeight := cs1.state.ValidatorsAtHeight(height)
	quorumType := validatorsAtProposalHeight.QuorumType
	quorumHash := validatorsAtProposalHeight.QuorumHash
	cs1.mtx.Unlock()

	require.NotNil(t, block, "Failed to createProposalBlock. Did you forget to add commit for previous block?")

	// Make proposal
	polRound, propBlockID := validRound, types.BlockID{Hash: block.Hash(), PartSetHeader: blockParts.Header()}
	proposal = types.NewProposal(height, 1, round, polRound, propBlockID, block.Header.Time)
	p := proposal.ToProto()

	proTxHash, _ := vs.GetProTxHash(ctx)
	pubKey, _ := vs.GetPubKey(ctx, validatorsAtProposalHeight.QuorumHash)

	signID, err := vs.SignProposal(ctx, chainID, quorumType, quorumHash, p)
	require.NoError(t, err)

	cs1.logger.Debug("signed proposal common test", "height", proposal.Height, "round", proposal.Round,
		"proposerProTxHash", proTxHash.ShortString(), "public key", pubKey.HexString(), "quorum type",
		validatorsAtProposalHeight.QuorumType, "quorum hash", validatorsAtProposalHeight.QuorumHash, "signID", signID.String())

	proposal.Signature = p.Signature

	return proposal, block
}

func addVotes(to *State, votes ...*types.Vote) {
	for _, vote := range votes {
		to.peerMsgQueue <- msgInfo{Msg: &VoteMessage{vote}}
	}
}

func signAddVotes(
	ctx context.Context,
	t *testing.T,
	to *State,
	voteType tmproto.SignedMsgType,
	chainID string,
	blockID types.BlockID,
	vss ...*validatorStub,
) {
	addVotes(to, signVotes(ctx, t, voteType, chainID, blockID, to.state.AppHash, to.Validators.QuorumType, to.Validators.QuorumHash, vss...)...)
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

	cs.mtx.RLock()
	defer cs.mtx.RUnlock()

	prevotes := cs.Votes.Prevotes(round)
	proTxHash, err := privVal.GetProTxHash(ctx)
	require.NoError(t, err)
	var vote *types.Vote
	if vote = prevotes.GetByProTxHash(proTxHash); vote == nil {
		panic("Failed to find prevote from validator")
	}
	if blockHash == nil {
		require.Nil(t, vote.BlockID.Hash, "Expected prevote to be for nil, got %X", vote.BlockID.Hash)
	} else {
		require.True(t, bytes.Equal(vote.BlockID.Hash, blockHash), "Expected prevote to be for %X, got %X", blockHash, vote.BlockID.Hash)
	}
}

func validateLastCommit(ctx context.Context, t *testing.T, cs *State, privVal *validatorStub, blockHash []byte) {
	t.Helper()

	commit := cs.LastCommit
	err := commit.ValidateBasic()
	require.NoError(t, err, "Expected commit to be valid %v, %v", commit, err)
	require.True(t, bytes.Equal(commit.BlockID.Hash, blockHash), "Expected commit to be for %X, got %X", blockHash, commit.BlockID.Hash)
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
	proTxHash, err := privVal.GetProTxHash(ctx)
	require.NoError(t, err)
	vote := precommits.GetByProTxHash(proTxHash)
	require.NotNil(t, vote, "Failed to find precommit from validator")

	if votedBlockHash == nil {
		require.Nil(t, vote.BlockID.Hash, "Expected precommit to be for nil")
	} else {
		require.True(t, bytes.Equal(vote.BlockID.Hash, votedBlockHash), "Expected precommit to be for proposal block")
	}

	rs := cs.GetRoundState()
	if lockedBlockHash == nil {
		require.False(t, rs.LockedRound != lockRound || rs.LockedBlock != nil,
			"Expected to be locked on nil at round %d. Got locked at round %d with block %v",
			lockRound,
			rs.LockedRound,
			rs.LockedBlock)
	} else {
		require.False(t, rs.LockedRound != lockRound || !bytes.Equal(rs.LockedBlock.Hash(), lockedBlockHash),
			"Expected block to be locked on round %d, got %d. Got locked block %X, expected %X",
			lockRound,
			rs.LockedRound,
			rs.LockedBlock.Hash(),
			lockedBlockHash)
	}
}

func subscribeToVoter(ctx context.Context, t *testing.T, cs *State, proTxHash []byte) <-chan tmpubsub.Message {
	t.Helper()

	ch := make(chan tmpubsub.Message, 1)
	if err := cs.eventBus.Observe(ctx, func(msg tmpubsub.Message) error {
		vote := msg.Data().(types.EventDataVote)
		// we only fire for our own votes
		if bytes.Equal(proTxHash, vote.Vote.ValidatorProTxHash) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ch <- msg:
			}
		}
		return nil
	}, types.EventQueryVote); err != nil {
		t.Fatalf("Failed to observe query %v: %v", types.EventQueryVote, err)
	}
	return ch
}

func subscribeToVoterBuffered(ctx context.Context, t *testing.T, cs *State, proTxHash []byte) <-chan tmpubsub.Message {
	t.Helper()
	votesSub, err := cs.eventBus.SubscribeWithArgs(ctx, tmpubsub.SubscribeArgs{
		ClientID: testSubscriber,
		Query:    types.EventQueryVote,
		Limit:    10})
	if err != nil {
		t.Fatalf("failed to subscribe %s to %v", testSubscriber, types.EventQueryVote)
	}
	ch := make(chan tmpubsub.Message, 10)
	go func() {
		for {
			msg, err := votesSub.Next(ctx)
			if err != nil {
				if !errors.Is(err, tmpubsub.ErrTerminated) && !errors.Is(err, context.Canceled) {
					t.Errorf("error terminating pubsub %s", err)
				}
				return
			}
			vote := msg.Data().(types.EventDataVote)
			// we only fire for our own votes
			if bytes.Equal(proTxHash, vote.Vote.ValidatorProTxHash) {
				select {
				case <-ctx.Done():
				case ch <- msg:
				}
			}
		}
	}()
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

	cfg, err := config.ResetTestRoot(t.TempDir(), "consensus_state_test")
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
	proxyAppConnMem := abciclient.NewLocalClient(logger, app)
	proxyAppConnCon := abciclient.NewLocalClient(logger, app)

	// Make Mempool

	mempool := mempool.NewTxMempool(
		logger.With("module", "mempool"),
		thisConfig.Mempool,
		proxyAppConnMem,
	)

	if thisConfig.Consensus.WaitForTxs() {
		mempool.EnableTxsAvailable()
	}

	evpool := sm.EmptyEvidencePool{}

	// Make State
	stateDB := dbm.NewMemDB()
	stateStore := sm.NewStore(stateDB)
	require.NoError(t, stateStore.Save(state))

	eventBus := eventbus.NewDefault(logger.With("module", "events"))
	require.NoError(t, eventBus.Start(ctx))

	blockExec := sm.NewBlockExecutor(stateStore, logger, proxyAppConnCon, mempool, evpool, blockStore, eventBus, sm.NopMetrics())
	cs, err := NewState(logger.With("module", "consensus"),
		thisConfig.Consensus,
		stateStore,
		blockExec,
		blockStore,
		mempool,
		evpool,
		eventBus,
	)
	if err != nil {
		t.Fatal(err)
	}

	cs.SetPrivValidator(ctx, pv)

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

type makeStateArgs struct {
	config      *config.Config
	logger      log.Logger
	validators  int
	application abci.Application
}

func makeState(ctx context.Context, t *testing.T, args makeStateArgs) (*State, []*validatorStub) {
	t.Helper()
	// Get State
	validators := 4
	if args.validators != 0 {
		validators = args.validators
	}
	var app abci.Application
	app = kvstore.NewApplication()
	if args.application != nil {
		app = args.application
	}
	if args.config == nil {
		args.config = configSetup(t)
	}
	if args.logger == nil {
		args.logger = log.NewNopLogger()
	}

	consensusParams := factory.ConsensusParams()
	// vote timeout increased because of bls12381 signing/verifying operations are longer performed than ed25519
	// and 10ms (previous value) is not enough
	consensusParams.Timeout.Vote = 50 * time.Millisecond
	consensusParams.Timeout.VoteDelta = 5 * time.Millisecond

	state, privVals := makeGenesisState(ctx, t, args.config, genesisStateArgs{
		Params:     consensusParams,
		Validators: validators,
	})

	vss := make([]*validatorStub, validators)

	cs := newState(ctx, t, args.logger, state, privVals[0], app)

	for i := 0; i < validators; i++ {
		vss[i] = newValidatorStub(privVals[i], int32(i), cs.state.InitialHeight)
	}

	return cs, vss
}

//-------------------------------------------------------------------------------

func ensureNoMessageBeforeTimeout(t *testing.T, ch <-chan tmpubsub.Message, timeout time.Duration,
	errorMessage string) {
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
	ensureNoMessageBeforeTimeout(
		t,
		ch,
		ensureTimeout,
		"We should be stuck waiting, not receiving new event on the channel")
}

func ensureNoNewRoundStep(t *testing.T, stepCh <-chan tmpubsub.Message) {
	t.Helper()
	ensureNoMessageBeforeTimeout(
		t,
		stepCh,
		ensureTimeout,
		"We should be stuck waiting, not receiving NewRoundStep event")
}

func ensureNoNewTimeout(t *testing.T, stepCh <-chan tmpubsub.Message, timeout int64) {
	t.Helper()
	timeoutDuration := time.Duration(timeout*10) * time.Nanosecond
	ensureNoMessageBeforeTimeout(
		t,
		stepCh,
		timeoutDuration,
		"We should be stuck waiting, not receiving NewTimeout event")
}

func ensureNewEvent(t *testing.T, ch <-chan tmpubsub.Message, height int64, round int32, timeout time.Duration) {
	t.Helper()
	msg := ensureMessageBeforeTimeout(t, ch, ensureTimeout)
	roundStateEvent, ok := msg.Data().(types.EventDataRoundState)
	require.True(t, ok,
		"expected a EventDataRoundState, got %T. Wrong subscription channel?",
		msg.Data())

	require.Equal(t, height, roundStateEvent.Height)
	require.Equal(t, round, roundStateEvent.Round)
	// TODO: We could check also for a step at this point!
}

func ensureNewRound(t *testing.T, roundCh <-chan tmpubsub.Message, height int64, round int32) {
	t.Helper()
	msg := ensureMessageBeforeTimeout(t, roundCh, ensureTimeout)
	newRoundEvent, ok := msg.Data().(types.EventDataNewRound)
	require.True(t, ok, "expected a EventDataNewRound, got %T. Wrong subscription channel?",
		msg.Data())

	require.Equal(t, height, newRoundEvent.Height)
	require.Equal(t, round, newRoundEvent.Round)
}

func ensureNewTimeout(t *testing.T, timeoutCh <-chan tmpubsub.Message, height int64, round int32, timeout int64) {
	t.Helper()
	timeoutDuration := time.Duration(timeout*10) * time.Nanosecond
	ensureNewEvent(t, timeoutCh, height, round, timeoutDuration)
}

func ensureNewProposal(t *testing.T, proposalCh <-chan tmpubsub.Message, height int64, round int32) types.BlockID {
	t.Helper()
	msg := ensureMessageBeforeTimeout(t, proposalCh, ensureTimeout)
	proposalEvent, ok := msg.Data().(types.EventDataCompleteProposal)
	require.True(t, ok, "expected a EventDataCompleteProposal, got %T. Wrong subscription channel?",
		msg.Data())
	require.Equal(t, height, proposalEvent.Height)
	require.Equal(t, round, proposalEvent.Round)
	return proposalEvent.BlockID
}

func ensureNewValidBlock(t *testing.T, validBlockCh <-chan tmpubsub.Message, height int64, round int32) {
	t.Helper()
	ensureNewEvent(t, validBlockCh, height, round, ensureTimeout)
}

func ensureNewBlock(t *testing.T, blockCh <-chan tmpubsub.Message, height int64) {
	t.Helper()
	msg := ensureMessageBeforeTimeout(t, blockCh, ensureTimeout)
	blockEvent, ok := msg.Data().(types.EventDataNewBlock)
	require.True(t, ok, "expected a EventDataNewBlock, got %T. Wrong subscription channel?",
		msg.Data())
	require.Equal(t, height, blockEvent.Block.Height)
}

func ensureNewBlockHeader(t *testing.T, blockCh <-chan tmpubsub.Message, height int64, blockHash tmbytes.HexBytes) {
	t.Helper()
	msg := ensureMessageBeforeTimeout(t, blockCh, ensureTimeout)
	blockHeaderEvent, ok := msg.Data().(types.EventDataNewBlockHeader)
	require.True(t, ok, "expected a EventDataNewBlockHeader, got %T. Wrong subscription channel?",
		msg.Data())

	require.Equal(t, height, blockHeaderEvent.Header.Height)
	require.True(t, bytes.Equal(blockHeaderEvent.Header.Hash(), blockHash))
}

func ensureLock(t *testing.T, lockCh <-chan tmpubsub.Message, height int64, round int32) {
	t.Helper()
	ensureNewEvent(t, lockCh, height, round, ensureTimeout)
}

func ensureRelock(t *testing.T, relockCh <-chan tmpubsub.Message, height int64, round int32) {
	t.Helper()
	ensureNewEvent(t, relockCh, height, round, ensureTimeout)
}

func ensureProposal(t *testing.T, proposalCh <-chan tmpubsub.Message, height int64, round int32, propID types.BlockID) {
	ensureProposalWithTimeout(t, proposalCh, height, round, &propID, ensureTimeout)
}

func ensureProposalWithTimeout(t *testing.T, proposalCh <-chan tmpubsub.Message, height int64, round int32, propID *types.BlockID, timeout time.Duration) {
	t.Helper()
	msg := ensureMessageBeforeTimeout(t, proposalCh, timeout)
	proposalEvent, ok := msg.Data().(types.EventDataCompleteProposal)
	require.True(t, ok, "expected a EventDataCompleteProposal, got %T. Wrong subscription channel?",
		msg.Data())
	require.Equal(t, height, proposalEvent.Height)
	require.Equal(t, round, proposalEvent.Round)
	if propID != nil {
		require.True(t, proposalEvent.BlockID.Equals(*propID),
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

func ensurePrevoteMatch(t *testing.T, voteCh <-chan tmpubsub.Message, height int64, round int32, hash []byte) {
	t.Helper()
	ensureVoteMatch(t, voteCh, height, round, hash, tmproto.PrevoteType)
}

func ensurePrecommitMatch(t *testing.T, voteCh <-chan tmpubsub.Message, height int64, round int32, hash []byte) {
	t.Helper()
	ensureVoteMatch(t, voteCh, height, round, hash, tmproto.PrecommitType)
}

func ensureVoteMatch(t *testing.T, voteCh <-chan tmpubsub.Message, height int64, round int32, hash []byte, voteType tmproto.SignedMsgType) {
	t.Helper()
	select {
	case <-time.After(ensureTimeout):
		t.Fatal("Timeout expired while waiting for NewVote event")
	case msg := <-voteCh:
		voteEvent, ok := msg.Data().(types.EventDataVote)
		require.True(t, ok, "expected a EventDataVote, got %T. Wrong subscription channel?",
			msg.Data())

		vote := voteEvent.Vote
		assert.Equal(t, height, vote.Height, "expected height %d, but got %d", height, vote.Height)
		assert.Equal(t, round, vote.Round, "expected round %d, but got %d", round, vote.Round)
		assert.Equal(t, voteType, vote.Type, "expected type %s, but got %s", voteType, vote.Type)
		if hash == nil {
			require.Nil(t, vote.BlockID.Hash, "Expected prevote to be for nil, got %X", vote.BlockID.Hash)
		} else {
			require.True(t, bytes.Equal(vote.BlockID.Hash, hash), "Expected prevote to be for %X, got %X", hash, vote.BlockID.Hash)
		}
	}
}

func ensureVote(t *testing.T, voteCh <-chan tmpubsub.Message, height int64, round int32, voteType tmproto.SignedMsgType) {
	t.Helper()
	msg := ensureMessageBeforeTimeout(t, voteCh, ensureTimeout)
	voteEvent, ok := msg.Data().(types.EventDataVote)
	require.True(t, ok, "expected a EventDataVote, got %T. Wrong subscription channel?",
		msg.Data())

	vote := voteEvent.Vote
	require.Equal(t, height, vote.Height, "expected height %d, but got %d", height, vote.Height)
	require.Equal(t, round, vote.Round, "expected round %d, but got %d", round, vote.Round)
	require.Equal(t, voteType, vote.Type, "expected type %s, but got %s", voteType, vote.Type)
}

func ensureNewEventOnChannel(t *testing.T, ch <-chan tmpubsub.Message) {
	t.Helper()
	ensureMessageBeforeTimeout(t, ch, ensureTimeout)
}

func ensureMessageBeforeTimeout(t *testing.T, ch <-chan tmpubsub.Message, to time.Duration) tmpubsub.Message {
	t.Helper()
	select {
	case <-time.After(to):
		t.Fatalf("Timeout expired while waiting for message")
	case msg := <-ch:
		return msg
	}
	panic("unreachable")
}

//-------------------------------------------------------------------------------
// consensus nets

// consensusLogger is a TestingLogger which uses a different
// color for each validator ("validator" key must exist).
func consensusLogger() log.Logger {
	return log.NewNopLogger().With("module", "consensus")
}

func makeConsensusState(
	ctx context.Context,
	t *testing.T,
	cfg *config.Config,
	nValidators int,
	testName string,
	tickerFunc func() TimeoutTicker,
	configOpts ...func(*config.Config),
) ([]*State, cleanupFunc) {
	t.Helper()
	tempDir := t.TempDir()

	genDoc, privVals := factory.RandGenesisDoc(cfg, nValidators, 1, factory.ConsensusParams())
	css := make([]*State, nValidators)
	logger := consensusLogger()

	closeFuncs := make([]func() error, 0, nValidators)
	configRootDirs := make([]string, 0, nValidators)

	for i := 0; i < nValidators; i++ {
		blockStore := store.NewBlockStore(dbm.NewMemDB()) // each state needs its own db
		state, err := sm.MakeGenesisState(genDoc)
		require.NoError(t, err)
		thisConfig, err := ResetConfig(tempDir, fmt.Sprintf("%s_%d", testName, i))
		require.NoError(t, err)

		configRootDirs = append(configRootDirs, thisConfig.RootDir)

		for _, opt := range configOpts {
			opt(thisConfig)
		}

		walDir := filepath.Dir(thisConfig.Consensus.WalFile())
		ensureDir(t, walDir, 0700)

		app := kvstore.NewApplication()
		closeFuncs = append(closeFuncs, app.Close)

		vals := types.TM2PB.ValidatorUpdates(state.Validators)
		_, err = app.InitChain(ctx, &abci.RequestInitChain{ValidatorSet: &vals})
		require.NoError(t, err)

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

	consParams := factory.ConsensusParams()
	consParams.Timeout.Propose = 1 * time.Second

	genDoc, privVals := factory.RandGenesisDoc(cfg, nValidators, 1, consParams)
	css := make([]*State, nPeers)
	t.Helper()
	logger := consensusLogger()
	var peer0Config *config.Config
	closeFuncs := make([]func() error, 0, nValidators)
	configRootDirs := make([]string, 0, nPeers)
	for i := 0; i < nPeers; i++ {
		state, _ := sm.MakeGenesisState(genDoc)
		thisConfig, err := ResetConfig(t.TempDir(), fmt.Sprintf("%s_%d", testName, i))
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
			tempKeyFile, err := os.CreateTemp(t.TempDir(), "priv_validator_key_")
			require.NoError(t, err)

			tempStateFile, err := os.CreateTemp(t.TempDir(), "priv_validator_state_")
			require.NoError(t, err)

			privVal = privval.GenFilePV(tempKeyFile.Name(), tempStateFile.Name())
			require.NoError(t, err)

			// These validator might not have the public keys, for testing purposes let's assume they don't
			state.Validators.HasPublicKeys = false
			state.NextValidators.HasPublicKeys = false
		}

		app := appFunc(logger, filepath.Join(cfg.DBDir(), fmt.Sprintf("%s_%d", testName, i)))
		if appCloser, ok := app.(io.Closer); ok {
			closeFuncs = append(closeFuncs, appCloser.Close)
		}
		vals := types.TM2PB.ValidatorUpdates(state.Validators)
		switch app.(type) {
		// simulate handshake, receive app version. If don't do this, replay test will fail
		case *kvstore.PersistentKVStoreApplication:
			state.Version.Consensus.App = kvstore.ProtocolVersion
		case *kvstore.Application:
			state.Version.Consensus.App = kvstore.ProtocolVersion
		}
		_, err = app.InitChain(ctx, &abci.RequestInitChain{ValidatorSet: &vals})
		require.NoError(t, err)
		// sm.SaveState(stateDB,state)	//height 1's validatorsInfo already saved in LoadStateFromDBOrGenesisDoc above

		proTxHash, _ := privVal.GetProTxHash(ctx)
		css[i] = newStateWithConfig(ctx, t,
			logger.With("validator", i, "node_proTxHash", proTxHash.ShortString(), "module", "consensus"),
			thisConfig, state, privVal, app)
		css[i].SetTimeoutTicker(tickerFunc())
	}
	return css, genDoc, peer0Config, func() {
		for _, closer := range closeFuncs {
			_ = closer()
		}
		for _, dir := range configRootDirs {
			os.RemoveAll(dir)
		}
	}
}

type genesisStateArgs struct {
	Validators int
	Power      int64
	Params     *types.ConsensusParams
	Time       time.Time
}

func makeGenesisState(ctx context.Context, t *testing.T, cfg *config.Config, args genesisStateArgs) (sm.State, []types.PrivValidator) {
	t.Helper()
	if args.Power == 0 {
		args.Power = 1
	}
	if args.Validators == 0 {
		args.Power = 4
	}
	if args.Params == nil {
		args.Params = types.DefaultConsensusParams()
	}
	if args.Time.IsZero() {
		args.Time = time.Now()
	}
	genDoc, privVals := factory.RandGenesisDoc(cfg, args.Validators, 1, args.Params)
	genDoc.GenesisTime = args.Time
	s0, err := sm.MakeGenesisState(genDoc)
	require.NoError(t, err)
	return s0, privVals
}

func newMockTickerFunc(onlyOnce bool) func() TimeoutTicker {
	return func() TimeoutTicker {
		return &mockTicker{
			c:        make(chan timeoutInfo, 100),
			onlyOnce: onlyOnce,
		}
	}
}

func newTickerFunc() func() TimeoutTicker {
	return func() TimeoutTicker { return NewTimeoutTicker(log.NewNopLogger()) }
}

// mock ticker only fires on RoundStepNewHeight
// and only once if onlyOnce=true
type mockTicker struct {
	c chan timeoutInfo

	mtx      sync.Mutex
	onlyOnce bool
	fired    bool
}

func (m *mockTicker) Start(context.Context) error { return nil }
func (m *mockTicker) Stop()                       {}
func (m *mockTicker) IsRunning() bool             { return false }

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

func newEpehemeralKVStore(_ log.Logger, _ string) abci.Application {
	return kvstore.NewApplication()
}

func signDataIsEqual(v1 *types.Vote, v2 *tmproto.Vote) bool {
	if v1 == nil || v2 == nil {
		return false
	}
	if len(v1.VoteExtensions) != len(v2.VoteExtensions) {
		return false
	}
	for i, ext := range v1.VoteExtensions {
		if !bytes.Equal(ext.Extension, v2.VoteExtensions[i].Extension) {
			return false
		}
	}
	return v1.Type == v2.Type &&
		bytes.Equal(v1.BlockID.Hash, v2.BlockID.GetHash()) &&
		v1.Height == v2.GetHeight() &&
		v1.Round == v2.Round &&
		bytes.Equal(v1.ValidatorProTxHash.Bytes(), v2.GetValidatorProTxHash()) &&
		v1.ValidatorIndex == v2.GetValidatorIndex()
}

type stateQuorumManager struct {
	states   []*State
	stateMap map[string]int
}

func newStateQuorumManager(states []*State) (*stateQuorumManager, error) {
	manager := stateQuorumManager{
		states:   states,
		stateMap: make(map[string]int),
	}
	for i, state := range states {
		proTxHash, err := state.privValidator.GetProTxHash(context.Background())
		if err != nil {
			return nil, err
		}
		manager.stateMap[proTxHash.String()] = i
	}
	return &manager, nil
}

func (s *stateQuorumManager) addValidator(height int64, cnt int) (*quorumData, error) {
	currentValidators := s.validatorSet()
	currentValidatorCount := len(currentValidators.Validators)
	proTxHashes := currentValidators.GetProTxHashes()
	for i := 0; i < cnt; i++ {
		proTxHash, err := s.states[currentValidatorCount+i].privValidator.GetProTxHash(context.Background())
		if err != nil {
			panic(err)
		}
		proTxHashes = append(proTxHashes, proTxHash)
	}
	return s.generateKeysAndUpdateState(proTxHashes, height)
}

func (s *stateQuorumManager) remValidators(height int64, cnt int) (*quorumData, error) {
	currentValidators := s.validatorSet()
	currentValidatorCount := len(currentValidators.Validators)
	if cnt >= currentValidatorCount {
		return nil, fmt.Errorf("you can not remove all validators")
	}
	validatorProTxHashes := currentValidators.GetProTxHashes()
	removedValidatorProTxHashes := validatorProTxHashes[len(validatorProTxHashes)-cnt:]
	return s.remValidatorsByProTxHash(height, removedValidatorProTxHashes)
}

func (s *stateQuorumManager) remValidatorsByProTxHash(height int64, removal []crypto.ProTxHash) (*quorumData, error) {
	currentValidators := s.validatorSet()
	removalMap := make(map[string]struct{})
	for _, proTxHash := range removal {
		removalMap[proTxHash.String()] = struct{}{}
	}
	l := len(currentValidators.GetProTxHashes()) - len(removal)
	validatorProTxHashes := make([]crypto.ProTxHash, 0, l)
	for _, validatorProTxHash := range currentValidators.GetProTxHashes() {
		if _, ok := removalMap[validatorProTxHash.String()]; !ok {
			validatorProTxHashes = append(validatorProTxHashes, validatorProTxHash)
		}
	}
	return s.generateKeysAndUpdateState(validatorProTxHashes, height)
}

func (s *stateQuorumManager) generateKeysAndUpdateState(
	proTxHashes []crypto.ProTxHash,
	height int64,
) (*quorumData, error) {
	// now that we have the list of all the protxhashes we need to regenerate the keys and the threshold public key
	lq, err := llmq.Generate(proTxHashes)
	if err != nil {
		return nil, err
	}
	qd := quorumData{
		Data:       *lq,
		quorumHash: crypto.RandQuorumHash(),
	}
	vsu, err := abci.LLMQToValidatorSetProto(*lq, abci.WithQuorumHash(qd.quorumHash))
	if err != nil {
		return nil, err
	}
	qd.tx, err = kvstore.MarshalValidatorSetUpdate(vsu)
	if err != nil {
		return nil, err
	}
	iter := lq.Iter()
	for iter.Next() {
		proTxHash, qks := iter.Value()
		_, err = s.updateState(
			proTxHash,
			qks.PrivKey,
			qd.quorumHash,
			lq.ThresholdPubKey,
			height+3,
		)
		if err != nil {
			return nil, err
		}
	}
	return &qd, nil
}

func (s *stateQuorumManager) updateState(
	proTxHash crypto.ProTxHash,
	privKey crypto.PrivKey,
	quorumHash crypto.QuorumHash,
	thresholdPubKey crypto.PubKey,
	height int64,
) (*types.Validator, error) {
	j, ok := s.stateMap[proTxHash.String()]
	if !ok {
		return nil, fmt.Errorf("a validator (%s) not found", proTxHash.ShortString())
	}
	privVal := s.states[j].privValidator
	privVal.UpdatePrivateKey(context.Background(), privKey, quorumHash, thresholdPubKey, height)
	return privVal.ExtractIntoValidator(context.Background(), quorumHash), nil
}

func (s *stateQuorumManager) validatorSet() *types.ValidatorSet {
	_, vals := s.states[0].GetValidatorSet()
	return vals
}

type quorumData struct {
	llmq.Data
	quorumHash crypto.QuorumHash
	tx         []byte
}
