//nolint: lll
package consensus

import (
	"bytes"
	"context"
	"fmt"
	abcicli "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/internal/p2p"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/dashevo/dashd-go/btcjson"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"

	"github.com/go-kit/log/term"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	cstypes "github.com/tendermint/tendermint/internal/consensus/types"
	tmsync "github.com/tendermint/tendermint/internal/libs/sync"
	mempoolv0 "github.com/tendermint/tendermint/internal/mempool/v0"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/internal/store"
	"github.com/tendermint/tendermint/internal/test/factory"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
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

const testMinPower int64 = types.DefaultDashVotingPower

func newValidatorStub(privValidator types.PrivValidator, valIndex int32, initialHeight int64) *validatorStub {
	return &validatorStub{
		Index:         valIndex,
		PrivValidator: privValidator,
		VotingPower:   testMinPower,
		Height:        initialHeight,
	}
}

func (vs *validatorStub) signVote(
	cfg *config.Config,
	voteType tmproto.SignedMsgType,
	hash []byte,
	lastAppHash []byte,
	quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash,
	header types.PartSetHeader) (*types.Vote, error) {

	proTxHash, err := vs.PrivValidator.GetProTxHash(context.Background())
	if err != nil {
		return nil, fmt.Errorf("can't get proTxHash: %w", err)
	}

	vote := &types.Vote{
		ValidatorIndex:     vs.Index,
		ValidatorProTxHash: proTxHash,
		Height:             vs.Height,
		Round:              vs.Round,
		Type:               voteType,
		BlockID:            types.BlockID{Hash: hash, PartSetHeader: header},
	}

	stateID := types.StateID{
		Height:      vote.Height - 1,
		LastAppHash: lastAppHash,
	}
	v := vote.ToProto()

	if err := vs.PrivValidator.SignVote(context.Background(), cfg.ChainID(), quorumType, quorumHash, v, stateID, nil); err != nil {
		return nil, fmt.Errorf("sign vote failed: %w", err)
	}

	// ref: signVote in FilePV, the vote should use the privious vote info when the sign data is the same.
	if signDataIsEqual(vs.lastVote, v) {
		v.BlockSignature = vs.lastVote.BlockSignature
		v.StateSignature = vs.lastVote.StateSignature
	}

	vote.BlockSignature = v.BlockSignature
	vote.StateSignature = v.StateSignature

	return vote, err
}

// SignDigest vote for type/hash/header
func signVote(vs *validatorStub, cfg *config.Config, voteType tmproto.SignedMsgType, hash []byte, lastAppHash []byte, quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash, header types.PartSetHeader) *types.Vote {
	v, err := vs.signVote(cfg, voteType, hash, lastAppHash, quorumType, quorumHash, header)
	if err != nil {
		panic(fmt.Errorf("failed to sign vote: %v", err))
	}

	vs.lastVote = v

	return v
}

func signVotes(
	cfg *config.Config,
	voteType tmproto.SignedMsgType,
	hash []byte,
	lastAppHash []byte,
	quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash,
	header types.PartSetHeader,
	vss ...*validatorStub) []*types.Vote {
	votes := make([]*types.Vote, len(vss))
	for i, vs := range vss {
		votes[i] = signVote(vs, cfg, voteType, hash, lastAppHash, quorumType, quorumHash, header)
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
	vssi, err := vss[i].GetProTxHash(context.Background())
	if err != nil {
		panic(err)
	}
	vssj, err := vss[j].GetProTxHash(context.Background())
	if err != nil {
		panic(err)
	}

	if vss[i].VotingPower == vss[j].VotingPower {
		return bytes.Compare(vssi.Bytes(), vssj.Bytes()) == -1
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

func startTestRound(cs *State, height int64, round int32) {
	cs.enterNewRound(height, round)
	cs.startRoutines(0)
}

// Create proposal block from cs1 but sign it with vs.
func decideProposal(
	cs1 *State,
	vs *validatorStub,
	height int64,
	round int32,
) (proposal *types.Proposal, block *types.Block) {
	cs1.mtx.Lock()
	block, blockParts := cs1.createProposalBlock()
	validRound := cs1.ValidRound
	chainID := cs1.state.ChainID

	validatorsAtProposalHeight := cs1.state.ValidatorsAtHeight(height)
	quorumType := validatorsAtProposalHeight.QuorumType
	quorumHash := validatorsAtProposalHeight.QuorumHash
	cs1.mtx.Unlock()
	if block == nil {
		panic("Failed to createProposalBlock. Did you forget to add commit for previous block?")
	}

	// Make proposal
	polRound, propBlockID := validRound, types.BlockID{Hash: block.Hash(), PartSetHeader: blockParts.Header()}
	proposal = types.NewProposal(height, 1, round, polRound, propBlockID)
	p := proposal.ToProto()

	proTxHash, _ := vs.GetProTxHash(context.Background())
	pubKey, _ := vs.GetPubKey(context.Background(), validatorsAtProposalHeight.QuorumHash)

	signID, err := vs.SignProposal(context.Background(), chainID, quorumType, quorumHash, p)

	if err != nil {
		panic(err)
	}
	cs1.Logger.Debug("signed proposal common test", "height", proposal.Height, "round", proposal.Round,
		"proposerProTxHash", proTxHash.ShortString(), "public key", pubKey.Bytes(), "quorum type",
		validatorsAtProposalHeight.QuorumType, "quorum hash", validatorsAtProposalHeight.QuorumHash, "signID", signID)

	proposal.Signature = p.Signature

	return proposal, block
}

func addVotes(to *State, votes ...*types.Vote) {
	for _, vote := range votes {
		to.peerMsgQueue <- msgInfo{Msg: &VoteMessage{vote}}
	}
}

func signAddVotes(
	cfg *config.Config,
	to *State,
	voteType tmproto.SignedMsgType,
	hash []byte,
	header types.PartSetHeader,
	vss ...*validatorStub,
) {
	votes := signVotes(cfg, voteType, hash, to.state.AppHash, to.Validators.QuorumType, to.Validators.QuorumHash, header, vss...)
	addVotes(to, votes...)
}

func validatePrevote(t *testing.T, cs *State, round int32, privVal *validatorStub, blockHash []byte) {
	prevotes := cs.Votes.Prevotes(round)
	proTxHash, err := privVal.GetProTxHash(context.Background())
	require.NoError(t, err)
	var vote *types.Vote
	if vote = prevotes.GetByProTxHash(proTxHash); vote == nil {
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

func validateLastCommit(t *testing.T, cs *State, privVal *validatorStub, blockHash []byte) {
	commit := cs.LastCommit
	err := commit.ValidateBasic()
	if err != nil {
		panic(fmt.Sprintf("Expected commit to be valid %v, %v", commit, err))
	}
	if !bytes.Equal(commit.BlockID.Hash, blockHash) {
		panic(fmt.Sprintf("Expected commit to be for %X, got %X", blockHash, commit.BlockID.Hash))
	}
}

func validatePrecommit(
	t *testing.T,
	cs *State,
	thisRound,
	lockRound int32,
	privVal *validatorStub,
	votedBlockHash,
	lockedBlockHash []byte,
) {
	precommits := cs.Votes.Precommits(thisRound)
	proTxHash, err := privVal.GetProTxHash(context.Background())
	require.NoError(t, err)
	var vote *types.Vote
	if vote = precommits.GetByProTxHash(proTxHash); vote == nil {
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
	t *testing.T,
	cs *State,
	thisRound,
	lockRound int32,
	privVal *validatorStub,
	votedBlockHash,
	lockedBlockHash []byte,
) {
	// verify the prevote
	validatePrevote(t, cs, thisRound, privVal, votedBlockHash)
	// verify precommit
	cs.mtx.Lock()
	validatePrecommit(t, cs, thisRound, lockRound, privVal, votedBlockHash, lockedBlockHash)
	cs.mtx.Unlock()
}

func subscribeToVoter(cs *State, proTxHash []byte) <-chan tmpubsub.Message {
	votesSub, err := cs.eventBus.SubscribeUnbuffered(context.Background(), testSubscriber, types.EventQueryVote)
	if err != nil {
		panic(fmt.Sprintf("failed to subscribe %s to %v", testSubscriber, types.EventQueryVote))
	}
	ch := make(chan tmpubsub.Message)
	go func() {
		for msg := range votesSub.Out() {
			vote := msg.Data().(types.EventDataVote)
			// we only fire for our own votes
			if bytes.Equal(proTxHash, vote.Vote.ValidatorProTxHash) {
				ch <- msg
			}
		}
	}()
	return ch
}

//-------------------------------------------------------------------------------
// consensus states

func newState(state sm.State, pv types.PrivValidator, app abci.Application) (*State, error) {
	cfg, err := config.ResetTestRoot("consensus_state_test")
	if err != nil {
		return nil, err
	}
	return newStateWithConfig(cfg, state, pv, app), nil
}

func newStateWithConfig(
	thisConfig *config.Config,
	state sm.State,
	pv types.PrivValidator,
	app abci.Application,
) *State {
	blockStore := store.NewBlockStore(dbm.NewMemDB())
	return newStateWithConfigAndBlockStore(thisConfig, state, pv, app, blockStore)
}

func newStateWithConfigAndBlockStore(
	thisConfig *config.Config,
	state sm.State,
	pv types.PrivValidator,
	app abci.Application,
	blockStore *store.BlockStore,
) *State {

	// one for mempool, one for consensus, one for signature validation
	mtx := new(tmsync.RWMutex)
	proxyAppConnMem := abcicli.NewLocalClient(mtx, app)
	proxyAppConnCon := abcicli.NewLocalClient(mtx, app)
	proxyAppConnQry := abcicli.NewLocalClient(mtx, app)

	// Make Mempool
	mempool := mempoolv0.NewCListMempool(thisConfig.Mempool, proxyAppConnMem, 0)
	mempool.SetLogger(log.TestingLogger().With("module", "mempool"))
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

	blockExec := sm.NewBlockExecutor(stateStore, log.TestingLogger(), proxyAppConnCon, proxyAppConnQry, mempool, evpool, nil)

	logger := log.TestingLogger().With("module", "consensus")
	cs := NewStateWithLogger(thisConfig.Consensus, state, blockExec, blockStore, mempool, evpool, logger, 0)
	cs.SetLogger(logger)
	cs.SetPrivValidator(pv)

	eventBus := types.NewEventBus()
	eventBus.SetLogger(log.TestingLogger().With("module", "events"))
	err := eventBus.Start()
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

func randState(cfg *config.Config, nValidators int) (*State, []*validatorStub, error) {
	// Get State
	state, privVals := randGenesisState(cfg, nValidators, false, 10)

	vss := make([]*validatorStub, nValidators)

	cs, err := newState(state, privVals[0], kvstore.NewApplication())
	if err != nil {
		return nil, nil, err
	}

	for i := 0; i < nValidators; i++ {
		vss[i] = newValidatorStub(privVals[i], int32(i), cs.state.InitialHeight)
	}

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
	return log.TestingLoggerWithColorFn(func(keyvals ...interface{}) term.FgBgColor {
		for i := 0; i < len(keyvals)-1; i += 2 {
			if keyvals[i] == "validator" {
				return term.FgBgColor{Fg: term.Color(uint8(keyvals[i+1].(int) + 1))}
			}
		}
		return term.FgBgColor{}
	}, "debug").With("module", "consensus")
}

func randConsensusState(
	t *testing.T,
	cfg *config.Config,
	nValidators int,
	testName string,
	tickerFunc func() TimeoutTicker,
	appFunc func() abci.Application,
	configOpts ...func(*config.Config),
) ([]*State, cleanupFunc) {

	genDoc, privVals := factory.RandGenesisDoc(cfg, nValidators, 1)
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
		app.InitChain(abci.RequestInitChain{ValidatorSet: &vals})

		css[i] = newStateWithConfigAndBlockStore(thisConfig, state, privVals[i], app, blockStore)
		css[i].SetTimeoutTicker(tickerFunc())
		css[i].SetLogger(logger.With("validator", i, "module", "consensus"))
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

func updateConsensusNetAddNewValidators(css []*State, height int64, addValCount int, validate bool) ([]*types.Validator, []crypto.ProTxHash, crypto.PubKey, crypto.QuorumHash) {
	currentHeight, currentValidators := css[0].GetValidatorSet()
	currentValidatorCount := len(currentValidators.Validators)

	if validate {
		for _, cssi := range css {
			height, validators := cssi.GetValidatorSet()
			if height != currentHeight {
				panic("they should all have the same heights")
			}
			if len(validators.Validators) != currentValidatorCount {
				panic("they should all have the same initial validator count")
			}
			if !currentValidators.Equals(validators) {
				panic("all validators should be the same")
			}
		}
	}

	validatorProTxHashes := currentValidators.GetProTxHashes()
	newValidatorProTxHashes := make([]crypto.ProTxHash, addValCount)
	for i := 0; i < addValCount; i++ {
		proTxHash, err := css[currentValidatorCount+i].privValidator.GetProTxHash()
		if err != nil {
			panic(err)
		}
		newValidatorProTxHashes[i] = proTxHash
	}
	validatorProTxHashes = append(validatorProTxHashes, newValidatorProTxHashes...)
	sort.Sort(crypto.SortProTxHash(validatorProTxHashes))
	// now that we have the list of all the protxhashes we need to regenerate the keys and the threshold public key
	validatorProTxHashes, privKeys, thresholdPublicKey := bls12381.CreatePrivLLMQDataOnProTxHashesDefaultThreshold(validatorProTxHashes)
	// privKeys are returned in order
	quorumHash := crypto.RandQuorumHash()
	var privVal types.PrivValidator
	updatedValidators := make([]*types.Validator, len(validatorProTxHashes))
	publicKeys := make([]crypto.PubKey, len(validatorProTxHashes))
	validatorProTxHashesAsByteArray := make([][]byte, len(validatorProTxHashes))
	for i := 0; i < len(validatorProTxHashes); i++ {
		privVal = css[i].privValidator
		privValProTxHash, err := privVal.GetProTxHash()
		if err != nil {
			panic(err)
		}
		for j, proTxHash := range validatorProTxHashes {
			if bytes.Equal(privValProTxHash.Bytes(), proTxHash.Bytes()) {
				privVal.UpdatePrivateKey(privKeys[j], quorumHash, thresholdPublicKey, height+3)
				updatedValidators[j] = privVal.ExtractIntoValidator(quorumHash)
				publicKeys[j] = privKeys[j].PubKey()
				if !bytes.Equal(updatedValidators[j].PubKey.Bytes(), publicKeys[j].Bytes()) {
					panic("the validator public key should match the public key")
				}
				validatorProTxHashesAsByteArray[j] = validatorProTxHashes[j].Bytes()
				break
			}
		}
	}
	// just do this for sanity testing
	recoveredThresholdPublicKey, err := bls12381.RecoverThresholdPublicKeyFromPublicKeys(publicKeys, validatorProTxHashesAsByteArray)
	if err != nil {
		panic(err)
	}
	if !bytes.Equal(recoveredThresholdPublicKey.Bytes(), thresholdPublicKey.Bytes()) {
		panic("the recovered public key should match the threshold public key")
	}
	return updatedValidators, newValidatorProTxHashes, thresholdPublicKey, quorumHash
}

func updateConsensusNetRemoveValidators(css []*State, height int64, removeValCount int, validate bool) ([]*types.Validator, []*types.Validator, crypto.PubKey, crypto.QuorumHash) {

	currentHeight, currentValidators := css[0].GetValidatorSet()
	currentValidatorCount := len(currentValidators.Validators)

	if removeValCount >= currentValidatorCount {
		panic("you can not remove all validators")
	}
	if validate {
		for _, cssi := range css {
			height, validators := cssi.GetValidatorSet()
			if height != currentHeight {
				panic("they should all have the same heights")
			}
			if len(validators.Validators) != currentValidatorCount {
				panic("they should all have the same initial validator count")
			}
			if !currentValidators.Equals(validators) {
				panic("all validators should be the same")
			}
		}
	}

	validatorProTxHashes := currentValidators.GetProTxHashes()
	removedValidatorProTxHashes := validatorProTxHashes[len(validatorProTxHashes)-removeValCount:]
	return updateConsensusNetRemoveValidatorsWithProTxHashes(css, height, removedValidatorProTxHashes, validate)
}

func updateConsensusNetRemoveValidatorsWithProTxHashes(css []*State, height int64, removalProTxHashes []crypto.ProTxHash, validate bool) ([]*types.Validator, []*types.Validator, crypto.PubKey, crypto.QuorumHash) {
	currentValidatorCount := len(css[0].Validators.Validators)
	currentValidators := css[0].Validators

	if validate {
		for _, cssi := range css {
			if len(cssi.Validators.Validators) != currentValidatorCount {
				panic("they should all have the same initial validator count")
			}
			if !currentValidators.Equals(cssi.Validators) {
				panic("all validators should be the same")
			}
		}
	}

	validatorProTxHashes := currentValidators.GetProTxHashes()
	var newValidatorProTxHashes []crypto.ProTxHash
	for _, validatorProTxHash := range validatorProTxHashes {
		found := false
		for _, removalProTxHash := range removalProTxHashes {
			if bytes.Equal(validatorProTxHash, removalProTxHash) {
				found = true
			}
		}
		if !found {
			newValidatorProTxHashes = append(newValidatorProTxHashes, validatorProTxHash)
		}
	}
	validatorProTxHashes = newValidatorProTxHashes
	sort.Sort(crypto.SortProTxHash(validatorProTxHashes))
	// now that we have the list of all the protxhashes we need to regenerate the keys and the threshold public key
	validatorProTxHashes, privKeys, thresholdPublicKey := bls12381.CreatePrivLLMQDataOnProTxHashesDefaultThreshold(validatorProTxHashes)
	// privKeys are returned in order
	quorumHash := crypto.RandQuorumHash()
	var privVal types.PrivValidator
	updatedValidators := make([]*types.Validator, len(validatorProTxHashes))
	removedValidators := make([]*types.Validator, len(removalProTxHashes))
	publicKeys := make([]crypto.PubKey, len(validatorProTxHashes))
	validatorProTxHashesAsByteArray := make([][]byte, len(validatorProTxHashes))
	for i, proTxHash := range validatorProTxHashes {
		for _, state := range css {
			stateProTxHash, err := state.privValidator.GetProTxHash()
			if err != nil {
				panic(err)
			}
			if bytes.Equal(stateProTxHash.Bytes(), proTxHash.Bytes()) {
				// we found the prival
				privVal = state.privValidator
				privVal.UpdatePrivateKey(privKeys[i], quorumHash, thresholdPublicKey, height+3)
				updatedValidators[i] = privVal.ExtractIntoValidator(quorumHash)
				publicKeys[i] = privKeys[i].PubKey()
				if !bytes.Equal(updatedValidators[i].PubKey.Bytes(), publicKeys[i].Bytes()) {
					panic("the validator public key should match the public key")
				}
				validatorProTxHashesAsByteArray[i] = validatorProTxHashes[i].Bytes()
				break
			}
		}
	}
	for i, proTxHash := range removalProTxHashes {
		_, removedValidator := currentValidators.GetByProTxHash(proTxHash)
		removedValidators[i] = removedValidator
	}
	// just do this for sanity testing
	recoveredThresholdPublicKey, err := bls12381.RecoverThresholdPublicKeyFromPublicKeys(publicKeys, validatorProTxHashesAsByteArray)
	if err != nil {
		panic(err)
	}
	if !bytes.Equal(recoveredThresholdPublicKey.Bytes(), thresholdPublicKey.Bytes()) {
		panic("the recovered public key should match the threshold public key")
	}
	return updatedValidators, removedValidators, thresholdPublicKey, quorumHash
}

// nPeers = nValidators + nNotValidator
func randConsensusNetWithPeers(
	cfg *config.Config,
	nValidators,
	nPeers int,
	testName string,
	tickerFunc func() TimeoutTicker,
	appFunc func(string) abci.Application,
) ([]*State, *types.GenesisDoc, *config.Config, cleanupFunc) {
	genDoc, privVals := factory.RandGenesisDoc(cfg, nValidators, 1)
	css := make([]*State, nPeers)
	logger := consensusLogger()
	var peer0Config *config.Config
	closeFuncs := make([]func() error, 0, nValidators)
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
			tempKeyFile, err := ioutil.TempFile("", "priv_validator_key_")
			if err != nil {
				panic(err)
			}
			tempStateFile, err := ioutil.TempFile("", "priv_validator_state_")
			if err != nil {
				panic(err)
			}
			privVal = privval.GenFilePV(tempKeyFile.Name(), tempStateFile.Name())

			// These validator might not have the public keys, for testing purposes let's assume they don't
			state.Validators.HasPublicKeys = false
			state.NextValidators.HasPublicKeys = false
		}

		app := appFunc(path.Join(config.DBDir(), fmt.Sprintf("%s_%d", testName, i)))
		if appCloser, ok := app.(io.Closer); ok {
			closeFuncs = append(closeFuncs, appCloser.Close)
		}
		vals := types.TM2PB.ValidatorUpdates(state.Validators)
		if _, ok := app.(*kvstore.PersistentKVStoreApplication); ok {
			// simulate handshake, receive app version. If don't do this, replay test will fail
			state.Version.Consensus.App = kvstore.ProtocolVersion
		}
		app.InitChain(abci.RequestInitChain{ValidatorSet: &vals})
		// sm.SaveState(stateDB,state)	//height 1's validatorsInfo already saved in LoadStateFromDBOrGenesisDoc above

		css[i] = newStateWithConfig(thisConfig, state, privVal, app)
		css[i].SetTimeoutTicker(tickerFunc())
		proTxHash, _ := privVal.GetProTxHash()
		css[i].SetLogger(logger.With("validator", i, "proTxHash", proTxHash.ShortString(), "module", "consensus"))
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

func getSwitchIndex(switches []*p2p.Switch, peer p2p.Peer) int {
	for i, s := range switches {
		if peer.NodeInfo().ID() == s.NodeInfo().ID() {
			return i
		}
	}
	panic("didnt find peer in switches")
}

//-------------------------------------------------------------------------------
// genesis



func randGenesisState(cfg *config.Config, numValidators int, randPower bool, minPower int64) (sm.State, []types.PrivValidator) {
	genDoc, privValidators := factory.RandGenesisDoc(cfg, numValidators, 1)
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

func (m *mockTicker) Start() error {
	return nil
}

func (m *mockTicker) Stop() error {
	return nil
}

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
	dir, err := ioutil.TempDir("", "persistent-kvstore")
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
		bytes.Equal(v1.ValidatorProTxHash.Bytes(), v2.GetValidatorProTxHash()) &&
		v1.ValidatorIndex == v2.GetValidatorIndex()
}
