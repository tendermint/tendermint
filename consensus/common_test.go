//nolint: lll
package consensus

import (
	"bytes"
	"context"
	"fmt"
	"github.com/dashevo/dashd-go/btcjson"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"

	"github.com/go-kit/kit/log/term"
	"github.com/stretchr/testify/require"

	"path"

	dbm "github.com/tendermint/tm-db"

	abcicli "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/counter"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	cfg "github.com/tendermint/tendermint/config"
	cstypes "github.com/tendermint/tendermint/consensus/types"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
	tmsync "github.com/tendermint/tendermint/libs/sync"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

const (
	testSubscriber = "test-client"
)

// A cleanupFunc cleans up any config / test files created for a particular
// test.
type cleanupFunc func()

// genesis, chain_id, priv_val
var (
	config                *cfg.Config // NOTE: must be reset for each _test.go file
	consensusReplayConfig *cfg.Config
	ensureTimeout         = time.Millisecond * 800
)

func ensureDir(dir string, mode os.FileMode) {
	if err := tmos.EnsureDir(dir, mode); err != nil {
		panic(err)
	}
}

func ResetConfig(name string) *cfg.Config {
	return cfg.ResetTestRoot(name)
}

//-------------------------------------------------------------------------------
// validator stub (a kvstore consensus peer we control)

type validatorStub struct {
	Index  int32 // Validator index. NOTE: we don't assume validator set changes.
	Height int64
	Round  int32
	types.PrivValidator
	VotingPower int64
}

var testMinPower int64 = types.DefaultDashVotingPower

func newValidatorStub(privValidator types.PrivValidator, valIndex int32) *validatorStub {
	return &validatorStub{
		Index:         valIndex,
		PrivValidator: privValidator,
		VotingPower:   testMinPower,
	}
}

func (vs *validatorStub) signVote(
	voteType tmproto.SignedMsgType,
	hash []byte,
	lastAppHash []byte,
	quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash,
	header types.PartSetHeader) (*types.Vote, error) {

	proTxHash, err := vs.PrivValidator.GetProTxHash()
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
		StateID:            types.StateID{LastAppHash: lastAppHash},
	}
	v := vote.ToProto()
	err = vs.PrivValidator.SignVote(config.ChainID(), quorumType, quorumHash, v)
	vote.BlockSignature = v.BlockSignature
	vote.StateSignature = v.StateSignature

	return vote, err
}

// SignDigest vote for type/hash/header
func signVote(vs *validatorStub, voteType tmproto.SignedMsgType, hash []byte, lastAppHash []byte, quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash, header types.PartSetHeader) *types.Vote {
	v, err := vs.signVote(voteType, hash, lastAppHash, quorumType, quorumHash, header)
	if err != nil {
		panic(fmt.Errorf("failed to sign vote: %v", err))
	}
	return v
}

func signVotes(
	voteType tmproto.SignedMsgType,
	hash []byte,
	lastAppHash []byte,
	quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash,
	header types.PartSetHeader,
	vss ...*validatorStub) []*types.Vote {
	votes := make([]*types.Vote, len(vss))
	for i, vs := range vss {
		votes[i] = signVote(vs, voteType, hash, lastAppHash, quorumType, quorumHash, header)
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
	vssi, err := vss[i].GetProTxHash()
	if err != nil {
		panic(err)
	}
	vssj, err := vss[j].GetProTxHash()
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
	quorumType := cs1.Validators.QuorumType
	quorumHash := cs1.Validators.QuorumHash
	cs1.mtx.Unlock()
	if block == nil {
		panic("Failed to createProposalBlock. Did you forget to add commit for previous block?")
	}

	// Make proposal
	polRound, propBlockID := validRound, types.BlockID{Hash: block.Hash(), PartSetHeader: blockParts.Header()}
	proposal = types.NewProposal(height, 1, round, polRound, propBlockID)
	p := proposal.ToProto()
	if err := vs.SignProposal(chainID, quorumType, quorumHash, p); err != nil {
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
	to *State,
	voteType tmproto.SignedMsgType,
	hash []byte,
	header types.PartSetHeader,
	vss ...*validatorStub,
) {
	votes := signVotes(voteType, hash, to.state.AppHash, to.Validators.QuorumType, to.Validators.QuorumHash, header, vss...)
	addVotes(to, votes...)
}

func validatePrevote(t *testing.T, cs *State, round int32, privVal *validatorStub, blockHash []byte) {
	prevotes := cs.Votes.Prevotes(round)
	proTxHash, err := privVal.GetProTxHash()
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

func validateLastPrecommit(t *testing.T, cs *State, privVal *validatorStub, blockHash []byte) {
	votes := cs.LastPrecommits
	proTxHash, err := privVal.GetProTxHash()
	require.NoError(t, err)
	var vote *types.Vote
	if vote = votes.GetByProTxHash(proTxHash); vote == nil {
		panic("Failed to find precommit from validator")
	}
	if !bytes.Equal(vote.BlockID.Hash, blockHash) {
		panic(fmt.Sprintf("Expected precommit to be for %X, got %X", blockHash, vote.BlockID.Hash))
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
	proTxHash, err := privVal.GetProTxHash()
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

func newState(state sm.State, pv types.PrivValidator, app abci.Application) *State {
	config := cfg.ResetTestRoot("consensus_state_test")
	proTxHash, err := pv.GetProTxHash()
	if err != nil {
		panic(err)
	}
	return newStateWithConfig(&proTxHash, config, state, pv, app)
}

func newStateWithConfig(
	nodeProTxHash *crypto.ProTxHash,
	thisConfig *cfg.Config,
	state sm.State,
	pv types.PrivValidator,
	app abci.Application,
) *State {
	blockDB := dbm.NewMemDB()
	return newStateWithConfigAndBlockStore(nodeProTxHash, thisConfig, state, pv, app, blockDB)
}

func newStateWithConfigAndBlockStore(
	nodeProTxHash *crypto.ProTxHash,
	thisConfig *cfg.Config,
	state sm.State,
	pv types.PrivValidator,
	app abci.Application,
	blockDB dbm.DB,
) *State {
	// Get BlockStore
	blockStore := store.NewBlockStore(blockDB)

	// one for mempool, one for consensus, one for signature validation
	mtx := new(tmsync.Mutex)
	proxyAppConnMem := abcicli.NewLocalClient(mtx, app)
	proxyAppConnCon := abcicli.NewLocalClient(mtx, app)
	proxyAppConnQry := abcicli.NewLocalClient(mtx, app)

	// Make Mempool
	mempool := mempl.NewCListMempool(thisConfig.Mempool, proxyAppConnMem, 0)
	mempool.SetLogger(log.TestingLogger().With("module", "mempool"))
	if thisConfig.Consensus.WaitForTxs() {
		mempool.EnableTxsAvailable()
	}

	evpool := sm.EmptyEvidencePool{}

	// Make State
	stateDB := blockDB
	stateStore := sm.NewStore(stateDB)
	if err := stateStore.Save(state); err != nil { // for save height 1's validators info
		panic(err)
	}

	blockExec := sm.NewBlockExecutor(nodeProTxHash, stateStore, log.TestingLogger(), proxyAppConnCon, proxyAppConnQry, mempool, evpool, nil)

	cs := NewState(thisConfig.Consensus, state, blockExec, blockStore, mempool, evpool)
	cs.SetLogger(log.TestingLogger().With("module", "consensus"))
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

func loadPrivValidator(config *cfg.Config) *privval.FilePV {
	privValidatorKeyFile := config.PrivValidatorKeyFile()
	ensureDir(filepath.Dir(privValidatorKeyFile), 0700)
	privValidatorStateFile := config.PrivValidatorStateFile()
	privValidator := privval.LoadOrGenFilePV(privValidatorKeyFile, privValidatorStateFile)
	privValidator.Reset()
	return privValidator
}

func randState(nValidators int) (*State, []*validatorStub) {
	// Get State
	state, privVals := randGenesisState(nValidators, false, 10)

	vss := make([]*validatorStub, nValidators)

	cs := newState(state, privVals[0], counter.NewApplication(true))

	for i := 0; i < nValidators; i++ {
		vss[i] = newValidatorStub(privVals[i], int32(i))
	}
	// since cs1 starts at 1
	incrementHeight(vss[1:]...)

	return cs, vss
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
	}).With("module", "consensus")
}

func randConsensusNet(nValidators int, testName string, tickerFunc func() TimeoutTicker,
	appFunc func() abci.Application, configOpts ...func(*cfg.Config)) ([]*State, cleanupFunc) {
	genDoc, privVals := randGenesisDoc(nValidators, false, 30)
	css := make([]*State, nValidators)
	logger := consensusLogger()
	configRootDirs := make([]string, 0, nValidators)
	for i := 0; i < nValidators; i++ {
		stateDB := dbm.NewMemDB() // each state needs its own db
		stateStore := sm.NewStore(stateDB)
		state, _ := stateStore.LoadFromDBOrGenesisDoc(genDoc)
		thisConfig := ResetConfig(fmt.Sprintf("%s_%d", testName, i))
		configRootDirs = append(configRootDirs, thisConfig.RootDir)
		for _, opt := range configOpts {
			opt(thisConfig)
		}
		ensureDir(filepath.Dir(thisConfig.Consensus.WalFile()), 0700) // dir for wal
		app := appFunc()
		vals := types.TM2PB.ValidatorUpdates(state.Validators)
		app.InitChain(abci.RequestInitChain{ValidatorSet: vals})

		proTxHash, err := privVals[i].GetProTxHash()
		if err != nil {
			panic(err)
		}

		css[i] = newStateWithConfigAndBlockStore(&proTxHash, thisConfig, state, privVals[i], app, stateDB)
		css[i].SetTimeoutTicker(tickerFunc())
		css[i].SetLogger(logger.With("validator", i, "module", "consensus"))
	}
	return css, func() {
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
				err := privVal.UpdatePrivateKey(privKeys[j], height+3)
				if err != nil {
					panic(err)
				}
				updatedValidators[j] = privVal.ExtractIntoValidator(height+3, quorumHash)
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
				err := privVal.UpdatePrivateKey(privKeys[i], height+3)
				if err != nil {
					panic(err)
				}
				updatedValidators[i] = privVal.ExtractIntoValidator(height+3, quorumHash)
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
	nValidators,
	nPeers int,
	testName string,
	tickerFunc func() TimeoutTicker,
	appFunc func(string) abci.Application,
) ([]*State, *types.GenesisDoc, *cfg.Config, cleanupFunc) {
	genDoc, privVals := randGenesisDoc(nValidators, false, testMinPower)
	css := make([]*State, nPeers)
	logger := consensusLogger()
	var peer0Config *cfg.Config
	configRootDirs := make([]string, 0, nPeers)
	for i := 0; i < nPeers; i++ {
		stateDB := dbm.NewMemDB() // each state needs its own db
		stateStore := sm.NewStore(stateDB)
		state, _ := stateStore.LoadFromDBOrGenesisDoc(genDoc)
		thisConfig := ResetConfig(fmt.Sprintf("%s_%d", testName, i))
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
		}

		app := appFunc(path.Join(config.DBDir(), fmt.Sprintf("%s_%d", testName, i)))
		vals := types.TM2PB.ValidatorUpdates(state.Validators)
		if _, ok := app.(*kvstore.PersistentKVStoreApplication); ok {
			// simulate handshake, receive app version. If don't do this, replay test will fail
			state.Version.Consensus.App = kvstore.ProtocolVersion
		}
		app.InitChain(abci.RequestInitChain{ValidatorSet: vals})
		// sm.SaveState(stateDB,state)	//height 1's validatorsInfo already saved in LoadStateFromDBOrGenesisDoc above

		proTxHash, err := privVals[i].GetProTxHash()
		if err != nil {
			panic(err)
		}

		css[i] = newStateWithConfig(&proTxHash, thisConfig, state, privVal, app)
		css[i].SetTimeoutTicker(tickerFunc())
		css[i].SetLogger(logger.With("validator", i, "module", "consensus"))
	}
	return css, genDoc, peer0Config, func() {
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

func randGenesisDoc(numValidators int, randPower bool, minPower int64) (*types.GenesisDoc, []types.PrivValidator) {
	validators := make([]types.GenesisValidator, numValidators)
	privValidators := make([]types.PrivValidator, numValidators)

	privateKeys, proTxHashes, thresholdPublicKey := bls12381.CreatePrivLLMQDataDefaultThreshold(numValidators)

	for i := 0; i < numValidators; i++ {
		val := types.NewValidatorDefaultVotingPower(privateKeys[i].PubKey(), proTxHashes[i])
		validators[i] = types.GenesisValidator{
			PubKey:    val.PubKey,
			Power:     val.VotingPower,
			ProTxHash: val.ProTxHash,
		}
		privValidators[i] = types.NewMockPVWithParams(privateKeys[i], proTxHashes[i], false, false)
	}
	sort.Sort(types.PrivValidatorsByProTxHash(privValidators))

	coreChainLock := types.NewMockChainLock(2)

	return &types.GenesisDoc{
		GenesisTime:                  tmtime.Now(),
		InitialHeight:                1,
		ChainID:                      config.ChainID(),
		Validators:                   validators,
		InitialCoreChainLockedHeight: 1,
		InitialProposalCoreChainLock: coreChainLock.ToProto(),
		ThresholdPublicKey:           thresholdPublicKey,
		QuorumHash:                   crypto.RandQuorumHash(),
	}, privValidators
}

func randGenesisState(numValidators int, randPower bool, minPower int64) (sm.State, []types.PrivValidator) {
	genDoc, privValidators := randGenesisDoc(numValidators, randPower, minPower)
	s0, _ := sm.MakeGenesisState(genDoc)
	return s0, privValidators
}

//------------------------------------
// mock ticker

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

//------------------------------------

func newCounter() abci.Application {
	return counter.NewApplication(true)
}

// func newPersistentKVStore() abci.Application {
//	 dir, err := ioutil.TempDir("", "persistent-kvstore")
//	 if err != nil {
//	 	 panic(err)
//	 }
//	 return kvstore.NewPersistentKVStoreApplication(dir)
// }

func newPersistentKVStoreWithPath(dbDir string) abci.Application {
	return kvstore.NewPersistentKVStoreApplication(dbDir)
}
