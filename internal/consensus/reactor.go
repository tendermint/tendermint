package consensus

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"

	cstypes "github.com/tendermint/tendermint/internal/consensus/types"
	"github.com/tendermint/tendermint/internal/eventbus"
	tmstrings "github.com/tendermint/tendermint/internal/libs/strings"
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
			Name:                "state",
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
			Name:                "data",
		},
		VoteChannel: {
			ID:                  VoteChannel,
			MessageType:         new(tmcons.Message),
			Priority:            10,
			SendQueueCapacity:   64,
			RecvBufferCapacity:  4096,
			RecvMessageCapacity: maxMsgSize,
			Name:                "vote",
		},
		VoteSetBitsChannel: {
			ID:                  VoteSetBitsChannel,
			MessageType:         new(tmcons.Message),
			Priority:            5,
			SendQueueCapacity:   8,
			RecvBufferCapacity:  128,
			RecvMessageCapacity: maxMsgSize,
			Name:                "voteSet",
		},
	}
}

const (
	StateChannel       = p2p.ChannelID(0x20)
	DataChannel        = p2p.ChannelID(0x21)
	VoteChannel        = p2p.ChannelID(0x22)
	VoteSetBitsChannel = p2p.ChannelID(0x23)

	maxMsgSize = 1048576 // 1MB; NOTE: keep in sync with types.PartSet sizes.

	blocksToContributeToBecomeGoodPeer  = 10000
	votesToContributeToBecomeGoodPeer   = 10000
	commitsToContributeToBecomeGoodPeer = 10000

	listenerIDConsensus = "consensus-reactor"
)

var errReactorClosed = errors.New("reactor is closed")

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
	rs          *cstypes.RoundState
	readySignal chan struct{} // closed when the node is ready to start consensus

	peerEvents p2p.PeerEventSubscriber
	chCreator  p2p.ChannelCreator
}

// NewReactor returns a reference to a new consensus reactor, which implements
// the service.Service interface. It accepts a logger, consensus state, references
// to relevant p2p Channels and a channel to listen for peer updates on. The
// reactor will close all p2p Channels when stopping.
func NewReactor(
	logger log.Logger,
	cs *State,
	channelCreator p2p.ChannelCreator,
	peerEvents p2p.PeerEventSubscriber,
	eventBus *eventbus.EventBus,
	waitSync bool,
	metrics *Metrics,
) *Reactor {
	r := &Reactor{
		logger:      logger,
		state:       cs,
		waitSync:    waitSync,
		rs:          cs.GetRoundState(),
		peers:       make(map[types.NodeID]*PeerState),
		eventBus:    eventBus,
		Metrics:     metrics,
		peerEvents:  peerEvents,
		chCreator:   channelCreator,
		readySignal: make(chan struct{}),
	}
	r.BaseService = *service.NewBaseService(logger, "Consensus", r)

	if !r.waitSync {
		close(r.readySignal)
	}

	return r
}

type channelBundle struct {
	state  p2p.Channel
	data   p2p.Channel
	vote   p2p.Channel
	votSet p2p.Channel
}

// OnStart starts separate go routines for each p2p Channel and listens for
// envelopes on each. In addition, it also listens for peer updates and handles
// messages on that p2p channel accordingly. The caller must be sure to execute
// OnStop to ensure the outbound p2p Channels are closed.
func (r *Reactor) OnStart(ctx context.Context) error {
	r.logger.Debug("consensus wait sync", "wait_sync", r.WaitSync())

	peerUpdates := r.peerEvents(ctx)

	var chBundle channelBundle
	var err error

	chans := getChannelDescriptors()
	chBundle.state, err = r.chCreator(ctx, chans[StateChannel])
	if err != nil {
		return err
	}

	chBundle.data, err = r.chCreator(ctx, chans[DataChannel])
	if err != nil {
		return err
	}

	chBundle.vote, err = r.chCreator(ctx, chans[VoteChannel])
	if err != nil {
		return err
	}

	chBundle.votSet, err = r.chCreator(ctx, chans[VoteSetBitsChannel])
	if err != nil {
		return err
	}

	// start routine that computes peer statistics for evaluating peer quality
	//
	// TODO: Evaluate if we need this to be synchronized via WaitGroup as to not
	// leak the goroutine when stopping the reactor.
	go r.peerStatsRoutine(ctx, peerUpdates)

	r.subscribeToBroadcastEvents(ctx, chBundle.state)

	if !r.WaitSync() {
		if err := r.state.Start(ctx); err != nil {
			return err
		}
	} else if err := r.state.updateStateFromStore(); err != nil {
		return err
	}

	go r.updateRoundStateRoutine(ctx)

	go r.processMsgCh(ctx, chBundle.state, chBundle)
	go r.processMsgCh(ctx, chBundle.data, chBundle)
	go r.processMsgCh(ctx, chBundle.vote, chBundle)
	go r.processMsgCh(ctx, chBundle.votSet, chBundle)
	go r.processPeerUpdates(ctx, peerUpdates, chBundle)

	return nil
}

// OnStop stops the reactor by signaling to all spawned goroutines to exit and
// blocking until they all exit, as well as unsubscribing from events and stopping
// state.
func (r *Reactor) OnStop() {
	r.state.Stop()

	if !r.WaitSync() {
		r.state.Wait()
	}
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
	r.state.updateToState(state, nil)
	if err := r.state.Start(ctx); err != nil {
		panic(fmt.Sprintf(`failed to start consensus state: %v

conS:
%+v

conR:
%+v`, err, r.state, r))
	}

	r.mtx.Lock()
	r.waitSync = false
	close(r.readySignal)
	r.mtx.Unlock()

	r.Metrics.BlockSyncing.Set(0)
	r.Metrics.StateSyncing.Set(0)

	if skipWAL {
		r.state.doWALCatchup = false
	}

	d := types.EventDataBlockSyncStatus{Complete: true, Height: state.LastBlockHeight}
	if err := r.eventBus.PublishEventBlockSyncStatus(d); err != nil {
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

// GetPeerState returns PeerState for a given NodeID.
func (r *Reactor) GetPeerState(peerID types.NodeID) (*PeerState, bool) {
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	ps, ok := r.peers[peerID]
	return ps, ok
}

// subscribeToBroadcastEvents subscribes for new round steps and votes using the
// internal pubsub defined in the consensus state to broadcast them to peers
// upon receiving.
func (r *Reactor) subscribeToBroadcastEvents(ctx context.Context, stateCh p2p.Channel) {
	onStopCh := r.state.getOnStopCh()

	err := r.state.evsw.AddListenerForEvent(
		listenerIDConsensus,
		types.EventNewRoundStepValue,
		func(data tmevents.EventData) error {
			rs := data.(*cstypes.RoundState)
			err := r.broadcast(ctx, stateCh, rs.NewRoundStepMessage())
			if err != nil {
				return err
			}
			r.logResult(err, r.logger, "broadcasting round step message", "height", rs.Height, "round", rs.Round)
			select {
			case onStopCh <- data.(*cstypes.RoundState):
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
		func(data tmevents.EventData) error {
			rs := data.(*cstypes.RoundState)
			err := r.broadcast(ctx, stateCh, rs.NewValidBlockMessage())
			r.logResult(err, r.logger, "broadcasting new valid block message", "height", rs.Height, "round", rs.Round)
			return err
		},
	)
	if err != nil {
		r.logger.Error("failed to add listener for events", "err", err)
	}

	err = r.state.evsw.AddListenerForEvent(
		listenerIDConsensus,
		types.EventVoteValue,
		func(data tmevents.EventData) error {
			vote := data.(*types.Vote)
			err := r.broadcast(ctx, stateCh, vote.HasVoteMessage())
			r.logResult(err, r.logger, "broadcasting HasVote message", "height", vote.Height, "round", vote.Round)
			return err
		},
	)
	if err != nil {
		r.logger.Error("failed to add listener for events", "err", err)
	}

	if err := r.state.evsw.AddListenerForEvent(listenerIDConsensus, types.EventCommitValue,
		func(data tmevents.EventData) error {
			commit := data.(*types.Commit)
			err := r.broadcast(ctx, stateCh, commit.HasCommitMessage())
			r.logResult(err, r.logger, "broadcasting HasVote message", "height", commit.Height, "round", commit.Round)
			return err
		}); err != nil {
		r.logger.Error("Error adding listener for events", "err", err)
	}
}

func (r *Reactor) updateRoundStateRoutine(ctx context.Context) {
	t := time.NewTicker(100 * time.Microsecond)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			rs := r.state.GetRoundState()
			r.mtx.Lock()
			r.rs = rs
			r.mtx.Unlock()
		}
	}
}

func (r *Reactor) getRoundState() *cstypes.RoundState {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.rs
}

func (r *Reactor) gossipDataForCatchup(ctx context.Context, rs *cstypes.RoundState, prs *cstypes.PeerRoundState, ps *PeerState, chans channelBundle) {
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

		if err := r.sendProposalBlockPart(ctx, chans.data, ps, part, prs.Height, prs.Round); err != nil {
			logger.Error("cannot send proposal block part to the peer", "error", err)
			time.Sleep(r.state.config.PeerGossipSleepDuration)
		}

		return
	}

	// block parts already delivered -  send commits?
	if rs.Height > 0 && !prs.HasCommit {
		if err := r.gossipCommit(ctx, chans.vote, rs, ps, prs); err != nil {
			logger.Error("cannot gossip commit to peer", "error", err)
		} else {
			time.Sleep(r.state.config.PeerGossipSleepDuration)
		}

		return
	}

	time.Sleep(r.state.config.PeerGossipSleepDuration)
}

func (r *Reactor) gossipDataRoutine(ctx context.Context, ps *PeerState, dataCh p2p.Channel, chans channelBundle) {
	logger := r.logger.With("peer", ps.peerID)

	timer := time.NewTimer(0)
	defer timer.Stop()

OUTER_LOOP:
	for {
		if !r.IsRunning() {
			return
		}

		timer.Reset(r.state.config.PeerGossipSleepDuration)

		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		rs := r.getRoundState()
		prs := ps.GetRoundState()

		isValidator := r.isValidator(ps.GetProTxHash())

		// Send proposal Block parts?
		if (isValidator && rs.ProposalBlockParts.HasHeader(prs.ProposalBlockPartSetHeader)) ||
			(prs.HasCommit && rs.ProposalBlockParts != nil) {
			if !isValidator && prs.HasCommit && prs.ProposalBlockParts == nil {
				// We can assume if they have the commit then they should have the same part set header
				ps.UpdateRoundState(func(prs *cstypes.PeerRoundState) {
					prs.ProposalBlockPartSetHeader = rs.ProposalBlockParts.Header()
					prs.ProposalBlockParts = bits.NewBitArray(int(rs.ProposalBlockParts.Header().Total))
				})
			}
			if index, ok := rs.ProposalBlockParts.BitArray().Sub(prs.ProposalBlockParts.Copy()).PickRandom(); ok {
				part := rs.ProposalBlockParts.GetPart(index)

				if err := r.sendProposalBlockPart(ctx, chans.data, ps, part, prs.Height, prs.Round); err != nil {
					logger.Error("cannot send proposal block part to the peer", "error", err)
					time.Sleep(r.state.config.PeerGossipSleepDuration)
				}
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
				} else {
					ps.InitProposalBlockParts(blockMeta.BlockID.PartSetHeader)
				}

				// Continue the loop since prs is a copy and not effected by this
				// initialization.
				continue OUTER_LOOP
			}

			r.gossipDataForCatchup(ctx, rs, prs, ps, chans)
			continue OUTER_LOOP
		}

		// if height and round don't match, sleep
		if (rs.Height != prs.Height) || (rs.Round != prs.Round) {
			continue OUTER_LOOP
		}

		// By here, height and round match.
		// Proposal block parts were already matched and sent if any were wanted.
		// (These can match on hash so the round doesn't matter)
		// Now consider sending other things, like the Proposal itself.

		// Send Proposal && ProposalPOL BitArray?
		if rs.Proposal != nil && !prs.Proposal && isValidator {
			// Proposal: share the proposal metadata with peer.
			{
				propProto := rs.Proposal.ToProto()
				err := r.send(ctx, ps, dataCh, &tmcons.Proposal{
					Proposal: *propProto,
				})
				r.logResult(err, logger, "sending proposal", "height", prs.Height, "round", prs.Round)

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

				err := r.send(ctx, ps, dataCh, &tmcons.ProposalPOL{
					Height:           rs.Height,
					ProposalPolRound: rs.Proposal.POLRound,
					ProposalPol:      *pPolProto,
				})
				r.logResult(err, logger, "sending POL", "height", prs.Height, "round", prs.Round)
			}
		}
	}
}

func (r *Reactor) sendProposalBlockPart(ctx context.Context, dataCh p2p.Channel, ps *PeerState, part *types.Part, height int64, round int32) error {
	partProto, err := part.ToProto()
	if err != nil {
		return fmt.Errorf("failed to convert block part to proto, error: %w", err)
	}

	err = r.send(ctx, ps, dataCh, &tmcons.BlockPart{
		Height: height, // not our height, so it does not matter
		Round:  round,  // not our height, so it does not matter
		Part:   *partProto,
	})

	r.logResult(err, r.logger, "sending block part for catchup", "round", round, "height", height, "index", part.Index, "peer", ps.peerID)
	if err == nil {
		ps.SetHasProposalBlockPart(height, round, int(part.Index))
	}
	return nil
}

// pickSendVote picks a vote and sends it to the peer. It will return true if
// there is a vote to send and false otherwise.
func (r *Reactor) pickSendVote(ctx context.Context, ps *PeerState, votes types.VoteSetReader, voteCh p2p.Channel) (bool, error) {
	vote, ok := ps.PickVoteToSend(votes)
	if !ok {
		return false, nil
	}

	psJSON, _ := ps.ToJSON()
	voteProto := vote.ToProto()
	err := r.send(ctx, ps, voteCh, &tmcons.Vote{
		Vote: voteProto,
	})
	r.logResult(
		err,
		r.logger,
		"sending vote message",
		"ps", psJSON,
		"peer", ps.peerID,
		"vote", vote,
		"peer_proTxHash", ps.GetProTxHash().ShortString(),
		"val_proTxHash", vote.ValidatorProTxHash.ShortString(),
		"height", vote.Height,
		"round", vote.Round,
		"size", voteProto.Size(),
		"isValidator", r.isValidator(vote.ValidatorProTxHash),
	)
	if err != nil {
		return false, err
	}
	if err := ps.SetHasVote(vote); err != nil {
		return false, err
	}

	return true, nil
}

func (r *Reactor) sendCommit(ctx context.Context, ps *PeerState, commit *types.Commit, voteCh p2p.Channel) error {
	if commit == nil {
		return fmt.Errorf("attempt to send nil commit to peer %s", ps.peerID)
	}
	protoCommit := commit.ToProto()

	err := r.send(ctx, ps, voteCh, &tmcons.Commit{
		Commit: protoCommit,
	})
	r.logResult(err, r.logger, "sending commit message", "height", commit.Height, "round", commit.Round, "peer", ps.peerID)
	return err
}

// send sends a message to provided channel.
// If to is nil, message will be broadcasted.
func (r *Reactor) send(ctx context.Context, ps *PeerState, channel p2p.Channel, msg proto.Message) error {
	select {
	case <-ctx.Done():
		return errReactorClosed
	default:
		return channel.Send(ctx, p2p.Envelope{
			To:      ps.peerID,
			Message: msg,
		})
	}
}

// broadcast sends a broadcast message to all peers connected to the `channel`.
func (r *Reactor) broadcast(ctx context.Context, channel p2p.Channel, msg proto.Message) error {
	select {
	case <-ctx.Done():
		return errReactorClosed
	default:
		return channel.Send(ctx, p2p.Envelope{
			Broadcast: true,
			Message:   msg,
		})
	}
}

// logResult creates a log that depends on value of err
func (r *Reactor) logResult(err error, logger log.Logger, message string, keyvals ...interface{}) bool {
	if err != nil {
		logger.Debug("error "+message, append(keyvals, "error", err))
		return false
	}
	logger.Debug("success "+message, keyvals...)
	return true
}

//func (r *Reactor) gossipVotesForHeight(rs *cstypes.RoundState, prs *cstypes.PeerRoundState, ps *PeerState) bool {
//	logger := r.Logger.With("height", prs.Height).With("peer", ps.peerID)
func (r *Reactor) gossipVotesForHeight(
	ctx context.Context,
	rs *cstypes.RoundState,
	prs *cstypes.PeerRoundState,
	ps *PeerState,
	voteCh p2p.Channel,
) (bool, error) {
	logger := r.logger.With("height", prs.Height).With("peer", ps.peerID)

	// if there are lastPrecommits to send...
	if prs.Step == cstypes.RoundStepNewHeight {
		if ok, err := r.pickSendVote(ctx, ps, rs.LastPrecommits, voteCh); err != nil {
			logger.Debug("picked previous precommit vote to send")
			return false, err
		} else if ok {
			logger.Debug("picked rs.LastPrecommits to send")
			return true, nil
		}
	}

	// if there are POL prevotes to send...
	if prs.Step <= cstypes.RoundStepPropose && prs.Round != -1 && prs.Round <= rs.Round && prs.ProposalPOLRound != -1 {
		if polPrevotes := rs.Votes.Prevotes(prs.ProposalPOLRound); polPrevotes != nil {
			if ok, err := r.pickSendVote(ctx, ps, polPrevotes, voteCh); err != nil {
				return false, err
			} else if ok {
				logger.Debug("picked rs.Prevotes(prs.ProposalPOLRound) to send", "round", prs.ProposalPOLRound)
				return true, nil
			}
		}
	}

	// if there are prevotes to send...
	if prs.Step <= cstypes.RoundStepPrevoteWait && prs.Round != -1 && prs.Round <= rs.Round {
		if ok, err := r.pickSendVote(ctx, ps, rs.Votes.Prevotes(prs.Round), voteCh); err != nil {
			return false, err
		} else if ok {
			logger.Debug("picked rs.Prevotes(prs.Round) to send", "round", prs.Round)
			return true, nil
		}
	}

	// if there are precommits to send...
	if prs.Step <= cstypes.RoundStepPrecommitWait && prs.Round != -1 && prs.Round <= rs.Round {
		if ok, err := r.pickSendVote(ctx, ps, rs.Votes.Precommits(prs.Round), voteCh); err != nil {
			return false, err
		} else if ok {
			logger.Debug("picked rs.Precommits(prs.Round) to send", "round", prs.Round)
			return true, nil
		}
	}

	// if there are prevotes to send...(which are needed because of validBlock mechanism)
	if prs.Round != -1 && prs.Round <= rs.Round {
		if ok, err := r.pickSendVote(ctx, ps, rs.Votes.Prevotes(prs.Round), voteCh); err != nil {
			return false, err
		} else if ok {
			logger.Debug("picked rs.Prevotes(prs.Round) to send", "round", prs.Round)
			return true, nil
		}
	}

	// if there are POLPrevotes to send...
	if prs.ProposalPOLRound != -1 {
		if polPrevotes := rs.Votes.Prevotes(prs.ProposalPOLRound); polPrevotes != nil {
			if ok, err := r.pickSendVote(ctx, ps, polPrevotes, voteCh); err != nil {
				return false, err
			} else if ok {
				logger.Debug("picked rs.Prevotes(prs.ProposalPOLRound) to send", "round", prs.ProposalPOLRound)
				return true, nil
			}
		}
	}

	return false, nil
}

// gossipCommit sends a commit to the peer
func (r *Reactor) gossipCommit(ctx context.Context, voteCh p2p.Channel, rs *cstypes.RoundState, ps *PeerState, prs *cstypes.PeerRoundState) error {
	// logger := r.Logger.With("height", rs.Height, "peer_height", prs.Height, "peer", ps.peerID)
	var commit *types.Commit
	blockStoreBase := r.state.blockStore.Base()

	if prs.Height+1 == rs.Height && !prs.HasCommit {
		commit = rs.LastCommit
	} else if rs.Height >= prs.Height+2 && prs.Height >= blockStoreBase && !prs.HasCommit {
		// Load the block commit for prs.Height, which contains precommit
		// signatures for prs.Height.
		commit = r.state.blockStore.LoadBlockCommit(prs.Height)
	}

	if commit == nil {
		//r.logger.Debug("cannot find a block",
		//	"prs.Height", prs.Height,
		//	"rs.Height", rs.Height,
		//	"blockStoreBase", blockStoreBase,
		//	"stack", string(debug.Stack()),
		//)
		return fmt.Errorf("commit at height %d not found", prs.Height)
	}

	if err := r.sendCommit(ctx, ps, commit, voteCh); err != nil {
		return fmt.Errorf("failed to send commit to peer: %w", err)
	}

	ps.SetHasCommit(commit)
	return nil // success
}

func (r *Reactor) gossipVotesAndCommitRoutine(ctx context.Context, voteCh p2p.Channel, ps *PeerState) {
	logger := r.logger.With("peer", ps.peerID)

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

		rs := r.getRoundState()
		prs := ps.GetRoundState()

		isValidator := r.isValidator(ps.GetProTxHash())

		//	If there are lastCommits to send...
		//prs.Step == cstypes.RoundStepNewHeight &&
		if prs.Height > 0 && prs.Height+1 == rs.Height && !prs.HasCommit {
			if err := r.gossipCommit(ctx, voteCh, rs, ps, prs); err != nil {
				logger.Error("cannot send LastCommit to peer node", "error", err)
			} else {
				logger.Info("sending LastCommit to peer node", "peer_height", prs.Height)
			}
			continue
		}

		// if height matches, then send LastCommit, Prevotes, and Precommits
		if isValidator && rs.Height == prs.Height {
			if ok, err := r.gossipVotesForHeight(ctx, rs, prs, ps, voteCh); err != nil {
				return
			} else if ok {
				continue
			}
		}

		// catchup logic -- if peer is lagging by more than 1, send Commit
		// note that peer can ignore a commit if it doesn't have a complete block,
		// so we might need to resend it until it notifies us that it's all right
		blockStoreBase := r.state.blockStore.Base()
		if rs.Height >= prs.Height+2 && prs.Height >= blockStoreBase && !prs.HasCommit {
			if err := r.gossipCommit(ctx, voteCh, rs, ps, prs); err != nil {
				logger.Error("cannot gossip commit to peer", "error", err)
			}
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
func (r *Reactor) queryMaj23Routine(ctx context.Context, stateCh p2p.Channel, ps *PeerState) {
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

		// If peer is not a validator, we do nothing
		if !r.isValidator(ps.GetProTxHash()) {
			time.Sleep(r.state.config.PeerQueryMaj23SleepDuration)
			continue
		}

		// TODO create more reliable copies of these
		// structures so the following go routines don't race
		rs := r.getRoundState()
		prs := ps.GetRoundState()

		wg := &sync.WaitGroup{}

		if rs.Height == prs.Height {
			wg.Add(1)
			go func(rs *cstypes.RoundState, prs *cstypes.PeerRoundState) {
				defer wg.Done()

				// maybe send Height/Round/Prevotes
				if maj23, ok := rs.Votes.Prevotes(prs.Round).TwoThirdsMajority(); ok {
					err := r.send(ctx, ps, stateCh, &tmcons.VoteSetMaj23{
						Height:  prs.Height,
						Round:   prs.Round,
						Type:    tmproto.PrevoteType,
						BlockID: maj23.ToProto(),
					})
					if err != nil {
						cancel()
					}
					r.logResult(err, r.logger, "sending prevotes", "height", prs.Height, "round", prs.Round)
				}
			}(rs, prs)

			if prs.ProposalPOLRound >= 0 {
				wg.Add(1)
				go func(rs *cstypes.RoundState, prs *cstypes.PeerRoundState) {
					defer wg.Done()

					// maybe send Height/Round/ProposalPOL
					if maj23, ok := rs.Votes.Prevotes(prs.ProposalPOLRound).TwoThirdsMajority(); ok {
						err := r.send(ctx, ps, stateCh, &tmcons.VoteSetMaj23{
							Height:  prs.Height,
							Round:   prs.ProposalPOLRound,
							Type:    tmproto.PrevoteType,
							BlockID: maj23.ToProto(),
						})
						if err != nil {
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
					err := r.send(ctx, ps, stateCh, &tmcons.VoteSetMaj23{
						Height:  prs.Height,
						Round:   prs.Round,
						Type:    tmproto.PrecommitType,
						BlockID: maj23.ToProto(),
					})
					if err != nil {
						cancel()
					}
					r.logResult(err, r.logger, "sending precommits", "height", prs.Height, "round", prs.Round)
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
						err := r.send(ctx, ps, stateCh, &tmcons.VoteSetMaj23{
							Height:  prs.Height,
							Round:   commit.Round,
							Type:    tmproto.PrecommitType,
							BlockID: commit.BlockID.ToProto(),
						})
						if err != nil {
							cancel()
						}
						r.logResult(err, r.logger, "sending catchup precommits", "height", prs.Height, "round", prs.Round)
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

func (r *Reactor) isValidator(proTxHash types.ProTxHash) bool {
	_, vset := r.state.GetValidatorSet()
	return vset.HasProTxHash(proTxHash)
}

// processPeerUpdate process a peer update message. For new or reconnected peers,
// we create a peer state if one does not exist for the peer, which should always
// be the case, and we spawn all the relevant goroutine to broadcast messages to
// the peer. During peer removal, we remove the peer for our set of peers and
// signal to all spawned goroutines to gracefully exit in a non-blocking manner.
func (r *Reactor) processPeerUpdate(ctx context.Context, peerUpdate p2p.PeerUpdate, chans channelBundle) {
	r.logger.Debug("received peer update", "peer", peerUpdate.NodeID, "status", peerUpdate.Status,
		"peer_proTxHash", peerUpdate.ProTxHash.ShortString())

	switch peerUpdate.Status {
	case p2p.PeerStatusUp:
		// Do not allow starting new broadcasting goroutines after reactor shutdown
		// has been initiated. This can happen after we've manually closed all
		// peer goroutines, but the router still sends in-flight peer updates.
		if !r.IsRunning() {
			return
		}
		r.peerUp(ctx, peerUpdate, 3, chans)
	case p2p.PeerStatusDown:
		r.peerDown(ctx, peerUpdate, chans)
	}
}

// peerUp starts the peer. It recursively retries up to `retries` times if the peer is already closing.
func (r *Reactor) peerUp(ctx context.Context, peerUpdate p2p.PeerUpdate, retries int, chans channelBundle) {
	if retries < 1 {
		r.logger.Error("peer up failed: max retries exceeded", "peer", peerUpdate.NodeID)
		return
	}

	r.mtx.Lock()
	defer r.mtx.Unlock()

	ps, ok := r.peers[peerUpdate.NodeID]
	if !ok {
		ps = NewPeerState(r.logger, peerUpdate.NodeID)
		ps.SetProTxHash(peerUpdate.ProTxHash)
		r.peers[peerUpdate.NodeID] = ps
	} else if len(peerUpdate.ProTxHash) > 0 {
		ps.SetProTxHash(peerUpdate.ProTxHash)
	}

	select {
	case <-ctx.Done():
		// Hmm, someone is closing this peer right now, let's wait and retry
		// Note: we run this in a goroutine to not block main goroutine in ps.broadcastWG.Wait()
		go func() {
			time.Sleep(r.state.config.PeerGossipSleepDuration)
			r.peerUp(ctx, peerUpdate, retries-1, chans)
		}()
		return
	default:
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
			go r.gossipDataRoutine(ctx, ps, chans.data, chans)
			go r.gossipVotesAndCommitRoutine(ctx, chans.vote, ps)
			go r.queryMaj23Routine(ctx, chans.state, ps)

			// Send our state to the peer. If we're block-syncing, broadcast a
			// RoundStepMessage later upon SwitchToConsensus().
			if !r.WaitSync() {
				go func() {
					rs := r.state.GetRoundState()
					err := r.send(ctx, ps, chans.state, rs.NewRoundStepMessage())
					r.logResult(err, r.logger, "sending round step msg", "height", rs.Height, "round", rs.Round)
				}()
			}
		}()
	}
}

func (r *Reactor) peerDown(ctx context.Context, peerUpdate p2p.PeerUpdate, chans channelBundle) {
	r.mtx.RLock()
	ps, ok := r.peers[peerUpdate.NodeID]
	r.mtx.RUnlock()

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

// handleStateMessage handles envelopes sent from peers on the StateChannel.
// An error is returned if the message is unrecognized or if validation fails.
// If we fail to find the peer state for the envelope sender, we perform a no-op
// and return. This can happen when we process the envelope after the peer is
// removed.
func (r *Reactor) handleStateMessage(ctx context.Context, envelope *p2p.Envelope, msgI Message, voteSetCh p2p.Channel) error {
	ps, ok := r.GetPeerState(envelope.From)
	if !ok || ps == nil {
		r.logger.Debug("failed to find peer state", "peer", envelope.From, "ch_id", "StateChannel")
		return nil
	}

	switch msg := envelope.Message.(type) {
	case *tmcons.NewRoundStep:
		initialHeight := r.state.InitialHeight()

		if err := msgI.(*NewRoundStepMessage).ValidateHeight(initialHeight); err != nil {
			r.logger.Error("peer sent us an invalid msg", "msg", msg, "err", err)
			return err
		}

		ps.ApplyNewRoundStepMessage(msgI.(*NewRoundStepMessage))

	case *tmcons.NewValidBlock:
		ps.ApplyNewValidBlockMessage(msgI.(*NewValidBlockMessage))

	case *tmcons.HasCommit:
		ps.ApplyHasCommitMessage(msgI.(*HasCommitMessage))

	case *tmcons.HasVote:
		if err := ps.ApplyHasVoteMessage(msgI.(*HasVoteMessage)); err != nil {
			r.logger.Error("applying HasVote message", "msg", msg, "err", err)
			return err
		}
	case *tmcons.VoteSetMaj23:
		height := r.state.CurrentHeight()
		votes := r.state.HeightVoteSet()

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

		if err := voteSetCh.Send(ctx, p2p.Envelope{
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
		logger.Info("ignoring message received during sync", "msg", tmstrings.LazySprintf("%T", msgI))
		return nil
	}

	logger.Debug("data channel processing", "msg", envelope.Message, "type", fmt.Sprintf("%T", envelope.Message))

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
		logger.Debug("failed to find peer state")
		return nil
	}

	if r.WaitSync() {
		logger.Info("ignoring message received during sync", "msg", msgI)
		return nil
	}

	logger.Debug("vote channel processing", "msg", envelope.Message, "type", fmt.Sprintf("%T", envelope.Message))

	switch msg := envelope.Message.(type) {
	case *tmcons.Commit:
		c, err := types.CommitFromProto(msg.Commit)
		if err != nil {
			return err
		}
		ps.SetHasCommit(c)

		cMsg := msgI.(*CommitMessage)
		r.state.peerMsgQueue <- msgInfo{cMsg, envelope.From, tmtime.Now()}
	case *tmcons.Vote:
		r.state.mtx.RLock()
		isValidator := r.state.Validators.HasProTxHash(r.state.privValidatorProTxHash)
		height, valSize, lastCommitSize := r.state.Height, r.state.Validators.Size(), r.state.LastPrecommits.Size()
		r.state.mtx.RUnlock()

		if isValidator { // ignore votes on non-validator nodes; TODO don't even send it
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
		}
	default:
		return fmt.Errorf("received unknown message on VoteChannel: %T", msg)
	}

	return nil
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
func (r *Reactor) handleMessage(ctx context.Context, envelope *p2p.Envelope, chans channelBundle) (err error) {
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
	if err = protoMsg.Wrap(envelope.Message); err != nil {
		return err
	}

	var msgI Message
	msgI, err = MsgFromProto(protoMsg)
	if err != nil {
		return err
	}

	//r.logger.Debug("received message", "ch_id", envelope.ChannelID, "message", msgI, "peer", envelope.From)

	switch envelope.ChannelID {
	case StateChannel:
		err = r.handleStateMessage(ctx, envelope, msgI, chans.votSet)
	case DataChannel:
		err = r.handleDataMessage(ctx, envelope, msgI)
	case VoteChannel:
		err = r.handleVoteMessage(ctx, envelope, msgI)
	case VoteSetBitsChannel:
		err = r.handleVoteSetBitsMessage(ctx, envelope, msgI)
	default:
		err = fmt.Errorf("unknown channel ID (%d) for envelope (%v)", envelope.ChannelID, envelope)
	}

	return err
}

// processMsgCh initiates a blocking process where we listen for and handle
// envelopes on the StateChannel or DataChannel or VoteChannel or VoteSetBitsChannel.
// Any error encountered during message execution will result in a PeerError being sent
// on the StateChannel or DataChannel or VoteChannel or VoteSetBitsChannel.
// When the reactor is stopped, we will catch the signal and close the p2p Channel gracefully.
func (r *Reactor) processMsgCh(ctx context.Context, msgCh p2p.Channel, chBundle channelBundle) {
	iter := msgCh.Receive(ctx)
	for iter.Next(ctx) {
		envelope := iter.Envelope()
		if err := r.handleMessage(ctx, envelope, chBundle); err != nil {
			r.logger.Error("failed to process message", "ch_id", envelope.ChannelID, "envelope", envelope, "err", err)
			if serr := msgCh.SendError(ctx, p2p.PeerError{
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
func (r *Reactor) processPeerUpdates(ctx context.Context, peerUpdates *p2p.PeerUpdates, chans channelBundle) {
	for {
		select {
		case <-ctx.Done():
			return
		case peerUpdate := <-peerUpdates.Updates():
			r.processPeerUpdate(ctx, peerUpdate, chans)
		}
	}
}

func (r *Reactor) peerStatsRoutine(ctx context.Context, peerUpdates *p2p.PeerUpdates) {
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
			case *CommitMessage:
				if numCommits := ps.RecordCommit(); numCommits%commitsToContributeToBecomeGoodPeer == 0 {
					peerUpdates.SendUpdate(ctx, p2p.PeerUpdate{
						NodeID: msg.PeerID,
						Status: p2p.PeerStatusGood,
					})
				}

			case *VoteMessage:
				if numVotes := ps.RecordVote(); numVotes%votesToContributeToBecomeGoodPeer == 0 {
					peerUpdates.SendUpdate(ctx, p2p.PeerUpdate{
						NodeID: msg.PeerID,
						Status: p2p.PeerStatusGood,
					})
				}

			case *BlockPartMessage:
				if numParts := ps.RecordBlockPart(); numParts%blocksToContributeToBecomeGoodPeer == 0 {
					peerUpdates.SendUpdate(ctx, p2p.PeerUpdate{
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
