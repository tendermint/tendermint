package consensus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"sort"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"

	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto"
	cstypes "github.com/tendermint/tendermint/internal/consensus/types"
	"github.com/tendermint/tendermint/internal/eventbus"
	"github.com/tendermint/tendermint/internal/jsontypes"
	"github.com/tendermint/tendermint/internal/libs/autofile"
	tmstrings "github.com/tendermint/tendermint/internal/libs/strings"
	sm "github.com/tendermint/tendermint/internal/state"
	tmevents "github.com/tendermint/tendermint/libs/events"
	"github.com/tendermint/tendermint/libs/log"
	tmmath "github.com/tendermint/tendermint/libs/math"
	tmos "github.com/tendermint/tendermint/libs/os"
	"github.com/tendermint/tendermint/libs/service"
	tmtime "github.com/tendermint/tendermint/libs/time"
	"github.com/tendermint/tendermint/privval"
	tmgrpc "github.com/tendermint/tendermint/privval/grpc"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

// Consensus sentinel errors
var (
	ErrInvalidProposalSignature   = errors.New("error invalid proposal signature")
	ErrInvalidProposalPOLRound    = errors.New("error invalid proposal POL round")
	ErrAddingVote                 = errors.New("error adding vote")
	ErrSignatureFoundInPastBlocks = errors.New("found signature from the same key")

	errPubKeyIsNotSet = errors.New("pubkey is not set. Look for \"Can't get private validator pubkey\" errors")
)

var msgQueueSize = 1000

// msgs from the reactor which may update the state
type msgInfo struct {
	Msg         Message
	PeerID      types.NodeID
	ReceiveTime time.Time
}

func (msgInfo) TypeTag() string { return "tendermint/wal/MsgInfo" }

type msgInfoJSON struct {
	Msg         json.RawMessage `json:"msg"`
	PeerID      types.NodeID    `json:"peer_key"`
	ReceiveTime time.Time       `json:"receive_time"`
}

func (m msgInfo) MarshalJSON() ([]byte, error) {
	msg, err := jsontypes.Marshal(m.Msg)
	if err != nil {
		return nil, err
	}
	return json.Marshal(msgInfoJSON{Msg: msg, PeerID: m.PeerID, ReceiveTime: m.ReceiveTime})
}

func (m *msgInfo) UnmarshalJSON(data []byte) error {
	var msg msgInfoJSON
	if err := json.Unmarshal(data, &msg); err != nil {
		return err
	}
	if err := jsontypes.Unmarshal(msg.Msg, &m.Msg); err != nil {
		return err
	}
	m.PeerID = msg.PeerID
	return nil
}

// internally generated messages which may update the state
type timeoutInfo struct {
	Duration time.Duration         `json:"duration,string"`
	Height   int64                 `json:"height,string"`
	Round    int32                 `json:"round"`
	Step     cstypes.RoundStepType `json:"step"`
}

func (timeoutInfo) TypeTag() string { return "tendermint/wal/TimeoutInfo" }

func (ti *timeoutInfo) String() string {
	return fmt.Sprintf("%v ; %d/%d %v", ti.Duration, ti.Height, ti.Round, ti.Step)
}

// interface to the mempool
type txNotifier interface {
	TxsAvailable() <-chan struct{}
}

// interface to the evidence pool
type evidencePool interface {
	// reports conflicting votes to the evidence pool to be processed into evidence
	ReportConflictingVotes(voteA, voteB *types.Vote)
}

// State handles execution of the consensus algorithm.
// It processes votes and proposals, and upon reaching agreement,
// commits blocks to the chain and executes them against the application.
// The internal state machine receives input from peers, the internal validator, and from a timer.
type State struct {
	service.BaseService
	logger log.Logger

	// config details
	config            *config.ConsensusConfig
	privValidator     types.PrivValidator // for signing votes
	privValidatorType types.PrivValidatorType

	// store blocks and commits
	blockStore sm.BlockStore

	stateStore        sm.Store
	skipBootstrapping bool

	// create and execute blocks
	blockExec *sm.BlockExecutor

	// notify us if txs are available
	txNotifier txNotifier

	// add evidence to the pool
	// when it's detected
	evpool evidencePool

	// internal state
	mtx sync.RWMutex
	cstypes.RoundState
	state sm.State // State until height-1.
	// privValidator pubkey, memoized for the duration of one block
	// to avoid extra requests to HSM
	privValidatorPubKey crypto.PubKey

	// state changes may be triggered by: msgs from peers,
	// msgs from ourself, or by timeouts
	peerMsgQueue     chan msgInfo
	internalMsgQueue chan msgInfo
	timeoutTicker    TimeoutTicker

	// information about about added votes and block parts are written on this channel
	// so statistics can be computed by reactor
	statsMsgQueue chan msgInfo

	// we use eventBus to trigger msg broadcasts in the reactor,
	// and to notify external subscribers, eg. through a websocket
	eventBus *eventbus.EventBus

	// a Write-Ahead Log ensures we can recover from any kind of crash
	// and helps us avoid signing conflicting votes
	wal          WAL
	replayMode   bool // so we don't log signing errors during replay
	doWALCatchup bool // determines if we even try to do the catchup

	// for tests where we want to limit the number of transitions the state makes
	nSteps int

	// some functions can be overwritten for testing
	decideProposal func(ctx context.Context, height int64, round int32)
	doPrevote      func(ctx context.Context, height int64, round int32)
	setProposal    func(proposal *types.Proposal, t time.Time) error

	// synchronous pubsub between consensus state and reactor.
	// state only emits EventNewRoundStep, EventValidBlock, and EventVote
	evsw tmevents.EventSwitch

	// for reporting metrics
	metrics *Metrics

	// wait the channel event happening for shutting down the state gracefully
	onStopCh chan *cstypes.RoundState
}

// StateOption sets an optional parameter on the State.
type StateOption func(*State)

// SkipStateStoreBootstrap is a state option forces the constructor to
// skip state bootstrapping during construction.
func SkipStateStoreBootstrap(sm *State) {
	sm.skipBootstrapping = true
}

// NewState returns a new State.
func NewState(
	logger log.Logger,
	cfg *config.ConsensusConfig,
	store sm.Store,
	blockExec *sm.BlockExecutor,
	blockStore sm.BlockStore,
	txNotifier txNotifier,
	evpool evidencePool,
	eventBus *eventbus.EventBus,
	options ...StateOption,
) (*State, error) {
	cs := &State{
		eventBus:         eventBus,
		logger:           logger,
		config:           cfg,
		blockExec:        blockExec,
		blockStore:       blockStore,
		stateStore:       store,
		txNotifier:       txNotifier,
		peerMsgQueue:     make(chan msgInfo, msgQueueSize),
		internalMsgQueue: make(chan msgInfo, msgQueueSize),
		timeoutTicker:    NewTimeoutTicker(logger),
		statsMsgQueue:    make(chan msgInfo, msgQueueSize),
		doWALCatchup:     true,
		wal:              nilWAL{},
		evpool:           evpool,
		evsw:             tmevents.NewEventSwitch(),
		metrics:          NopMetrics(),
		onStopCh:         make(chan *cstypes.RoundState),
	}

	// set function defaults (may be overwritten before calling Start)
	cs.decideProposal = cs.defaultDecideProposal
	cs.doPrevote = cs.defaultDoPrevote
	cs.setProposal = cs.defaultSetProposal

	// NOTE: we do not call scheduleRound0 yet, we do that upon Start()
	cs.BaseService = *service.NewBaseService(logger, "State", cs)
	for _, option := range options {
		option(cs)
	}

	// this is not ideal, but it lets the consensus tests start
	// node-fragments gracefully while letting the nodes
	// themselves avoid this.
	if !cs.skipBootstrapping {
		if err := cs.updateStateFromStore(); err != nil {
			return nil, err
		}
	}

	return cs, nil
}

func (cs *State) updateStateFromStore() error {
	state, err := cs.stateStore.Load()
	if err != nil {
		return fmt.Errorf("loading state: %w", err)
	}
	if state.IsEmpty() {
		return nil
	}

	eq, err := state.Equals(cs.state)
	if err != nil {
		return fmt.Errorf("comparing state: %w", err)
	}
	// if the new state is equivalent to the old state, we should not trigger a state update.
	if eq {
		return nil
	}

	// We have no votes, so reconstruct LastCommit from SeenCommit.
	if state.LastBlockHeight > 0 {
		cs.reconstructLastCommit(state)
	}

	cs.updateToState(state)

	return nil
}

// StateMetrics sets the metrics.
func StateMetrics(metrics *Metrics) StateOption {
	return func(cs *State) { cs.metrics = metrics }
}

// String returns a string.
func (cs *State) String() string {
	// better not to access shared variables
	return "ConsensusState"
}

// GetState returns a copy of the chain state.
func (cs *State) GetState() sm.State {
	cs.mtx.RLock()
	defer cs.mtx.RUnlock()
	return cs.state.Copy()
}

// GetLastHeight returns the last height committed.
// If there were no blocks, returns 0.
func (cs *State) GetLastHeight() int64 {
	cs.mtx.RLock()
	defer cs.mtx.RUnlock()
	return cs.RoundState.Height - 1
}

// GetRoundState returns a shallow copy of the internal consensus state.
func (cs *State) GetRoundState() *cstypes.RoundState {
	cs.mtx.RLock()
	defer cs.mtx.RUnlock()

	// NOTE: this might be dodgy, as RoundState itself isn't thread
	// safe as it contains a number of pointers and is explicitly
	// not thread safe.
	rs := cs.RoundState // copy
	return &rs
}

// GetRoundStateJSON returns a json of RoundState.
func (cs *State) GetRoundStateJSON() ([]byte, error) {
	cs.mtx.RLock()
	defer cs.mtx.RUnlock()
	return json.Marshal(cs.RoundState)
}

// GetRoundStateSimpleJSON returns a json of RoundStateSimple
func (cs *State) GetRoundStateSimpleJSON() ([]byte, error) {
	cs.mtx.RLock()
	defer cs.mtx.RUnlock()
	return json.Marshal(cs.RoundState.RoundStateSimple())
}

// GetValidators returns a copy of the current validators.
func (cs *State) GetValidators() (int64, []*types.Validator) {
	cs.mtx.RLock()
	defer cs.mtx.RUnlock()
	return cs.state.LastBlockHeight, cs.state.Validators.Copy().Validators
}

// SetPrivValidator sets the private validator account for signing votes. It
// immediately requests pubkey and caches it.
func (cs *State) SetPrivValidator(ctx context.Context, priv types.PrivValidator) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	cs.privValidator = priv

	if priv != nil {
		switch t := priv.(type) {
		case *privval.RetrySignerClient:
			cs.privValidatorType = types.RetrySignerClient
		case *privval.FilePV:
			cs.privValidatorType = types.FileSignerClient
		case *privval.SignerClient:
			cs.privValidatorType = types.SignerSocketClient
		case *tmgrpc.SignerClient:
			cs.privValidatorType = types.SignerGRPCClient
		case types.MockPV:
			cs.privValidatorType = types.MockSignerClient
		case *types.ErroringMockPV:
			cs.privValidatorType = types.ErrorMockSignerClient
		default:
			cs.logger.Error("unsupported priv validator type", "err",
				fmt.Errorf("error privValidatorType %s", t))
		}
	}

	if err := cs.updatePrivValidatorPubKey(ctx); err != nil {
		cs.logger.Error("failed to get private validator pubkey", "err", err)
	}
}

// SetTimeoutTicker sets the local timer. It may be useful to overwrite for
// testing.
func (cs *State) SetTimeoutTicker(timeoutTicker TimeoutTicker) {
	cs.mtx.Lock()
	cs.timeoutTicker = timeoutTicker
	cs.mtx.Unlock()
}

// LoadCommit loads the commit for a given height.
func (cs *State) LoadCommit(height int64) *types.Commit {
	cs.mtx.RLock()
	defer cs.mtx.RUnlock()

	if height == cs.blockStore.Height() {
		commit := cs.blockStore.LoadSeenCommit()
		// NOTE: Retrieving the height of the most recent block and retrieving
		// the most recent commit does not currently occur as an atomic
		// operation. We check the height and commit here in case a more recent
		// commit has arrived since retrieving the latest height.
		if commit != nil && commit.Height == height {
			return commit
		}
	}

	return cs.blockStore.LoadBlockCommit(height)
}

// OnStart loads the latest state via the WAL, and starts the timeout and
// receive routines.
func (cs *State) OnStart(ctx context.Context) error {
	if err := cs.updateStateFromStore(); err != nil {
		return err
	}

	// We may set the WAL in testing before calling Start, so only OpenWAL if its
	// still the nilWAL.
	if _, ok := cs.wal.(nilWAL); ok {
		if err := cs.loadWalFile(ctx); err != nil {
			return err
		}
	}

	// we need the timeoutRoutine for replay so
	// we don't block on the tick chan.
	// NOTE: we will get a build up of garbage go routines
	// firing on the tockChan until the receiveRoutine is started
	// to deal with them (by that point, at most one will be valid)
	if err := cs.timeoutTicker.Start(ctx); err != nil {
		return err
	}

	// We may have lost some votes if the process crashed reload from consensus
	// log to catchup.
	if cs.doWALCatchup {
		repairAttempted := false

	LOOP:
		for {
			err := cs.catchupReplay(ctx, cs.Height)
			switch {
			case err == nil:
				break LOOP

			case !IsDataCorruptionError(err):
				cs.logger.Error("error on catchup replay; proceeding to start state anyway", "err", err)
				break LOOP

			case repairAttempted:
				return err
			}

			cs.logger.Error("the WAL file is corrupted; attempting repair", "err", err)

			// 1) prep work
			cs.wal.Stop()

			repairAttempted = true

			// 2) backup original WAL file
			corruptedFile := fmt.Sprintf("%s.CORRUPTED", cs.config.WalFile())
			if err := tmos.CopyFile(cs.config.WalFile(), corruptedFile); err != nil {
				return err
			}

			cs.logger.Debug("backed up WAL file", "src", cs.config.WalFile(), "dst", corruptedFile)

			// 3) try to repair (WAL file will be overwritten!)
			if err := repairWalFile(corruptedFile, cs.config.WalFile()); err != nil {
				cs.logger.Error("the WAL repair failed", "err", err)
				return err
			}

			cs.logger.Info("successful WAL repair")

			// reload WAL file
			if err := cs.loadWalFile(ctx); err != nil {
				return err
			}
		}
	}

	// Double Signing Risk Reduction
	if err := cs.checkDoubleSigningRisk(cs.Height); err != nil {
		return err
	}

	// now start the receiveRoutine
	go cs.receiveRoutine(ctx, 0)

	// schedule the first round!
	// use GetRoundState so we don't race the receiveRoutine for access
	cs.scheduleRound0(cs.GetRoundState())

	return nil
}

// timeoutRoutine: receive requests for timeouts on tickChan and fire timeouts on tockChan
// receiveRoutine: serializes processing of proposoals, block parts, votes; coordinates state transitions
//
// this is only used in tests.
func (cs *State) startRoutines(ctx context.Context, maxSteps int) {
	err := cs.timeoutTicker.Start(ctx)
	if err != nil {
		cs.logger.Error("failed to start timeout ticker", "err", err)
		return
	}

	go cs.receiveRoutine(ctx, maxSteps)
}

// loadWalFile loads WAL data from file. It overwrites cs.wal.
func (cs *State) loadWalFile(ctx context.Context) error {
	wal, err := cs.OpenWAL(ctx, cs.config.WalFile())
	if err != nil {
		cs.logger.Error("failed to load state WAL", "err", err)
		return err
	}

	cs.wal = wal
	return nil
}

func (cs *State) getOnStopCh() chan *cstypes.RoundState {
	cs.mtx.RLock()
	defer cs.mtx.RUnlock()

	return cs.onStopCh
}

// OnStop implements service.Service.
func (cs *State) OnStop() {
	// If the node is committing a new block, wait until it is finished!
	if cs.GetRoundState().Step == cstypes.RoundStepCommit {
		select {
		case <-cs.getOnStopCh():
		case <-time.After(cs.state.ConsensusParams.Timeout.Commit):
			cs.logger.Error("OnStop: timeout waiting for commit to finish", "time", cs.state.ConsensusParams.Timeout.Commit)
		}
	}

	if cs.timeoutTicker.IsRunning() {
		cs.timeoutTicker.Stop()
	}
	// WAL is stopped in receiveRoutine.
}

// OpenWAL opens a file to log all consensus messages and timeouts for
// deterministic accountability.
func (cs *State) OpenWAL(ctx context.Context, walFile string) (WAL, error) {
	wal, err := NewWAL(ctx, cs.logger.With("wal", walFile), walFile)
	if err != nil {
		cs.logger.Error("failed to open WAL", "file", walFile, "err", err)
		return nil, err
	}

	if err := wal.Start(ctx); err != nil {
		cs.logger.Error("failed to start WAL", "err", err)
		return nil, err
	}

	return wal, nil
}

//------------------------------------------------------------
// Public interface for passing messages into the consensus state, possibly causing a state transition.
// If peerID == "", the msg is considered internal.
// Messages are added to the appropriate queue (peer or internal).
// If the queue is full, the function may block.
// TODO: should these return anything or let callers just use events?

// AddVote inputs a vote.
func (cs *State) AddVote(ctx context.Context, vote *types.Vote, peerID types.NodeID) error {
	if peerID == "" {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case cs.internalMsgQueue <- msgInfo{&VoteMessage{vote}, "", tmtime.Now()}:
			return nil
		}
	} else {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case cs.peerMsgQueue <- msgInfo{&VoteMessage{vote}, peerID, tmtime.Now()}:
			return nil
		}
	}

	// TODO: wait for event?!
}

// SetProposal inputs a proposal.
func (cs *State) SetProposal(ctx context.Context, proposal *types.Proposal, peerID types.NodeID) error {

	if peerID == "" {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case cs.internalMsgQueue <- msgInfo{&ProposalMessage{proposal}, "", tmtime.Now()}:
			return nil
		}
	} else {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case cs.peerMsgQueue <- msgInfo{&ProposalMessage{proposal}, peerID, tmtime.Now()}:
			return nil
		}
	}

	// TODO: wait for event?!
}

// AddProposalBlockPart inputs a part of the proposal block.
func (cs *State) AddProposalBlockPart(ctx context.Context, height int64, round int32, part *types.Part, peerID types.NodeID) error {
	if peerID == "" {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case cs.internalMsgQueue <- msgInfo{&BlockPartMessage{height, round, part}, "", tmtime.Now()}:
			return nil
		}
	} else {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case cs.peerMsgQueue <- msgInfo{&BlockPartMessage{height, round, part}, peerID, tmtime.Now()}:
			return nil
		}
	}

	// TODO: wait for event?!
}

// SetProposalAndBlock inputs the proposal and all block parts.
func (cs *State) SetProposalAndBlock(
	ctx context.Context,
	proposal *types.Proposal,
	block *types.Block,
	parts *types.PartSet,
	peerID types.NodeID,
) error {

	if err := cs.SetProposal(ctx, proposal, peerID); err != nil {
		return err
	}

	for i := 0; i < int(parts.Total()); i++ {
		part := parts.GetPart(i)
		if err := cs.AddProposalBlockPart(ctx, proposal.Height, proposal.Round, part, peerID); err != nil {
			return err
		}
	}

	return nil
}

//------------------------------------------------------------
// internal functions for managing the state

func (cs *State) updateHeight(height int64) {
	cs.metrics.Height.Set(float64(height))
	cs.Height = height
}

func (cs *State) updateRoundStep(round int32, step cstypes.RoundStepType) {
	if !cs.replayMode {
		if round != cs.Round || round == 0 && step == cstypes.RoundStepNewRound {
			cs.metrics.MarkRound(cs.Round, cs.StartTime)
		}
		if cs.Step != step {
			cs.metrics.MarkStep(cs.Step)
		}
	}
	cs.Round = round
	cs.Step = step
}

// enterNewRound(height, 0) at cs.StartTime.
func (cs *State) scheduleRound0(rs *cstypes.RoundState) {
	// cs.logger.Info("scheduleRound0", "now", tmtime.Now(), "startTime", cs.StartTime)
	sleepDuration := rs.StartTime.Sub(tmtime.Now())
	cs.scheduleTimeout(sleepDuration, rs.Height, 0, cstypes.RoundStepNewHeight)
}

// Attempt to schedule a timeout (by sending timeoutInfo on the tickChan)
func (cs *State) scheduleTimeout(duration time.Duration, height int64, round int32, step cstypes.RoundStepType) {
	cs.timeoutTicker.ScheduleTimeout(timeoutInfo{duration, height, round, step})
}

// send a msg into the receiveRoutine regarding our own proposal, block part, or vote
func (cs *State) sendInternalMessage(ctx context.Context, mi msgInfo) {
	select {
	case <-ctx.Done():
	case cs.internalMsgQueue <- mi:
	default:
		// NOTE: using the go-routine means our votes can
		// be processed out of order.
		// TODO: use CList here for strict determinism and
		// attempt push to internalMsgQueue in receiveRoutine
		cs.logger.Debug("internal msg queue is full; using a go-routine")
		go func() {
			select {
			case <-ctx.Done():
			case cs.internalMsgQueue <- mi:
			}
		}()
	}
}

// Reconstruct the LastCommit from either SeenCommit or the ExtendedCommit. SeenCommit
// and ExtendedCommit are saved along with the block. If VoteExtensions are required
// the method will panic on an absent ExtendedCommit or an ExtendedCommit without
// extension data.
func (cs *State) reconstructLastCommit(state sm.State) {
	extensionsEnabled := cs.state.ConsensusParams.ABCI.VoteExtensionsEnabled(state.LastBlockHeight)
	if !extensionsEnabled {
		votes, err := cs.votesFromSeenCommit(state)
		if err != nil {
			panic(fmt.Sprintf("failed to reconstruct last commit; %s", err))
		}
		cs.LastCommit = votes
		return
	}

	votes, err := cs.votesFromExtendedCommit(state)
	if err != nil {
		panic(fmt.Sprintf("failed to reconstruct last extended commit; %s", err))
	}
	cs.LastCommit = votes
}

func (cs *State) votesFromExtendedCommit(state sm.State) (*types.VoteSet, error) {
	ec := cs.blockStore.LoadBlockExtendedCommit(state.LastBlockHeight)
	if ec == nil {
		return nil, fmt.Errorf("extended commit for height %v not found", state.LastBlockHeight)
	}
	vs := ec.ToExtendedVoteSet(state.ChainID, state.LastValidators)
	if !vs.HasTwoThirdsMajority() {
		return nil, errors.New("extended commit does not have +2/3 majority")
	}
	return vs, nil
}

func (cs *State) votesFromSeenCommit(state sm.State) (*types.VoteSet, error) {
	commit := cs.blockStore.LoadSeenCommit()
	if commit == nil || commit.Height != state.LastBlockHeight {
		commit = cs.blockStore.LoadBlockCommit(state.LastBlockHeight)
	}
	if commit == nil {
		return nil, fmt.Errorf("commit for height %v not found", state.LastBlockHeight)
	}

	vs := commit.ToVoteSet(state.ChainID, state.LastValidators)
	if !vs.HasTwoThirdsMajority() {
		return nil, errors.New("commit does not have +2/3 majority")
	}
	return vs, nil
}

// Updates State and increments height to match that of state.
// The round becomes 0 and cs.Step becomes cstypes.RoundStepNewHeight.
func (cs *State) updateToState(state sm.State) {
	if cs.CommitRound > -1 && 0 < cs.Height && cs.Height != state.LastBlockHeight {
		panic(fmt.Sprintf(
			"updateToState() expected state height of %v but found %v",
			cs.Height, state.LastBlockHeight,
		))
	}

	if !cs.state.IsEmpty() {
		if cs.state.LastBlockHeight > 0 && cs.state.LastBlockHeight+1 != cs.Height {
			// This might happen when someone else is mutating cs.state.
			// Someone forgot to pass in state.Copy() somewhere?!
			panic(fmt.Sprintf(
				"inconsistent cs.state.LastBlockHeight+1 %v vs cs.Height %v",
				cs.state.LastBlockHeight+1, cs.Height,
			))
		}
		if cs.state.LastBlockHeight > 0 && cs.Height == cs.state.InitialHeight {
			panic(fmt.Sprintf(
				"inconsistent cs.state.LastBlockHeight %v, expected 0 for initial height %v",
				cs.state.LastBlockHeight, cs.state.InitialHeight,
			))
		}

		// If state isn't further out than cs.state, just ignore.
		// This happens when SwitchToConsensus() is called in the reactor.
		// We don't want to reset e.g. the Votes, but we still want to
		// signal the new round step, because other services (eg. txNotifier)
		// depend on having an up-to-date peer state!
		if state.LastBlockHeight <= cs.state.LastBlockHeight {
			cs.logger.Debug("ignoring updateToState()",
				"new_height", state.LastBlockHeight+1,
				"old_height", cs.state.LastBlockHeight+1)
			cs.newStep()
			return
		}
	}

	// Reset fields based on state.
	validators := state.Validators

	switch {
	case state.LastBlockHeight == 0: // Very first commit should be empty.
		cs.LastCommit = (*types.VoteSet)(nil)
	case cs.CommitRound > -1 && cs.Votes != nil: // Otherwise, use cs.Votes
		if !cs.Votes.Precommits(cs.CommitRound).HasTwoThirdsMajority() {
			panic(fmt.Sprintf(
				"wanted to form a commit, but precommits (H/R: %d/%d) didn't have 2/3+: %v",
				state.LastBlockHeight, cs.CommitRound, cs.Votes.Precommits(cs.CommitRound),
			))
		}

		cs.LastCommit = cs.Votes.Precommits(cs.CommitRound)

	case cs.LastCommit == nil:
		// NOTE: when Tendermint starts, it has no votes. reconstructLastCommit
		// must be called to reconstruct LastCommit from SeenCommit.
		panic(fmt.Sprintf(
			"last commit cannot be empty after initial block (H:%d)",
			state.LastBlockHeight+1,
		))
	}

	// Next desired block height
	height := state.LastBlockHeight + 1
	if height == 1 {
		height = state.InitialHeight
	}

	// RoundState fields
	cs.updateHeight(height)
	cs.updateRoundStep(0, cstypes.RoundStepNewHeight)

	if cs.CommitTime.IsZero() {
		// "Now" makes it easier to sync up dev nodes.
		// We add timeoutCommit to allow transactions
		// to be gathered for the first block.
		// And alternative solution that relies on clocks:
		// cs.StartTime = state.LastBlockTime.Add(timeoutCommit)
		cs.StartTime = cs.commitTime(tmtime.Now())
	} else {
		cs.StartTime = cs.commitTime(cs.CommitTime)
	}

	cs.Validators = validators
	cs.Proposal = nil
	cs.ProposalReceiveTime = time.Time{}
	cs.ProposalBlock = nil
	cs.ProposalBlockParts = nil
	cs.LockedRound = -1
	cs.LockedBlock = nil
	cs.LockedBlockParts = nil
	cs.ValidRound = -1
	cs.ValidBlock = nil
	cs.ValidBlockParts = nil
	if state.ConsensusParams.ABCI.VoteExtensionsEnabled(height) {
		cs.Votes = cstypes.NewExtendedHeightVoteSet(state.ChainID, height, validators)
	} else {
		cs.Votes = cstypes.NewHeightVoteSet(state.ChainID, height, validators)
	}
	cs.CommitRound = -1
	cs.LastValidators = state.LastValidators
	cs.TriggeredTimeoutPrecommit = false

	cs.state = state

	// Finally, broadcast RoundState
	cs.newStep()
}

func (cs *State) newStep() {
	rs := cs.RoundStateEvent()
	if err := cs.wal.Write(rs); err != nil {
		cs.logger.Error("failed writing to WAL", "err", err)
	}

	cs.nSteps++

	// newStep is called by updateToState in NewState before the eventBus is set!
	if cs.eventBus != nil {
		if err := cs.eventBus.PublishEventNewRoundStep(rs); err != nil {
			cs.logger.Error("failed publishing new round step", "err", err)
		}

		cs.evsw.FireEvent(types.EventNewRoundStepValue, &cs.RoundState)
	}
}

//-----------------------------------------
// the main go routines

// receiveRoutine handles messages which may cause state transitions.
// it's argument (n) is the number of messages to process before exiting - use 0 to run forever
// It keeps the RoundState and is the only thing that updates it.
// Updates (state transitions) happen on timeouts, complete proposals, and 2/3 majorities.
// State must be locked before any internal state is updated.
func (cs *State) receiveRoutine(ctx context.Context, maxSteps int) {
	onExit := func(cs *State) {
		// NOTE: the internalMsgQueue may have signed messages from our
		// priv_val that haven't hit the WAL, but its ok because
		// priv_val tracks LastSig

		// close wal now that we're done writing to it
		cs.wal.Stop()
		cs.wal.Wait()
	}

	defer func() {
		if r := recover(); r != nil {
			cs.logger.Error("CONSENSUS FAILURE!!!", "err", r, "stack", string(debug.Stack()))

			// Make a best-effort attempt to close the WAL, but otherwise do not
			// attempt to gracefully terminate. Once consensus has irrecoverably
			// failed, any additional progress we permit the node to make may
			// complicate diagnosing and recovering from the failure.
			onExit(cs)

			// There are a couple of cases where the we
			// panic with an error from deeper within the
			// state machine and in these cases, typically
			// during a normal shutdown, we can continue
			// with normal shutdown with safety. These
			// cases are:
			if err, ok := r.(error); ok {
				// TODO(creachadair): In ordinary operation, the WAL autofile should
				// never be closed. This only happens during shutdown and production
				// nodes usually halt by panicking. Many existing tests, however,
				// assume a clean shutdown is possible. Prior to #8111, we were
				// swallowing the panic in receiveRoutine, making that appear to
				// work. Filtering this specific error is slightly risky, but should
				// affect only unit tests. In any case, not re-panicking here only
				// preserves the pre-existing behavior for this one error type.
				if errors.Is(err, autofile.ErrAutoFileClosed) {
					return
				}

				// don't re-panic if the panic is just an
				// error and we're already trying to shut down
				if ctx.Err() != nil {
					return

				}
			}

			// Re-panic to ensure the node terminates.
			//
			panic(r)
		}
	}()

	for {
		if maxSteps > 0 {
			if cs.nSteps >= maxSteps {
				cs.logger.Debug("reached max steps; exiting receive routine")
				cs.nSteps = 0
				return
			}
		}

		select {
		case <-cs.txNotifier.TxsAvailable():
			cs.handleTxsAvailable(ctx)

		case mi := <-cs.peerMsgQueue:
			if err := cs.wal.Write(mi); err != nil {
				cs.logger.Error("failed writing to WAL", "err", err)
			}
			// handles proposals, block parts, votes
			// may generate internal events (votes, complete proposals, 2/3 majorities)
			cs.handleMsg(ctx, mi)

		case mi := <-cs.internalMsgQueue:
			err := cs.wal.WriteSync(mi) // NOTE: fsync
			if err != nil {
				panic(fmt.Errorf(
					"failed to write %v msg to consensus WAL due to %w; check your file system and restart the node",
					mi, err,
				))
			}

			// handles proposals, block parts, votes
			cs.handleMsg(ctx, mi)

		case ti := <-cs.timeoutTicker.Chan(): // tockChan:
			if err := cs.wal.Write(ti); err != nil {
				cs.logger.Error("failed writing to WAL", "err", err)
			}

			// if the timeout is relevant to the rs
			// go to the next step
			cs.handleTimeout(ctx, ti, cs.RoundState)

		case <-ctx.Done():
			onExit(cs)
			return

		}
		// TODO should we handle context cancels here?
	}
}

// state transitions on complete-proposal, 2/3-any, 2/3-one
func (cs *State) handleMsg(ctx context.Context, mi msgInfo) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()
	var (
		added bool
		err   error
	)

	msg, peerID := mi.Msg, mi.PeerID

	switch msg := msg.(type) {
	case *ProposalMessage:
		// will not cause transition.
		// once proposal is set, we can receive block parts
		err = cs.setProposal(msg.Proposal, mi.ReceiveTime)

	case *BlockPartMessage:
		// if the proposal is complete, we'll enterPrevote or tryFinalizeCommit
		added, err = cs.addProposalBlockPart(msg, peerID)

		// We unlock here to yield to any routines that need to read the the RoundState.
		// Previously, this code held the lock from the point at which the final block
		// part was received until the block executed against the application.
		// This prevented the reactor from being able to retrieve the most updated
		// version of the RoundState. The reactor needs the updated RoundState to
		// gossip the now completed block.
		//
		// This code can be further improved by either always operating on a copy
		// of RoundState and only locking when switching out State's copy of
		// RoundState with the updated copy or by emitting RoundState events in
		// more places for routines depending on it to listen for.
		cs.mtx.Unlock()

		cs.mtx.Lock()
		if added && cs.ProposalBlockParts.IsComplete() {
			cs.handleCompleteProposal(ctx, msg.Height)
		}
		if added {
			select {
			case cs.statsMsgQueue <- mi:
			case <-ctx.Done():
				return
			}
		}

		if err != nil && msg.Round != cs.Round {
			cs.logger.Debug("received block part from wrong round",
				"height", cs.Height,
				"cs_round", cs.Round,
				"block_round", msg.Round)
			err = nil
		}

	case *VoteMessage:
		// attempt to add the vote and dupeout the validator if its a duplicate signature
		// if the vote gives us a 2/3-any or 2/3-one, we transition
		added, err = cs.tryAddVote(ctx, msg.Vote, peerID)
		if added {
			select {
			case cs.statsMsgQueue <- mi:
			case <-ctx.Done():
				return
			}
		}

		// TODO: punish peer
		// We probably don't want to stop the peer here. The vote does not
		// necessarily comes from a malicious peer but can be just broadcasted by
		// a typical peer.
		// https://github.com/tendermint/tendermint/issues/1281

		// NOTE: the vote is broadcast to peers by the reactor listening
		// for vote events

		// TODO: If rs.Height == vote.Height && rs.Round < vote.Round,
		// the peer is sending us CatchupCommit precommits.
		// We could make note of this and help filter in broadcastHasVoteMessage().

	default:
		cs.logger.Error("unknown msg type", "type", tmstrings.LazySprintf("%T", msg))
		return
	}

	if err != nil {
		cs.logger.Error(
			"failed to process message",
			"height", cs.Height,
			"round", cs.Round,
			"peer", peerID,
			"msg_type", fmt.Sprintf("%T", msg),
			"err", err,
		)
	}
}

func (cs *State) handleTimeout(
	ctx context.Context,
	ti timeoutInfo,
	rs cstypes.RoundState,
) {
	cs.logger.Debug("received tock", "timeout", ti.Duration, "height", ti.Height, "round", ti.Round, "step", ti.Step)

	// timeouts must be for current height, round, step
	if ti.Height != rs.Height || ti.Round < rs.Round || (ti.Round == rs.Round && ti.Step < rs.Step) {
		cs.logger.Debug("ignoring tock because we are ahead", "height", rs.Height, "round", rs.Round, "step", rs.Step)
		return
	}

	// the timeout will now cause a state transition
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	switch ti.Step {
	case cstypes.RoundStepNewHeight:
		// NewRound event fired from enterNewRound.
		// XXX: should we fire timeout here (for timeout commit)?
		cs.enterNewRound(ctx, ti.Height, 0)

	case cstypes.RoundStepNewRound:
		cs.enterPropose(ctx, ti.Height, 0)

	case cstypes.RoundStepPropose:
		if err := cs.eventBus.PublishEventTimeoutPropose(cs.RoundStateEvent()); err != nil {
			cs.logger.Error("failed publishing timeout propose", "err", err)
		}

		cs.enterPrevote(ctx, ti.Height, ti.Round)

	case cstypes.RoundStepPrevoteWait:
		if err := cs.eventBus.PublishEventTimeoutWait(cs.RoundStateEvent()); err != nil {
			cs.logger.Error("failed publishing timeout wait", "err", err)
		}

		cs.enterPrecommit(ctx, ti.Height, ti.Round)

	case cstypes.RoundStepPrecommitWait:
		if err := cs.eventBus.PublishEventTimeoutWait(cs.RoundStateEvent()); err != nil {
			cs.logger.Error("failed publishing timeout wait", "err", err)
		}

		cs.enterPrecommit(ctx, ti.Height, ti.Round)
		cs.enterNewRound(ctx, ti.Height, ti.Round+1)

	default:
		panic(fmt.Sprintf("invalid timeout step: %v", ti.Step))
	}

}

func (cs *State) handleTxsAvailable(ctx context.Context) {
	cs.mtx.Lock()
	defer cs.mtx.Unlock()

	// We only need to do this for round 0.
	if cs.Round != 0 {
		return
	}

	switch cs.Step {
	case cstypes.RoundStepNewHeight: // timeoutCommit phase
		if cs.needProofBlock(cs.Height) {
			// enterPropose will be called by enterNewRound
			return
		}

		// +1ms to ensure RoundStepNewRound timeout always happens after RoundStepNewHeight
		timeoutCommit := cs.StartTime.Sub(tmtime.Now()) + 1*time.Millisecond
		cs.scheduleTimeout(timeoutCommit, cs.Height, 0, cstypes.RoundStepNewRound)

	case cstypes.RoundStepNewRound: // after timeoutCommit
		cs.enterPropose(ctx, cs.Height, 0)
	}
}

//-----------------------------------------------------------------------------
// State functions
// Used internally by handleTimeout and handleMsg to make state transitions

// Enter: `timeoutNewHeight` by startTime (commitTime+timeoutCommit),
//	or, if SkipTimeoutCommit==true, after receiving all precommits from (height,round-1)
// Enter: `timeoutPrecommits` after any +2/3 precommits from (height,round-1)
// Enter: +2/3 precommits for nil at (height,round-1)
// Enter: +2/3 prevotes any or +2/3 precommits for block or any from (height, round)
// NOTE: cs.StartTime was already set for height.
func (cs *State) enterNewRound(ctx context.Context, height int64, round int32) {
	// TODO: remove panics in this function and return an error

	logger := cs.logger.With("height", height, "round", round)

	if cs.Height != height || round < cs.Round || (cs.Round == round && cs.Step != cstypes.RoundStepNewHeight) {
		logger.Debug("entering new round with invalid args",
			"height", cs.Height,
			"round", cs.Round,
			"step", cs.Step)
		return
	}

	if now := tmtime.Now(); cs.StartTime.After(now) {
		logger.Debug("need to set a buffer and log message here for sanity", "start_time", cs.StartTime, "now", now)
	}

	logger.Debug("entering new round",
		"height", cs.Height,
		"round", cs.Round,
		"step", cs.Step)

	// increment validators if necessary
	validators := cs.Validators
	if cs.Round < round {
		validators = validators.Copy()
		r, err := tmmath.SafeSubInt32(round, cs.Round)
		if err != nil {
			panic(err)
		}
		validators.IncrementProposerPriority(r)
	}

	// Setup new round
	// we don't fire newStep for this step,
	// but we fire an event, so update the round step first
	cs.updateRoundStep(round, cstypes.RoundStepNewRound)
	cs.Validators = validators
	if round == 0 {
		// We've already reset these upon new height,
		// and meanwhile we might have received a proposal
		// for round 0.
	} else {
		logger.Debug("resetting proposal info")
		cs.Proposal = nil
		cs.ProposalReceiveTime = time.Time{}
		cs.ProposalBlock = nil
		cs.ProposalBlockParts = nil
	}

	r, err := tmmath.SafeAddInt32(round, 1)
	if err != nil {
		panic(err)
	}

	cs.Votes.SetRound(r) // also track next round (round+1) to allow round-skipping
	cs.TriggeredTimeoutPrecommit = false

	if err := cs.eventBus.PublishEventNewRound(cs.NewRoundEvent()); err != nil {
		cs.logger.Error("failed publishing new round", "err", err)
	}
	// Wait for txs to be available in the mempool
	// before we enterPropose in round 0. If the last block changed the app hash,
	// we may need an empty "proof" block, and enterPropose immediately.
	waitForTxs := cs.config.WaitForTxs() && round == 0 && !cs.needProofBlock(height)
	if waitForTxs {
		if cs.config.CreateEmptyBlocksInterval > 0 {
			cs.scheduleTimeout(cs.config.CreateEmptyBlocksInterval, height, round,
				cstypes.RoundStepNewRound)
		}
		return
	}

	cs.enterPropose(ctx, height, round)
}

// needProofBlock returns true on the first height (so the genesis app hash is signed right away)
// and where the last block (height-1) caused the app hash to change
func (cs *State) needProofBlock(height int64) bool {
	if height == cs.state.InitialHeight {
		return true
	}

	lastBlockMeta := cs.blockStore.LoadBlockMeta(height - 1)
	if lastBlockMeta == nil {
		panic(fmt.Sprintf("needProofBlock: last block meta for height %d not found", height-1))
	}

	return !bytes.Equal(cs.state.AppHash, lastBlockMeta.Header.AppHash)
}

// Enter (CreateEmptyBlocks): from enterNewRound(height,round)
// Enter (CreateEmptyBlocks, CreateEmptyBlocksInterval > 0 ):
//		after enterNewRound(height,round), after timeout of CreateEmptyBlocksInterval
// Enter (!CreateEmptyBlocks) : after enterNewRound(height,round), once txs are in the mempool
func (cs *State) enterPropose(ctx context.Context, height int64, round int32) {
	logger := cs.logger.With("height", height, "round", round)

	if cs.Height != height || round < cs.Round || (cs.Round == round && cstypes.RoundStepPropose <= cs.Step) {
		logger.Debug("entering propose step with invalid args",
			"height", cs.Height,
			"round", cs.Round,
			"step", cs.Step)
		return
	}

	// If this validator is the proposer of this round, and the previous block time is later than
	// our local clock time, wait to propose until our local clock time has passed the block time.
	if cs.privValidatorPubKey != nil && cs.isProposer(cs.privValidatorPubKey.Address()) {
		proposerWaitTime := proposerWaitTime(tmtime.DefaultSource{}, cs.state.LastBlockTime)
		if proposerWaitTime > 0 {
			cs.scheduleTimeout(proposerWaitTime, height, round, cstypes.RoundStepNewRound)
			return
		}
	}

	logger.Debug("entering propose step",
		"height", cs.Height,
		"round", cs.Round,
		"step", cs.Step)

	defer func() {
		// Done enterPropose:
		cs.updateRoundStep(round, cstypes.RoundStepPropose)
		cs.newStep()

		// If we have the whole proposal + POL, then goto Prevote now.
		// else, we'll enterPrevote when the rest of the proposal is received (in AddProposalBlockPart),
		// or else after timeoutPropose
		if cs.isProposalComplete() {
			cs.enterPrevote(ctx, height, cs.Round)
		}
	}()

	// If we don't get the proposal and all block parts quick enough, enterPrevote
	cs.scheduleTimeout(cs.proposeTimeout(round), height, round, cstypes.RoundStepPropose)

	// Nothing more to do if we're not a validator
	if cs.privValidator == nil {
		logger.Debug("propose step; not proposing since node is not a validator")
		return
	}

	if cs.privValidatorPubKey == nil {
		// If this node is a validator & proposer in the current round, it will
		// miss the opportunity to create a block.
		logger.Error("propose step; empty priv validator public key", "err", errPubKeyIsNotSet)
		return
	}

	addr := cs.privValidatorPubKey.Address()

	// if not a validator, we're done
	if !cs.Validators.HasAddress(addr) {
		logger.Debug("propose step; not proposing since node is not in the validator set",
			"addr", addr,
			"vals", cs.Validators)
		return
	}

	if cs.isProposer(addr) {
		logger.Debug("propose step; our turn to propose",
			"proposer", addr)

		cs.decideProposal(ctx, height, round)
	} else {
		logger.Debug("propose step; not our turn to propose",
			"proposer", cs.Validators.GetProposer().Address)
	}
}

func (cs *State) isProposer(address []byte) bool {
	return bytes.Equal(cs.Validators.GetProposer().Address, address)
}

func (cs *State) defaultDecideProposal(ctx context.Context, height int64, round int32) {
	var block *types.Block
	var blockParts *types.PartSet

	// Decide on block
	if cs.ValidBlock != nil {
		// If there is valid block, choose that.
		block, blockParts = cs.ValidBlock, cs.ValidBlockParts
	} else {
		// Create a new proposal block from state/txs from the mempool.
		var err error
		block, err = cs.createProposalBlock(ctx)
		if err != nil {
			cs.logger.Error("unable to create proposal block", "error", err)
			return
		} else if block == nil {
			return
		}
		cs.metrics.ProposalCreateCount.Add(1)
		blockParts, err = block.MakePartSet(types.BlockPartSizeBytes)
		if err != nil {
			cs.logger.Error("unable to create proposal block part set", "error", err)
			return
		}
	}

	// Flush the WAL. Otherwise, we may not recompute the same proposal to sign,
	// and the privValidator will refuse to sign anything.
	if err := cs.wal.FlushAndSync(); err != nil {
		cs.logger.Error("failed flushing WAL to disk")
	}

	// Make proposal
	propBlockID := types.BlockID{Hash: block.Hash(), PartSetHeader: blockParts.Header()}
	proposal := types.NewProposal(height, round, cs.ValidRound, propBlockID, block.Header.Time)
	p := proposal.ToProto()

	// wait the max amount we would wait for a proposal
	ctxto, cancel := context.WithTimeout(ctx, cs.state.ConsensusParams.Timeout.Propose)
	defer cancel()
	if err := cs.privValidator.SignProposal(ctxto, cs.state.ChainID, p); err == nil {
		proposal.Signature = p.Signature

		// send proposal and block parts on internal msg queue
		cs.sendInternalMessage(ctx, msgInfo{&ProposalMessage{proposal}, "", tmtime.Now()})

		for i := 0; i < int(blockParts.Total()); i++ {
			part := blockParts.GetPart(i)
			cs.sendInternalMessage(ctx, msgInfo{&BlockPartMessage{cs.Height, cs.Round, part}, "", tmtime.Now()})
		}

		cs.logger.Debug("signed proposal", "height", height, "round", round, "proposal", proposal)
	} else if !cs.replayMode {
		cs.logger.Error("propose step; failed signing proposal", "height", height, "round", round, "err", err)
	}
}

// Returns true if the proposal block is complete &&
// (if POLRound was proposed, we have +2/3 prevotes from there).
func (cs *State) isProposalComplete() bool {
	if cs.Proposal == nil || cs.ProposalBlock == nil {
		return false
	}
	// we have the proposal. if there's a POLRound,
	// make sure we have the prevotes from it too
	if cs.Proposal.POLRound < 0 {
		return true
	}
	// if this is false the proposer is lying or we haven't received the POL yet
	return cs.Votes.Prevotes(cs.Proposal.POLRound).HasTwoThirdsMajority()

}

// Create the next block to propose and return it. Returns nil block upon error.
//
// We really only need to return the parts, but the block is returned for
// convenience so we can log the proposal block.
//
// NOTE: keep it side-effect free for clarity.
// CONTRACT: cs.privValidator is not nil.
func (cs *State) createProposalBlock(ctx context.Context) (*types.Block, error) {
	if cs.privValidator == nil {
		return nil, errors.New("entered createProposalBlock with privValidator being nil")
	}

	// TODO(sergio): wouldn't it be easier if CreateProposalBlock accepted cs.LastCommit directly?
	var lastExtCommit *types.ExtendedCommit
	switch {
	case cs.Height == cs.state.InitialHeight:
		// We're creating a proposal for the first block.
		// The commit is empty, but not nil.
		lastExtCommit = &types.ExtendedCommit{}

	case cs.LastCommit.HasTwoThirdsMajority():
		// Make the commit from LastCommit
		lastExtCommit = cs.LastCommit.MakeExtendedCommit()

	default: // This shouldn't happen.
		cs.logger.Error("propose step; cannot propose anything without commit for the previous block")
		return nil, nil
	}

	if cs.privValidatorPubKey == nil {
		// If this node is a validator & proposer in the current round, it will
		// miss the opportunity to create a block.
		cs.logger.Error("propose step; empty priv validator public key", "err", errPubKeyIsNotSet)
		return nil, nil
	}

	proposerAddr := cs.privValidatorPubKey.Address()

	ret, err := cs.blockExec.CreateProposalBlock(ctx, cs.Height, cs.state, lastExtCommit, proposerAddr)
	if err != nil {
		panic(err)
	}
	return ret, nil
}

// Enter: `timeoutPropose` after entering Propose.
// Enter: proposal block and POL is ready.
// If we received a valid proposal within this round and we are not locked on a block,
// we will prevote for block.
// Otherwise, if we receive a valid proposal that matches the block we are
// locked on or matches a block that received a POL in a round later than our
// locked round, prevote for the proposal, otherwise vote nil.
func (cs *State) enterPrevote(ctx context.Context, height int64, round int32) {
	logger := cs.logger.With("height", height, "round", round)

	if cs.Height != height || round < cs.Round || (cs.Round == round && cstypes.RoundStepPrevote <= cs.Step) {
		logger.Debug("entering prevote step with invalid args",
			"height", cs.Height,
			"round", cs.Round,
			"step", cs.Step)
		return
	}

	defer func() {
		// Done enterPrevote:
		cs.updateRoundStep(round, cstypes.RoundStepPrevote)
		cs.newStep()
	}()

	logger.Debug("entering prevote step",
		"height", cs.Height,
		"round", cs.Round,
		"step", cs.Step)

	// Sign and broadcast vote as necessary
	cs.doPrevote(ctx, height, round)

	// Once `addVote` hits any +2/3 prevotes, we will go to PrevoteWait
	// (so we have more time to try and collect +2/3 prevotes for a single block)
}

func (cs *State) proposalIsTimely() bool {
	sp := cs.state.ConsensusParams.Synchrony.SynchronyParamsOrDefaults()
	return cs.Proposal.IsTimely(cs.ProposalReceiveTime, sp, cs.Round)
}

func (cs *State) defaultDoPrevote(ctx context.Context, height int64, round int32) {
	logger := cs.logger.With("height", height, "round", round)

	// Check that a proposed block was not received within this round (and thus executing this from a timeout).
	if cs.ProposalBlock == nil {
		logger.Debug("prevote step: ProposalBlock is nil; prevoting nil")
		cs.signAddVote(ctx, tmproto.PrevoteType, nil, types.PartSetHeader{})
		return
	}

	if cs.Proposal == nil {
		logger.Debug("prevote step: did not receive proposal; prevoting nil")
		cs.signAddVote(ctx, tmproto.PrevoteType, nil, types.PartSetHeader{})
		return
	}

	if !cs.Proposal.Timestamp.Equal(cs.ProposalBlock.Header.Time) {
		logger.Debug("prevote step: proposal timestamp not equal; prevoting nil")
		cs.signAddVote(ctx, tmproto.PrevoteType, nil, types.PartSetHeader{})
		return
	}

	sp := cs.state.ConsensusParams.Synchrony.SynchronyParamsOrDefaults()
	//TODO: Remove this temporary fix when the complete solution is ready. See #8739
	if !cs.replayMode && cs.Proposal.POLRound == -1 && cs.LockedRound == -1 && !cs.proposalIsTimely() {
		logger.Debug("prevote step: Proposal is not timely; prevoting nil",
			"proposed", tmtime.Canonical(cs.Proposal.Timestamp).Format(time.RFC3339Nano),
			"received", tmtime.Canonical(cs.ProposalReceiveTime).Format(time.RFC3339Nano),
			"msg_delay", sp.MessageDelay,
			"precision", sp.Precision)
		cs.signAddVote(ctx, tmproto.PrevoteType, nil, types.PartSetHeader{})
		return
	}

	// Validate proposal block, from Tendermint's perspective
	err := cs.blockExec.ValidateBlock(ctx, cs.state, cs.ProposalBlock)
	if err != nil {
		// ProposalBlock is invalid, prevote nil.
		logger.Error("prevote step: consensus deems this block invalid; prevoting nil",
			"err", err)
		cs.signAddVote(ctx, tmproto.PrevoteType, nil, types.PartSetHeader{})
		return
	}

	/*
		The block has now passed Tendermint's validation rules.
		Before prevoting the block received from the proposer for the current round and height,
		we request the Application, via the ProcessProposal, ABCI call to confirm that the block is
		valid. If the Application does not accept the block, Tendermint prevotes nil.

		WARNING: misuse of block rejection by the Application can seriously compromise Tendermint's
		liveness properties. Please see PrepareProposal-ProcessProposal coherence and determinism
		properties in the ABCI++ specification.
	*/
	isAppValid, err := cs.blockExec.ProcessProposal(ctx, cs.ProposalBlock, cs.state)
	if err != nil {
		panic(fmt.Sprintf("ProcessProposal: %v", err))
	}
	cs.metrics.MarkProposalProcessed(isAppValid)

	// Vote nil if the Application rejected the block
	if !isAppValid {
		logger.Error("prevote step: state machine rejected a proposed block; this should not happen:"+
			"the proposer may be misbehaving; prevoting nil", "err", err)
		cs.signAddVote(ctx, tmproto.PrevoteType, nil, types.PartSetHeader{})
		return
	}

	/*
		22: upon <PROPOSAL, h_p, round_p, v, −1> from proposer(h_p, round_p) while step_p = propose do
		23: if valid(v) && (lockedRound_p = −1 || lockedValue_p = v) then
		24: broadcast <PREVOTE, h_p, round_p, id(v)>

		Here, cs.Proposal.POLRound corresponds to the -1 in the above algorithm rule.
		This means that the proposer is producing a new proposal that has not previously
		seen a 2/3 majority by the network.

		If we have already locked on a different value that is different from the proposed value,
		we prevote nil since we are locked on a different value. Otherwise, if we're not locked on a block
		or the proposal matches our locked block, we prevote the proposal.
	*/
	if cs.Proposal.POLRound == -1 {
		if cs.LockedRound == -1 {
			logger.Debug("prevote step: ProposalBlock is valid and there is no locked block; prevoting the proposal")
			cs.signAddVote(ctx, tmproto.PrevoteType, cs.ProposalBlock.Hash(), cs.ProposalBlockParts.Header())
			return
		}
		if cs.ProposalBlock.HashesTo(cs.LockedBlock.Hash()) {
			logger.Debug("prevote step: ProposalBlock is valid and matches our locked block; prevoting the proposal")
			cs.signAddVote(ctx, tmproto.PrevoteType, cs.ProposalBlock.Hash(), cs.ProposalBlockParts.Header())
			return
		}
	}

	/*
		28: upon <PROPOSAL, h_p, round_p, v, v_r> from proposer(h_p, round_p) AND 2f + 1 <PREVOTE, h_p, v_r, id(v)> while
		step_p = propose && (v_r ≥ 0 && v_r < round_p) do
		29: if valid(v) && (lockedRound_p ≤ v_r || lockedValue_p = v) then
		30: broadcast <PREVOTE, h_p, round_p, id(v)>

		This rule is a bit confusing but breaks down as follows:

		If we see a proposal in the current round for value 'v' that lists its valid round as 'v_r'
		AND this validator saw a 2/3 majority of the voting power prevote 'v' in round 'v_r', then we will
		issue a prevote for 'v' in this round if 'v' is valid and either matches our locked value OR
		'v_r' is a round greater than or equal to our current locked round.

		'v_r' can be a round greater than to our current locked round if a 2/3 majority of
		the network prevoted a value in round 'v_r' but we did not lock on it, possibly because we
		missed the proposal in round 'v_r'.
	*/
	blockID, ok := cs.Votes.Prevotes(cs.Proposal.POLRound).TwoThirdsMajority()
	if ok && cs.ProposalBlock.HashesTo(blockID.Hash) && cs.Proposal.POLRound >= 0 && cs.Proposal.POLRound < cs.Round {
		if cs.LockedRound <= cs.Proposal.POLRound {
			logger.Debug("prevote step: ProposalBlock is valid and received a 2/3 majority in a round later than the locked round",
				"outcome", "prevoting the proposal")
			cs.signAddVote(ctx, tmproto.PrevoteType, cs.ProposalBlock.Hash(), cs.ProposalBlockParts.Header())
			return
		}
		if cs.ProposalBlock.HashesTo(cs.LockedBlock.Hash()) {
			logger.Debug("prevote step: ProposalBlock is valid and matches our locked block; prevoting the proposal")
			cs.signAddVote(ctx, tmproto.PrevoteType, cs.ProposalBlock.Hash(), cs.ProposalBlockParts.Header())
			return
		}
	}

	logger.Debug("prevote step: ProposalBlock is valid but was not our locked block or did not receive a more recent majority",
		"outcome", "prevoting nil")
	cs.signAddVote(ctx, tmproto.PrevoteType, nil, types.PartSetHeader{})
}

// Enter: any +2/3 prevotes at next round.
func (cs *State) enterPrevoteWait(height int64, round int32) {
	logger := cs.logger.With("height", height, "round", round)

	if cs.Height != height || round < cs.Round || (cs.Round == round && cstypes.RoundStepPrevoteWait <= cs.Step) {
		logger.Debug("entering prevote wait step with invalid args",
			"height", cs.Height,
			"round", cs.Round,
			"step", cs.Step)
		return
	}

	if !cs.Votes.Prevotes(round).HasTwoThirdsAny() {
		panic(fmt.Sprintf(
			"entering prevote wait step (%v/%v), but prevotes does not have any +2/3 votes",
			height, round,
		))
	}

	logger.Debug("entering prevote wait step",
		"height", cs.Height,
		"round", cs.Round,
		"step", cs.Step)

	defer func() {
		// Done enterPrevoteWait:
		cs.updateRoundStep(round, cstypes.RoundStepPrevoteWait)
		cs.newStep()
	}()

	// Wait for some more prevotes; enterPrecommit
	cs.scheduleTimeout(cs.voteTimeout(round), height, round, cstypes.RoundStepPrevoteWait)
}

// Enter: `timeoutPrevote` after any +2/3 prevotes.
// Enter: `timeoutPrecommit` after any +2/3 precommits.
// Enter: +2/3 precomits for block or nil.
// Lock & precommit the ProposalBlock if we have enough prevotes for it (a POL in this round)
// else, precommit nil otherwise.
func (cs *State) enterPrecommit(ctx context.Context, height int64, round int32) {
	logger := cs.logger.With("new_height", height, "new_round", round)

	if cs.Height != height || round < cs.Round || (cs.Round == round && cstypes.RoundStepPrecommit <= cs.Step) {
		logger.Debug("entering precommit step with invalid args",
			"height", cs.Height,
			"round", cs.Round,
			"step", cs.Step)

		return
	}

	logger.Debug("entering precommit step",
		"height", cs.Height,
		"round", cs.Round,
		"step", cs.Step)

	defer func() {
		// Done enterPrecommit:
		cs.updateRoundStep(round, cstypes.RoundStepPrecommit)
		cs.newStep()
	}()

	// check for a polka
	blockID, ok := cs.Votes.Prevotes(round).TwoThirdsMajority()

	// If we don't have a polka, we must precommit nil.
	if !ok {
		if cs.LockedBlock != nil {
			logger.Debug("precommit step; no +2/3 prevotes during enterPrecommit while we are locked; precommitting nil")
		} else {
			logger.Debug("precommit step; no +2/3 prevotes during enterPrecommit; precommitting nil")
		}

		cs.signAddVote(ctx, tmproto.PrecommitType, nil, types.PartSetHeader{})
		return
	}

	// At this point +2/3 prevoted for a particular block or nil.
	if err := cs.eventBus.PublishEventPolka(cs.RoundStateEvent()); err != nil {
		logger.Error("failed publishing polka", "err", err)
	}

	// the latest POLRound should be this round.
	polRound, _ := cs.Votes.POLInfo()
	if polRound < round {
		panic(fmt.Sprintf("this POLRound should be %v but got %v", round, polRound))
	}

	// +2/3 prevoted nil. Precommit nil.
	if blockID.IsNil() {
		logger.Debug("precommit step: +2/3 prevoted for nil; precommitting nil")
		cs.signAddVote(ctx, tmproto.PrecommitType, nil, types.PartSetHeader{})
		return
	}
	// At this point, +2/3 prevoted for a particular block.

	// If we never received a proposal for this block, we must precommit nil
	if cs.Proposal == nil || cs.ProposalBlock == nil {
		logger.Debug("precommit step; did not receive proposal, precommitting nil")
		cs.signAddVote(ctx, tmproto.PrecommitType, nil, types.PartSetHeader{})
		return
	}

	// If the proposal time does not match the block time, precommit nil.
	if !cs.Proposal.Timestamp.Equal(cs.ProposalBlock.Header.Time) {
		logger.Debug("precommit step: proposal timestamp not equal; precommitting nil")
		cs.signAddVote(ctx, tmproto.PrecommitType, nil, types.PartSetHeader{})
		return
	}

	// If we're already locked on that block, precommit it, and update the LockedRound
	if cs.LockedBlock.HashesTo(blockID.Hash) {
		logger.Debug("precommit step: +2/3 prevoted locked block; relocking")
		cs.LockedRound = round

		if err := cs.eventBus.PublishEventRelock(cs.RoundStateEvent()); err != nil {
			logger.Error("precommit step: failed publishing event relock", "err", err)
		}

		cs.signAddVote(ctx, tmproto.PrecommitType, blockID.Hash, blockID.PartSetHeader)
		return
	}

	// If greater than 2/3 of the voting power on the network prevoted for
	// the proposed block, update our locked block to this block and issue a
	// precommit vote for it.
	if cs.ProposalBlock.HashesTo(blockID.Hash) {
		logger.Debug("precommit step: +2/3 prevoted proposal block; locking", "hash", blockID.Hash)

		// Validate the block.
		if err := cs.blockExec.ValidateBlock(ctx, cs.state, cs.ProposalBlock); err != nil {
			panic(fmt.Sprintf("precommit step: +2/3 prevoted for an invalid block %v; relocking", err))
		}

		cs.LockedRound = round
		cs.LockedBlock = cs.ProposalBlock
		cs.LockedBlockParts = cs.ProposalBlockParts

		if err := cs.eventBus.PublishEventLock(cs.RoundStateEvent()); err != nil {
			logger.Error("precommit step: failed publishing event lock", "err", err)
		}

		cs.signAddVote(ctx, tmproto.PrecommitType, blockID.Hash, blockID.PartSetHeader)
		return
	}

	// There was a polka in this round for a block we don't have.
	// Fetch that block, and precommit nil.
	logger.Debug("precommit step: +2/3 prevotes for a block we do not have; voting nil", "block_id", blockID)

	if !cs.ProposalBlockParts.HasHeader(blockID.PartSetHeader) {
		cs.ProposalBlock = nil
		cs.metrics.MarkBlockGossipStarted()
		cs.ProposalBlockParts = types.NewPartSetFromHeader(blockID.PartSetHeader)
	}

	cs.signAddVote(ctx, tmproto.PrecommitType, nil, types.PartSetHeader{})
}

// Enter: any +2/3 precommits for next round.
func (cs *State) enterPrecommitWait(height int64, round int32) {
	logger := cs.logger.With("new_height", height, "new_round", round)

	if cs.Height != height || round < cs.Round || (cs.Round == round && cs.TriggeredTimeoutPrecommit) {
		logger.Debug("entering precommit wait step with invalid args",
			"triggered_timeout", cs.TriggeredTimeoutPrecommit,
			"height", cs.Height,
			"round", cs.Round)
		return
	}

	if !cs.Votes.Precommits(round).HasTwoThirdsAny() {
		panic(fmt.Sprintf(
			"entering precommit wait step (%v/%v), but precommits does not have any +2/3 votes",
			height, round,
		))
	}

	logger.Debug("entering precommit wait step",
		"height", cs.Height,
		"round", cs.Round,
		"step", cs.Step)

	defer func() {
		// Done enterPrecommitWait:
		cs.TriggeredTimeoutPrecommit = true
		cs.newStep()
	}()

	// wait for some more precommits; enterNewRound
	cs.scheduleTimeout(cs.voteTimeout(round), height, round, cstypes.RoundStepPrecommitWait)
}

// Enter: +2/3 precommits for block
func (cs *State) enterCommit(ctx context.Context, height int64, commitRound int32) {
	logger := cs.logger.With("new_height", height, "commit_round", commitRound)

	if cs.Height != height || cstypes.RoundStepCommit <= cs.Step {
		logger.Debug("entering commit step with invalid args",
			"height", cs.Height,
			"round", cs.Round,
			"step", cs.Step)
		return
	}

	logger.Debug("entering commit step",
		"height", cs.Height,
		"round", cs.Round,
		"step", cs.Step)

	defer func() {
		// Done enterCommit:
		// keep cs.Round the same, commitRound points to the right Precommits set.
		cs.updateRoundStep(cs.Round, cstypes.RoundStepCommit)
		cs.CommitRound = commitRound
		cs.CommitTime = tmtime.Now()
		cs.newStep()

		// Maybe finalize immediately.
		cs.tryFinalizeCommit(ctx, height)
	}()

	blockID, ok := cs.Votes.Precommits(commitRound).TwoThirdsMajority()
	if !ok {
		panic("RunActionCommit() expects +2/3 precommits")
	}

	// The Locked* fields no longer matter.
	// Move them over to ProposalBlock if they match the commit hash,
	// otherwise they'll be cleared in updateToState.
	if cs.LockedBlock.HashesTo(blockID.Hash) {
		logger.Debug("commit is for a locked block; set ProposalBlock=LockedBlock", "block_hash", blockID.Hash)
		cs.ProposalBlock = cs.LockedBlock
		cs.ProposalBlockParts = cs.LockedBlockParts
	}

	// If we don't have the block being committed, set up to get it.
	if !cs.ProposalBlock.HashesTo(blockID.Hash) {
		if !cs.ProposalBlockParts.HasHeader(blockID.PartSetHeader) {
			logger.Info(
				"commit is for a block we do not know about; set ProposalBlock=nil",
				"proposal", cs.ProposalBlock.Hash(),
				"commit", blockID.Hash,
			)

			// We're getting the wrong block.
			// Set up ProposalBlockParts and keep waiting.
			cs.ProposalBlock = nil
			cs.metrics.MarkBlockGossipStarted()
			cs.ProposalBlockParts = types.NewPartSetFromHeader(blockID.PartSetHeader)

			if err := cs.eventBus.PublishEventValidBlock(cs.RoundStateEvent()); err != nil {
				logger.Error("failed publishing valid block", "err", err)
			}

			cs.evsw.FireEvent(types.EventValidBlockValue, &cs.RoundState)
		}
	}
}

// If we have the block AND +2/3 commits for it, finalize.
func (cs *State) tryFinalizeCommit(ctx context.Context, height int64) {
	if cs.Height != height {
		panic(fmt.Sprintf("tryFinalizeCommit() cs.Height: %v vs height: %v", cs.Height, height))
	}

	logger := cs.logger.With("height", height)

	blockID, ok := cs.Votes.Precommits(cs.CommitRound).TwoThirdsMajority()
	if !ok || blockID.IsNil() {
		logger.Error("failed attempt to finalize commit; there was no +2/3 majority or +2/3 was for nil")
		return
	}

	if !cs.ProposalBlock.HashesTo(blockID.Hash) {
		// TODO: this happens every time if we're not a validator (ugly logs)
		// TODO: ^^ wait, why does it matter that we're a validator?
		logger.Debug("failed attempt to finalize commit; we do not have the commit block",
			"proposal_block", tmstrings.LazyBlockHash(cs.ProposalBlock),
			"commit_block", blockID.Hash,
		)
		return
	}

	cs.finalizeCommit(ctx, height)
}

// Increment height and goto cstypes.RoundStepNewHeight
func (cs *State) finalizeCommit(ctx context.Context, height int64) {
	logger := cs.logger.With("height", height)

	if cs.Height != height || cs.Step != cstypes.RoundStepCommit {
		logger.Debug(
			"entering finalize commit step",
			"current", fmt.Sprintf("%v/%v/%v", cs.Height, cs.Round, cs.Step),
		)
		return
	}

	cs.calculatePrevoteMessageDelayMetrics()

	blockID, ok := cs.Votes.Precommits(cs.CommitRound).TwoThirdsMajority()
	block, blockParts := cs.ProposalBlock, cs.ProposalBlockParts

	if !ok {
		panic("cannot finalize commit; commit does not have 2/3 majority")
	}
	if !blockParts.HasHeader(blockID.PartSetHeader) {
		panic("expected ProposalBlockParts header to be commit header")
	}
	if !block.HashesTo(blockID.Hash) {
		panic("cannot finalize commit; proposal block does not hash to commit hash")
	}

	if err := cs.blockExec.ValidateBlock(ctx, cs.state, block); err != nil {
		panic(fmt.Errorf("+2/3 committed an invalid block: %w", err))
	}

	logger.Info(
		"finalizing commit of block",
		"hash", tmstrings.LazyBlockHash(block),
		"root", block.AppHash,
		"num_txs", len(block.Txs),
	)

	// Save to blockStore.
	if cs.blockStore.Height() < block.Height {
		// NOTE: the seenCommit is local justification to commit this block,
		// but may differ from the LastCommit included in the next block
		seenExtendedCommit := cs.Votes.Precommits(cs.CommitRound).MakeExtendedCommit()
		if cs.state.ConsensusParams.ABCI.VoteExtensionsEnabled(block.Height) {
			cs.blockStore.SaveBlockWithExtendedCommit(block, blockParts, seenExtendedCommit)
		} else {
			cs.blockStore.SaveBlock(block, blockParts, seenExtendedCommit.ToCommit())
		}
	} else {
		// Happens during replay if we already saved the block but didn't commit
		logger.Debug("calling finalizeCommit on already stored block", "height", block.Height)
	}

	// Write EndHeightMessage{} for this height, implying that the blockstore
	// has saved the block.
	//
	// If we crash before writing this EndHeightMessage{}, we will recover by
	// running ApplyBlock during the ABCI handshake when we restart.  If we
	// didn't save the block to the blockstore before writing
	// EndHeightMessage{}, we'd have to change WAL replay -- currently it
	// complains about replaying for heights where an #ENDHEIGHT entry already
	// exists.
	//
	// Either way, the State should not be resumed until we
	// successfully call ApplyBlock (ie. later here, or in Handshake after
	// restart).
	endMsg := EndHeightMessage{height}
	if err := cs.wal.WriteSync(endMsg); err != nil { // NOTE: fsync
		panic(fmt.Errorf(
			"failed to write %v msg to consensus WAL due to %w; check your file system and restart the node",
			endMsg, err,
		))
	}

	// Create a copy of the state for staging and an event cache for txs.
	stateCopy := cs.state.Copy()

	// Execute and commit the block, update and save the state, and update the mempool.
	// NOTE The block.AppHash wont reflect these txs until the next block.
	stateCopy, err := cs.blockExec.ApplyBlock(ctx,
		stateCopy,
		types.BlockID{
			Hash:          block.Hash(),
			PartSetHeader: blockParts.Header(),
		},
		block,
	)
	if err != nil {
		logger.Error("failed to apply block", "err", err)
		return
	}

	// must be called before we update state
	cs.RecordMetrics(height, block)

	// NewHeightStep!
	cs.updateToState(stateCopy)

	// Private validator might have changed it's key pair => refetch pubkey.
	if err := cs.updatePrivValidatorPubKey(ctx); err != nil {
		logger.Error("failed to get private validator pubkey", "err", err)
	}

	// cs.StartTime is already set.
	// Schedule Round0 to start soon.
	cs.scheduleRound0(&cs.RoundState)

	// By here,
	// * cs.Height has been increment to height+1
	// * cs.Step is now cstypes.RoundStepNewHeight
	// * cs.StartTime is set to when we will start round0.
}

func (cs *State) RecordMetrics(height int64, block *types.Block) {
	cs.metrics.Validators.Set(float64(cs.Validators.Size()))
	cs.metrics.ValidatorsPower.Set(float64(cs.Validators.TotalVotingPower()))

	var (
		missingValidators      int
		missingValidatorsPower int64
	)
	// height=0 -> MissingValidators and MissingValidatorsPower are both 0.
	// Remember that the first LastCommit is intentionally empty, so it's not
	// fair to increment missing validators number.
	if height > cs.state.InitialHeight {
		// Sanity check that commit size matches validator set size - only applies
		// after first block.
		var (
			commitSize = block.LastCommit.Size()
			valSetLen  = len(cs.LastValidators.Validators)
			address    types.Address
		)
		if commitSize != valSetLen {
			cs.logger.Error("commit size doesn't match valset",
				"size", commitSize,
				"valset_len", valSetLen,
				"height", block.Height,
				"extra", tmstrings.LazySprintf("\n%v\n\n%v", block.LastCommit.Signatures, cs.LastValidators.Validators))
			return
		}

		if cs.privValidator != nil {
			if cs.privValidatorPubKey == nil {
				// Metrics won't be updated, but it's not critical.
				cs.logger.Error("recordMetrics", "err", errPubKeyIsNotSet)
			} else {
				address = cs.privValidatorPubKey.Address()
			}
		}

		for i, val := range cs.LastValidators.Validators {
			commitSig := block.LastCommit.Signatures[i]
			if commitSig.BlockIDFlag == types.BlockIDFlagAbsent {
				missingValidators++
				missingValidatorsPower += val.VotingPower
			}

			if bytes.Equal(val.Address, address) {
				label := []string{
					"validator_address", val.Address.String(),
				}
				cs.metrics.ValidatorPower.With(label...).Set(float64(val.VotingPower))
				if commitSig.BlockIDFlag == types.BlockIDFlagCommit {
					cs.metrics.ValidatorLastSignedHeight.With(label...).Set(float64(height))
				} else {
					cs.metrics.ValidatorMissedBlocks.With(label...).Add(float64(1))
				}
			}

		}
	}
	cs.metrics.MissingValidators.Set(float64(missingValidators))
	cs.metrics.MissingValidatorsPower.Set(float64(missingValidatorsPower))

	// NOTE: byzantine validators power and count is only for consensus evidence i.e. duplicate vote
	var (
		byzantineValidatorsPower int64
		byzantineValidatorsCount int64
	)

	for _, ev := range block.Evidence {
		if dve, ok := ev.(*types.DuplicateVoteEvidence); ok {
			if _, val := cs.Validators.GetByAddress(dve.VoteA.ValidatorAddress); val != nil {
				byzantineValidatorsCount++
				byzantineValidatorsPower += val.VotingPower
			}
		}
	}
	cs.metrics.ByzantineValidators.Set(float64(byzantineValidatorsCount))
	cs.metrics.ByzantineValidatorsPower.Set(float64(byzantineValidatorsPower))

	if height > 1 {
		lastBlockMeta := cs.blockStore.LoadBlockMeta(height - 1)
		if lastBlockMeta != nil {
			cs.metrics.BlockIntervalSeconds.Observe(
				block.Time.Sub(lastBlockMeta.Header.Time).Seconds(),
			)
		}
	}

	cs.metrics.NumTxs.Set(float64(len(block.Data.Txs)))
	cs.metrics.TotalTxs.Add(float64(len(block.Data.Txs)))
	cs.metrics.BlockSizeBytes.Observe(float64(block.Size()))
	cs.metrics.CommittedHeight.Set(float64(block.Height))
}

//-----------------------------------------------------------------------------

func (cs *State) defaultSetProposal(proposal *types.Proposal, recvTime time.Time) error {
	// Already have one
	// TODO: possibly catch double proposals
	if cs.Proposal != nil || proposal == nil {
		return nil
	}

	// Does not apply
	if proposal.Height != cs.Height || proposal.Round != cs.Round {
		return nil
	}

	// Verify POLRound, which must be -1 or in range [0, proposal.Round).
	if proposal.POLRound < -1 ||
		(proposal.POLRound >= 0 && proposal.POLRound >= proposal.Round) {
		return ErrInvalidProposalPOLRound
	}

	p := proposal.ToProto()
	// Verify signature
	if !cs.Validators.GetProposer().PubKey.VerifySignature(
		types.ProposalSignBytes(cs.state.ChainID, p), proposal.Signature,
	) {
		return ErrInvalidProposalSignature
	}

	proposal.Signature = p.Signature
	cs.Proposal = proposal
	cs.ProposalReceiveTime = recvTime
	cs.calculateProposalTimestampDifferenceMetric()
	// We don't update cs.ProposalBlockParts if it is already set.
	// This happens if we're already in cstypes.RoundStepCommit or if there is a valid block in the current round.
	// TODO: We can check if Proposal is for a different block as this is a sign of misbehavior!
	if cs.ProposalBlockParts == nil {
		cs.metrics.MarkBlockGossipStarted()
		cs.ProposalBlockParts = types.NewPartSetFromHeader(proposal.BlockID.PartSetHeader)
	}

	cs.logger.Info("received proposal", "proposal", proposal)
	return nil
}

// NOTE: block is not necessarily valid.
// Asynchronously triggers either enterPrevote (before we timeout of propose) or tryFinalizeCommit,
// once we have the full block.
func (cs *State) addProposalBlockPart(
	msg *BlockPartMessage,
	peerID types.NodeID,
) (added bool, err error) {
	height, round, part := msg.Height, msg.Round, msg.Part

	// Blocks might be reused, so round mismatch is OK
	if cs.Height != height {
		cs.logger.Debug("received block part from wrong height", "height", height, "round", round)
		cs.metrics.BlockGossipPartsReceived.With("matches_current", "false").Add(1)
		return false, nil
	}

	// We're not expecting a block part.
	if cs.ProposalBlockParts == nil {
		cs.metrics.BlockGossipPartsReceived.With("matches_current", "false").Add(1)
		// NOTE: this can happen when we've gone to a higher round and
		// then receive parts from the previous round - not necessarily a bad peer.
		cs.logger.Debug("received a block part when we are not expecting any",
			"height", height,
			"round", round,
			"index", part.Index,
			"peer", peerID,
		)
		return false, nil
	}

	added, err = cs.ProposalBlockParts.AddPart(part)
	if err != nil {
		if errors.Is(err, types.ErrPartSetInvalidProof) || errors.Is(err, types.ErrPartSetUnexpectedIndex) {
			cs.metrics.BlockGossipPartsReceived.With("matches_current", "false").Add(1)
		}
		return added, err
	}

	cs.metrics.BlockGossipPartsReceived.With("matches_current", "true").Add(1)

	if cs.ProposalBlockParts.ByteSize() > cs.state.ConsensusParams.Block.MaxBytes {
		return added, fmt.Errorf("total size of proposal block parts exceeds maximum block bytes (%d > %d)",
			cs.ProposalBlockParts.ByteSize(), cs.state.ConsensusParams.Block.MaxBytes,
		)
	}
	if added && cs.ProposalBlockParts.IsComplete() {
		cs.metrics.MarkBlockGossipComplete()
		bz, err := io.ReadAll(cs.ProposalBlockParts.GetReader())
		if err != nil {
			return added, err
		}

		var pbb = new(tmproto.Block)
		err = proto.Unmarshal(bz, pbb)
		if err != nil {
			return added, err
		}

		block, err := types.BlockFromProto(pbb)
		if err != nil {
			return added, err
		}

		cs.ProposalBlock = block

		// NOTE: it's possible to receive complete proposal blocks for future rounds without having the proposal
		cs.logger.Info("received complete proposal block", "height", cs.ProposalBlock.Height, "hash", cs.ProposalBlock.Hash())

		if err := cs.eventBus.PublishEventCompleteProposal(cs.CompleteProposalEvent()); err != nil {
			cs.logger.Error("failed publishing event complete proposal", "err", err)
		}
	}

	return added, nil
}
func (cs *State) handleCompleteProposal(ctx context.Context, height int64) {
	// Update Valid* if we can.
	prevotes := cs.Votes.Prevotes(cs.Round)
	blockID, hasTwoThirds := prevotes.TwoThirdsMajority()
	if hasTwoThirds && !blockID.IsNil() && (cs.ValidRound < cs.Round) {
		if cs.ProposalBlock.HashesTo(blockID.Hash) {
			cs.logger.Debug("updating valid block to new proposal block",
				"valid_round", cs.Round,
				"valid_block_hash", tmstrings.LazyBlockHash(cs.ProposalBlock))

			cs.ValidRound = cs.Round
			cs.ValidBlock = cs.ProposalBlock
			cs.ValidBlockParts = cs.ProposalBlockParts
		}
		// TODO: In case there is +2/3 majority in Prevotes set for some
		// block and cs.ProposalBlock contains different block, either
		// proposer is faulty or voting power of faulty processes is more
		// than 1/3. We should trigger in the future accountability
		// procedure at this point.
	}

	if cs.Step <= cstypes.RoundStepPropose && cs.isProposalComplete() {
		// Move onto the next step
		cs.enterPrevote(ctx, height, cs.Round)
		if hasTwoThirds { // this is optimisation as this will be triggered when prevote is added
			cs.enterPrecommit(ctx, height, cs.Round)
		}
	} else if cs.Step == cstypes.RoundStepCommit {
		// If we're waiting on the proposal block...
		cs.tryFinalizeCommit(ctx, height)
	}
}

// Attempt to add the vote. if its a duplicate signature, dupeout the validator
func (cs *State) tryAddVote(ctx context.Context, vote *types.Vote, peerID types.NodeID) (bool, error) {
	added, err := cs.addVote(ctx, vote, peerID)
	if err != nil {
		// If the vote height is off, we'll just ignore it,
		// But if it's a conflicting sig, add it to the cs.evpool.
		// If it's otherwise invalid, punish peer.
		// nolint: gocritic
		if voteErr, ok := err.(*types.ErrVoteConflictingVotes); ok {
			if cs.privValidatorPubKey == nil {
				return false, errPubKeyIsNotSet
			}

			if bytes.Equal(vote.ValidatorAddress, cs.privValidatorPubKey.Address()) {
				cs.logger.Error("found conflicting vote from ourselves; did you unsafe_reset a validator?",
					"height", vote.Height,
					"round", vote.Round,
					"type", vote.Type)

				return added, err
			}

			// report conflicting votes to the evidence pool
			cs.evpool.ReportConflictingVotes(voteErr.VoteA, voteErr.VoteB)
			cs.logger.Debug("found and sent conflicting votes to the evidence pool",
				"vote_a", voteErr.VoteA,
				"vote_b", voteErr.VoteB)

			return added, err
		} else if errors.Is(err, types.ErrVoteNonDeterministicSignature) {
			cs.logger.Debug("vote has non-deterministic signature", "err", err)
		} else {
			// Either
			// 1) bad peer OR
			// 2) not a bad peer? this can also err sometimes with "Unexpected step" OR
			// 3) tmkms use with multiple validators connecting to a single tmkms instance
			//		(https://github.com/tendermint/tendermint/issues/3839).
			cs.logger.Info("failed attempting to add vote", "err", err)
			return added, ErrAddingVote
		}
	}

	return added, nil
}

func (cs *State) addVote(
	ctx context.Context,
	vote *types.Vote,
	peerID types.NodeID,
) (added bool, err error) {
	cs.logger.Debug("adding vote",
		"vote_height", vote.Height,
		"vote_type", vote.Type,
		"val_index", vote.ValidatorIndex,
		"cs_height", cs.Height)

	if vote.Height < cs.Height || (vote.Height == cs.Height && vote.Round < cs.Round) {
		cs.metrics.MarkLateVote(vote.Type)
	}

	// A precommit for the previous height?
	// These come in while we wait timeoutCommit
	if vote.Height+1 == cs.Height && vote.Type == tmproto.PrecommitType {
		if cs.Step != cstypes.RoundStepNewHeight {
			// Late precommit at prior height is ignored
			cs.logger.Debug("precommit vote came in after commit timeout and has been ignored", "vote", vote)
			return
		}

		added, err = cs.LastCommit.AddVote(vote)
		if !added {
			return
		}

		cs.logger.Debug("added vote to last precommits", "last_commit", cs.LastCommit.StringShort())
		if err := cs.eventBus.PublishEventVote(types.EventDataVote{Vote: vote}); err != nil {
			return added, err
		}

		cs.evsw.FireEvent(types.EventVoteValue, vote)

		// if we can skip timeoutCommit and have all the votes now,
		if cs.bypassCommitTimeout() && cs.LastCommit.HasAll() {
			// go straight to new round (skip timeout commit)
			// cs.scheduleTimeout(time.Duration(0), cs.Height, 0, cstypes.RoundStepNewHeight)
			cs.enterNewRound(ctx, cs.Height, 0)
		}

		return
	}

	// Height mismatch is ignored.
	// Not necessarily a bad peer, but not favorable behavior.
	if vote.Height != cs.Height {
		cs.logger.Debug("vote ignored and not added", "vote_height", vote.Height, "cs_height", cs.Height, "peer", peerID)
		return
	}

	// Check to see if the chain is configured to extend votes.
	if cs.state.ConsensusParams.ABCI.VoteExtensionsEnabled(cs.Height) {
		// The chain is configured to extend votes, check that the vote is
		// not for a nil block and verify the extensions signature against the
		// corresponding public key.

		var myAddr []byte
		if cs.privValidatorPubKey != nil {
			myAddr = cs.privValidatorPubKey.Address()
		}
		// Verify VoteExtension if precommit and not nil
		// https://github.com/tendermint/tendermint/issues/8487
		if vote.Type == tmproto.PrecommitType && !vote.BlockID.IsNil() &&
			!bytes.Equal(vote.ValidatorAddress, myAddr) { // Skip the VerifyVoteExtension call if the vote was issued by this validator.

			// The core fields of the vote message were already validated in the
			// consensus reactor when the vote was received.
			// Here, we verify the signature of the vote extension included in the vote
			// message.
			_, val := cs.state.Validators.GetByIndex(vote.ValidatorIndex)
			if err := vote.VerifyExtension(cs.state.ChainID, val.PubKey); err != nil {
				return false, err
			}

			err := cs.blockExec.VerifyVoteExtension(ctx, vote)
			cs.metrics.MarkVoteExtensionReceived(err == nil)
			if err != nil {
				return false, err
			}
		}
	} else {
		// Vote extensions are not enabled on the network.
		// strip the extension data from the vote in case any is present.
		//
		// TODO punish a peer if it sent a vote with an extension when the feature
		// is disabled on the network.
		// https://github.com/tendermint/tendermint/issues/8565
		if stripped := vote.StripExtension(); stripped {
			cs.logger.Error("vote included extension data but vote extensions are not enabled", "peer", peerID)
		}
	}

	height := cs.Height
	added, err = cs.Votes.AddVote(vote, peerID)
	if !added {
		// Either duplicate, or error upon cs.Votes.AddByIndex()
		return
	}
	if vote.Round == cs.Round {
		vals := cs.state.Validators
		_, val := vals.GetByIndex(vote.ValidatorIndex)
		cs.metrics.MarkVoteReceived(vote.Type, val.VotingPower, vals.TotalVotingPower())
	}

	if err := cs.eventBus.PublishEventVote(types.EventDataVote{Vote: vote}); err != nil {
		return added, err
	}
	cs.evsw.FireEvent(types.EventVoteValue, vote)

	switch vote.Type {
	case tmproto.PrevoteType:
		prevotes := cs.Votes.Prevotes(vote.Round)
		cs.logger.Debug("added vote to prevote", "vote", vote, "prevotes", prevotes.StringShort())

		// Check to see if >2/3 of the voting power on the network voted for any non-nil block.
		if blockID, ok := prevotes.TwoThirdsMajority(); ok && !blockID.IsNil() {
			// Greater than 2/3 of the voting power on the network voted for some
			// non-nil block

			// Update Valid* if we can.
			if cs.ValidRound < vote.Round && vote.Round == cs.Round {
				if cs.ProposalBlock.HashesTo(blockID.Hash) {
					cs.logger.Debug("updating valid block because of POL", "valid_round", cs.ValidRound, "pol_round", vote.Round)
					cs.ValidRound = vote.Round
					cs.ValidBlock = cs.ProposalBlock
					cs.ValidBlockParts = cs.ProposalBlockParts
				} else {
					cs.logger.Debug("valid block we do not know about; set ProposalBlock=nil",
						"proposal", tmstrings.LazyBlockHash(cs.ProposalBlock),
						"block_id", blockID.Hash)

					// we're getting the wrong block
					cs.ProposalBlock = nil
				}

				if !cs.ProposalBlockParts.HasHeader(blockID.PartSetHeader) {
					cs.metrics.MarkBlockGossipStarted()
					cs.ProposalBlockParts = types.NewPartSetFromHeader(blockID.PartSetHeader)
				}

				cs.evsw.FireEvent(types.EventValidBlockValue, &cs.RoundState)
				if err := cs.eventBus.PublishEventValidBlock(cs.RoundStateEvent()); err != nil {
					return added, err
				}
			}
		}

		// If +2/3 prevotes for *anything* for future round:
		switch {
		case cs.Round < vote.Round && prevotes.HasTwoThirdsAny():
			// Round-skip if there is any 2/3+ of votes ahead of us
			cs.enterNewRound(ctx, height, vote.Round)

		case cs.Round == vote.Round && cstypes.RoundStepPrevote <= cs.Step: // current round
			blockID, ok := prevotes.TwoThirdsMajority()
			if ok && (cs.isProposalComplete() || blockID.IsNil()) {
				cs.enterPrecommit(ctx, height, vote.Round)
			} else if prevotes.HasTwoThirdsAny() {
				cs.enterPrevoteWait(height, vote.Round)
			}

		case cs.Proposal != nil && 0 <= cs.Proposal.POLRound && cs.Proposal.POLRound == vote.Round:
			// If the proposal is now complete, enter prevote of cs.Round.
			if cs.isProposalComplete() {
				cs.enterPrevote(ctx, height, cs.Round)
			}
		}

	case tmproto.PrecommitType:
		precommits := cs.Votes.Precommits(vote.Round)
		cs.logger.Debug("added vote to precommit",
			"height", vote.Height,
			"round", vote.Round,
			"validator", vote.ValidatorAddress.String(),
			"vote_timestamp", vote.Timestamp,
			"data", precommits.LogString())

		blockID, ok := precommits.TwoThirdsMajority()
		if ok {
			// Executed as TwoThirdsMajority could be from a higher round
			cs.enterNewRound(ctx, height, vote.Round)
			cs.enterPrecommit(ctx, height, vote.Round)

			if !blockID.IsNil() {
				cs.enterCommit(ctx, height, vote.Round)
				if cs.bypassCommitTimeout() && precommits.HasAll() {
					cs.enterNewRound(ctx, cs.Height, 0)
				}
			} else {
				cs.enterPrecommitWait(height, vote.Round)
			}
		} else if cs.Round <= vote.Round && precommits.HasTwoThirdsAny() {
			cs.enterNewRound(ctx, height, vote.Round)
			cs.enterPrecommitWait(height, vote.Round)
		}

	default:
		panic(fmt.Sprintf("unexpected vote type %v", vote.Type))
	}

	return added, err
}

// CONTRACT: cs.privValidator is not nil.
func (cs *State) signVote(
	ctx context.Context,
	msgType tmproto.SignedMsgType,
	hash []byte,
	header types.PartSetHeader,
) (*types.Vote, error) {
	// Flush the WAL. Otherwise, we may not recompute the same vote to sign,
	// and the privValidator will refuse to sign anything.
	if err := cs.wal.FlushAndSync(); err != nil {
		return nil, err
	}

	if cs.privValidatorPubKey == nil {
		return nil, errPubKeyIsNotSet
	}

	addr := cs.privValidatorPubKey.Address()
	valIdx, _ := cs.Validators.GetByAddress(addr)

	vote := &types.Vote{
		ValidatorAddress: addr,
		ValidatorIndex:   valIdx,
		Height:           cs.Height,
		Round:            cs.Round,
		Timestamp:        tmtime.Now(),
		Type:             msgType,
		BlockID:          types.BlockID{Hash: hash, PartSetHeader: header},
	}

	// If the signedMessageType is for precommit,
	// use our local precommit Timeout as the max wait time for getting a singed commit. The same goes for prevote.
	timeout := time.Second
	if msgType == tmproto.PrecommitType && !vote.BlockID.IsNil() {
		timeout = cs.voteTimeout(cs.Round)
		// if the signedMessage type is for a non-nil precommit, add
		// VoteExtension
		if cs.state.ConsensusParams.ABCI.VoteExtensionsEnabled(cs.Height) {
			ext, err := cs.blockExec.ExtendVote(ctx, vote)
			if err != nil {
				return nil, err
			}
			vote.Extension = ext
		}
	}

	v := vote.ToProto()

	ctxto, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	err := cs.privValidator.SignVote(ctxto, cs.state.ChainID, v)
	vote.Signature = v.Signature
	vote.ExtensionSignature = v.ExtensionSignature
	vote.Timestamp = v.Timestamp

	return vote, err
}

// sign the vote and publish on internalMsgQueue
func (cs *State) signAddVote(
	ctx context.Context,
	msgType tmproto.SignedMsgType,
	hash []byte,
	header types.PartSetHeader,
) *types.Vote {
	if cs.privValidator == nil { // the node does not have a key
		return nil
	}

	if cs.privValidatorPubKey == nil {
		// Vote won't be signed, but it's not critical.
		cs.logger.Error("signAddVote", "err", errPubKeyIsNotSet)
		return nil
	}

	// If the node not in the validator set, do nothing.
	if !cs.Validators.HasAddress(cs.privValidatorPubKey.Address()) {
		return nil
	}

	// TODO: pass pubKey to signVote
	vote, err := cs.signVote(ctx, msgType, hash, header)
	if err != nil {
		cs.logger.Error("failed signing vote", "height", cs.Height, "round", cs.Round, "vote", vote, "err", err)
		return nil
	}
	if !cs.state.ConsensusParams.ABCI.VoteExtensionsEnabled(vote.Height) {
		// The signer will sign the extension, make sure to remove the data on the way out
		vote.StripExtension()
	}
	cs.sendInternalMessage(ctx, msgInfo{&VoteMessage{vote}, "", tmtime.Now()})
	cs.logger.Debug("signed and pushed vote", "height", cs.Height, "round", cs.Round, "vote", vote)
	return vote
}

// updatePrivValidatorPubKey get's the private validator public key and
// memoizes it. This func returns an error if the private validator is not
// responding or responds with an error.
func (cs *State) updatePrivValidatorPubKey(rctx context.Context) error {
	if cs.privValidator == nil {
		return nil
	}

	timeout := cs.voteTimeout(cs.Round)

	// no GetPubKey retry beyond the proposal/voting in RetrySignerClient
	if cs.Step >= cstypes.RoundStepPrecommit && cs.privValidatorType == types.RetrySignerClient {
		timeout = 0
	}

	// set context timeout depending on the configuration and the State step,
	// this helps in avoiding blocking of the remote signer connection.
	ctxto, cancel := context.WithTimeout(rctx, timeout)
	defer cancel()
	pubKey, err := cs.privValidator.GetPubKey(ctxto)
	if err != nil {
		return err
	}
	cs.privValidatorPubKey = pubKey
	return nil
}

// look back to check existence of the node's consensus votes before joining consensus
func (cs *State) checkDoubleSigningRisk(height int64) error {
	if cs.privValidator != nil && cs.privValidatorPubKey != nil && cs.config.DoubleSignCheckHeight > 0 && height > 0 {
		valAddr := cs.privValidatorPubKey.Address()
		doubleSignCheckHeight := cs.config.DoubleSignCheckHeight
		if doubleSignCheckHeight > height {
			doubleSignCheckHeight = height
		}

		for i := int64(1); i < doubleSignCheckHeight; i++ {
			lastCommit := cs.LoadCommit(height - i)
			if lastCommit != nil {
				for sigIdx, s := range lastCommit.Signatures {
					if s.BlockIDFlag == types.BlockIDFlagCommit && bytes.Equal(s.ValidatorAddress, valAddr) {
						cs.logger.Info("found signature from the same key", "sig", s, "idx", sigIdx, "height", height-i)
						return ErrSignatureFoundInPastBlocks
					}
				}
			}
		}
	}

	return nil
}

func (cs *State) calculatePrevoteMessageDelayMetrics() {
	if cs.Proposal == nil {
		return
	}
	ps := cs.Votes.Prevotes(cs.Round)
	pl := ps.List()

	sort.Slice(pl, func(i, j int) bool {
		return pl[i].Timestamp.Before(pl[j].Timestamp)
	})

	var votingPowerSeen int64
	for _, v := range pl {
		_, val := cs.Validators.GetByAddress(v.ValidatorAddress)
		votingPowerSeen += val.VotingPower
		if votingPowerSeen >= cs.Validators.TotalVotingPower()*2/3+1 {
			cs.metrics.QuorumPrevoteDelay.With("proposer_address", cs.Validators.GetProposer().Address.String()).Set(v.Timestamp.Sub(cs.Proposal.Timestamp).Seconds())
			break
		}
	}
	if ps.HasAll() {
		cs.metrics.FullPrevoteDelay.With("proposer_address", cs.Validators.GetProposer().Address.String()).Set(pl[len(pl)-1].Timestamp.Sub(cs.Proposal.Timestamp).Seconds())
	}
}

//---------------------------------------------------------

func CompareHRS(h1 int64, r1 int32, s1 cstypes.RoundStepType, h2 int64, r2 int32, s2 cstypes.RoundStepType) int {
	if h1 < h2 {
		return -1
	} else if h1 > h2 {
		return 1
	}
	if r1 < r2 {
		return -1
	} else if r1 > r2 {
		return 1
	}
	if s1 < s2 {
		return -1
	} else if s1 > s2 {
		return 1
	}
	return 0
}

// repairWalFile decodes messages from src (until the decoder errors) and
// writes them to dst.
func repairWalFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	var (
		dec = NewWALDecoder(in)
		enc = NewWALEncoder(out)
	)

	// best-case repair (until first error is encountered)
	for {
		msg, err := dec.Decode()
		if err != nil {
			break
		}

		err = enc.Encode(msg)
		if err != nil {
			return fmt.Errorf("failed to encode msg: %w", err)
		}
	}

	return nil
}

func (cs *State) proposeTimeout(round int32) time.Duration {
	tp := cs.state.ConsensusParams.Timeout.TimeoutParamsOrDefaults()
	p := tp.Propose
	if cs.config.UnsafeProposeTimeoutOverride != 0 {
		p = cs.config.UnsafeProposeTimeoutOverride
	}
	pd := tp.ProposeDelta
	if cs.config.UnsafeProposeTimeoutDeltaOverride != 0 {
		pd = cs.config.UnsafeProposeTimeoutDeltaOverride
	}
	return time.Duration(
		p.Nanoseconds()+pd.Nanoseconds()*int64(round),
	) * time.Nanosecond
}

func (cs *State) voteTimeout(round int32) time.Duration {
	tp := cs.state.ConsensusParams.Timeout.TimeoutParamsOrDefaults()
	v := tp.Vote
	if cs.config.UnsafeVoteTimeoutOverride != 0 {
		v = cs.config.UnsafeVoteTimeoutOverride
	}
	vd := tp.VoteDelta
	if cs.config.UnsafeVoteTimeoutDeltaOverride != 0 {
		vd = cs.config.UnsafeVoteTimeoutDeltaOverride
	}
	return time.Duration(
		v.Nanoseconds()+vd.Nanoseconds()*int64(round),
	) * time.Nanosecond
}

func (cs *State) commitTime(t time.Time) time.Time {
	c := cs.state.ConsensusParams.Timeout.Commit
	if cs.config.UnsafeCommitTimeoutOverride != 0 {
		c = cs.config.UnsafeProposeTimeoutOverride
	}
	return t.Add(c)
}

func (cs *State) bypassCommitTimeout() bool {
	if cs.config.UnsafeBypassCommitTimeoutOverride != nil {
		return *cs.config.UnsafeBypassCommitTimeoutOverride
	}
	return cs.state.ConsensusParams.Timeout.BypassCommitTimeout
}

func (cs *State) calculateProposalTimestampDifferenceMetric() {
	if cs.Proposal != nil && cs.Proposal.POLRound == -1 {
		sp := cs.state.ConsensusParams.Synchrony.SynchronyParamsOrDefaults()
		isTimely := cs.Proposal.IsTimely(cs.ProposalReceiveTime, sp, cs.Round)
		cs.metrics.ProposalTimestampDifference.With("is_timely", fmt.Sprintf("%t", isTimely)).
			Observe(cs.ProposalReceiveTime.Sub(cs.Proposal.Timestamp).Seconds())
	}
}

// proposerWaitTime determines how long the proposer should wait to propose its next block.
// If the result is zero, a block can be proposed immediately.
//
// Block times must be monotonically increasing, so if the block time of the previous
// block is larger than the proposer's current time, then the proposer will sleep
// until its local clock exceeds the previous block time.
func proposerWaitTime(lt tmtime.Source, bt time.Time) time.Duration {
	t := lt.Now()
	if bt.After(t) {
		return bt.Sub(t)
	}
	return 0
}
