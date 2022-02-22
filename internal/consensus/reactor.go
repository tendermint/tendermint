package consensus

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	cstypes "github.com/tendermint/tendermint/internal/consensus/types"
	"github.com/tendermint/tendermint/internal/eventbus"
	"github.com/tendermint/tendermint/internal/p2p"
	sm "github.com/tendermint/tendermint/internal/state"
	"github.com/tendermint/tendermint/libs/bits"
	tmevents "github.com/tendermint/tendermint/libs/events"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	tmtime "github.com/tendermint/tendermint/libs/time"
	tmcons "github.com/tendermint/tendermint/proto/tendermint/consensus"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

var (
	_ service.Service = (*Reactor)(nil)
	_ p2p.Wrapper     = (*tmcons.Message)(nil)
)

// GetChannelDescriptor produces an instance of a descriptor for this
// package's required channels.
func getChannelDescriptors() map[p2p.ChannelID]*p2p.ChannelDescriptor {
	return map[p2p.ChannelID]*p2p.ChannelDescriptor{
		StateChannel: {
			ID:                  StateChannel,
			MessageType:         new(tmcons.Message),
			Priority:            8,
			SendQueueCapacity:   64,
			RecvMessageCapacity: maxMsgSize,
			RecvBufferCapacity:  128,
		},
		DataChannel: {
			// TODO: Consider a split between gossiping current block and catchup
			// stuff. Once we gossip the whole block there is nothing left to send
			// until next height or round.
			ID:                  DataChannel,
			MessageType:         new(tmcons.Message),
			Priority:            12,
			SendQueueCapacity:   64,
			RecvBufferCapacity:  512,
			RecvMessageCapacity: maxMsgSize,
		},
		VoteChannel: {
			ID:                  VoteChannel,
			MessageType:         new(tmcons.Message),
			Priority:            10,
			SendQueueCapacity:   64,
			RecvBufferCapacity:  128,
			RecvMessageCapacity: maxMsgSize,
		},
		VoteSetBitsChannel: {
			ID:                  VoteSetBitsChannel,
			MessageType:         new(tmcons.Message),
			Priority:            5,
			SendQueueCapacity:   8,
			RecvBufferCapacity:  128,
			RecvMessageCapacity: maxMsgSize,
		},
	}
}

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

// NOTE: Temporary interface for switching to block sync, we should get rid of v0.
// See: https://github.com/tendermint/tendermint/issues/4595
type BlockSyncReactor interface {
	SwitchToBlockSync(context.Context, sm.State) error

	GetMaxPeerBlockHeight() int64

	// GetTotalSyncedTime returns the time duration since the blocksync starting.
	GetTotalSyncedTime() time.Duration

	// GetRemainingSyncTime returns the estimating time the node will be fully synced,
	// if will return 0 if the blocksync does not perform or the number of block synced is
	// too small (less than 100).
	GetRemainingSyncTime() time.Duration
}

//go:generate ../../scripts/mockery_generate.sh ConsSyncReactor
// ConsSyncReactor defines an interface used for testing abilities of node.startStateSync.
type ConsSyncReactor interface {
	SwitchToConsensus(sm.State, bool)
	SetStateSyncingMetrics(float64)
	SetBlockSyncingMetrics(float64)
}

// Reactor defines a reactor for the consensus service.
type Reactor struct {
	service.BaseService
	logger log.Logger

	state    *State
	eventBus *eventbus.EventBus
	Metrics  *Metrics

	mtx         sync.RWMutex
	peers       map[types.NodeID]*PeerState
	waitSync    bool
	readySignal chan struct{} // closed when the node is ready to start consensus

	stateCh       *p2p.Channel
	dataCh        *p2p.Channel
	voteCh        *p2p.Channel
	voteSetBitsCh *p2p.Channel
	peerUpdates   *p2p.PeerUpdates
}

// NewReactor returns a reference to a new consensus reactor, which implements
// the service.Service interface. It accepts a logger, consensus state, references
// to relevant p2p Channels and a channel to listen for peer updates on. The
// reactor will close all p2p Channels when stopping.
func NewReactor(
	ctx context.Context,
	logger log.Logger,
	cs *State,
	channelCreator p2p.ChannelCreator,
	peerUpdates *p2p.PeerUpdates,
	waitSync bool,
	metrics *Metrics,
) (*Reactor, error) {
	chans := getChannelDescriptors()
	stateCh, err := channelCreator(ctx, chans[StateChannel])
	if err != nil {
		return nil, err
	}

	dataCh, err := channelCreator(ctx, chans[DataChannel])
	if err != nil {
		return nil, err
	}

	voteCh, err := channelCreator(ctx, chans[VoteChannel])
	if err != nil {
		return nil, err
	}

	voteSetBitsCh, err := channelCreator(ctx, chans[VoteSetBitsChannel])
	if err != nil {
		return nil, err
	}
	r := &Reactor{
		logger:        logger,
		state:         cs,
		waitSync:      waitSync,
		peers:         make(map[types.NodeID]*PeerState),
		Metrics:       metrics,
		stateCh:       stateCh,
		dataCh:        dataCh,
		voteCh:        voteCh,
		voteSetBitsCh: voteSetBitsCh,
		peerUpdates:   peerUpdates,
		readySignal:   make(chan struct{}),
	}
	r.BaseService = *service.NewBaseService(logger, "Consensus", r)

	if !r.waitSync {
		close(r.readySignal)
	}

	return r, nil
}

// OnStart starts separate go routines for each p2p Channel and listens for
// envelopes on each. In addition, it also listens for peer updates and handles
// messages on that p2p channel accordingly. The caller must be sure to execute
// OnStop to ensure the outbound p2p Channels are closed.
func (r *Reactor) OnStart(ctx context.Context) error {
	r.logger.Debug("consensus wait sync", "wait_sync", r.WaitSync())

	// start routine that computes peer statistics for evaluating peer quality
	//
	// TODO: Evaluate if we need this to be synchronized via WaitGroup as to not
	// leak the goroutine when stopping the reactor.
	go r.peerStatsRoutine(ctx)

	r.subscribeToBroadcastEvents()

	if !r.WaitSync() {
		if err := r.state.Start(ctx); err != nil {
			return err
		}
	}

	go r.processStateCh(ctx)
	go r.processDataCh(ctx)
	go r.processVoteCh(ctx)
	go r.processVoteSetBitsCh(ctx)
	go r.processPeerUpdates(ctx)

	return nil
}

// OnStop stops the reactor by signaling to all spawned goroutines to exit and
// blocking until they all exit, as well as unsubscribing from events and stopping
// state.
func (r *Reactor) OnStop() {
	r.unsubscribeFromBroadcastEvents()

	r.state.Stop()

	if !r.WaitSync() {
		r.state.Wait()
	}
}

// SetEventBus sets the reactor's event bus.
func (r *Reactor) SetEventBus(b *eventbus.EventBus) {
	r.eventBus = b
	r.state.SetEventBus(b)
}

// WaitSync returns whether the consensus reactor is waiting for state/block sync.
func (r *Reactor) WaitSync() bool {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	return r.waitSync
}

// SwitchToConsensus switches from block-sync mode to consensus mode. It resets
// the state, turns off block-sync, and starts the consensus state-machine.
func (r *Reactor) SwitchToConsensus(ctx context.Context, state sm.State, skipWAL bool) {
	r.logger.Info("switching to consensus")

	// we have no votes, so reconstruct LastCommit from SeenCommit
	if state.LastBlockHeight > 0 {
		r.state.reconstructLastCommit(state)
	}

	// NOTE: The line below causes broadcastNewRoundStepRoutine() to broadcast a
	// NewRoundStepMessage.
	r.state.updateToState(ctx, state)

	r.mtx.Lock()
	r.waitSync = false
	close(r.readySignal)
	r.mtx.Unlock()

	r.Metrics.BlockSyncing.Set(0)
	r.Metrics.StateSyncing.Set(0)

	if skipWAL {
		r.state.doWALCatchup = false
	}

	if err := r.state.Start(ctx); err != nil {
		panic(fmt.Sprintf(`failed to start consensus state: %v

conS:
%+v

conR:
%+v`, err, r.state, r))
	}

	d := types.EventDataBlockSyncStatus{Complete: true, Height: state.LastBlockHeight}
	if err := r.eventBus.PublishEventBlockSyncStatus(ctx, d); err != nil {
		r.logger.Error("failed to emit the blocksync complete event", "err", err)
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
func (r *Reactor) GetPeerState(peerID types.NodeID) (*PeerState, bool) {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	ps, ok := r.peers[peerID]
	return ps, ok
}

func (r *Reactor) broadcastNewRoundStepMessage(ctx context.Context, rs *cstypes.RoundState) error {
	return r.stateCh.Send(ctx, p2p.Envelope{
		Broadcast: true,
		Message:   makeRoundStepMessage(rs),
	})
}

func (r *Reactor) broadcastNewValidBlockMessage(ctx context.Context, rs *cstypes.RoundState) error {
	psHeader := rs.ProposalBlockParts.Header()
	return r.stateCh.Send(ctx, p2p.Envelope{
		Broadcast: true,
		Message: &tmcons.NewValidBlock{
			Height:             rs.Height,
			Round:              rs.Round,
			BlockPartSetHeader: psHeader.ToProto(),
			BlockParts:         rs.ProposalBlockParts.BitArray().ToProto(),
			IsCommit:           rs.Step == cstypes.RoundStepCommit,
		},
	})
}

func (r *Reactor) broadcastHasVoteMessage(ctx context.Context, vote *types.Vote) error {
	return r.stateCh.Send(ctx, p2p.Envelope{
		Broadcast: true,
		Message: &tmcons.HasVote{
			Height: vote.Height,
			Round:  vote.Round,
			Type:   vote.Type,
			Index:  vote.ValidatorIndex,
		},
	})
}

// subscribeToBroadcastEvents subscribes for new round steps and votes using the
// internal pubsub defined in the consensus state to broadcast them to peers
// upon receiving.
func (r *Reactor) subscribeToBroadcastEvents() {
	err := r.state.evsw.AddListenerForEvent(
		listenerIDConsensus,
		types.EventNewRoundStepValue,
		func(ctx context.Context, data tmevents.EventData) error {
			if err := r.broadcastNewRoundStepMessage(ctx, data.(*cstypes.RoundState)); err != nil {
				return err
			}
			select {
			case r.state.onStopCh <- data.(*cstypes.RoundState):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			default:
				return nil
			}
		},
	)
	if err != nil {
		r.logger.Error("failed to add listener for events", "err", err)
	}

	err = r.state.evsw.AddListenerForEvent(
		listenerIDConsensus,
		types.EventValidBlockValue,
		func(ctx context.Context, data tmevents.EventData) error {
			return r.broadcastNewValidBlockMessage(ctx, data.(*cstypes.RoundState))
		},
	)
	if err != nil {
		r.logger.Error("failed to add listener for events", "err", err)
	}

	err = r.state.evsw.AddListenerForEvent(
		listenerIDConsensus,
		types.EventVoteValue,
		func(ctx context.Context, data tmevents.EventData) error {
			return r.broadcastHasVoteMessage(ctx, data.(*types.Vote))
		},
	)
	if err != nil {
		r.logger.Error("failed to add listener for events", "err", err)
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

func (r *Reactor) sendNewRoundStepMessage(ctx context.Context, peerID types.NodeID) error {
	return r.stateCh.Send(ctx, p2p.Envelope{
		To:      peerID,
		Message: makeRoundStepMessage(r.state.GetRoundState()),
	})
}

func (r *Reactor) gossipDataForCatchup(ctx context.Context, rs *cstypes.RoundState, prs *cstypes.PeerRoundState, ps *PeerState) {
	logger := r.logger.With("height", prs.Height).With("peer", ps.peerID)

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
		_ = r.dataCh.Send(ctx, p2p.Envelope{
			To: ps.peerID,
			Message: &tmcons.BlockPart{
				Height: prs.Height, // not our height, so it does not matter.
				Round:  prs.Round,  // not our height, so it does not matter
				Part:   *partProto,
			},
		})

		return
	}

	time.Sleep(r.state.config.PeerGossipSleepDuration)
}

func (r *Reactor) gossipDataRoutine(ctx context.Context, ps *PeerState) {
	logger := r.logger.With("peer", ps.peerID)

	timer := time.NewTimer(0)
	defer timer.Stop()

OUTER_LOOP:
	for {
		if !r.IsRunning() {
			return
		}

		select {
		case <-ctx.Done():
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
				if err := r.dataCh.Send(ctx, p2p.Envelope{
					To: ps.peerID,
					Message: &tmcons.BlockPart{
						Height: rs.Height, // this tells peer that this part applies to us
						Round:  rs.Round,  // this tells peer that this part applies to us
						Part:   *partProto,
					},
				}); err != nil {
					return
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

					timer.Reset(r.state.config.PeerGossipSleepDuration)
					select {
					case <-timer.C:
					case <-ctx.Done():
						return
					}
				} else {
					ps.InitProposalBlockParts(blockMeta.BlockID.PartSetHeader)
				}

				// Continue the loop since prs is a copy and not effected by this
				// initialization.
				continue OUTER_LOOP
			}

			r.gossipDataForCatchup(ctx, rs, prs, ps)
			continue OUTER_LOOP
		}

		// if height and round don't match, sleep
		if (rs.Height != prs.Height) || (rs.Round != prs.Round) {
			timer.Reset(r.state.config.PeerGossipSleepDuration)
			select {
			case <-timer.C:
			case <-ctx.Done():
				return
			}
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
				if err := r.dataCh.Send(ctx, p2p.Envelope{
					To: ps.peerID,
					Message: &tmcons.Proposal{
						Proposal: *propProto,
					},
				}); err != nil {
					return
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
				if err := r.dataCh.Send(ctx, p2p.Envelope{
					To: ps.peerID,
					Message: &tmcons.ProposalPOL{
						Height:           rs.Height,
						ProposalPolRound: rs.Proposal.POLRound,
						ProposalPol:      *pPolProto,
					},
				}); err != nil {
					return
				}
			}

			continue OUTER_LOOP
		}

		// nothing to do -- sleep
		timer.Reset(r.state.config.PeerGossipSleepDuration)
		select {
		case <-timer.C:
		case <-ctx.Done():
			return
		}
		continue OUTER_LOOP
	}
}

// pickSendVote picks a vote and sends it to the peer. It will return true if
// there is a vote to send and false otherwise.
func (r *Reactor) pickSendVote(ctx context.Context, ps *PeerState, votes types.VoteSetReader) (bool, error) {
	vote, ok := ps.PickVoteToSend(votes)
	if !ok {
		return false, nil
	}

	r.logger.Debug("sending vote message", "ps", ps, "vote", vote)
	if err := r.voteCh.Send(ctx, p2p.Envelope{
		To: ps.peerID,
		Message: &tmcons.Vote{
			Vote: vote.ToProto(),
		},
	}); err != nil {
		return false, err
	}

	if err := ps.SetHasVote(vote); err != nil {
		return false, err
	}

	return true, nil
}

func (r *Reactor) gossipVotesForHeight(
	ctx context.Context,
	rs *cstypes.RoundState,
	prs *cstypes.PeerRoundState,
	ps *PeerState,
) (bool, error) {
	logger := r.logger.With("height", prs.Height).With("peer", ps.peerID)

	// if there are lastCommits to send...
	if prs.Step == cstypes.RoundStepNewHeight {
		if ok, err := r.pickSendVote(ctx, ps, rs.LastCommit); err != nil {
			return false, err
		} else if ok {
			logger.Debug("picked rs.LastCommit to send")
			return true, nil

		}
	}

	// if there are POL prevotes to send...
	if prs.Step <= cstypes.RoundStepPropose && prs.Round != -1 && prs.Round <= rs.Round && prs.ProposalPOLRound != -1 {
		if polPrevotes := rs.Votes.Prevotes(prs.ProposalPOLRound); polPrevotes != nil {
			if ok, err := r.pickSendVote(ctx, ps, polPrevotes); err != nil {
				return false, err
			} else if ok {
				logger.Debug("picked rs.Prevotes(prs.ProposalPOLRound) to send", "round", prs.ProposalPOLRound)
				return true, nil
			}
		}
	}

	// if there are prevotes to send...
	if prs.Step <= cstypes.RoundStepPrevoteWait && prs.Round != -1 && prs.Round <= rs.Round {
		if ok, err := r.pickSendVote(ctx, ps, rs.Votes.Prevotes(prs.Round)); err != nil {
			return false, err
		} else if ok {
			logger.Debug("picked rs.Prevotes(prs.Round) to send", "round", prs.Round)
			return true, nil
		}
	}

	// if there are precommits to send...
	if prs.Step <= cstypes.RoundStepPrecommitWait && prs.Round != -1 && prs.Round <= rs.Round {
		if ok, err := r.pickSendVote(ctx, ps, rs.Votes.Precommits(prs.Round)); err != nil {
			return false, err
		} else if ok {
			logger.Debug("picked rs.Precommits(prs.Round) to send", "round", prs.Round)
			return true, nil
		}
	}

	// if there are prevotes to send...(which are needed because of validBlock mechanism)
	if prs.Round != -1 && prs.Round <= rs.Round {
		if ok, err := r.pickSendVote(ctx, ps, rs.Votes.Prevotes(prs.Round)); err != nil {
			return false, err
		} else if ok {
			logger.Debug("picked rs.Prevotes(prs.Round) to send", "round", prs.Round)
			return true, nil
		}
	}

	// if there are POLPrevotes to send...
	if prs.ProposalPOLRound != -1 {
		if polPrevotes := rs.Votes.Prevotes(prs.ProposalPOLRound); polPrevotes != nil {
			if ok, err := r.pickSendVote(ctx, ps, polPrevotes); err != nil {
				return false, err
			} else if ok {
				logger.Debug("picked rs.Prevotes(prs.ProposalPOLRound) to send", "round", prs.ProposalPOLRound)
				return true, nil
			}
		}
	}

	return false, nil
}

func (r *Reactor) gossipVotesRoutine(ctx context.Context, ps *PeerState) {
	logger := r.logger.With("peer", ps.peerID)

	// XXX: simple hack to throttle logs upon sleep
	logThrottle := 0

	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		if !r.IsRunning() {
			return
		}

		select {
		case <-ctx.Done():
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
			if ok, err := r.gossipVotesForHeight(ctx, rs, prs, ps); err != nil {
				return
			} else if ok {
				continue
			}
		}

		// special catchup logic -- if peer is lagging by height 1, send LastCommit
		if prs.Height != 0 && rs.Height == prs.Height+1 {
			if ok, err := r.pickSendVote(ctx, ps, rs.LastCommit); err != nil {
				return
			} else if ok {
				logger.Debug("picked rs.LastCommit to send", "height", prs.Height)
				continue
			}
		}

		// catchup logic -- if peer is lagging by more than 1, send Commit
		blockStoreBase := r.state.blockStore.Base()
		if blockStoreBase > 0 && prs.Height != 0 && rs.Height >= prs.Height+2 && prs.Height >= blockStoreBase {
			// Load the block commit for prs.Height, which contains precommit
			// signatures for prs.Height.
			if commit := r.state.blockStore.LoadBlockCommit(prs.Height); commit != nil {
				if ok, err := r.pickSendVote(ctx, ps, commit); err != nil {
					return
				} else if ok {
					logger.Debug("picked Catchup commit to send", "height", prs.Height)
					continue
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

		timer.Reset(r.state.config.PeerGossipSleepDuration)
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}
}

// NOTE: `queryMaj23Routine` has a simple crude design since it only comes
// into play for liveness when there's a signature DDoS attack happening.
func (r *Reactor) queryMaj23Routine(ctx context.Context, ps *PeerState) {
	timer := time.NewTimer(0)
	defer timer.Stop()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for {
		if !ps.IsRunning() {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		if !ps.IsRunning() {
			return
		}

		rs := r.state.GetRoundState()
		prs := ps.GetRoundState()
		// TODO create more reliable coppies of these
		// structures so the following go routines don't race

		wg := &sync.WaitGroup{}

		if rs.Height == prs.Height {
			wg.Add(1)
			go func(rs *cstypes.RoundState, prs *cstypes.PeerRoundState) {
				defer wg.Done()

				// maybe send Height/Round/Prevotes
				if maj23, ok := rs.Votes.Prevotes(prs.Round).TwoThirdsMajority(); ok {
					if err := r.stateCh.Send(ctx, p2p.Envelope{
						To: ps.peerID,
						Message: &tmcons.VoteSetMaj23{
							Height:  prs.Height,
							Round:   prs.Round,
							Type:    tmproto.PrevoteType,
							BlockID: maj23.ToProto(),
						},
					}); err != nil {
						cancel()
					}
				}
			}(rs, prs)

			if prs.ProposalPOLRound >= 0 {
				wg.Add(1)
				go func(rs *cstypes.RoundState, prs *cstypes.PeerRoundState) {
					defer wg.Done()

					// maybe send Height/Round/ProposalPOL
					if maj23, ok := rs.Votes.Prevotes(prs.ProposalPOLRound).TwoThirdsMajority(); ok {
						if err := r.stateCh.Send(ctx, p2p.Envelope{
							To: ps.peerID,
							Message: &tmcons.VoteSetMaj23{
								Height:  prs.Height,
								Round:   prs.ProposalPOLRound,
								Type:    tmproto.PrevoteType,
								BlockID: maj23.ToProto(),
							},
						}); err != nil {
							cancel()
						}
					}
				}(rs, prs)
			}

			wg.Add(1)
			go func(rs *cstypes.RoundState, prs *cstypes.PeerRoundState) {
				defer wg.Done()

				// maybe send Height/Round/Precommits
				if maj23, ok := rs.Votes.Precommits(prs.Round).TwoThirdsMajority(); ok {
					if err := r.stateCh.Send(ctx, p2p.Envelope{
						To: ps.peerID,
						Message: &tmcons.VoteSetMaj23{
							Height:  prs.Height,
							Round:   prs.Round,
							Type:    tmproto.PrecommitType,
							BlockID: maj23.ToProto(),
						},
					}); err != nil {
						cancel()
					}
				}
			}(rs, prs)
		}

		// Little point sending LastCommitRound/LastCommit, these are fleeting and
		// non-blocking.
		if prs.CatchupCommitRound != -1 && prs.Height > 0 {
			wg.Add(1)
			go func(rs *cstypes.RoundState, prs *cstypes.PeerRoundState) {
				defer wg.Done()

				if prs.Height <= r.state.blockStore.Height() && prs.Height >= r.state.blockStore.Base() {
					// maybe send Height/CatchupCommitRound/CatchupCommit
					if commit := r.state.LoadCommit(prs.Height); commit != nil {
						if err := r.stateCh.Send(ctx, p2p.Envelope{
							To: ps.peerID,
							Message: &tmcons.VoteSetMaj23{
								Height:  prs.Height,
								Round:   commit.Round,
								Type:    tmproto.PrecommitType,
								BlockID: commit.BlockID.ToProto(),
							},
						}); err != nil {
							cancel()
						}
					}
				}
			}(rs, prs)
		}

		waitSignal := make(chan struct{})
		go func() { defer close(waitSignal); wg.Wait() }()

		select {
		case <-waitSignal:
			timer.Reset(r.state.config.PeerQueryMaj23SleepDuration)
		case <-ctx.Done():
			return
		}
	}
}

// processPeerUpdate process a peer update message. For new or reconnected peers,
// we create a peer state if one does not exist for the peer, which should always
// be the case, and we spawn all the relevant goroutine to broadcast messages to
// the peer. During peer removal, we remove the peer for our set of peers and
// signal to all spawned goroutines to gracefully exit in a non-blocking manner.
func (r *Reactor) processPeerUpdate(ctx context.Context, peerUpdate p2p.PeerUpdate) {
	r.logger.Debug("received peer update", "peer", peerUpdate.NodeID, "status", peerUpdate.Status)

	r.mtx.Lock()
	defer r.mtx.Unlock()

	switch peerUpdate.Status {
	case p2p.PeerStatusUp:
		// Do not allow starting new broadcasting goroutines after reactor shutdown
		// has been initiated. This can happen after we've manually closed all
		// peer goroutines, but the router still sends in-flight peer updates.
		if !r.IsRunning() {
			return
		}

		ps, ok := r.peers[peerUpdate.NodeID]
		if !ok {
			ps = NewPeerState(r.logger, peerUpdate.NodeID)
			r.peers[peerUpdate.NodeID] = ps
		}

		if !ps.IsRunning() {
			// Set the peer state's closer to signal to all spawned goroutines to exit
			// when the peer is removed. We also set the running state to ensure we
			// do not spawn multiple instances of the same goroutines and finally we
			// set the waitgroup counter so we know when all goroutines have exited.
			ps.SetRunning(true)
			ctx, ps.cancel = context.WithCancel(ctx)

			go func() {
				select {
				case <-ctx.Done():
					return
				case <-r.readySignal:
				}
				// do nothing if the peer has
				// stopped while we've been waiting.
				if !ps.IsRunning() {
					return
				}
				// start goroutines for this peer
				go r.gossipDataRoutine(ctx, ps)
				go r.gossipVotesRoutine(ctx, ps)
				go r.queryMaj23Routine(ctx, ps)

				// Send our state to the peer. If we're block-syncing, broadcast a
				// RoundStepMessage later upon SwitchToConsensus().
				if !r.WaitSync() {
					go func() { _ = r.sendNewRoundStepMessage(ctx, ps.peerID) }()
				}

			}()
		}

	case p2p.PeerStatusDown:
		ps, ok := r.peers[peerUpdate.NodeID]
		if ok && ps.IsRunning() {
			// signal to all spawned goroutines for the peer to gracefully exit
			go func() {
				r.mtx.Lock()
				delete(r.peers, peerUpdate.NodeID)
				r.mtx.Unlock()

				ps.SetRunning(false)
				ps.cancel()
			}()
		}
	}
}

// handleStateMessage handles envelopes sent from peers on the StateChannel.
// An error is returned if the message is unrecognized or if validation fails.
// If we fail to find the peer state for the envelope sender, we perform a no-op
// and return. This can happen when we process the envelope after the peer is
// removed.
func (r *Reactor) handleStateMessage(ctx context.Context, envelope *p2p.Envelope, msgI Message) error {
	ps, ok := r.GetPeerState(envelope.From)
	if !ok || ps == nil {
		r.logger.Debug("failed to find peer state", "peer", envelope.From, "ch_id", "StateChannel")
		return nil
	}

	switch msg := envelope.Message.(type) {
	case *tmcons.NewRoundStep:
		r.state.mtx.RLock()
		initialHeight := r.state.state.InitialHeight
		r.state.mtx.RUnlock()

		if err := msgI.(*NewRoundStepMessage).ValidateHeight(initialHeight); err != nil {
			r.logger.Error("peer sent us an invalid msg", "msg", msg, "err", err)
			return err
		}

		ps.ApplyNewRoundStepMessage(msgI.(*NewRoundStepMessage))

	case *tmcons.NewValidBlock:
		ps.ApplyNewValidBlockMessage(msgI.(*NewValidBlockMessage))

	case *tmcons.HasVote:
		if err := ps.ApplyHasVoteMessage(msgI.(*HasVoteMessage)); err != nil {
			r.logger.Error("applying HasVote message", "msg", msg, "err", err)
			return err
		}
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

		if err := r.voteSetBitsCh.Send(ctx, p2p.Envelope{
			To:      envelope.From,
			Message: eMsg,
		}); err != nil {
			return err
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
func (r *Reactor) handleDataMessage(ctx context.Context, envelope *p2p.Envelope, msgI Message) error {
	logger := r.logger.With("peer", envelope.From, "ch_id", "DataChannel")

	ps, ok := r.GetPeerState(envelope.From)
	if !ok || ps == nil {
		r.logger.Debug("failed to find peer state")
		return nil
	}

	if r.WaitSync() {
		logger.Info("ignoring message received during sync", "msg", fmt.Sprintf("%T", msgI))
		return nil
	}

	switch msg := envelope.Message.(type) {
	case *tmcons.Proposal:
		pMsg := msgI.(*ProposalMessage)

		ps.SetHasProposal(pMsg.Proposal)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case r.state.peerMsgQueue <- msgInfo{pMsg, envelope.From, tmtime.Now()}:
		}
	case *tmcons.ProposalPOL:
		ps.ApplyProposalPOLMessage(msgI.(*ProposalPOLMessage))
	case *tmcons.BlockPart:
		bpMsg := msgI.(*BlockPartMessage)

		ps.SetHasProposalBlockPart(bpMsg.Height, bpMsg.Round, int(bpMsg.Part.Index))
		r.Metrics.BlockParts.With("peer_id", string(envelope.From)).Add(1)
		select {
		case r.state.peerMsgQueue <- msgInfo{bpMsg, envelope.From, tmtime.Now()}:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}

	default:
		return fmt.Errorf("received unknown message on DataChannel: %T", msg)
	}

	return nil
}

// handleVoteMessage handles envelopes sent from peers on the VoteChannel. If we
// fail to find the peer state for the envelope sender, we perform a no-op and
// return. This can happen when we process the envelope after the peer is
// removed.
func (r *Reactor) handleVoteMessage(ctx context.Context, envelope *p2p.Envelope, msgI Message) error {
	logger := r.logger.With("peer", envelope.From, "ch_id", "VoteChannel")

	ps, ok := r.GetPeerState(envelope.From)
	if !ok || ps == nil {
		r.logger.Debug("failed to find peer state")
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
		if err := ps.SetHasVote(vMsg.Vote); err != nil {
			return err
		}

		select {
		case r.state.peerMsgQueue <- msgInfo{vMsg, envelope.From, tmtime.Now()}:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	default:
		return fmt.Errorf("received unknown message on VoteChannel: %T", msg)
	}
}

// handleVoteSetBitsMessage handles envelopes sent from peers on the
// VoteSetBitsChannel. If we fail to find the peer state for the envelope sender,
// we perform a no-op and return. This can happen when we process the envelope
// after the peer is removed.
func (r *Reactor) handleVoteSetBitsMessage(ctx context.Context, envelope *p2p.Envelope, msgI Message) error {
	logger := r.logger.With("peer", envelope.From, "ch_id", "VoteSetBitsChannel")

	ps, ok := r.GetPeerState(envelope.From)
	if !ok || ps == nil {
		r.logger.Debug("failed to find peer state")
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
// NOTE: We process these messages even when we're block syncing. Messages affect
// either a peer state or the consensus state. Peer state updates can happen in
// parallel, but processing of proposals, block parts, and votes are ordered by
// the p2p channel.
//
// NOTE: We block on consensus state for proposals, block parts, and votes.
func (r *Reactor) handleMessage(ctx context.Context, chID p2p.ChannelID, envelope *p2p.Envelope) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("panic in processing message: %v", e)
			r.logger.Error(
				"recovering from processing message panic",
				"err", err,
				"stack", string(debug.Stack()),
			)
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

	r.logger.Debug("received message", "ch_id", chID, "message", msgI, "peer", envelope.From)

	switch chID {
	case StateChannel:
		err = r.handleStateMessage(ctx, envelope, msgI)

	case DataChannel:
		err = r.handleDataMessage(ctx, envelope, msgI)

	case VoteChannel:
		err = r.handleVoteMessage(ctx, envelope, msgI)

	case VoteSetBitsChannel:
		err = r.handleVoteSetBitsMessage(ctx, envelope, msgI)

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
func (r *Reactor) processStateCh(ctx context.Context) {
	iter := r.stateCh.Receive(ctx)
	for iter.Next(ctx) {
		envelope := iter.Envelope()
		if err := r.handleMessage(ctx, r.stateCh.ID, envelope); err != nil {
			r.logger.Error("failed to process message", "ch_id", r.stateCh.ID, "envelope", envelope, "err", err)
			if serr := r.stateCh.SendError(ctx, p2p.PeerError{
				NodeID: envelope.From,
				Err:    err,
			}); serr != nil {
				return
			}
		}
	}
}

// processDataCh initiates a blocking process where we listen for and handle
// envelopes on the DataChannel. Any error encountered during message
// execution will result in a PeerError being sent on the DataChannel. When
// the reactor is stopped, we will catch the signal and close the p2p Channel
// gracefully.
func (r *Reactor) processDataCh(ctx context.Context) {
	iter := r.dataCh.Receive(ctx)
	for iter.Next(ctx) {
		envelope := iter.Envelope()
		if err := r.handleMessage(ctx, r.dataCh.ID, envelope); err != nil {
			r.logger.Error("failed to process message", "ch_id", r.dataCh.ID, "envelope", envelope, "err", err)
			if serr := r.dataCh.SendError(ctx, p2p.PeerError{
				NodeID: envelope.From,
				Err:    err,
			}); serr != nil {
				return
			}
		}
	}
}

// processVoteCh initiates a blocking process where we listen for and handle
// envelopes on the VoteChannel. Any error encountered during message
// execution will result in a PeerError being sent on the VoteChannel. When
// the reactor is stopped, we will catch the signal and close the p2p Channel
// gracefully.
func (r *Reactor) processVoteCh(ctx context.Context) {
	iter := r.voteCh.Receive(ctx)
	for iter.Next(ctx) {
		envelope := iter.Envelope()
		if err := r.handleMessage(ctx, r.voteCh.ID, envelope); err != nil {
			r.logger.Error("failed to process message", "ch_id", r.voteCh.ID, "envelope", envelope, "err", err)
			if serr := r.voteCh.SendError(ctx, p2p.PeerError{
				NodeID: envelope.From,
				Err:    err,
			}); serr != nil {
				return
			}
		}
	}
}

// processVoteCh initiates a blocking process where we listen for and handle
// envelopes on the VoteSetBitsChannel. Any error encountered during message
// execution will result in a PeerError being sent on the VoteSetBitsChannel.
// When the reactor is stopped, we will catch the signal and close the p2p
// Channel gracefully.
func (r *Reactor) processVoteSetBitsCh(ctx context.Context) {
	iter := r.voteSetBitsCh.Receive(ctx)
	for iter.Next(ctx) {
		envelope := iter.Envelope()

		if err := r.handleMessage(ctx, r.voteSetBitsCh.ID, envelope); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return
			}

			r.logger.Error("failed to process message", "ch_id", r.voteSetBitsCh.ID, "envelope", envelope, "err", err)
			if serr := r.voteSetBitsCh.SendError(ctx, p2p.PeerError{
				NodeID: envelope.From,
				Err:    err,
			}); serr != nil {
				return
			}
		}
	}
}

// processPeerUpdates initiates a blocking process where we listen for and handle
// PeerUpdate messages. When the reactor is stopped, we will catch the signal and
// close the p2p PeerUpdatesCh gracefully.
func (r *Reactor) processPeerUpdates(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case peerUpdate := <-r.peerUpdates.Updates():
			r.processPeerUpdate(ctx, peerUpdate)
		}
	}
}

func (r *Reactor) peerStatsRoutine(ctx context.Context) {
	for {
		if !r.IsRunning() {
			r.logger.Info("stopping peerStatsRoutine")
			return
		}

		select {
		case msg := <-r.state.statsMsgQueue:
			ps, ok := r.GetPeerState(msg.PeerID)
			if !ok || ps == nil {
				r.logger.Debug("attempt to update stats for non-existent peer", "peer", msg.PeerID)
				continue
			}

			switch msg.Msg.(type) {
			case *VoteMessage:
				if numVotes := ps.RecordVote(); numVotes%votesToContributeToBecomeGoodPeer == 0 {
					r.peerUpdates.SendUpdate(ctx, p2p.PeerUpdate{
						NodeID: msg.PeerID,
						Status: p2p.PeerStatusGood,
					})
				}

			case *BlockPartMessage:
				if numParts := ps.RecordBlockPart(); numParts%blocksToContributeToBecomeGoodPeer == 0 {
					r.peerUpdates.SendUpdate(ctx, p2p.PeerUpdate{
						NodeID: msg.PeerID,
						Status: p2p.PeerStatusGood,
					})
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (r *Reactor) GetConsensusState() *State {
	return r.state
}

func (r *Reactor) SetStateSyncingMetrics(v float64) {
	r.Metrics.StateSyncing.Set(v)
}

func (r *Reactor) SetBlockSyncingMetrics(v float64) {
	r.Metrics.BlockSyncing.Set(v)
}
