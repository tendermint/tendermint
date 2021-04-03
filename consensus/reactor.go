package consensus

import (
	"fmt"
	"time"

	cstypes "github.com/tendermint/tendermint/consensus/types"
	"github.com/tendermint/tendermint/libs/bits"
	tmevents "github.com/tendermint/tendermint/libs/events"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	tmsync "github.com/tendermint/tendermint/libs/sync"
	"github.com/tendermint/tendermint/p2p"
	tmcons "github.com/tendermint/tendermint/proto/tendermint/consensus"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

var (
	_ service.Service = (*Reactor)(nil)
	_ p2p.Wrapper     = (*tmcons.Message)(nil)

	// ChannelShims contains a map of ChannelDescriptorShim objects, where each
	// object wraps a reference to a legacy p2p ChannelDescriptor and the corresponding
	// p2p proto.Message the new p2p Channel is responsible for handling.
	//
	//
	// TODO: Remove once p2p refactor is complete.
	// ref: https://github.com/tendermint/tendermint/issues/5670
	ChannelShims = map[p2p.ChannelID]*p2p.ChannelDescriptorShim{
		StateChannel: {
			MsgType: new(tmcons.Message),
			Descriptor: &p2p.ChannelDescriptor{
				ID:                  byte(StateChannel),
				Priority:            6,
				SendQueueCapacity:   100,
				RecvMessageCapacity: maxMsgSize,

				MaxSendBytes: 12000,
			},
		},
		DataChannel: {
			MsgType: new(tmcons.Message),
			Descriptor: &p2p.ChannelDescriptor{
				// TODO: Consider a split between gossiping current block and catchup
				// stuff. Once we gossip the whole block there is nothing left to send
				// until next height or round.
				ID:                  byte(DataChannel),
				Priority:            10,
				SendQueueCapacity:   100,
				RecvBufferCapacity:  50 * 4096,
				RecvMessageCapacity: maxMsgSize,

				MaxSendBytes: 40000,
			},
		},
		VoteChannel: {
			MsgType: new(tmcons.Message),
			Descriptor: &p2p.ChannelDescriptor{
				ID:                  byte(VoteChannel),
				Priority:            7,
				SendQueueCapacity:   100,
				RecvBufferCapacity:  100 * 100,
				RecvMessageCapacity: maxMsgSize,

				MaxSendBytes: 150,
			},
		},
		VoteSetBitsChannel: {
			MsgType: new(tmcons.Message),
			Descriptor: &p2p.ChannelDescriptor{
				ID:                  byte(VoteSetBitsChannel),
				Priority:            1,
				SendQueueCapacity:   2,
				RecvBufferCapacity:  1024,
				RecvMessageCapacity: maxMsgSize,

				MaxSendBytes: 50,
			},
		},
	}
)

const (
	StateChannel       = p2p.ChannelID(0x20)
	DataChannel        = p2p.ChannelID(0x21)
	VoteChannel        = p2p.ChannelID(0x22)
	VoteSetBitsChannel = p2p.ChannelID(0x23)

	maxMsgSize = 1048576 // 1MB; NOTE: keep in sync with types.PartSet sizes.

	blocksToContributeToBecomeGoodPeer = 10000
	votesToContributeToBecomeGoodPeer  = 10000

	listenerIDConsensus = "consensus-reactor"
)

type ReactorOption func(*Reactor)

// Reactor defines a reactor for the consensus service.
type Reactor struct {
	service.BaseService

	state    *State
	eventBus *types.EventBus
	Metrics  *Metrics

	mtx      tmsync.RWMutex
	peers    map[p2p.NodeID]*PeerState
	waitSync bool

	stateCh       *p2p.Channel
	dataCh        *p2p.Channel
	voteCh        *p2p.Channel
	voteSetBitsCh *p2p.Channel
	peerUpdates   *p2p.PeerUpdates

	// NOTE: We need a dedicated stateCloseCh channel for signaling closure of
	// the StateChannel due to the fact that the StateChannel message handler
	// performs a send on the VoteSetBitsChannel. This is an antipattern, so having
	// this dedicated channel,stateCloseCh, is necessary in order to avoid data races.
	stateCloseCh chan struct{}
	closeCh      chan struct{}
}

// NewReactor returns a reference to a new consensus reactor, which implements
// the service.Service interface. It accepts a logger, consensus state, references
// to relevant p2p Channels and a channel to listen for peer updates on. The
// reactor will close all p2p Channels when stopping.
func NewReactor(
	logger log.Logger,
	cs *State,
	stateCh *p2p.Channel,
	dataCh *p2p.Channel,
	voteCh *p2p.Channel,
	voteSetBitsCh *p2p.Channel,
	peerUpdates *p2p.PeerUpdates,
	waitSync bool,
	options ...ReactorOption,
) *Reactor {

	r := &Reactor{
		state:         cs,
		waitSync:      waitSync,
		peers:         make(map[p2p.NodeID]*PeerState),
		Metrics:       NopMetrics(),
		stateCh:       stateCh,
		dataCh:        dataCh,
		voteCh:        voteCh,
		voteSetBitsCh: voteSetBitsCh,
		peerUpdates:   peerUpdates,
		stateCloseCh:  make(chan struct{}),
		closeCh:       make(chan struct{}),
	}
	r.BaseService = *service.NewBaseService(logger, "Consensus", r)

	for _, opt := range options {
		opt(r)
	}

	return r
}

// OnStart starts separate go routines for each p2p Channel and listens for
// envelopes on each. In addition, it also listens for peer updates and handles
// messages on that p2p channel accordingly. The caller must be sure to execute
// OnStop to ensure the outbound p2p Channels are closed.
func (r *Reactor) OnStart() error {
	r.Logger.Debug("consensus wait sync", "wait_sync", r.WaitSync())

	// start routine that computes peer statistics for evaluating peer quality
	//
	// TODO: Evaluate if we need this to be synchronized via WaitGroup as to not
	// leak the goroutine when stopping the reactor.
	go r.peerStatsRoutine()

	r.subscribeToBroadcastEvents()

	if !r.WaitSync() {
		if err := r.state.Start(); err != nil {
			return err
		}
	}

	go r.processStateCh()
	go r.processDataCh()
	go r.processVoteCh()
	go r.processVoteSetBitsCh()
	go r.processPeerUpdates()

	return nil
}

// OnStop stops the reactor by signaling to all spawned goroutines to exit and
// blocking until they all exit, as well as unsubscribing from events and stopping
// state.
func (r *Reactor) OnStop() {
	r.unsubscribeFromBroadcastEvents()

	if err := r.state.Stop(); err != nil {
		r.Logger.Error("failed to stop consensus state", "err", err)
	}

	if !r.WaitSync() {
		r.state.Wait()
	}

	r.mtx.Lock()
	peers := r.peers
	r.mtx.Unlock()

	// wait for all spawned peer goroutines to gracefully exit
	for _, ps := range peers {
		ps.closer.Close()
	}
	for _, ps := range peers {
		ps.broadcastWG.Wait()
	}

	// Close the StateChannel goroutine separately since it uses its own channel
	// to signal closure.
	close(r.stateCloseCh)
	<-r.stateCh.Done()

	// Close closeCh to signal to all spawned goroutines to gracefully exit. All
	// p2p Channels should execute Close().
	close(r.closeCh)

	// Wait for all p2p Channels to be closed before returning. This ensures we
	// can easily reason about synchronization of all p2p Channels and ensure no
	// panics will occur.
	<-r.voteSetBitsCh.Done()
	<-r.dataCh.Done()
	<-r.voteCh.Done()
	<-r.peerUpdates.Done()
}

// SetEventBus sets the reactor's event bus.
func (r *Reactor) SetEventBus(b *types.EventBus) {
	r.eventBus = b
	r.state.SetEventBus(b)
}

// WaitSync returns whether the consensus reactor is waiting for state/fast sync.
func (r *Reactor) WaitSync() bool {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.waitSync
}

// ReactorMetrics sets the reactor's metrics as an option function.
func ReactorMetrics(metrics *Metrics) ReactorOption {
	return func(r *Reactor) { r.Metrics = metrics }
}

// SwitchToConsensus switches from fast-sync mode to consensus mode. It resets
// the state, turns off fast-sync, and starts the consensus state-machine.
func (r *Reactor) SwitchToConsensus(state sm.State, skipWAL bool) {
	r.Logger.Info("switching to consensus")

	// we have no votes, so reconstruct LastCommit from SeenCommit
	if state.LastBlockHeight > 0 {
		r.state.reconstructLastCommit(state)
	}

	// NOTE: The line below causes broadcastNewRoundStepRoutine() to broadcast a
	// NewRoundStepMessage.
	r.state.updateToState(state)

	r.mtx.Lock()
	r.waitSync = false
	r.mtx.Unlock()

	r.Metrics.FastSyncing.Set(0)
	r.Metrics.StateSyncing.Set(0)

	if skipWAL {
		r.state.doWALCatchup = false
	}

	if err := r.state.Start(); err != nil {
		panic(fmt.Sprintf(`failed to start consensus state: %v

conS:
%+v

conR:
%+v`, err, r.state, r))
	}
}

// String returns a string representation of the Reactor.
//
// NOTE: For now, it is just a hard-coded string to avoid accessing unprotected
// shared variables.
//
// TODO: improve!
func (r *Reactor) String() string {
	return "ConsensusReactor"
}

// StringIndented returns an indented string representation of the Reactor.
func (r *Reactor) StringIndented(indent string) string {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	s := "ConsensusReactor{\n"
	s += indent + "  " + r.state.StringIndented(indent+"  ") + "\n"

	for _, ps := range r.peers {
		s += indent + "  " + ps.StringIndented(indent+"  ") + "\n"
	}

	s += indent + "}"
	return s
}

// GetPeerState returns PeerState for a given NodeID.
func (r *Reactor) GetPeerState(peerID p2p.NodeID) (*PeerState, bool) {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	ps, ok := r.peers[peerID]
	return ps, ok
}

func (r *Reactor) broadcastNewRoundStepMessage(rs *cstypes.RoundState) {
	r.stateCh.Out <- p2p.Envelope{
		Broadcast: true,
		Message:   makeRoundStepMessage(rs),
	}
}

func (r *Reactor) broadcastNewValidBlockMessage(rs *cstypes.RoundState) {
	psHeader := rs.ProposalBlockParts.Header()
	r.stateCh.Out <- p2p.Envelope{
		Broadcast: true,
		Message: &tmcons.NewValidBlock{
			Height:             rs.Height,
			Round:              rs.Round,
			BlockPartSetHeader: psHeader.ToProto(),
			BlockParts:         rs.ProposalBlockParts.BitArray().ToProto(),
			IsCommit:           rs.Step == cstypes.RoundStepCommit,
		},
	}
}

func (r *Reactor) broadcastHasVoteMessage(vote *types.Vote) {
	r.stateCh.Out <- p2p.Envelope{
		Broadcast: true,
		Message: &tmcons.HasVote{
			Height: vote.Height,
			Round:  vote.Round,
			Type:   vote.Type,
			Index:  vote.ValidatorIndex,
		},
	}
}

// subscribeToBroadcastEvents subscribes for new round steps and votes using the
// internal pubsub defined in the consensus state to broadcast them to peers
// upon receiving.
func (r *Reactor) subscribeToBroadcastEvents() {
	err := r.state.evsw.AddListenerForEvent(
		listenerIDConsensus,
		types.EventNewRoundStep,
		func(data tmevents.EventData) {
			r.broadcastNewRoundStepMessage(data.(*cstypes.RoundState))
		},
	)
	if err != nil {
		r.Logger.Error("failed to add listener for events", "err", err)
	}

	err = r.state.evsw.AddListenerForEvent(
		listenerIDConsensus,
		types.EventValidBlock,
		func(data tmevents.EventData) {
			r.broadcastNewValidBlockMessage(data.(*cstypes.RoundState))
		},
	)
	if err != nil {
		r.Logger.Error("failed to add listener for events", "err", err)
	}

	err = r.state.evsw.AddListenerForEvent(
		listenerIDConsensus,
		types.EventVote,
		func(data tmevents.EventData) {
			r.broadcastHasVoteMessage(data.(*types.Vote))
		},
	)
	if err != nil {
		r.Logger.Error("failed to add listener for events", "err", err)
	}
}

func (r *Reactor) unsubscribeFromBroadcastEvents() {
	r.state.evsw.RemoveListener(listenerIDConsensus)
}

func makeRoundStepMessage(rs *cstypes.RoundState) *tmcons.NewRoundStep {
	return &tmcons.NewRoundStep{
		Height:                rs.Height,
		Round:                 rs.Round,
		Step:                  uint32(rs.Step),
		SecondsSinceStartTime: int64(time.Since(rs.StartTime).Seconds()),
		LastCommitRound:       rs.LastCommit.GetRound(),
	}
}

func (r *Reactor) sendNewRoundStepMessage(peerID p2p.NodeID) {
	rs := r.state.GetRoundState()
	msg := makeRoundStepMessage(rs)
	r.stateCh.Out <- p2p.Envelope{
		To:      peerID,
		Message: msg,
	}
}

func (r *Reactor) gossipDataForCatchup(rs *cstypes.RoundState, prs *cstypes.PeerRoundState, ps *PeerState) {
	logger := r.Logger.With("height", prs.Height).With("peer", ps.peerID)

	if index, ok := prs.ProposalBlockParts.Not().PickRandom(); ok {
		// ensure that the peer's PartSetHeader is correct
		blockMeta := r.state.blockStore.LoadBlockMeta(prs.Height)
		if blockMeta == nil {
			logger.Error(
				"failed to load block meta",
				"our_height", rs.Height,
				"blockstore_base", r.state.blockStore.Base(),
				"blockstore_height", r.state.blockStore.Height(),
			)

			time.Sleep(r.state.config.PeerGossipSleepDuration)
			return
		} else if !blockMeta.BlockID.PartSetHeader.Equals(prs.ProposalBlockPartSetHeader) {
			logger.Info(
				"peer ProposalBlockPartSetHeader mismatch; sleeping",
				"block_part_set_header", blockMeta.BlockID.PartSetHeader,
				"peer_block_part_set_header", prs.ProposalBlockPartSetHeader,
			)

			time.Sleep(r.state.config.PeerGossipSleepDuration)
			return
		}

		part := r.state.blockStore.LoadBlockPart(prs.Height, index)
		if part == nil {
			logger.Error(
				"failed to load block part",
				"index", index,
				"block_part_set_header", blockMeta.BlockID.PartSetHeader,
				"peer_block_part_set_header", prs.ProposalBlockPartSetHeader,
			)

			time.Sleep(r.state.config.PeerGossipSleepDuration)
			return
		}

		partProto, err := part.ToProto()
		if err != nil {
			logger.Error("failed to convert block part to proto", "err", err)

			time.Sleep(r.state.config.PeerGossipSleepDuration)
			return
		}

		logger.Debug("sending block part for catchup", "round", prs.Round, "index", index)
		r.dataCh.Out <- p2p.Envelope{
			To: ps.peerID,
			Message: &tmcons.BlockPart{
				Height: prs.Height, // not our height, so it does not matter.
				Round:  prs.Round,  // not our height, so it does not matter
				Part:   *partProto,
			},
		}

		return
	}

	time.Sleep(r.state.config.PeerGossipSleepDuration)
}

func (r *Reactor) gossipDataRoutine(ps *PeerState) {
	logger := r.Logger.With("peer", ps.peerID)

	defer ps.broadcastWG.Done()

OUTER_LOOP:
	for {
		if !r.IsRunning() {
			return
		}

		select {
		case <-ps.closer.Done():
			// The peer is marked for removal via a PeerUpdate as the doneCh was
			// explicitly closed to signal we should exit.
			return

		default:
		}

		rs := r.state.GetRoundState()
		prs := ps.GetRoundState()

		// Send proposal Block parts?
		if rs.ProposalBlockParts.HasHeader(prs.ProposalBlockPartSetHeader) {
			if index, ok := rs.ProposalBlockParts.BitArray().Sub(prs.ProposalBlockParts.Copy()).PickRandom(); ok {
				part := rs.ProposalBlockParts.GetPart(index)
				partProto, err := part.ToProto()
				if err != nil {
					logger.Error("failed to convert block part to proto", "err", err)
					return
				}

				logger.Debug("sending block part", "height", prs.Height, "round", prs.Round)
				r.dataCh.Out <- p2p.Envelope{
					To: ps.peerID,
					Message: &tmcons.BlockPart{
						Height: rs.Height, // this tells peer that this part applies to us
						Round:  rs.Round,  // this tells peer that this part applies to us
						Part:   *partProto,
					},
				}

				ps.SetHasProposalBlockPart(prs.Height, prs.Round, index)
				continue OUTER_LOOP
			}
		}

		// if the peer is on a previous height that we have, help catch up
		blockStoreBase := r.state.blockStore.Base()
		if blockStoreBase > 0 && 0 < prs.Height && prs.Height < rs.Height && prs.Height >= blockStoreBase {
			heightLogger := logger.With("height", prs.Height)

			// If we never received the commit message from the peer, the block parts
			// will not be initialized.
			if prs.ProposalBlockParts == nil {
				blockMeta := r.state.blockStore.LoadBlockMeta(prs.Height)
				if blockMeta == nil {
					heightLogger.Error(
						"failed to load block meta",
						"blockstoreBase", blockStoreBase,
						"blockstoreHeight", r.state.blockStore.Height(),
					)
					time.Sleep(r.state.config.PeerGossipSleepDuration)
				} else {
					ps.InitProposalBlockParts(blockMeta.BlockID.PartSetHeader)
				}

				// Continue the loop since prs is a copy and not effected by this
				// initialization.
				continue OUTER_LOOP
			}

			r.gossipDataForCatchup(rs, prs, ps)
			continue OUTER_LOOP
		}

		// if height and round don't match, sleep
		if (rs.Height != prs.Height) || (rs.Round != prs.Round) {
			time.Sleep(r.state.config.PeerGossipSleepDuration)
			continue OUTER_LOOP
		}

		// By here, height and round match.
		// Proposal block parts were already matched and sent if any were wanted.
		// (These can match on hash so the round doesn't matter)
		// Now consider sending other things, like the Proposal itself.

		// Send Proposal && ProposalPOL BitArray?
		if rs.Proposal != nil && !prs.Proposal {
			// Proposal: share the proposal metadata with peer.
			{
				propProto := rs.Proposal.ToProto()

				logger.Debug("sending proposal", "height", prs.Height, "round", prs.Round)
				r.dataCh.Out <- p2p.Envelope{
					To: ps.peerID,
					Message: &tmcons.Proposal{
						Proposal: *propProto,
					},
				}

				// NOTE: A peer might have received a different proposal message, so
				// this Proposal msg will be rejected!
				ps.SetHasProposal(rs.Proposal)
			}

			// ProposalPOL: lets peer know which POL votes we have so far. The peer
			// must receive ProposalMessage first. Note, rs.Proposal was validated,
			// so rs.Proposal.POLRound <= rs.Round, so we definitely have
			// rs.Votes.Prevotes(rs.Proposal.POLRound).
			if 0 <= rs.Proposal.POLRound {
				pPol := rs.Votes.Prevotes(rs.Proposal.POLRound).BitArray()
				pPolProto := pPol.ToProto()

				logger.Debug("sending POL", "height", prs.Height, "round", prs.Round)
				r.dataCh.Out <- p2p.Envelope{
					To: ps.peerID,
					Message: &tmcons.ProposalPOL{
						Height:           rs.Height,
						ProposalPolRound: rs.Proposal.POLRound,
						ProposalPol:      *pPolProto,
					},
				}
			}

			continue OUTER_LOOP
		}

		// nothing to do -- sleep
		time.Sleep(r.state.config.PeerGossipSleepDuration)
		continue OUTER_LOOP
	}
}

// pickSendVote picks a vote and sends it to the peer. It will return true if
// there is a vote to send and false otherwise.
func (r *Reactor) pickSendVote(ps *PeerState, votes types.VoteSetReader) bool {
	if vote, ok := ps.PickVoteToSend(votes); ok {
		r.Logger.Debug("sending vote message", "ps", ps, "vote", vote)
		r.voteCh.Out <- p2p.Envelope{
			To: ps.peerID,
			Message: &tmcons.Vote{
				Vote: vote.ToProto(),
			},
		}

		ps.SetHasVote(vote)
		return true
	}

	return false
}

func (r *Reactor) gossipVotesForHeight(rs *cstypes.RoundState, prs *cstypes.PeerRoundState, ps *PeerState) bool {
	logger := r.Logger.With("height", prs.Height).With("peer", ps.peerID)

	// if there are lastCommits to send...
	if prs.Step == cstypes.RoundStepNewHeight {
		if r.pickSendVote(ps, rs.LastCommit) {
			logger.Debug("picked rs.LastCommit to send")
			return true
		}
	}

	// if there are POL prevotes to send...
	if prs.Step <= cstypes.RoundStepPropose && prs.Round != -1 && prs.Round <= rs.Round && prs.ProposalPOLRound != -1 {
		if polPrevotes := rs.Votes.Prevotes(prs.ProposalPOLRound); polPrevotes != nil {
			if r.pickSendVote(ps, polPrevotes) {
				logger.Debug("picked rs.Prevotes(prs.ProposalPOLRound) to send", "round", prs.ProposalPOLRound)
				return true
			}
		}
	}

	// if there are prevotes to send...
	if prs.Step <= cstypes.RoundStepPrevoteWait && prs.Round != -1 && prs.Round <= rs.Round {
		if r.pickSendVote(ps, rs.Votes.Prevotes(prs.Round)) {
			logger.Debug("picked rs.Prevotes(prs.Round) to send", "round", prs.Round)
			return true
		}
	}

	// if there are precommits to send...
	if prs.Step <= cstypes.RoundStepPrecommitWait && prs.Round != -1 && prs.Round <= rs.Round {
		if r.pickSendVote(ps, rs.Votes.Precommits(prs.Round)) {
			logger.Debug("picked rs.Precommits(prs.Round) to send", "round", prs.Round)
			return true
		}
	}

	// if there are prevotes to send...(which are needed because of validBlock mechanism)
	if prs.Round != -1 && prs.Round <= rs.Round {
		if r.pickSendVote(ps, rs.Votes.Prevotes(prs.Round)) {
			logger.Debug("picked rs.Prevotes(prs.Round) to send", "round", prs.Round)
			return true
		}
	}

	// if there are POLPrevotes to send...
	if prs.ProposalPOLRound != -1 {
		if polPrevotes := rs.Votes.Prevotes(prs.ProposalPOLRound); polPrevotes != nil {
			if r.pickSendVote(ps, polPrevotes) {
				logger.Debug("picked rs.Prevotes(prs.ProposalPOLRound) to send", "round", prs.ProposalPOLRound)
				return true
			}
		}
	}

	return false
}

func (r *Reactor) gossipVotesRoutine(ps *PeerState) {
	logger := r.Logger.With("peer", ps.peerID)

	defer ps.broadcastWG.Done()

	// XXX: simple hack to throttle logs upon sleep
	logThrottle := 0

OUTER_LOOP:
	for {
		if !r.IsRunning() {
			return
		}

		select {
		case <-ps.closer.Done():
			// The peer is marked for removal via a PeerUpdate as the doneCh was
			// explicitly closed to signal we should exit.
			return

		default:
		}

		rs := r.state.GetRoundState()
		prs := ps.GetRoundState()

		switch logThrottle {
		case 1: // first sleep
			logThrottle = 2
		case 2: // no more sleep
			logThrottle = 0
		}

		// if height matches, then send LastCommit, Prevotes, and Precommits
		if rs.Height == prs.Height {
			if r.gossipVotesForHeight(rs, prs, ps) {
				continue OUTER_LOOP
			}
		}

		// special catchup logic -- if peer is lagging by height 1, send LastCommit
		if prs.Height != 0 && rs.Height == prs.Height+1 {
			if r.pickSendVote(ps, rs.LastCommit) {
				logger.Debug("picked rs.LastCommit to send", "height", prs.Height)
				continue OUTER_LOOP
			}
		}

		// catchup logic -- if peer is lagging by more than 1, send Commit
		blockStoreBase := r.state.blockStore.Base()
		if blockStoreBase > 0 && prs.Height != 0 && rs.Height >= prs.Height+2 && prs.Height >= blockStoreBase {
			// Load the block commit for prs.Height, which contains precommit
			// signatures for prs.Height.
			if commit := r.state.blockStore.LoadBlockCommit(prs.Height); commit != nil {
				if r.pickSendVote(ps, commit) {
					logger.Debug("picked Catchup commit to send", "height", prs.Height)
					continue OUTER_LOOP
				}
			}
		}

		if logThrottle == 0 {
			// we sent nothing -- sleep
			logThrottle = 1
			logger.Debug(
				"no votes to send; sleeping",
				"rs.Height", rs.Height,
				"prs.Height", prs.Height,
				"localPV", rs.Votes.Prevotes(rs.Round).BitArray(), "peerPV", prs.Prevotes,
				"localPC", rs.Votes.Precommits(rs.Round).BitArray(), "peerPC", prs.Precommits,
			)
		} else if logThrottle == 2 {
			logThrottle = 1
		}

		time.Sleep(r.state.config.PeerGossipSleepDuration)
		continue OUTER_LOOP
	}
}

// NOTE: `queryMaj23Routine` has a simple crude design since it only comes
// into play for liveness when there's a signature DDoS attack happening.
func (r *Reactor) queryMaj23Routine(ps *PeerState) {
	defer ps.broadcastWG.Done()

OUTER_LOOP:
	for {
		if !r.IsRunning() {
			return
		}

		select {
		case <-ps.closer.Done():
			// The peer is marked for removal via a PeerUpdate as the doneCh was
			// explicitly closed to signal we should exit.
			return

		default:
		}

		// maybe send Height/Round/Prevotes
		{
			rs := r.state.GetRoundState()
			prs := ps.GetRoundState()

			if rs.Height == prs.Height {
				if maj23, ok := rs.Votes.Prevotes(prs.Round).TwoThirdsMajority(); ok {
					r.stateCh.Out <- p2p.Envelope{
						To: ps.peerID,
						Message: &tmcons.VoteSetMaj23{
							Height:  prs.Height,
							Round:   prs.Round,
							Type:    tmproto.PrevoteType,
							BlockID: maj23.ToProto(),
						},
					}

					time.Sleep(r.state.config.PeerQueryMaj23SleepDuration)
				}
			}
		}

		// maybe send Height/Round/Precommits
		{
			rs := r.state.GetRoundState()
			prs := ps.GetRoundState()

			if rs.Height == prs.Height {
				if maj23, ok := rs.Votes.Precommits(prs.Round).TwoThirdsMajority(); ok {
					r.stateCh.Out <- p2p.Envelope{
						To: ps.peerID,
						Message: &tmcons.VoteSetMaj23{
							Height:  prs.Height,
							Round:   prs.Round,
							Type:    tmproto.PrecommitType,
							BlockID: maj23.ToProto(),
						},
					}

					time.Sleep(r.state.config.PeerQueryMaj23SleepDuration)
				}
			}
		}

		// maybe send Height/Round/ProposalPOL
		{
			rs := r.state.GetRoundState()
			prs := ps.GetRoundState()

			if rs.Height == prs.Height && prs.ProposalPOLRound >= 0 {
				if maj23, ok := rs.Votes.Prevotes(prs.ProposalPOLRound).TwoThirdsMajority(); ok {
					r.stateCh.Out <- p2p.Envelope{
						To: ps.peerID,
						Message: &tmcons.VoteSetMaj23{
							Height:  prs.Height,
							Round:   prs.ProposalPOLRound,
							Type:    tmproto.PrevoteType,
							BlockID: maj23.ToProto(),
						},
					}

					time.Sleep(r.state.config.PeerQueryMaj23SleepDuration)
				}
			}
		}

		// Little point sending LastCommitRound/LastCommit, these are fleeting and
		// non-blocking.

		// maybe send Height/CatchupCommitRound/CatchupCommit
		{
			prs := ps.GetRoundState()

			if prs.CatchupCommitRound != -1 && prs.Height > 0 && prs.Height <= r.state.blockStore.Height() &&
				prs.Height >= r.state.blockStore.Base() {
				if commit := r.state.LoadCommit(prs.Height); commit != nil {
					r.stateCh.Out <- p2p.Envelope{
						To: ps.peerID,
						Message: &tmcons.VoteSetMaj23{
							Height:  prs.Height,
							Round:   commit.Round,
							Type:    tmproto.PrecommitType,
							BlockID: commit.BlockID.ToProto(),
						},
					}

					time.Sleep(r.state.config.PeerQueryMaj23SleepDuration)
				}
			}
		}

		time.Sleep(r.state.config.PeerQueryMaj23SleepDuration)
		continue OUTER_LOOP
	}
}

// processPeerUpdate process a peer update message. For new or reconnected peers,
// we create a peer state if one does not exist for the peer, which should always
// be the case, and we spawn all the relevant goroutine to broadcast messages to
// the peer. During peer removal, we remove the peer for our set of peers and
// signal to all spawned goroutines to gracefully exit in a non-blocking manner.
func (r *Reactor) processPeerUpdate(peerUpdate p2p.PeerUpdate) {
	r.Logger.Debug("received peer update", "peer", peerUpdate.NodeID, "status", peerUpdate.Status)

	r.mtx.Lock()
	defer r.mtx.Unlock()

	switch peerUpdate.Status {
	case p2p.PeerStatusUp:
		// Do not allow starting new broadcasting goroutines after reactor shutdown
		// has been initiated. This can happen after we've manually closed all
		// peer goroutines and closed r.closeCh, but the router still sends in-flight
		// peer updates.
		if !r.IsRunning() {
			return
		}

		var (
			ps *PeerState
			ok bool
		)

		ps, ok = r.peers[peerUpdate.NodeID]
		if !ok {
			ps = NewPeerState(r.Logger, peerUpdate.NodeID)
			r.peers[peerUpdate.NodeID] = ps
		}

		if !ps.IsRunning() {
			// Set the peer state's closer to signal to all spawned goroutines to exit
			// when the peer is removed. We also set the running state to ensure we
			// do not spawn multiple instances of the same goroutines and finally we
			// set the waitgroup counter so we know when all goroutines have exited.
			ps.broadcastWG.Add(3)
			ps.SetRunning(true)

			// start goroutines for this peer
			go r.gossipDataRoutine(ps)
			go r.gossipVotesRoutine(ps)
			go r.queryMaj23Routine(ps)

			// Send our state to the peer. If we're fast-syncing, broadcast a
			// RoundStepMessage later upon SwitchToConsensus().
			if !r.waitSync {
				go r.sendNewRoundStepMessage(ps.peerID)
			}
		}

	case p2p.PeerStatusDown:
		ps, ok := r.peers[peerUpdate.NodeID]
		if ok && ps.IsRunning() {
			// signal to all spawned goroutines for the peer to gracefully exit
			ps.closer.Close()

			go func() {
				// Wait for all spawned broadcast goroutines to exit before marking the
				// peer state as no longer running and removal from the peers map.
				ps.broadcastWG.Wait()

				r.mtx.Lock()
				delete(r.peers, peerUpdate.NodeID)
				r.mtx.Unlock()

				ps.SetRunning(false)
			}()
		}
	}
}

// handleStateMessage handles envelopes sent from peers on the StateChannel.
// An error is returned if the message is unrecognized or if validation fails.
// If we fail to find the peer state for the envelope sender, we perform a no-op
// and return. This can happen when we process the envelope after the peer is
// removed.
func (r *Reactor) handleStateMessage(envelope p2p.Envelope, msgI Message) error {
	ps, ok := r.GetPeerState(envelope.From)
	if !ok || ps == nil {
		r.Logger.Debug("failed to find peer state", "peer", envelope.From, "ch_id", "StateChannel")
		return nil
	}

	switch msg := envelope.Message.(type) {
	case *tmcons.NewRoundStep:
		r.state.mtx.RLock()
		initialHeight := r.state.state.InitialHeight
		r.state.mtx.RUnlock()

		if err := msgI.(*NewRoundStepMessage).ValidateHeight(initialHeight); err != nil {
			r.Logger.Error("peer sent us an invalid msg", "msg", msg, "err", err)
			return err
		}

		ps.ApplyNewRoundStepMessage(msgI.(*NewRoundStepMessage))

	case *tmcons.NewValidBlock:
		ps.ApplyNewValidBlockMessage(msgI.(*NewValidBlockMessage))

	case *tmcons.HasVote:
		ps.ApplyHasVoteMessage(msgI.(*HasVoteMessage))

	case *tmcons.VoteSetMaj23:
		r.state.mtx.RLock()
		height, votes := r.state.Height, r.state.Votes
		r.state.mtx.RUnlock()

		if height != msg.Height {
			return nil
		}

		vsmMsg := msgI.(*VoteSetMaj23Message)

		// peer claims to have a maj23 for some BlockID at <H,R,S>
		err := votes.SetPeerMaj23(msg.Round, msg.Type, ps.peerID, vsmMsg.BlockID)
		if err != nil {
			return err
		}

		// Respond with a VoteSetBitsMessage showing which votes we have and
		// consequently shows which we don't have.
		var ourVotes *bits.BitArray
		switch vsmMsg.Type {
		case tmproto.PrevoteType:
			ourVotes = votes.Prevotes(msg.Round).BitArrayByBlockID(vsmMsg.BlockID)

		case tmproto.PrecommitType:
			ourVotes = votes.Precommits(msg.Round).BitArrayByBlockID(vsmMsg.BlockID)

		default:
			panic("bad VoteSetBitsMessage field type; forgot to add a check in ValidateBasic?")
		}

		eMsg := &tmcons.VoteSetBits{
			Height:  msg.Height,
			Round:   msg.Round,
			Type:    msg.Type,
			BlockID: msg.BlockID,
		}

		if votesProto := ourVotes.ToProto(); votesProto != nil {
			eMsg.Votes = *votesProto
		}

		r.voteSetBitsCh.Out <- p2p.Envelope{
			To:      envelope.From,
			Message: eMsg,
		}

	default:
		return fmt.Errorf("received unknown message on StateChannel: %T", msg)
	}

	return nil
}

// handleDataMessage handles envelopes sent from peers on the DataChannel. If we
// fail to find the peer state for the envelope sender, we perform a no-op and
// return. This can happen when we process the envelope after the peer is
// removed.
func (r *Reactor) handleDataMessage(envelope p2p.Envelope, msgI Message) error {
	logger := r.Logger.With("peer", envelope.From, "ch_id", "DataChannel")

	ps, ok := r.GetPeerState(envelope.From)
	if !ok || ps == nil {
		r.Logger.Debug("failed to find peer state")
		return nil
	}

	if r.WaitSync() {
		logger.Info("ignoring message received during sync", "msg", msgI)
		return nil
	}

	switch msg := envelope.Message.(type) {
	case *tmcons.Proposal:
		pMsg := msgI.(*ProposalMessage)

		ps.SetHasProposal(pMsg.Proposal)
		r.state.peerMsgQueue <- msgInfo{pMsg, envelope.From}

	case *tmcons.ProposalPOL:
		ps.ApplyProposalPOLMessage(msgI.(*ProposalPOLMessage))

	case *tmcons.BlockPart:
		bpMsg := msgI.(*BlockPartMessage)

		ps.SetHasProposalBlockPart(bpMsg.Height, bpMsg.Round, int(bpMsg.Part.Index))
		r.Metrics.BlockParts.With("peer_id", string(envelope.From)).Add(1)
		r.state.peerMsgQueue <- msgInfo{bpMsg, envelope.From}

	default:
		return fmt.Errorf("received unknown message on DataChannel: %T", msg)
	}

	return nil
}

// handleVoteMessage handles envelopes sent from peers on the VoteChannel. If we
// fail to find the peer state for the envelope sender, we perform a no-op and
// return. This can happen when we process the envelope after the peer is
// removed.
func (r *Reactor) handleVoteMessage(envelope p2p.Envelope, msgI Message) error {
	logger := r.Logger.With("peer", envelope.From, "ch_id", "VoteChannel")

	ps, ok := r.GetPeerState(envelope.From)
	if !ok || ps == nil {
		r.Logger.Debug("failed to find peer state")
		return nil
	}

	if r.WaitSync() {
		logger.Info("ignoring message received during sync", "msg", msgI)
		return nil
	}

	switch msg := envelope.Message.(type) {
	case *tmcons.Vote:
		r.state.mtx.RLock()
		height, valSize, lastCommitSize := r.state.Height, r.state.Validators.Size(), r.state.LastCommit.Size()
		r.state.mtx.RUnlock()

		vMsg := msgI.(*VoteMessage)

		ps.EnsureVoteBitArrays(height, valSize)
		ps.EnsureVoteBitArrays(height-1, lastCommitSize)
		ps.SetHasVote(vMsg.Vote)

		r.state.peerMsgQueue <- msgInfo{vMsg, envelope.From}

	default:
		return fmt.Errorf("received unknown message on VoteChannel: %T", msg)
	}

	return nil
}

// handleVoteSetBitsMessage handles envelopes sent from peers on the
// VoteSetBitsChannel. If we fail to find the peer state for the envelope sender,
// we perform a no-op and return. This can happen when we process the envelope
// after the peer is removed.
func (r *Reactor) handleVoteSetBitsMessage(envelope p2p.Envelope, msgI Message) error {
	logger := r.Logger.With("peer", envelope.From, "ch_id", "VoteSetBitsChannel")

	ps, ok := r.GetPeerState(envelope.From)
	if !ok || ps == nil {
		r.Logger.Debug("failed to find peer state")
		return nil
	}

	if r.WaitSync() {
		logger.Info("ignoring message received during sync", "msg", msgI)
		return nil
	}

	switch msg := envelope.Message.(type) {
	case *tmcons.VoteSetBits:
		r.state.mtx.RLock()
		height, votes := r.state.Height, r.state.Votes
		r.state.mtx.RUnlock()

		vsbMsg := msgI.(*VoteSetBitsMessage)

		if height == msg.Height {
			var ourVotes *bits.BitArray

			switch msg.Type {
			case tmproto.PrevoteType:
				ourVotes = votes.Prevotes(msg.Round).BitArrayByBlockID(vsbMsg.BlockID)

			case tmproto.PrecommitType:
				ourVotes = votes.Precommits(msg.Round).BitArrayByBlockID(vsbMsg.BlockID)

			default:
				panic("bad VoteSetBitsMessage field type; forgot to add a check in ValidateBasic?")
			}

			ps.ApplyVoteSetBitsMessage(vsbMsg, ourVotes)
		} else {
			ps.ApplyVoteSetBitsMessage(vsbMsg, nil)
		}

	default:
		return fmt.Errorf("received unknown message on VoteSetBitsChannel: %T", msg)
	}

	return nil
}

// handleMessage handles an Envelope sent from a peer on a specific p2p Channel.
// It will handle errors and any possible panics gracefully. A caller can handle
// any error returned by sending a PeerError on the respective channel.
//
// NOTE: We process these messages even when we're fast_syncing. Messages affect
// either a peer state or the consensus state. Peer state updates can happen in
// parallel, but processing of proposals, block parts, and votes are ordered by
// the p2p channel.
//
// NOTE: We block on consensus state for proposals, block parts, and votes.
func (r *Reactor) handleMessage(chID p2p.ChannelID, envelope p2p.Envelope) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("panic in processing message: %v", e)
			r.Logger.Error("recovering from processing message panic", "err", err)
		}
	}()

	// We wrap the envelope's message in a Proto wire type so we can convert back
	// the domain type that individual channel message handlers can work with. We
	// do this here once to avoid having to do it for each individual message type.
	// and because a large part of the core business logic depends on these
	// domain types opposed to simply working with the Proto types.
	protoMsg := new(tmcons.Message)
	if err := protoMsg.Wrap(envelope.Message); err != nil {
		return err
	}

	msgI, err := MsgFromProto(protoMsg)
	if err != nil {
		return err
	}

	r.Logger.Debug("received message", "ch_id", chID, "message", msgI, "peer", envelope.From)

	switch chID {
	case StateChannel:
		err = r.handleStateMessage(envelope, msgI)

	case DataChannel:
		err = r.handleDataMessage(envelope, msgI)

	case VoteChannel:
		err = r.handleVoteMessage(envelope, msgI)

	case VoteSetBitsChannel:
		err = r.handleVoteSetBitsMessage(envelope, msgI)

	default:
		err = fmt.Errorf("unknown channel ID (%d) for envelope (%v)", chID, envelope)
	}

	return err
}

// processStateCh initiates a blocking process where we listen for and handle
// envelopes on the StateChannel. Any error encountered during message
// execution will result in a PeerError being sent on the StateChannel. When
// the reactor is stopped, we will catch the signal and close the p2p Channel
// gracefully.
func (r *Reactor) processStateCh() {
	defer r.stateCh.Close()

	for {
		select {
		case envelope := <-r.stateCh.In:
			if err := r.handleMessage(r.stateCh.ID, envelope); err != nil {
				r.Logger.Error("failed to process message", "ch_id", r.stateCh.ID, "envelope", envelope, "err", err)
				r.stateCh.Error <- p2p.PeerError{
					NodeID: envelope.From,
					Err:    err,
				}
			}

		case <-r.stateCloseCh:
			r.Logger.Debug("stopped listening on StateChannel; closing...")
			return
		}
	}
}

// processDataCh initiates a blocking process where we listen for and handle
// envelopes on the DataChannel. Any error encountered during message
// execution will result in a PeerError being sent on the DataChannel. When
// the reactor is stopped, we will catch the signal and close the p2p Channel
// gracefully.
func (r *Reactor) processDataCh() {
	defer r.dataCh.Close()

	for {
		select {
		case envelope := <-r.dataCh.In:
			if err := r.handleMessage(r.dataCh.ID, envelope); err != nil {
				r.Logger.Error("failed to process message", "ch_id", r.dataCh.ID, "envelope", envelope, "err", err)
				r.dataCh.Error <- p2p.PeerError{
					NodeID: envelope.From,
					Err:    err,
				}
			}

		case <-r.closeCh:
			r.Logger.Debug("stopped listening on DataChannel; closing...")
			return
		}
	}
}

// processVoteCh initiates a blocking process where we listen for and handle
// envelopes on the VoteChannel. Any error encountered during message
// execution will result in a PeerError being sent on the VoteChannel. When
// the reactor is stopped, we will catch the signal and close the p2p Channel
// gracefully.
func (r *Reactor) processVoteCh() {
	defer r.voteCh.Close()

	for {
		select {
		case envelope := <-r.voteCh.In:
			if err := r.handleMessage(r.voteCh.ID, envelope); err != nil {
				r.Logger.Error("failed to process message", "ch_id", r.voteCh.ID, "envelope", envelope, "err", err)
				r.voteCh.Error <- p2p.PeerError{
					NodeID: envelope.From,
					Err:    err,
				}
			}

		case <-r.closeCh:
			r.Logger.Debug("stopped listening on VoteChannel; closing...")
			return
		}
	}
}

// processVoteCh initiates a blocking process where we listen for and handle
// envelopes on the VoteSetBitsChannel. Any error encountered during message
// execution will result in a PeerError being sent on the VoteSetBitsChannel.
// When the reactor is stopped, we will catch the signal and close the p2p
// Channel gracefully.
func (r *Reactor) processVoteSetBitsCh() {
	defer r.voteSetBitsCh.Close()

	for {
		select {
		case envelope := <-r.voteSetBitsCh.In:
			if err := r.handleMessage(r.voteSetBitsCh.ID, envelope); err != nil {
				r.Logger.Error("failed to process message", "ch_id", r.voteSetBitsCh.ID, "envelope", envelope, "err", err)
				r.voteSetBitsCh.Error <- p2p.PeerError{
					NodeID: envelope.From,
					Err:    err,
				}
			}

		case <-r.closeCh:
			r.Logger.Debug("stopped listening on VoteSetBitsChannel; closing...")
			return
		}
	}
}

// processPeerUpdates initiates a blocking process where we listen for and handle
// PeerUpdate messages. When the reactor is stopped, we will catch the signal and
// close the p2p PeerUpdatesCh gracefully.
func (r *Reactor) processPeerUpdates() {
	defer r.peerUpdates.Close()

	for {
		select {
		case peerUpdate := <-r.peerUpdates.Updates():
			r.processPeerUpdate(peerUpdate)

		case <-r.closeCh:
			r.Logger.Debug("stopped listening on peer updates channel; closing...")
			return
		}
	}
}

func (r *Reactor) peerStatsRoutine() {
	for {
		if !r.IsRunning() {
			r.Logger.Info("stopping peerStatsRoutine")
			return
		}

		select {
		case msg := <-r.state.statsMsgQueue:
			ps, ok := r.GetPeerState(msg.PeerID)
			if !ok || ps == nil {
				r.Logger.Debug("attempt to update stats for non-existent peer", "peer", msg.PeerID)
				continue
			}

			switch msg.Msg.(type) {
			case *VoteMessage:
				if numVotes := ps.RecordVote(); numVotes%votesToContributeToBecomeGoodPeer == 0 {
					r.peerUpdates.SendUpdate(p2p.PeerUpdate{
						NodeID: msg.PeerID,
						Status: p2p.PeerStatusGood,
					})
				}

			case *BlockPartMessage:
				if numParts := ps.RecordBlockPart(); numParts%blocksToContributeToBecomeGoodPeer == 0 {
					r.peerUpdates.SendUpdate(p2p.PeerUpdate{
						NodeID: msg.PeerID,
						Status: p2p.PeerStatusGood,
					})
				}
			}
		case <-r.closeCh:
			return
		}
	}
}
