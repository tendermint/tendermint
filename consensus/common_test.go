package consensus

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	abcicli "github.com/tendermint/tendermint/abci/client"
	"github.com/tendermint/tendermint/abci/example/kvstore"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/config"
	cstypes "github.com/tendermint/tendermint/consensus/types"
	"github.com/tendermint/tendermint/internal/test"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/libs/log"
	tmos "github.com/tendermint/tendermint/libs/os"
	tmpubsub "github.com/tendermint/tendermint/libs/pubsub"
	tmsync "github.com/tendermint/tendermint/libs/sync"
	mempl "github.com/tendermint/tendermint/mempool"
	mempoolv0 "github.com/tendermint/tendermint/mempool/v0"
	mempoolv1 "github.com/tendermint/tendermint/mempool/v1"
	"github.com/tendermint/tendermint/privval"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/proxy"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/store"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

const (
	testSubscriber = "test-client"

	// genesis, chain_id, priv_val
	ensureTimeout = time.Millisecond * 200
)

func ensureDir(dir string, mode os.FileMode) {
	if err := tmos.EnsureDir(dir, mode); err != nil {
		panic(err)
	}
}

func ResetConfig(name string) *config.Config {
	return test.ResetTestRoot(name)
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

var testMinPower int64 = 10

func newValidatorStub(privValidator types.PrivValidator, valIndex int32) *validatorStub {
	return &validatorStub{
		Index:         valIndex,
		PrivValidator: privValidator,
		VotingPower:   testMinPower,
	}
}

func (vs *validatorStub) signVote(
	voteType tmproto.SignedMsgType,
	chainID string,
	blockID types.BlockID,
	voteExtension []byte,
) (*types.Vote, error) {

	pubKey, err := vs.PrivValidator.GetPubKey()
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
		BlockID:          blockID,
		Extension:        voteExtension,
	}
	v := vote.ToProto()
	if err := vs.PrivValidator.SignVote(chainID, v); err != nil {
		return nil, fmt.Errorf("sign vote failed: %w", err)
	}

	// ref: signVote in FilePV, the vote should use the privious vote info when the sign data is the same.
	if signDataIsEqual(vs.lastVote, v) {
		v.Signature = vs.lastVote.Signature
		v.Timestamp = vs.lastVote.Timestamp
		v.ExtensionSignature = vs.lastVote.ExtensionSignature
	}

	vote.Signature = v.Signature
	vote.Timestamp = v.Timestamp
	vote.ExtensionSignature = v.ExtensionSignature

	return vote, err
}

// Sign vote for type/hash/header
func signVote(
	t *testing.T,
	vs *validatorStub,
	voteType tmproto.SignedMsgType,
	chainID string,
	blockID types.BlockID) *types.Vote {

	var ext []byte
	// Only non-nil precommits are allowed to carry vote extensions.
	if voteType == tmproto.PrecommitType && !blockID.IsNil() {
		ext = []byte("extension")
	}
	v, err := vs.signVote(voteType, chainID, blockID, ext)
	require.NoError(t, err, "failed to sign vote")

	vs.lastVote = v

	return v
}

func signVotes(
	t *testing.T,
	voteType tmproto.SignedMsgType,
	chainID string,
	blockID types.BlockID,
	vss ...*validatorStub,
) []*types.Vote {
	votes := make([]*types.Vote, len(vss))
	for i, vs := range vss {
		votes[i] = signVote(t, vs, voteType, chainID, blockID)
	}
	return votes
}

func signAddPrecommitWithExtension(
	t *testing.T,
	cs *State,
	blockID types.BlockID,
	extension []byte,
	stub *validatorStub) {
	v, err := stub.signVote(tmproto.PrecommitType, test.DefaultTestChainID, blockID, extension)
	require.NoError(t, err, "failed to sign vote")
	addVotes(cs, v)
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
	vssi, err := vss[i].GetPubKey()
	if err != nil {
		panic(err)
	}
	vssj, err := vss[j].GetPubKey()
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

func startTestRound(cs *State, height int64, round int32) {
	cs.enterNewRound(height, round)
	cs.startRoutines(0)
}

// Create proposal block from cs1 but sign it with vs.
func decideProposal(
	t *testing.T,
	cs1 *State,
	vs *validatorStub,
	height int64,
	round int32,
) (proposal *types.Proposal, block *types.Block) {
	cs1.mtx.Lock()
	block, err := cs1.createProposalBlock()
	require.NoError(t, err)
	blockParts, err := block.MakePartSet(types.BlockPartSizeBytes)
	require.NoError(t, err)
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
	if err := vs.SignProposal(chainID, p); err != nil {
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
	t *testing.T,
	to *State,
	voteType tmproto.SignedMsgType,
	blockID types.BlockID,
	vss ...*validatorStub,
) {
	addVotes(to, signVotes(t, voteType, test.DefaultTestChainID, blockID, vss...)...)
}

func validatePrevote(t *testing.T, cs *State, round int32, privVal *validatorStub, blockHash []byte) {
	prevotes := cs.Votes.Prevotes(round)
	pubKey, err := privVal.GetPubKey()
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

func validateLastPrecommit(t *testing.T, cs *State, privVal *validatorStub, blockHash []byte) {
	votes := cs.LastCommit
	pv, err := privVal.GetPubKey()
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
	t *testing.T,
	cs *State,
	thisRound,
	lockRound int32,
	privVal *validatorStub,
	votedBlockHash,
	lockedBlockHash []byte,
) {
	precommits := cs.Votes.Precommits(thisRound)
	pv, err := privVal.GetPubKey()
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

func subscribeToVoter(cs *State, addr []byte) <-chan tmpubsub.Message {
	votesSub, err := cs.eventBus.SubscribeUnbuffered(context.Background(), testSubscriber, types.EventQueryVote)
	if err != nil {
		panic(fmt.Sprintf("failed to subscribe %s to %v", testSubscriber, types.EventQueryVote))
	}
	ch := make(chan tmpubsub.Message)
	go func() {
		for msg := range votesSub.Out() {
			vote := msg.Data().(types.EventDataVote)
			// we only fire for our own votes
			if bytes.Equal(addr, vote.Vote.ValidatorAddress) {
				ch <- msg
			}
		}
	}()
	return ch
}

//-------------------------------------------------------------------------------
// consensus states

type makeStateArgs struct {
	config      *config.Config
	params      *types.ConsensusParams
	validators  int
	application abci.Application
}

func makeState(t *testing.T, args makeStateArgs) (*State, []*validatorStub) {
	t.Helper()
	// Get State
	validators := 4
	if args.validators != 0 {
		validators = args.validators
	}
	var app abci.Application
	app = kvstore.NewInMemoryApplication()
	if args.application != nil {
		app = args.application
	}
	if args.config == nil {
		args.config = config.TestConfig()
	}
	args.config.SetRoot(t.TempDir())

	cp := test.ConsensusParams()
	if args.params != nil {
		cp = args.params
	}

	state, privVals := makeGenesisState(t, genesisStateArgs{
		params:     cp,
		validators: validators,
	})

	vss := make([]*validatorStub, validators)

	cs := newState(t, args.config, state, privVals[0], app)

	for i := 0; i < validators; i++ {
		vss[i] = newValidatorStub(privVals[i], int32(i))
	}
	// since cs1 starts at 1
	incrementHeight(vss[1:]...)

	return cs, vss
}

func newState(
	t *testing.T,
	cfg *config.Config,
	state sm.State,
	pv types.PrivValidator,
	app abci.Application,
) *State {
	t.Helper()

	// Get BlockStore
	blockStore := store.NewBlockStore(dbm.NewMemDB())

	// one for mempool, one for consensus
	mtx := new(tmsync.Mutex)

	proxyAppConnCon := proxy.NewAppConnConsensus(abcicli.NewLocalClient(mtx, app), proxy.NopMetrics())
	proxyAppConnMem := proxy.NewAppConnMempool(abcicli.NewLocalClient(mtx, app), proxy.NopMetrics())
	// Make Mempool
	memplMetrics := mempl.NopMetrics()

	// Make Mempool
	var mempool mempl.Mempool

	switch cfg.Mempool.Version {
	case config.MempoolV0:
		mempool = mempoolv0.NewCListMempool(cfg.Mempool,
			proxyAppConnMem,
			state.LastBlockHeight,
			mempoolv0.WithMetrics(memplMetrics),
			mempoolv0.WithPreCheck(sm.TxPreCheck(state)),
			mempoolv0.WithPostCheck(sm.TxPostCheck(state)))
	case config.MempoolV1:
		logger := consensusLogger()
		mempool = mempoolv1.NewTxMempool(logger,
			cfg.Mempool,
			proxyAppConnMem,
			state.LastBlockHeight,
			mempoolv1.WithMetrics(memplMetrics),
			mempoolv1.WithPreCheck(sm.TxPreCheck(state)),
			mempoolv1.WithPostCheck(sm.TxPostCheck(state)),
		)
	}
	if cfg.Consensus.WaitForTxs() {
		mempool.EnableTxsAvailable()
	}

	evpool := sm.EmptyEvidencePool{}

	// Make State
	stateStore := sm.NewStore(dbm.NewMemDB(), sm.StoreOptions{
		DiscardFinalizeBlockResponses: false,
	})

	err := stateStore.Save(state) // for save height 1's validators info
	require.NoError(t, err)

	blockExec := sm.NewBlockExecutor(stateStore, log.TestingLogger(), proxyAppConnCon, mempool, evpool, blockStore)
	cs := NewState(cfg.Consensus, state, blockExec, blockStore, mempool, evpool)
	cs.SetLogger(log.TestingLogger().With("module", "consensus"))
	cs.SetPrivValidator(pv)

	eventBus := types.NewEventBus()
	eventBus.SetLogger(log.TestingLogger().With("module", "events"))
	err = eventBus.Start()
	require.NoError(t, err)
	cs.SetEventBus(eventBus)
	return cs
}

func loadPrivValidator(config *config.Config) *privval.FilePV {
	privValidatorKeyFile := config.PrivValidatorKeyFile()
	ensureDir(filepath.Dir(privValidatorKeyFile), 0700)
	privValidatorStateFile := config.PrivValidatorStateFile()
	privValidator := privval.LoadOrGenFilePV(privValidatorKeyFile, privValidatorStateFile)
	privValidator.Reset()
	return privValidator
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

type makeNetworkArgs struct {
	config        *config.Config
	params        *types.ConsensusParams
	validators    int
	nonValidators int
	tickerFunc    func() TimeoutTicker
	appfactory    func() abci.Application
}

func makeNetwork(t *testing.T, args makeNetworkArgs) ([]*State, []types.PrivValidator, *config.Config) {
	t.Helper()

	if args.config == nil {
		args.config = config.TestConfig()
	}
	args.config.SetRoot(t.TempDir())

	if args.appfactory == nil {
		args.appfactory = func() abci.Application {
			return kvstore.NewInMemoryApplication()
		}
	}

	if args.tickerFunc == nil {
		args.tickerFunc = newMockTickerFunc(true)
	}

	if args.validators == 0 {
		args.validators = 4
	}

	state, privVals := makeGenesisState(t, genesisStateArgs{
		validators: args.validators,
		params:     args.params,
	})

	for i := 0; i < args.nonValidators; i++ {
		privVals = append(privVals, types.NewMockPV())
	}

	css := make([]*State, args.validators+args.nonValidators)
	logger := consensusLogger()
	for i := 0; i < args.validators+args.nonValidators; i++ {
		app := args.appfactory()
		vals := types.TM2PB.ValidatorUpdates(state.Validators)
		_, err := app.InitChain(context.Background(), &abci.RequestInitChain{Validators: vals})
		require.NoError(t, err)

		css[i] = newState(t, args.config, state, privVals[i], app)
		css[i].SetTimeoutTicker(args.tickerFunc())
		css[i].SetLogger(logger.With("validator", i, "module", "consensus"))
	}

	return css, privVals, args.config
}

//-------------------------------------------------------------------------------
// genesis

type genesisStateArgs struct {
	validators int
	power      int64
	params     *types.ConsensusParams
	time       time.Time
}

func makeGenesisState(t *testing.T, args genesisStateArgs) (sm.State, []types.PrivValidator) {
	t.Helper()
	if args.power == 0 {
		args.power = 1
	}
	if args.validators == 0 {
		args.validators = 4
	}
	valSet, privValidators := test.ValidatorSet(t, args.validators, args.power)
	if args.params == nil {
		args.params = test.ConsensusParams()
	}
	if args.time.IsZero() {
		args.time = time.Now()
	}
	genDoc := test.GenesisDoc("", args.time, valSet.Validators, args.params)
	state, err := sm.MakeGenesisState(genDoc)
	require.NoError(t, err)
	return state, privValidators
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

func newKVStore() abci.Application {
	return kvstore.NewInMemoryApplication()
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
