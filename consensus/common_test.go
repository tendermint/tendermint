package consensus

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/kit/log/term"

	abcicli "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/counter"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	bc "github.com/tendermint/tendermint/blockchain"
	cfg "github.com/tendermint/tendermint/config"
	cstypes "github.com/tendermint/tendermint/consensus/types"
	cmn "github.com/tendermint/tendermint/libs/common"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/libs/log"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
	sm "github.com/tendermint/tendermint/state"
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
var config *cfg.Config // NOTE: must be reset for each _test.go file
var consensusReplayConfig *cfg.Config
var ensureTimeout = time.Millisecond * 100

func ensureDir(dir string, mode os.FileMode) {
	if err := cmn.EnsureDir(dir, mode); err != nil {
		panic(err)
	}
}

func ResetConfig(name string) *cfg.Config {
	return cfg.ResetTestRoot(name)
}

//-------------------------------------------------------------------------------
// validator stub (a kvstore consensus peer we control)

type validatorStub struct {
	Index  int // Validator index. NOTE: we don't assume validator set changes.
	Height int64
	Round  int
	types.PrivValidator
}

var testMinPower int64 = 10

func NewValidatorStub(privValidator types.PrivValidator, valIndex int) *validatorStub {
	return &validatorStub{
		Index:         valIndex,
		PrivValidator: privValidator,
	}
}

func (vs *validatorStub) signVote(voteType types.SignedMsgType, hash []byte, header types.PartSetHeader) (*types.Vote, error) {
	addr := vs.PrivValidator.GetPubKey().Address()
	vote := &types.Vote{
		ValidatorIndex:   vs.Index,
		ValidatorAddress: addr,
		Height:           vs.Height,
		Round:            vs.Round,
		Timestamp:        tmtime.Now(),
		Type:             voteType,
		BlockID:          types.BlockID{hash, header},
	}
	err := vs.PrivValidator.SignVote(config.ChainID(), vote)
	return vote, err
}

// Sign vote for type/hash/header
func signVote(vs *validatorStub, voteType types.SignedMsgType, hash []byte, header types.PartSetHeader) *types.Vote {
	v, err := vs.signVote(voteType, hash, header)
	if err != nil {
		panic(fmt.Errorf("failed to sign vote: %v", err))
	}
	return v
}

func signVotes(voteType types.SignedMsgType, hash []byte, header types.PartSetHeader, vss ...*validatorStub) []*types.Vote {
	votes := make([]*types.Vote, len(vss))
	for i, vs := range vss {
		votes[i] = signVote(vs, voteType, hash, header)
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

//-------------------------------------------------------------------------------
// Functions for transitioning the consensus state

func startTestRound(cs *ConsensusState, height int64, round int) {
	cs.enterNewRound(height, round)
	cs.startRoutines(0)
}

// Create proposal block from cs1 but sign it with vs
func decideProposal(cs1 *ConsensusState, vs *validatorStub, height int64, round int) (proposal *types.Proposal, block *types.Block) {
	cs1.mtx.Lock()
	block, blockParts := cs1.createProposalBlock()
	cs1.mtx.Unlock()
	if block == nil { // on error
		panic("error creating proposal block")
	}

	// Make proposal
	cs1.mtx.RLock()
	validRound := cs1.ValidRound
	chainID := cs1.state.ChainID
	cs1.mtx.RUnlock()
	polRound, propBlockID := validRound, types.BlockID{block.Hash(), blockParts.Header()}
	proposal = types.NewProposal(height, round, polRound, propBlockID)
	if err := vs.SignProposal(chainID, proposal); err != nil {
		panic(err)
	}
	return
}

func addVotes(to *ConsensusState, votes ...*types.Vote) {
	for _, vote := range votes {
		to.peerMsgQueue <- msgInfo{Msg: &VoteMessage{vote}}
	}
}

func signAddVotes(to *ConsensusState, voteType types.SignedMsgType, hash []byte, header types.PartSetHeader, vss ...*validatorStub) {
	votes := signVotes(voteType, hash, header, vss...)
	addVotes(to, votes...)
}

func validatePrevote(t *testing.T, cs *ConsensusState, round int, privVal *validatorStub, blockHash []byte) {
	prevotes := cs.Votes.Prevotes(round)
	address := privVal.GetPubKey().Address()
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

func validateLastPrecommit(t *testing.T, cs *ConsensusState, privVal *validatorStub, blockHash []byte) {
	votes := cs.LastCommit
	address := privVal.GetPubKey().Address()
	var vote *types.Vote
	if vote = votes.GetByAddress(address); vote == nil {
		panic("Failed to find precommit from validator")
	}
	if !bytes.Equal(vote.BlockID.Hash, blockHash) {
		panic(fmt.Sprintf("Expected precommit to be for %X, got %X", blockHash, vote.BlockID.Hash))
	}
}

func validatePrecommit(t *testing.T, cs *ConsensusState, thisRound, lockRound int, privVal *validatorStub, votedBlockHash, lockedBlockHash []byte) {
	precommits := cs.Votes.Precommits(thisRound)
	address := privVal.GetPubKey().Address()
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
			panic(fmt.Sprintf("Expected to be locked on nil at round %d. Got locked at round %d with block %v", lockRound, cs.LockedRound, cs.LockedBlock))
		}
	} else {
		if cs.LockedRound != lockRound || !bytes.Equal(cs.LockedBlock.Hash(), lockedBlockHash) {
			panic(fmt.Sprintf("Expected block to be locked on round %d, got %d. Got locked block %X, expected %X", lockRound, cs.LockedRound, cs.LockedBlock.Hash(), lockedBlockHash))
		}
	}

}

func validatePrevoteAndPrecommit(t *testing.T, cs *ConsensusState, thisRound, lockRound int, privVal *validatorStub, votedBlockHash, lockedBlockHash []byte) {
	// verify the prevote
	validatePrevote(t, cs, thisRound, privVal, votedBlockHash)
	// verify precommit
	cs.mtx.Lock()
	validatePrecommit(t, cs, thisRound, lockRound, privVal, votedBlockHash, lockedBlockHash)
	cs.mtx.Unlock()
}

// genesis
func subscribeToVoter(cs *ConsensusState, addr []byte) chan interface{} {
	voteCh0 := make(chan interface{})
	err := cs.eventBus.Subscribe(context.Background(), testSubscriber, types.EventQueryVote, voteCh0)
	if err != nil {
		panic(fmt.Sprintf("failed to subscribe %s to %v", testSubscriber, types.EventQueryVote))
	}
	voteCh := make(chan interface{})
	go func() {
		for v := range voteCh0 {
			vote := v.(types.EventDataVote)
			// we only fire for our own votes
			if bytes.Equal(addr, vote.Vote.ValidatorAddress) {
				voteCh <- v
			}
		}
	}()
	return voteCh
}

//-------------------------------------------------------------------------------
// consensus states

func newConsensusState(state sm.State, pv types.PrivValidator, app abci.Application) *ConsensusState {
	config := cfg.ResetTestRoot("consensus_state_test")
	return newConsensusStateWithConfig(config, state, pv, app)
}

func newConsensusStateWithConfig(thisConfig *cfg.Config, state sm.State, pv types.PrivValidator, app abci.Application) *ConsensusState {
	blockDB := dbm.NewMemDB()
	return newConsensusStateWithConfigAndBlockStore(thisConfig, state, pv, app, blockDB)
}

func newConsensusStateWithConfigAndBlockStore(thisConfig *cfg.Config, state sm.State, pv types.PrivValidator, app abci.Application, blockDB dbm.DB) *ConsensusState {
	// Get BlockStore
	blockStore := bc.NewBlockStore(blockDB)

	// one for mempool, one for consensus
	mtx := new(sync.Mutex)
	proxyAppConnMem := abcicli.NewLocalClient(mtx, app)
	proxyAppConnCon := abcicli.NewLocalClient(mtx, app)

	// Make Mempool
	mempool := mempl.NewMempool(thisConfig.Mempool, proxyAppConnMem, 0)
	mempool.SetLogger(log.TestingLogger().With("module", "mempool"))
	if thisConfig.Consensus.WaitForTxs() {
		mempool.EnableTxsAvailable()
	}

	// mock the evidence pool
	evpool := sm.MockEvidencePool{}

	// Make ConsensusState
	stateDB := dbm.NewMemDB()
	blockExec := sm.NewBlockExecutor(stateDB, log.TestingLogger(), proxyAppConnCon, mempool, evpool)
	cs := NewConsensusState(thisConfig.Consensus, state, blockExec, blockStore, mempool, evpool)
	cs.SetLogger(log.TestingLogger().With("module", "consensus"))
	cs.SetPrivValidator(pv)

	eventBus := types.NewEventBus()
	eventBus.SetLogger(log.TestingLogger().With("module", "events"))
	eventBus.Start()
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

func randConsensusState(nValidators int) (*ConsensusState, []*validatorStub) {
	// Get State
	state, privVals := randGenesisState(nValidators, false, 10)

	vss := make([]*validatorStub, nValidators)

	cs := newConsensusState(state, privVals[0], counter.NewCounterApplication(true))

	for i := 0; i < nValidators; i++ {
		vss[i] = NewValidatorStub(privVals[i], i)
	}
	// since cs1 starts at 1
	incrementHeight(vss[1:]...)

	return cs, vss
}

//-------------------------------------------------------------------------------

func ensureNoNewEvent(ch <-chan interface{}, timeout time.Duration,
	errorMessage string) {
	select {
	case <-time.After(timeout):
		break
	case <-ch:
		panic(errorMessage)
	}
}

func ensureNoNewEventOnChannel(ch <-chan interface{}) {
	ensureNoNewEvent(
		ch,
		ensureTimeout,
		"We should be stuck waiting, not receiving new event on the channel")
}

func ensureNoNewRoundStep(stepCh <-chan interface{}) {
	ensureNoNewEvent(
		stepCh,
		ensureTimeout,
		"We should be stuck waiting, not receiving NewRoundStep event")
}

func ensureNoNewUnlock(unlockCh <-chan interface{}) {
	ensureNoNewEvent(
		unlockCh,
		ensureTimeout,
		"We should be stuck waiting, not receiving Unlock event")
}

func ensureNoNewTimeout(stepCh <-chan interface{}, timeout int64) {
	timeoutDuration := time.Duration(timeout*5) * time.Nanosecond
	ensureNoNewEvent(
		stepCh,
		timeoutDuration,
		"We should be stuck waiting, not receiving NewTimeout event")
}

func ensureNewEvent(
	ch <-chan interface{},
	height int64,
	round int,
	timeout time.Duration,
	errorMessage string) {

	select {
	case <-time.After(timeout):
		panic(errorMessage)
	case ev := <-ch:
		rs, ok := ev.(types.EventDataRoundState)
		if !ok {
			panic(
				fmt.Sprintf(
					"expected a EventDataRoundState, got %v.Wrong subscription channel?",
					reflect.TypeOf(rs)))
		}
		if rs.Height != height {
			panic(fmt.Sprintf("expected height %v, got %v", height, rs.Height))
		}
		if rs.Round != round {
			panic(fmt.Sprintf("expected round %v, got %v", round, rs.Round))
		}
		// TODO: We could check also for a step at this point!
	}
}

func ensureNewRound(roundCh <-chan interface{}, height int64, round int) {
	select {
	case <-time.After(ensureTimeout):
		panic("Timeout expired while waiting for NewRound event")
	case ev := <-roundCh:
		rs, ok := ev.(types.EventDataNewRound)
		if !ok {
			panic(
				fmt.Sprintf(
					"expected a EventDataNewRound, got %v.Wrong subscription channel?",
					reflect.TypeOf(rs)))
		}
		if rs.Height != height {
			panic(fmt.Sprintf("expected height %v, got %v", height, rs.Height))
		}
		if rs.Round != round {
			panic(fmt.Sprintf("expected round %v, got %v", round, rs.Round))
		}
	}
}

func ensureNewTimeout(timeoutCh <-chan interface{}, height int64, round int, timeout int64) {
	timeoutDuration := time.Duration(timeout*5) * time.Nanosecond
	ensureNewEvent(timeoutCh, height, round, timeoutDuration,
		"Timeout expired while waiting for NewTimeout event")
}

func ensureNewProposal(proposalCh <-chan interface{}, height int64, round int) {
	select {
	case <-time.After(ensureTimeout):
		panic("Timeout expired while waiting for NewProposal event")
	case ev := <-proposalCh:
		rs, ok := ev.(types.EventDataCompleteProposal)
		if !ok {
			panic(
				fmt.Sprintf(
					"expected a EventDataCompleteProposal, got %v.Wrong subscription channel?",
					reflect.TypeOf(rs)))
		}
		if rs.Height != height {
			panic(fmt.Sprintf("expected height %v, got %v", height, rs.Height))
		}
		if rs.Round != round {
			panic(fmt.Sprintf("expected round %v, got %v", round, rs.Round))
		}
	}
}

func ensureNewValidBlock(validBlockCh <-chan interface{}, height int64, round int) {
	ensureNewEvent(validBlockCh, height, round, ensureTimeout,
		"Timeout expired while waiting for NewValidBlock event")
}

func ensureNewBlock(blockCh <-chan interface{}, height int64) {
	select {
	case <-time.After(ensureTimeout):
		panic("Timeout expired while waiting for NewBlock event")
	case ev := <-blockCh:
		block, ok := ev.(types.EventDataNewBlock)
		if !ok {
			panic(fmt.Sprintf("expected a *types.EventDataNewBlock, "+
				"got %v. wrong subscription channel?",
				reflect.TypeOf(block)))
		}
		if block.Block.Height != height {
			panic(fmt.Sprintf("expected height %v, got %v", height, block.Block.Height))
		}
	}
}

func ensureNewBlockHeader(blockCh <-chan interface{}, height int64, blockHash cmn.HexBytes) {
	select {
	case <-time.After(ensureTimeout):
		panic("Timeout expired while waiting for NewBlockHeader event")
	case ev := <-blockCh:
		blockHeader, ok := ev.(types.EventDataNewBlockHeader)
		if !ok {
			panic(fmt.Sprintf("expected a *types.EventDataNewBlockHeader, "+
				"got %v. wrong subscription channel?",
				reflect.TypeOf(blockHeader)))
		}
		if blockHeader.Header.Height != height {
			panic(fmt.Sprintf("expected height %v, got %v", height, blockHeader.Header.Height))
		}
		if !bytes.Equal(blockHeader.Header.Hash(), blockHash) {
			panic(fmt.Sprintf("expected header %X, got %X", blockHash, blockHeader.Header.Hash()))
		}
	}
}

func ensureNewUnlock(unlockCh <-chan interface{}, height int64, round int) {
	ensureNewEvent(unlockCh, height, round, ensureTimeout,
		"Timeout expired while waiting for NewUnlock event")
}

func ensureVote(voteCh <-chan interface{}, height int64, round int,
	voteType types.SignedMsgType) {
	select {
	case <-time.After(ensureTimeout):
		panic("Timeout expired while waiting for NewVote event")
	case v := <-voteCh:
		edv, ok := v.(types.EventDataVote)
		if !ok {
			panic(fmt.Sprintf("expected a *types.Vote, "+
				"got %v. wrong subscription channel?",
				reflect.TypeOf(v)))
		}
		vote := edv.Vote
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

func ensureProposal(proposalCh <-chan interface{}, height int64, round int, propId types.BlockID) {
	select {
	case <-time.After(ensureTimeout):
		panic("Timeout expired while waiting for NewProposal event")
	case ev := <-proposalCh:
		rs, ok := ev.(types.EventDataCompleteProposal)
		if !ok {
			panic(
				fmt.Sprintf(
					"expected a EventDataCompleteProposal, got %v.Wrong subscription channel?",
					reflect.TypeOf(rs)))
		}
		if rs.Height != height {
			panic(fmt.Sprintf("expected height %v, got %v", height, rs.Height))
		}
		if rs.Round != round {
			panic(fmt.Sprintf("expected round %v, got %v", round, rs.Round))
		}
		if !rs.BlockID.Equals(propId) {
			panic("Proposed block does not match expected block")
		}
	}
}

func ensurePrecommit(voteCh <-chan interface{}, height int64, round int) {
	ensureVote(voteCh, height, round, types.PrecommitType)
}

func ensurePrevote(voteCh <-chan interface{}, height int64, round int) {
	ensureVote(voteCh, height, round, types.PrevoteType)
}

func ensureNewEventOnChannel(ch <-chan interface{}) {
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
	appFunc func() abci.Application, configOpts ...func(*cfg.Config)) ([]*ConsensusState, cleanupFunc) {
	genDoc, privVals := randGenesisDoc(nValidators, false, 30)
	css := make([]*ConsensusState, nValidators)
	logger := consensusLogger()
	configRootDirs := make([]string, 0, nValidators)
	for i := 0; i < nValidators; i++ {
		stateDB := dbm.NewMemDB() // each state needs its own db
		state, _ := sm.LoadStateFromDBOrGenesisDoc(stateDB, genDoc)
		thisConfig := ResetConfig(fmt.Sprintf("%s_%d", testName, i))
		configRootDirs = append(configRootDirs, thisConfig.RootDir)
		for _, opt := range configOpts {
			opt(thisConfig)
		}
		ensureDir(filepath.Dir(thisConfig.Consensus.WalFile()), 0700) // dir for wal
		app := appFunc()
		vals := types.TM2PB.ValidatorUpdates(state.Validators)
		app.InitChain(abci.RequestInitChain{Validators: vals})

		css[i] = newConsensusStateWithConfig(thisConfig, state, privVals[i], app)
		css[i].SetTimeoutTicker(tickerFunc())
		css[i].SetLogger(logger.With("validator", i, "module", "consensus"))
	}
	return css, func() {
		for _, dir := range configRootDirs {
			os.RemoveAll(dir)
		}
	}
}

// nPeers = nValidators + nNotValidator
func randConsensusNetWithPeers(nValidators, nPeers int, testName string, tickerFunc func() TimeoutTicker,
	appFunc func() abci.Application) ([]*ConsensusState, cleanupFunc) {

	genDoc, privVals := randGenesisDoc(nValidators, false, testMinPower)
	css := make([]*ConsensusState, nPeers)
	logger := consensusLogger()
	configRootDirs := make([]string, 0, nPeers)
	for i := 0; i < nPeers; i++ {
		stateDB := dbm.NewMemDB() // each state needs its own db
		state, _ := sm.LoadStateFromDBOrGenesisDoc(stateDB, genDoc)
		thisConfig := ResetConfig(fmt.Sprintf("%s_%d", testName, i))
		configRootDirs = append(configRootDirs, thisConfig.RootDir)
		ensureDir(filepath.Dir(thisConfig.Consensus.WalFile()), 0700) // dir for wal
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

		app := appFunc()
		vals := types.TM2PB.ValidatorUpdates(state.Validators)
		app.InitChain(abci.RequestInitChain{Validators: vals})

		css[i] = newConsensusStateWithConfig(thisConfig, state, privVal, app)
		css[i].SetTimeoutTicker(tickerFunc())
		css[i].SetLogger(logger.With("validator", i, "module", "consensus"))
	}
	return css, func() {
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
	for i := 0; i < numValidators; i++ {
		val, privVal := types.RandValidator(randPower, minPower)
		validators[i] = types.GenesisValidator{
			PubKey: val.PubKey,
			Power:  val.VotingPower,
		}
		privValidators[i] = privVal
	}
	sort.Sort(types.PrivValidatorsByAddress(privValidators))

	return &types.GenesisDoc{
		GenesisTime: tmtime.Now(),
		ChainID:     config.ChainID(),
		Validators:  validators,
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
	return counter.NewCounterApplication(true)
}

func newPersistentKVStore() abci.Application {
	dir, err := ioutil.TempDir("", "persistent-kvstore")
	if err != nil {
		panic(err)
	}
	return kvstore.NewPersistentKVStoreApplication(dir)
}
