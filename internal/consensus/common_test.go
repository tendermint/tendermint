package consensus

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
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
	tmsync "github.com/tendermint/tendermint/internal/libs/sync"
	"github.com/tendermint/tendermint/internal/mempool"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/store"
	"github.com/tendermint/tendermint/internal/test/factory"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
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

func ensureDir(dir string, mode os.FileMode) {
	if err := tmos.EnsureDir(dir, mode); err != nil {
		panic(err)
	}
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
	vs *validatorStub,
	cfg *config.Config,
	voteType tmproto.SignedMsgType,
	hash []byte,
	header types.PartSetHeader,
) *types.Vote {

	v, err := vs.signVote(ctx, cfg, voteType, hash, header)
	if err != nil {
		panic(fmt.Errorf("failed to sign vote: %v", err))
	}

	vs.lastVote = v

	return v
}

func signVotes(
	ctx context.Context,
	cfg *config.Config,
	voteType tmproto.SignedMsgType,
	hash []byte,
	header types.PartSetHeader,
	vss ...*validatorStub) []*types.Vote {
	votes := make([]*types.Vote, len(vss))
	for i, vs := range vss {
		votes[i] = signVote(ctx, vs, cfg, voteType, hash, header)
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

func (vss ValidatorStubsByPower) Less(i, j int) bool {
	vssi, err := vss[i].GetPubKey(context.TODO())
	if err != nil {
		panic(err)
	}
	vssj, err := vss[j].GetPubKey(context.TODO())
	if err != nil {
		panic(err)
	}

	if vss[i].VotingPower == vss[j].VotingPower {
		return bytes.Compare(vssi.Address(), vssj.Address()) == -1
	}
	return vss[i].VotingPower > vss[j].VotingPower
}

func (vss ValidatorStubsByPower) Swap(i, j int) {
	it := vss[i]
	vss[i] = vss[j]
	vss[i].Index = int32(i)
	vss[j] = it
	vss[j].Index = int32(j)
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
	cs1 *State,
	vs *validatorStub,
	height int64,
	round int32,
) (proposal *types.Proposal, block *types.Block) {
	cs1.mtx.Lock()
	block, blockParts := cs1.createProposalBlock()
	validRound := cs1.ValidRound
	chainID := cs1.state.ChainID
	cs1.mtx.Unlock()
	if block == nil {
		panic("Failed to createProposalBlock. Did you forget to add commit for previous block?")
	}

	// Make proposal
	polRound, propBlockID := validRound, types.BlockID{Hash: block.Hash(), PartSetHeader: blockParts.Header()}
	proposal = types.NewProposal(height, round, polRound, propBlockID)
	p := proposal.ToProto()
	if err := vs.SignProposal(ctx, chainID, p); err != nil {
		panic(err)
	}

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
	cfg *config.Config,
	to *State,
	voteType tmproto.SignedMsgType,
	hash []byte,
	header types.PartSetHeader,
	vss ...*validatorStub,
) {
	addVotes(to, signVotes(ctx, cfg, voteType, hash, header, vss...)...)
}

func validatePrevote(
	ctx context.Context,
	t *testing.T,
	cs *State,
	round int32,
	privVal *validatorStub,
	blockHash []byte,
) {
	prevotes := cs.Votes.Prevotes(round)
	pubKey, err := privVal.GetPubKey(ctx)
	require.NoError(t, err)
	address := pubKey.Address()
	var vote *types.Vote
	if vote = prevotes.GetByAddress(address); vote == nil {
		panic("Failed to find prevote from validator")
	}
	if blockHash == nil {
		if vote.BlockID.Hash != nil {
			panic(fmt.Sprintf("Expected prevote to be for nil, got %X", vote.BlockID.Hash))
		}
	} else {
		if !bytes.Equal(vote.BlockID.Hash, blockHash) {
			panic(fmt.Sprintf("Expected prevote to be for %X, got %X", blockHash, vote.BlockID.Hash))
		}
	}
}

func validateLastPrecommit(ctx context.Context, t *testing.T, cs *State, privVal *validatorStub, blockHash []byte) {
	votes := cs.LastCommit
	pv, err := privVal.GetPubKey(ctx)
	require.NoError(t, err)
	address := pv.Address()
	var vote *types.Vote
	if vote = votes.GetByAddress(address); vote == nil {
		panic("Failed to find precommit from validator")
	}
	if !bytes.Equal(vote.BlockID.Hash, blockHash) {
		panic(fmt.Sprintf("Expected precommit to be for %X, got %X", blockHash, vote.BlockID.Hash))
	}
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
	precommits := cs.Votes.Precommits(thisRound)
	pv, err := privVal.GetPubKey(ctx)
	require.NoError(t, err)
	address := pv.Address()
	var vote *types.Vote
	if vote = precommits.GetByAddress(address); vote == nil {
		panic("Failed to find precommit from validator")
	}

	if votedBlockHash == nil {
		if vote.BlockID.Hash != nil {
			panic("Expected precommit to be for nil")
		}
	} else {
		if !bytes.Equal(vote.BlockID.Hash, votedBlockHash) {
			panic("Expected precommit to be for proposal block")
		}
	}

	if lockedBlockHash == nil {
		if cs.LockedRound != lockRound || cs.LockedBlock != nil {
			panic(fmt.Sprintf(
				"Expected to be locked on nil at round %d. Got locked at round %d with block %v",
				lockRound,
				cs.LockedRound,
				cs.LockedBlock))
		}
	} else {
		if cs.LockedRound != lockRound || !bytes.Equal(cs.LockedBlock.Hash(), lockedBlockHash) {
			panic(fmt.Sprintf(
				"Expected block to be locked on round %d, got %d. Got locked block %X, expected %X",
				lockRound,
				cs.LockedRound,
				cs.LockedBlock.Hash(),
				lockedBlockHash))
		}
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
	logger log.Logger,
	state sm.State,
	pv types.PrivValidator,
	app abci.Application,
) (*State, error) {
	cfg, err := config.ResetTestRoot("consensus_state_test")
	if err != nil {
		return nil, err
	}

	return newStateWithConfig(ctx, logger, cfg, state, pv, app), nil
}

func newStateWithConfig(
	ctx context.Context,
	logger log.Logger,
	thisConfig *config.Config,
	state sm.State,
	pv types.PrivValidator,
	app abci.Application,
) *State {
	return newStateWithConfigAndBlockStore(ctx, logger, thisConfig, state, pv, app, store.NewBlockStore(dbm.NewMemDB()))
}

func newStateWithConfigAndBlockStore(
	ctx context.Context,
	logger log.Logger,
	thisConfig *config.Config,
	state sm.State,
	pv types.PrivValidator,
	app abci.Application,
	blockStore *store.BlockStore,
) *State {
	// one for mempool, one for consensus
	mtx := new(tmsync.Mutex)
	proxyAppConnMem := abciclient.NewLocalClient(mtx, app)
	proxyAppConnCon := abciclient.NewLocalClient(mtx, app)

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
	if err := stateStore.Save(state); err != nil { // for save height 1's validators info
		panic(err)
	}

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
	cs.SetPrivValidator(pv)

	eventBus := eventbus.NewDefault(logger.With("module", "events"))
	err := eventBus.Start(ctx)
	if err != nil {
		panic(err)
	}
	cs.SetEventBus(eventBus)
	return cs
}

func loadPrivValidator(cfg *config.Config) *privval.FilePV {
	privValidatorKeyFile := cfg.PrivValidator.KeyFile()
	ensureDir(filepath.Dir(privValidatorKeyFile), 0700)
	privValidatorStateFile := cfg.PrivValidator.StateFile()
	privValidator, err := privval.LoadOrGenFilePV(privValidatorKeyFile, privValidatorStateFile)
	if err != nil {
		panic(err)
	}
	privValidator.Reset()
	return privValidator
}

func randState(
	ctx context.Context,
	cfg *config.Config,
	logger log.Logger,
	nValidators int,
) (*State, []*validatorStub, error) {
	// Get State
	state, privVals := randGenesisState(cfg, nValidators, false, 10)

	vss := make([]*validatorStub, nValidators)

	cs, err := newState(ctx, logger, state, privVals[0], kvstore.NewApplication())
	if err != nil {
		return nil, nil, err
	}

	for i := 0; i < nValidators; i++ {
		vss[i] = newValidatorStub(privVals[i], int32(i))
	}
	// since cs1 starts at 1
	incrementHeight(vss[1:]...)

	return cs, vss, nil
}

//-------------------------------------------------------------------------------

func ensureNoNewEvent(ch <-chan tmpubsub.Message, timeout time.Duration,
	errorMessage string) {
	select {
	case <-time.After(timeout):
		break
	case <-ch:
		panic(errorMessage)
	}
}

func ensureNoNewEventOnChannel(ch <-chan tmpubsub.Message) {
	ensureNoNewEvent(
		ch,
		ensureTimeout,
		"We should be stuck waiting, not receiving new event on the channel")
}

func ensureNoNewRoundStep(stepCh <-chan tmpubsub.Message) {
	ensureNoNewEvent(
		stepCh,
		ensureTimeout,
		"We should be stuck waiting, not receiving NewRoundStep event")
}

func ensureNoNewUnlock(unlockCh <-chan tmpubsub.Message) {
	ensureNoNewEvent(
		unlockCh,
		ensureTimeout,
		"We should be stuck waiting, not receiving Unlock event")
}

func ensureNoNewTimeout(stepCh <-chan tmpubsub.Message, timeout int64) {
	timeoutDuration := time.Duration(timeout*10) * time.Nanosecond
	ensureNoNewEvent(
		stepCh,
		timeoutDuration,
		"We should be stuck waiting, not receiving NewTimeout event")
}

func ensureNewEvent(ch <-chan tmpubsub.Message, height int64, round int32, timeout time.Duration, errorMessage string) {
	select {
	case <-time.After(timeout):
		panic(errorMessage)
	case msg := <-ch:
		roundStateEvent, ok := msg.Data().(types.EventDataRoundState)
		if !ok {
			panic(fmt.Sprintf("expected a EventDataRoundState, got %T. Wrong subscription channel?",
				msg.Data()))
		}
		if roundStateEvent.Height != height {
			panic(fmt.Sprintf("expected height %v, got %v", height, roundStateEvent.Height))
		}
		if roundStateEvent.Round != round {
			panic(fmt.Sprintf("expected round %v, got %v", round, roundStateEvent.Round))
		}
		// TODO: We could check also for a step at this point!
	}
}

func ensureNewRound(roundCh <-chan tmpubsub.Message, height int64, round int32) {
	select {
	case <-time.After(ensureTimeout):
		panic("Timeout expired while waiting for NewRound event")
	case msg := <-roundCh:
		newRoundEvent, ok := msg.Data().(types.EventDataNewRound)
		if !ok {
			panic(fmt.Sprintf("expected a EventDataNewRound, got %T. Wrong subscription channel?",
				msg.Data()))
		}
		if newRoundEvent.Height != height {
			panic(fmt.Sprintf("expected height %v, got %v", height, newRoundEvent.Height))
		}
		if newRoundEvent.Round != round {
			panic(fmt.Sprintf("expected round %v, got %v", round, newRoundEvent.Round))
		}
	}
}

func ensureNewTimeout(timeoutCh <-chan tmpubsub.Message, height int64, round int32, timeout int64) {
	timeoutDuration := time.Duration(timeout*10) * time.Nanosecond
	ensureNewEvent(timeoutCh, height, round, timeoutDuration,
		"Timeout expired while waiting for NewTimeout event")
}

func ensureNewProposal(proposalCh <-chan tmpubsub.Message, height int64, round int32) {
	select {
	case <-time.After(ensureTimeout):
		panic("Timeout expired while waiting for NewProposal event")
	case msg := <-proposalCh:
		proposalEvent, ok := msg.Data().(types.EventDataCompleteProposal)
		if !ok {
			panic(fmt.Sprintf("expected a EventDataCompleteProposal, got %T. Wrong subscription channel?",
				msg.Data()))
		}
		if proposalEvent.Height != height {
			panic(fmt.Sprintf("expected height %v, got %v", height, proposalEvent.Height))
		}
		if proposalEvent.Round != round {
			panic(fmt.Sprintf("expected round %v, got %v", round, proposalEvent.Round))
		}
	}
}

func ensureNewValidBlock(validBlockCh <-chan tmpubsub.Message, height int64, round int32) {
	ensureNewEvent(validBlockCh, height, round, ensureTimeout,
		"Timeout expired while waiting for NewValidBlock event")
}

func ensureNewBlock(blockCh <-chan tmpubsub.Message, height int64) {
	select {
	case <-time.After(ensureTimeout):
		panic("Timeout expired while waiting for NewBlock event")
	case msg := <-blockCh:
		blockEvent, ok := msg.Data().(types.EventDataNewBlock)
		if !ok {
			panic(fmt.Sprintf("expected a EventDataNewBlock, got %T. Wrong subscription channel?",
				msg.Data()))
		}
		if blockEvent.Block.Height != height {
			panic(fmt.Sprintf("expected height %v, got %v", height, blockEvent.Block.Height))
		}
	}
}

func ensureNewBlockHeader(blockCh <-chan tmpubsub.Message, height int64, blockHash tmbytes.HexBytes) {
	select {
	case <-time.After(ensureTimeout):
		panic("Timeout expired while waiting for NewBlockHeader event")
	case msg := <-blockCh:
		blockHeaderEvent, ok := msg.Data().(types.EventDataNewBlockHeader)
		if !ok {
			panic(fmt.Sprintf("expected a EventDataNewBlockHeader, got %T. Wrong subscription channel?",
				msg.Data()))
		}
		if blockHeaderEvent.Header.Height != height {
			panic(fmt.Sprintf("expected height %v, got %v", height, blockHeaderEvent.Header.Height))
		}
		if !bytes.Equal(blockHeaderEvent.Header.Hash(), blockHash) {
			panic(fmt.Sprintf("expected header %X, got %X", blockHash, blockHeaderEvent.Header.Hash()))
		}
	}
}

func ensureNewUnlock(unlockCh <-chan tmpubsub.Message, height int64, round int32) {
	ensureNewEvent(unlockCh, height, round, ensureTimeout,
		"Timeout expired while waiting for NewUnlock event")
}

func ensureProposal(proposalCh <-chan tmpubsub.Message, height int64, round int32, propID types.BlockID) {
	select {
	case <-time.After(ensureTimeout):
		panic("Timeout expired while waiting for NewProposal event")
	case msg := <-proposalCh:
		proposalEvent, ok := msg.Data().(types.EventDataCompleteProposal)
		if !ok {
			panic(fmt.Sprintf("expected a EventDataCompleteProposal, got %T. Wrong subscription channel?",
				msg.Data()))
		}
		if proposalEvent.Height != height {
			panic(fmt.Sprintf("expected height %v, got %v", height, proposalEvent.Height))
		}
		if proposalEvent.Round != round {
			panic(fmt.Sprintf("expected round %v, got %v", round, proposalEvent.Round))
		}
		if !proposalEvent.BlockID.Equals(propID) {
			panic(fmt.Sprintf("Proposed block does not match expected block (%v != %v)", proposalEvent.BlockID, propID))
		}
	}
}

func ensurePrecommit(voteCh <-chan tmpubsub.Message, height int64, round int32) {
	ensureVote(voteCh, height, round, tmproto.PrecommitType)
}

func ensurePrevote(voteCh <-chan tmpubsub.Message, height int64, round int32) {
	ensureVote(voteCh, height, round, tmproto.PrevoteType)
}

func ensureVote(voteCh <-chan tmpubsub.Message, height int64, round int32,
	voteType tmproto.SignedMsgType) {
	select {
	case <-time.After(ensureTimeout):
		panic("Timeout expired while waiting for NewVote event")
	case msg := <-voteCh:
		voteEvent, ok := msg.Data().(types.EventDataVote)
		if !ok {
			panic(fmt.Sprintf("expected a EventDataVote, got %T. Wrong subscription channel?",
				msg.Data()))
		}
		vote := voteEvent.Vote
		if vote.Height != height {
			panic(fmt.Sprintf("expected height %v, got %v", height, vote.Height))
		}
		if vote.Round != round {
			panic(fmt.Sprintf("expected round %v, got %v", round, vote.Round))
		}
		if vote.Type != voteType {
			panic(fmt.Sprintf("expected type %v, got %v", voteType, vote.Type))
		}
	}
}

func ensurePrecommitTimeout(ch <-chan tmpubsub.Message) {
	select {
	case <-time.After(ensureTimeout):
		panic("Timeout expired while waiting for the Precommit to Timeout")
	case <-ch:
	}
}

func ensureNewEventOnChannel(ch <-chan tmpubsub.Message) {
	select {
	case <-time.After(ensureTimeout):
		panic("Timeout expired while waiting for new activity on the channel")
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
	appFunc func() abci.Application,
	configOpts ...func(*config.Config),
) ([]*State, cleanupFunc) {

	genDoc, privVals := factory.RandGenesisDoc(cfg, nValidators, false, 30)
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

		ensureDir(filepath.Dir(thisConfig.Consensus.WalFile()), 0700) // dir for wal

		app := appFunc()

		if appCloser, ok := app.(io.Closer); ok {
			closeFuncs = append(closeFuncs, appCloser.Close)
		}

		vals := types.TM2PB.ValidatorUpdates(state.Validators)
		app.InitChain(abci.RequestInitChain{Validators: vals})

		l := logger.With("validator", i, "module", "consensus")
		css[i] = newStateWithConfigAndBlockStore(ctx, l, thisConfig, state, privVals[i], app, blockStore)
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
	cfg *config.Config,
	nValidators,
	nPeers int,
	testName string,
	tickerFunc func() TimeoutTicker,
	appFunc func(string) abci.Application,
) ([]*State, *types.GenesisDoc, *config.Config, cleanupFunc) {
	genDoc, privVals := factory.RandGenesisDoc(cfg, nValidators, false, testMinPower)
	css := make([]*State, nPeers)
	logger := consensusLogger()

	var peer0Config *config.Config
	configRootDirs := make([]string, 0, nPeers)
	for i := 0; i < nPeers; i++ {
		state, _ := sm.MakeGenesisState(genDoc)
		thisConfig, err := ResetConfig(fmt.Sprintf("%s_%d", testName, i))
		if err != nil {
			panic(err)
		}

		configRootDirs = append(configRootDirs, thisConfig.RootDir)
		ensureDir(filepath.Dir(thisConfig.Consensus.WalFile()), 0700) // dir for wal
		if i == 0 {
			peer0Config = thisConfig
		}
		var privVal types.PrivValidator
		if i < nValidators {
			privVal = privVals[i]
		} else {
			tempKeyFile, err := os.CreateTemp("", "priv_validator_key_")
			if err != nil {
				panic(err)
			}
			tempStateFile, err := os.CreateTemp("", "priv_validator_state_")
			if err != nil {
				panic(err)
			}

			privVal, err = privval.GenFilePV(tempKeyFile.Name(), tempStateFile.Name(), "")
			if err != nil {
				panic(err)
			}
		}

		app := appFunc(path.Join(cfg.DBDir(), fmt.Sprintf("%s_%d", testName, i)))
		vals := types.TM2PB.ValidatorUpdates(state.Validators)
		if _, ok := app.(*kvstore.PersistentKVStoreApplication); ok {
			// simulate handshake, receive app version. If don't do this, replay test will fail
			state.Version.Consensus.App = kvstore.ProtocolVersion
		}
		app.InitChain(abci.RequestInitChain{Validators: vals})
		// sm.SaveState(stateDB,state)	//height 1's validatorsInfo already saved in LoadStateFromDBOrGenesisDoc above

		css[i] = newStateWithConfig(ctx, logger.With("validator", i, "module", "consensus"), thisConfig, state, privVal, app)
		css[i].SetTimeoutTicker(tickerFunc())
	}
	return css, genDoc, peer0Config, func() {
		for _, dir := range configRootDirs {
			os.RemoveAll(dir)
		}
	}
}

func randGenesisState(
	cfg *config.Config,
	numValidators int,
	randPower bool,
	minPower int64) (sm.State, []types.PrivValidator) {

	genDoc, privValidators := factory.RandGenesisDoc(cfg, numValidators, randPower, minPower)
	s0, _ := sm.MakeGenesisState(genDoc)
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

func newPersistentKVStore() abci.Application {
	dir, err := os.MkdirTemp("", "persistent-kvstore")
	if err != nil {
		panic(err)
	}
	return kvstore.NewPersistentKVStoreApplication(dir)
}

func newKVStore() abci.Application {
	return kvstore.NewApplication()
}

func newPersistentKVStoreWithPath(dbDir string) abci.Application {
	return kvstore.NewPersistentKVStoreApplication(dbDir)
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
