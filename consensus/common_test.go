package consensus

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"sync"
	"testing"
	"time"

	abcicli "github.com/tendermint/abci/client"
	abci "github.com/tendermint/abci/types"
	bc "github.com/tendermint/tendermint/blockchain"
	cfg "github.com/tendermint/tendermint/config"
	cstypes "github.com/tendermint/tendermint/consensus/types"
	mempl "github.com/tendermint/tendermint/mempool"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tmlibs/common"
	dbm "github.com/tendermint/tmlibs/db"
	"github.com/tendermint/tmlibs/log"

	"github.com/tendermint/abci/example/counter"
	"github.com/tendermint/abci/example/kvstore"

	"github.com/go-kit/kit/log/term"
)

const (
	testSubscriber = "test-client"
)

// genesis, chain_id, priv_val
var config *cfg.Config              // NOTE: must be reset for each _test.go file
var ensureTimeout = time.Second * 1 // must be in seconds because CreateEmptyBlocksInterval is

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

func (vs *validatorStub) signVote(voteType byte, hash []byte, header types.PartSetHeader) (*types.Vote, error) {
	vote := &types.Vote{
		ValidatorIndex:   vs.Index,
		ValidatorAddress: vs.PrivValidator.GetAddress(),
		Height:           vs.Height,
		Round:            vs.Round,
		Timestamp:        time.Now().UTC(),
		Type:             voteType,
		BlockID:          types.BlockID{hash, header},
	}
	err := vs.PrivValidator.SignVote(config.ChainID(), vote)
	return vote, err
}

// Sign vote for type/hash/header
func signVote(vs *validatorStub, voteType byte, hash []byte, header types.PartSetHeader) *types.Vote {
	v, err := vs.signVote(voteType, hash, header)
	if err != nil {
		panic(fmt.Errorf("failed to sign vote: %v", err))
	}
	return v
}

func signVotes(voteType byte, hash []byte, header types.PartSetHeader, vss ...*validatorStub) []*types.Vote {
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
	block, blockParts := cs1.createProposalBlock()
	if block == nil { // on error
		panic("error creating proposal block")
	}

	// Make proposal
	polRound, polBlockID := cs1.Votes.POLInfo()
	proposal = types.NewProposal(height, round, blockParts.Header(), polRound, polBlockID)
	if err := vs.SignProposal(cs1.state.ChainID, proposal); err != nil {
		panic(err)
	}
	return
}

func addVotes(to *ConsensusState, votes ...*types.Vote) {
	for _, vote := range votes {
		to.peerMsgQueue <- msgInfo{Msg: &VoteMessage{vote}}
	}
}

func signAddVotes(to *ConsensusState, voteType byte, hash []byte, header types.PartSetHeader, vss ...*validatorStub) {
	votes := signVotes(voteType, hash, header, vss...)
	addVotes(to, votes...)
}

func validatePrevote(t *testing.T, cs *ConsensusState, round int, privVal *validatorStub, blockHash []byte) {
	prevotes := cs.Votes.Prevotes(round)
	var vote *types.Vote
	if vote = prevotes.GetByAddress(privVal.GetAddress()); vote == nil {
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
	var vote *types.Vote
	if vote = votes.GetByAddress(privVal.GetAddress()); vote == nil {
		panic("Failed to find precommit from validator")
	}
	if !bytes.Equal(vote.BlockID.Hash, blockHash) {
		panic(fmt.Sprintf("Expected precommit to be for %X, got %X", blockHash, vote.BlockID.Hash))
	}
}

func validatePrecommit(t *testing.T, cs *ConsensusState, thisRound, lockRound int, privVal *validatorStub, votedBlockHash, lockedBlockHash []byte) {
	precommits := cs.Votes.Precommits(thisRound)
	var vote *types.Vote
	if vote = precommits.GetByAddress(privVal.GetAddress()); vote == nil {
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
	cs := NewConsensusState(thisConfig.Consensus, state, blockExec, blockStore, mempool, evpool, NopMetrics())
	cs.SetLogger(log.TestingLogger().With("module", "consensus"))
	cs.SetPrivValidator(pv)

	eventBus := types.NewEventBus()
	eventBus.SetLogger(log.TestingLogger().With("module", "events"))
	eventBus.Start()
	cs.SetEventBus(eventBus)
	return cs
}

func loadPrivValidator(config *cfg.Config) *privval.FilePV {
	privValidatorFile := config.PrivValidatorFile()
	ensureDir(path.Dir(privValidatorFile), 0700)
	privValidator := privval.LoadOrGenFilePV(privValidatorFile)
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

func ensureNoNewStep(stepCh <-chan interface{}) {
	timer := time.NewTimer(ensureTimeout)
	select {
	case <-timer.C:
		break
	case <-stepCh:
		panic("We should be stuck waiting, not moving to the next step")
	}
}

func ensureNewStep(stepCh <-chan interface{}) {
	timer := time.NewTimer(ensureTimeout)
	select {
	case <-timer.C:
		panic("We shouldnt be stuck waiting")
	case <-stepCh:
		break
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

func randConsensusNet(nValidators int, testName string, tickerFunc func() TimeoutTicker, appFunc func() abci.Application, configOpts ...func(*cfg.Config)) []*ConsensusState {
	genDoc, privVals := randGenesisDoc(nValidators, false, 30)
	css := make([]*ConsensusState, nValidators)
	logger := consensusLogger()
	for i := 0; i < nValidators; i++ {
		stateDB := dbm.NewMemDB() // each state needs its own db
		state, _ := sm.LoadStateFromDBOrGenesisDoc(stateDB, genDoc)
		thisConfig := ResetConfig(cmn.Fmt("%s_%d", testName, i))
		for _, opt := range configOpts {
			opt(thisConfig)
		}
		ensureDir(path.Dir(thisConfig.Consensus.WalFile()), 0700) // dir for wal
		app := appFunc()
		vals := types.TM2PB.Validators(state.Validators)
		app.InitChain(abci.RequestInitChain{Validators: vals})

		css[i] = newConsensusStateWithConfig(thisConfig, state, privVals[i], app)
		css[i].SetTimeoutTicker(tickerFunc())
		css[i].SetLogger(logger.With("validator", i, "module", "consensus"))
	}
	return css
}

// nPeers = nValidators + nNotValidator
func randConsensusNetWithPeers(nValidators, nPeers int, testName string, tickerFunc func() TimeoutTicker, appFunc func() abci.Application) []*ConsensusState {
	genDoc, privVals := randGenesisDoc(nValidators, false, testMinPower)
	css := make([]*ConsensusState, nPeers)
	logger := consensusLogger()
	for i := 0; i < nPeers; i++ {
		stateDB := dbm.NewMemDB() // each state needs its own db
		state, _ := sm.LoadStateFromDBOrGenesisDoc(stateDB, genDoc)
		thisConfig := ResetConfig(cmn.Fmt("%s_%d", testName, i))
		ensureDir(path.Dir(thisConfig.Consensus.WalFile()), 0700) // dir for wal
		var privVal types.PrivValidator
		if i < nValidators {
			privVal = privVals[i]
		} else {
			_, tempFilePath := cmn.Tempfile("priv_validator_")
			privVal = privval.GenFilePV(tempFilePath)
		}

		app := appFunc()
		vals := types.TM2PB.Validators(state.Validators)
		app.InitChain(abci.RequestInitChain{Validators: vals})

		css[i] = newConsensusStateWithConfig(thisConfig, state, privVal, app)
		css[i].SetTimeoutTicker(tickerFunc())
		css[i].SetLogger(logger.With("validator", i, "module", "consensus"))
	}
	return css
}

func getSwitchIndex(switches []*p2p.Switch, peer p2p.Peer) int {
	for i, s := range switches {
		if peer.NodeInfo().ID == s.NodeInfo().ID {
			return i
		}
	}
	panic("didnt find peer in switches")
	return -1
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
		GenesisTime: time.Now(),
		ChainID:     config.ChainID(),
		Validators:  validators,
	}, privValidators
}

func randGenesisState(numValidators int, randPower bool, minPower int64) (sm.State, []types.PrivValidator) {
	genDoc, privValidators := randGenesisDoc(numValidators, randPower, minPower)
	s0, _ := sm.MakeGenesisState(genDoc)
	db := dbm.NewMemDB()
	sm.SaveState(db, s0)
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

func (mockTicker) SetLogger(log.Logger) {
}

//------------------------------------

func newCounter() abci.Application {
	return counter.NewCounterApplication(true)
}

func newPersistentKVStore() abci.Application {
	dir, _ := ioutil.TempDir("/tmp", "persistent-kvstore")
	return kvstore.NewPersistentKVStoreApplication(dir)
}
