package consensus

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/gogo/protobuf/proto"

	cstypes "github.com/tendermint/tendermint/consensus/types"
	"github.com/tendermint/tendermint/libs/bits"
	"github.com/tendermint/tendermint/libs/cmap"
	tmevents "github.com/tendermint/tendermint/libs/events"
	tmjson "github.com/tendermint/tendermint/libs/json"
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

	conS     *State
	peers    *cmap.CMap
	eventBus *types.EventBus
	Metrics  *Metrics

	mtx      tmsync.RWMutex
	waitSync bool

	stateCh       *p2p.Channel
	dataCh        *p2p.Channel
	voteCh        *p2p.Channel
	voteSetBitsCh *p2p.Channel
	peerUpdates   *p2p.PeerUpdatesCh
	closeCh       chan struct{}
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
	peerUpdates *p2p.PeerUpdatesCh,
	waitSync bool,
	options ...ReactorOption,
) *Reactor {

	r := &Reactor{
		conS:          cs,
		waitSync:      waitSync,
		peers:         cmap.NewCMap(),
		Metrics:       NopMetrics(),
		stateCh:       stateCh,
		dataCh:        dataCh,
		voteCh:        voteCh,
		voteSetBitsCh: voteSetBitsCh,
		peerUpdates:   peerUpdates,
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
		if err := r.conS.Start(); err != nil {
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

	if err := r.conS.Stop(); err != nil {
		r.Logger.Error("failed to stop consensus state", "err", err)
	}

	if !r.WaitSync() {
		r.conS.Wait()
	}

	// Close closeCh to signal to all spawned goroutines to gracefully exit. All
	// p2p Channels should execute Close().
	close(r.closeCh)

	// Wait for all p2p Channels to be closed before returning. This ensures we
	// can easily reason about synchronization of all p2p Channels and ensure no
	// panics will occur.
	<-r.stateCh.Done()
	<-r.dataCh.Done()
	<-r.voteCh.Done()
	<-r.voteSetBitsCh.Done()
	<-r.peerUpdates.Done()
}

// subscribeToBroadcastEvents subscribes for new round steps and votes using the
// internal pubsub defined in the consensus state to broadcast them to peers
// upon receiving.
func (r *Reactor) subscribeToBroadcastEvents() {
	err := r.conS.evsw.AddListenerForEvent(
		listenerIDConsensus,
		types.EventNewRoundStep,
		func(data tmevents.EventData) {
			r.broadcastNewRoundStepMessage(data.(*cstypes.RoundState))
		},
	)
	if err != nil {
		r.Logger.Error("failed to add listener for events", "err", err)
	}

	err = r.conS.evsw.AddListenerForEvent(
		listenerIDConsensus,
		types.EventValidBlock,
		func(data tmevents.EventData) {
			r.broadcastNewValidBlockMessage(data.(*cstypes.RoundState))
		},
	)
	if err != nil {
		r.Logger.Error("failed to add listener for events", "err", err)
	}

	err = r.conS.evsw.AddListenerForEvent(
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
	r.conS.evsw.RemoveListener(listenerIDConsensus)
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
	rs := r.conS.GetRoundState()
	msg := makeRoundStepMessage(rs)
	r.stateCh.Out() <- p2p.Envelope{
		To:      peerID,
		Message: msg,
	}
}

func (r *Reactor) processPeerUpdate(peerUpdate p2p.PeerUpdate) {
	r.Logger.Debug("received peer update", "peer", peerUpdate.PeerID, "status", peerUpdate.Status)

	switch peerUpdate.Status {
	case p2p.PeerStatusNew, p2p.PeerStatusUp:
		psKey := string(peerUpdate.PeerID)

		// save the peer if the ID is non-empty and we haven't seen this peer before
		if psKey != "" && !r.peers.Has(psKey) {
			peerState := NewPeerState(r.Logger, peerUpdate.PeerID)
			r.peers.Set(psKey, peerState)
		}

		peerState, ok := r.peers.Get(psKey).(*PeerState)
		if !ok {
			panic(fmt.Sprintf("peer %v has no state", peerState.peerID))
		}

		// begin routines for this peer
		//
		// TODO: Evaluate if we need this to be synchronized via WaitGroup as to not
		// leak the goroutine when stopping the reactor.
		go r.gossipDataRoutine(peerState)
		go r.gossipVotesRoutine(peerState)
		go r.queryMaj23Routine(peerState)

		// Send our state to the peer. If we're fast-syncing, broadcast a
		// RoundStepMessage later upon SwitchToConsensus().
		if !r.WaitSync() {
			r.sendNewRoundStepMessage(peerState.peerID)
		}

	case p2p.PeerStatusDown, p2p.PeerStatusRemoved, p2p.PeerStatusBanned:
		// TODO: Handle update. The original reactor performed a no-op here, but we
		// may need to do some cleanup.
	}
}

// handleMessage handles an Envelope sent from a peer on a specific p2p Channel.
// It will handle errors and any possible panics gracefully. A caller can handle
// any error returned by sending a PeerError on the respective channel.
func (r *Reactor) handleMessage(chID p2p.ChannelID, envelope p2p.Envelope) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("panic in processing message: %v", e)
			r.Logger.Error("recovering from processing message panic", "err", err)
		}
	}()

	r.Logger.Debug("received message", "message", envelope.Message, "peer", envelope.From)

	switch chID {
	case StateChannel:
		// TODO: Handle message

	case DataChannel:
		// TODO: Handle message

	case VoteChannel:
		// TODO: Handle message

	case VoteSetBitsChannel:
		// TODO: Handle message

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
		case envelope := <-r.stateCh.In():
			if err := r.handleMessage(r.stateCh.ID(), envelope); err != nil {
				r.Logger.Error("failed to process message", "ch_id", r.stateCh.ID(), "envelope", envelope, "err", err)
				r.stateCh.Error() <- p2p.PeerError{
					PeerID:   envelope.From,
					Err:      err,
					Severity: p2p.PeerErrorSeverityLow,
				}
			}

		case <-r.closeCh:
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
		case envelope := <-r.dataCh.In():
			if err := r.handleMessage(r.dataCh.ID(), envelope); err != nil {
				r.Logger.Error("failed to process message", "ch_id", r.dataCh.ID(), "envelope", envelope, "err", err)
				r.dataCh.Error() <- p2p.PeerError{
					PeerID:   envelope.From,
					Err:      err,
					Severity: p2p.PeerErrorSeverityLow,
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
		case envelope := <-r.voteCh.In():
			if err := r.handleMessage(r.voteCh.ID(), envelope); err != nil {
				r.Logger.Error("failed to process message", "ch_id", r.voteCh.ID(), "envelope", envelope, "err", err)
				r.voteCh.Error() <- p2p.PeerError{
					PeerID:   envelope.From,
					Err:      err,
					Severity: p2p.PeerErrorSeverityLow,
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
		case envelope := <-r.voteSetBitsCh.In():
			if err := r.handleMessage(r.voteSetBitsCh.ID(), envelope); err != nil {
				r.Logger.Error("failed to process message", "ch_id", r.voteSetBitsCh.ID(), "envelope", envelope, "err", err)
				r.voteSetBitsCh.Error() <- p2p.PeerError{
					PeerID:   envelope.From,
					Err:      err,
					Severity: p2p.PeerErrorSeverityLow,
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

// SwitchToConsensus switches from fast-sync mode to consensus mode. It resets
// the state, turns off fast-sync, and starts the consensus state-machine.
func (r *Reactor) SwitchToConsensus(state sm.State, skipWAL bool) {
	r.Logger.Info("switching to consensus")

	// we have no votes, so reconstruct LastCommit from SeenCommit
	if state.LastBlockHeight > 0 {
		r.conS.reconstructLastCommit(state)
	}

	// NOTE: The line below causes broadcastNewRoundStepRoutine() to broadcast a
	// NewRoundStepMessage.
	r.conS.updateToState(state)

	r.mtx.Lock()
	r.waitSync = false
	r.mtx.Unlock()

	r.Metrics.FastSyncing.Set(0)
	r.Metrics.StateSyncing.Set(0)

	if skipWAL {
		r.conS.doWALCatchup = false
	}

	if err := r.conS.Start(); err != nil {
		panic(fmt.Sprintf(`failed to start consensus state: %v

conS:
%+v

conR:
%+v`, err, r.conS, r))
	}
}

// ############################################################################
// ############################################################################
// ############################################################################

// Receive implements Reactor
// NOTE: We process these messages even when we're fast_syncing.
// Messages affect either a peer state or the consensus state.
// Peer state updates can happen in parallel, but processing of
// proposals, block parts, and votes are ordered by the receiveRoutine
// NOTE: blocks on consensus state for proposals, block parts, and votes
// XXX: do not call any methods that can block or incur heavy processing.
// https://github.com/tendermint/tendermint/issues/2888
func (r *Reactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	if !r.IsRunning() {
		r.Logger.Debug("Receive", "src", src, "chId", chID, "bytes", msgBytes)
		return
	}

	msg, err := decodeMsg(msgBytes)
	if err != nil {
		r.Logger.Error("Error decoding message", "src", src, "chId", chID, "err", err)
		r.Switch.StopPeerForError(src, err)
		return
	}

	if err = msg.ValidateBasic(); err != nil {
		r.Logger.Error("Peer sent us invalid msg", "peer", src, "msg", msg, "err", err)
		r.Switch.StopPeerForError(src, err)
		return
	}

	r.Logger.Debug("Receive", "src", src, "chId", chID, "msg", msg)

	// Get peer states
	ps, ok := src.Get(types.PeerStateKey).(*PeerState)
	if !ok {
		panic(fmt.Sprintf("Peer %v has no state", src))
	}

	switch chID {
	case StateChannel:
		switch msg := msg.(type) {
		case *NewRoundStepMessage:
			r.conS.mtx.Lock()
			initialHeight := r.conS.state.InitialHeight
			r.conS.mtx.Unlock()
			if err = msg.ValidateHeight(initialHeight); err != nil {
				r.Logger.Error("Peer sent us invalid msg", "peer", src, "msg", msg, "err", err)
				r.Switch.StopPeerForError(src, err)
				return
			}
			ps.ApplyNewRoundStepMessage(msg)
		case *NewValidBlockMessage:
			ps.ApplyNewValidBlockMessage(msg)
		case *HasVoteMessage:
			ps.ApplyHasVoteMessage(msg)
		case *VoteSetMaj23Message:
			cs := r.conS
			cs.mtx.Lock()
			height, votes := cs.Height, cs.Votes
			cs.mtx.Unlock()
			if height != msg.Height {
				return
			}
			// Peer claims to have a maj23 for some BlockID at H,R,S,
			err := votes.SetPeerMaj23(msg.Round, msg.Type, ps.peer.ID(), msg.BlockID)
			if err != nil {
				r.Switch.StopPeerForError(src, err)
				return
			}
			// Respond with a VoteSetBitsMessage showing which votes we have.
			// (and consequently shows which we don't have)
			var ourVotes *bits.BitArray
			switch msg.Type {
			case tmproto.PrevoteType:
				ourVotes = votes.Prevotes(msg.Round).BitArrayByBlockID(msg.BlockID)
			case tmproto.PrecommitType:
				ourVotes = votes.Precommits(msg.Round).BitArrayByBlockID(msg.BlockID)
			default:
				panic("Bad VoteSetBitsMessage field Type. Forgot to add a check in ValidateBasic?")
			}
			src.TrySend(VoteSetBitsChannel, MustEncode(&VoteSetBitsMessage{
				Height:  msg.Height,
				Round:   msg.Round,
				Type:    msg.Type,
				BlockID: msg.BlockID,
				Votes:   ourVotes,
			}))
		default:
			r.Logger.Error(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
		}

	case DataChannel:
		if r.WaitSync() {
			r.Logger.Info("Ignoring message received during sync", "msg", msg)
			return
		}
		switch msg := msg.(type) {
		case *ProposalMessage:
			ps.SetHasProposal(msg.Proposal)
			r.conS.peerMsgQueue <- msgInfo{msg, src.ID()}
		case *ProposalPOLMessage:
			ps.ApplyProposalPOLMessage(msg)
		case *BlockPartMessage:
			ps.SetHasProposalBlockPart(msg.Height, msg.Round, int(msg.Part.Index))
			r.Metrics.BlockParts.With("peer_id", string(src.ID())).Add(1)
			r.conS.peerMsgQueue <- msgInfo{msg, src.ID()}
		default:
			r.Logger.Error(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
		}

	case VoteChannel:
		if r.WaitSync() {
			r.Logger.Info("Ignoring message received during sync", "msg", msg)
			return
		}
		switch msg := msg.(type) {
		case *VoteMessage:
			cs := r.conS
			cs.mtx.RLock()
			height, valSize, lastCommitSize := cs.Height, cs.Validators.Size(), cs.LastCommit.Size()
			cs.mtx.RUnlock()
			ps.EnsureVoteBitArrays(height, valSize)
			ps.EnsureVoteBitArrays(height-1, lastCommitSize)
			ps.SetHasVote(msg.Vote)

			cs.peerMsgQueue <- msgInfo{msg, src.ID()}

		default:
			// don't punish (leave room for soft upgrades)
			r.Logger.Error(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
		}

	case VoteSetBitsChannel:
		if r.WaitSync() {
			r.Logger.Info("Ignoring message received during sync", "msg", msg)
			return
		}
		switch msg := msg.(type) {
		case *VoteSetBitsMessage:
			cs := r.conS
			cs.mtx.Lock()
			height, votes := cs.Height, cs.Votes
			cs.mtx.Unlock()

			if height == msg.Height {
				var ourVotes *bits.BitArray
				switch msg.Type {
				case tmproto.PrevoteType:
					ourVotes = votes.Prevotes(msg.Round).BitArrayByBlockID(msg.BlockID)
				case tmproto.PrecommitType:
					ourVotes = votes.Precommits(msg.Round).BitArrayByBlockID(msg.BlockID)
				default:
					panic("Bad VoteSetBitsMessage field Type. Forgot to add a check in ValidateBasic?")
				}
				ps.ApplyVoteSetBitsMessage(msg, ourVotes)
			} else {
				ps.ApplyVoteSetBitsMessage(msg, nil)
			}
		default:
			// don't punish (leave room for soft upgrades)
			r.Logger.Error(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
		}

	default:
		r.Logger.Error(fmt.Sprintf("Unknown chId %X", chID))
	}
}

// SetEventBus sets event bus.
func (r *Reactor) SetEventBus(b *types.EventBus) {
	r.eventBus = b
	r.conS.SetEventBus(b)
}

// WaitSync returns whether the consensus reactor is waiting for state/fast sync.
func (r *Reactor) WaitSync() bool {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.waitSync
}

//--------------------------------------

func (r *Reactor) broadcastNewRoundStepMessage(rs *cstypes.RoundState) {
	nrsMsg := makeRoundStepMessage(rs)
	r.Switch.Broadcast(StateChannel, MustEncode(nrsMsg))
}

func (r *Reactor) broadcastNewValidBlockMessage(rs *cstypes.RoundState) {
	csMsg := &NewValidBlockMessage{
		Height:             rs.Height,
		Round:              rs.Round,
		BlockPartSetHeader: rs.ProposalBlockParts.Header(),
		BlockParts:         rs.ProposalBlockParts.BitArray(),
		IsCommit:           rs.Step == cstypes.RoundStepCommit,
	}
	r.Switch.Broadcast(StateChannel, MustEncode(csMsg))
}

// Broadcasts HasVoteMessage to peers that care.
func (r *Reactor) broadcastHasVoteMessage(vote *types.Vote) {
	msg := &HasVoteMessage{
		Height: vote.Height,
		Round:  vote.Round,
		Type:   vote.Type,
		Index:  vote.ValidatorIndex,
	}
	r.Switch.Broadcast(StateChannel, MustEncode(msg))
	/*
		// TODO: Make this broadcast more selective.
		for _, peer := range r.Switch.Peers().List() {
			ps, ok := peer.Get(PeerStateKey).(*PeerState)
			if !ok {
				panic(fmt.Sprintf("Peer %v has no state", peer))
			}
			prs := ps.GetRoundState()
			if prs.Height == vote.Height {
				// TODO: Also filter on round?
				peer.TrySend(StateChannel, struct{ ConsensusMessage }{msg})
			} else {
				// Height doesn't match
				// TODO: check a field, maybe CatchupCommitRound?
				// TODO: But that requires changing the struct field comment.
			}
		}
	*/
}

func (r *Reactor) gossipDataRoutine(peer p2p.Peer, ps *PeerState) {
	logger := r.Logger.With("peer", peer)

OUTER_LOOP:
	for {
		// Manage disconnects from self or peer.
		if !peer.IsRunning() || !r.IsRunning() {
			logger.Info("Stopping gossipDataRoutine for peer")
			return
		}
		rs := r.conS.GetRoundState()
		prs := ps.GetRoundState()

		// Send proposal Block parts?
		if rs.ProposalBlockParts.HasHeader(prs.ProposalBlockPartSetHeader) {
			if index, ok := rs.ProposalBlockParts.BitArray().Sub(prs.ProposalBlockParts.Copy()).PickRandom(); ok {
				part := rs.ProposalBlockParts.GetPart(index)
				msg := &BlockPartMessage{
					Height: rs.Height, // This tells peer that this part applies to us.
					Round:  rs.Round,  // This tells peer that this part applies to us.
					Part:   part,
				}
				logger.Debug("Sending block part", "height", prs.Height, "round", prs.Round)
				if peer.Send(DataChannel, MustEncode(msg)) {
					ps.SetHasProposalBlockPart(prs.Height, prs.Round, index)
				}
				continue OUTER_LOOP
			}
		}

		// If the peer is on a previous height that we have, help catch up.
		if (0 < prs.Height) && (prs.Height < rs.Height) && (prs.Height >= r.conS.blockStore.Base()) {
			heightLogger := logger.With("height", prs.Height)

			// if we never received the commit message from the peer, the block parts wont be initialized
			if prs.ProposalBlockParts == nil {
				blockMeta := r.conS.blockStore.LoadBlockMeta(prs.Height)
				if blockMeta == nil {
					heightLogger.Error("Failed to load block meta",
						"blockstoreBase", r.conS.blockStore.Base(), "blockstoreHeight", r.conS.blockStore.Height())
					time.Sleep(r.conS.config.PeerGossipSleepDuration)
				} else {
					ps.InitProposalBlockParts(blockMeta.BlockID.PartSetHeader)
				}
				// continue the loop since prs is a copy and not effected by this initialization
				continue OUTER_LOOP
			}
			r.gossipDataForCatchup(heightLogger, rs, prs, ps, peer)
			continue OUTER_LOOP
		}

		// If height and round don't match, sleep.
		if (rs.Height != prs.Height) || (rs.Round != prs.Round) {
			// logger.Info("Peer Height|Round mismatch, sleeping",
			// "peerHeight", prs.Height, "peerRound", prs.Round, "peer", peer)
			time.Sleep(r.conS.config.PeerGossipSleepDuration)
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
				msg := &ProposalMessage{Proposal: rs.Proposal}
				logger.Debug("Sending proposal", "height", prs.Height, "round", prs.Round)
				if peer.Send(DataChannel, MustEncode(msg)) {
					// NOTE[ZM]: A peer might have received different proposal msg so this Proposal msg will be rejected!
					ps.SetHasProposal(rs.Proposal)
				}
			}
			// ProposalPOL: lets peer know which POL votes we have so far.
			// Peer must receive ProposalMessage first.
			// rs.Proposal was validated, so rs.Proposal.POLRound <= rs.Round,
			// so we definitely have rs.Votes.Prevotes(rs.Proposal.POLRound).
			if 0 <= rs.Proposal.POLRound {
				msg := &ProposalPOLMessage{
					Height:           rs.Height,
					ProposalPOLRound: rs.Proposal.POLRound,
					ProposalPOL:      rs.Votes.Prevotes(rs.Proposal.POLRound).BitArray(),
				}
				logger.Debug("Sending POL", "height", prs.Height, "round", prs.Round)
				peer.Send(DataChannel, MustEncode(msg))
			}
			continue OUTER_LOOP
		}

		// Nothing to do. Sleep.
		time.Sleep(r.conS.config.PeerGossipSleepDuration)
		continue OUTER_LOOP
	}
}

func (r *Reactor) gossipDataForCatchup(logger log.Logger, rs *cstypes.RoundState,
	prs *cstypes.PeerRoundState, ps *PeerState, peer p2p.Peer) {

	if index, ok := prs.ProposalBlockParts.Not().PickRandom(); ok {
		// Ensure that the peer's PartSetHeader is correct
		blockMeta := r.conS.blockStore.LoadBlockMeta(prs.Height)
		if blockMeta == nil {
			logger.Error("Failed to load block meta", "ourHeight", rs.Height,
				"blockstoreBase", r.conS.blockStore.Base(), "blockstoreHeight", r.conS.blockStore.Height())
			time.Sleep(r.conS.config.PeerGossipSleepDuration)
			return
		} else if !blockMeta.BlockID.PartSetHeader.Equals(prs.ProposalBlockPartSetHeader) {
			logger.Info("Peer ProposalBlockPartSetHeader mismatch, sleeping",
				"blockPartSetHeader", blockMeta.BlockID.PartSetHeader, "peerBlockPartSetHeader", prs.ProposalBlockPartSetHeader)
			time.Sleep(r.conS.config.PeerGossipSleepDuration)
			return
		}
		// Load the part
		part := r.conS.blockStore.LoadBlockPart(prs.Height, index)
		if part == nil {
			logger.Error("Could not load part", "index", index,
				"blockPartSetHeader", blockMeta.BlockID.PartSetHeader, "peerBlockPartSetHeader", prs.ProposalBlockPartSetHeader)
			time.Sleep(r.conS.config.PeerGossipSleepDuration)
			return
		}
		// Send the part
		msg := &BlockPartMessage{
			Height: prs.Height, // Not our height, so it doesn't matter.
			Round:  prs.Round,  // Not our height, so it doesn't matter.
			Part:   part,
		}
		logger.Debug("Sending block part for catchup", "round", prs.Round, "index", index)
		if peer.Send(DataChannel, MustEncode(msg)) {
			ps.SetHasProposalBlockPart(prs.Height, prs.Round, index)
		} else {
			logger.Debug("Sending block part for catchup failed")
		}
		return
	}
	//  logger.Info("No parts to send in catch-up, sleeping")
	time.Sleep(r.conS.config.PeerGossipSleepDuration)
}

func (r *Reactor) gossipVotesRoutine(peer p2p.Peer, ps *PeerState) {
	logger := r.Logger.With("peer", peer)

	// Simple hack to throttle logs upon sleep.
	var sleeping = 0

OUTER_LOOP:
	for {
		// Manage disconnects from self or peer.
		if !peer.IsRunning() || !r.IsRunning() {
			logger.Info("Stopping gossipVotesRoutine for peer")
			return
		}
		rs := r.conS.GetRoundState()
		prs := ps.GetRoundState()

		switch sleeping {
		case 1: // First sleep
			sleeping = 2
		case 2: // No more sleep
			sleeping = 0
		}

		// logger.Debug("gossipVotesRoutine", "rsHeight", rs.Height, "rsRound", rs.Round,
		// "prsHeight", prs.Height, "prsRound", prs.Round, "prsStep", prs.Step)

		// If height matches, then send LastCommit, Prevotes, Precommits.
		if rs.Height == prs.Height {
			heightLogger := logger.With("height", prs.Height)
			if r.gossipVotesForHeight(heightLogger, rs, prs, ps) {
				continue OUTER_LOOP
			}
		}

		// Special catchup logic.
		// If peer is lagging by height 1, send LastCommit.
		if prs.Height != 0 && rs.Height == prs.Height+1 {
			if ps.PickSendVote(rs.LastCommit) {
				logger.Debug("Picked rs.LastCommit to send", "height", prs.Height)
				continue OUTER_LOOP
			}
		}

		// Catchup logic
		// If peer is lagging by more than 1, send Commit.
		if prs.Height != 0 && rs.Height >= prs.Height+2 && prs.Height >= r.conS.blockStore.Base() {
			// Load the block commit for prs.Height,
			// which contains precommit signatures for prs.Height.
			if commit := r.conS.blockStore.LoadBlockCommit(prs.Height); commit != nil {
				if ps.PickSendVote(commit) {
					logger.Debug("Picked Catchup commit to send", "height", prs.Height)
					continue OUTER_LOOP
				}
			}
		}

		if sleeping == 0 {
			// We sent nothing. Sleep...
			sleeping = 1
			logger.Debug("No votes to send, sleeping", "rs.Height", rs.Height, "prs.Height", prs.Height,
				"localPV", rs.Votes.Prevotes(rs.Round).BitArray(), "peerPV", prs.Prevotes,
				"localPC", rs.Votes.Precommits(rs.Round).BitArray(), "peerPC", prs.Precommits)
		} else if sleeping == 2 {
			// Continued sleep...
			sleeping = 1
		}

		time.Sleep(r.conS.config.PeerGossipSleepDuration)
		continue OUTER_LOOP
	}
}

func (r *Reactor) gossipVotesForHeight(
	logger log.Logger,
	rs *cstypes.RoundState,
	prs *cstypes.PeerRoundState,
	ps *PeerState,
) bool {

	// If there are lastCommits to send...
	if prs.Step == cstypes.RoundStepNewHeight {
		if ps.PickSendVote(rs.LastCommit) {
			logger.Debug("Picked rs.LastCommit to send")
			return true
		}
	}
	// If there are POL prevotes to send...
	if prs.Step <= cstypes.RoundStepPropose && prs.Round != -1 && prs.Round <= rs.Round && prs.ProposalPOLRound != -1 {
		if polPrevotes := rs.Votes.Prevotes(prs.ProposalPOLRound); polPrevotes != nil {
			if ps.PickSendVote(polPrevotes) {
				logger.Debug("Picked rs.Prevotes(prs.ProposalPOLRound) to send",
					"round", prs.ProposalPOLRound)
				return true
			}
		}
	}
	// If there are prevotes to send...
	if prs.Step <= cstypes.RoundStepPrevoteWait && prs.Round != -1 && prs.Round <= rs.Round {
		if ps.PickSendVote(rs.Votes.Prevotes(prs.Round)) {
			logger.Debug("Picked rs.Prevotes(prs.Round) to send", "round", prs.Round)
			return true
		}
	}
	// If there are precommits to send...
	if prs.Step <= cstypes.RoundStepPrecommitWait && prs.Round != -1 && prs.Round <= rs.Round {
		if ps.PickSendVote(rs.Votes.Precommits(prs.Round)) {
			logger.Debug("Picked rs.Precommits(prs.Round) to send", "round", prs.Round)
			return true
		}
	}
	// If there are prevotes to send...Needed because of validBlock mechanism
	if prs.Round != -1 && prs.Round <= rs.Round {
		if ps.PickSendVote(rs.Votes.Prevotes(prs.Round)) {
			logger.Debug("Picked rs.Prevotes(prs.Round) to send", "round", prs.Round)
			return true
		}
	}
	// If there are POLPrevotes to send...
	if prs.ProposalPOLRound != -1 {
		if polPrevotes := rs.Votes.Prevotes(prs.ProposalPOLRound); polPrevotes != nil {
			if ps.PickSendVote(polPrevotes) {
				logger.Debug("Picked rs.Prevotes(prs.ProposalPOLRound) to send",
					"round", prs.ProposalPOLRound)
				return true
			}
		}
	}

	return false
}

// NOTE: `queryMaj23Routine` has a simple crude design since it only comes
// into play for liveness when there's a signature DDoS attack happening.
func (r *Reactor) queryMaj23Routine(peer p2p.Peer, ps *PeerState) {
	logger := r.Logger.With("peer", peer)

OUTER_LOOP:
	for {
		// Manage disconnects from self or peer.
		if !peer.IsRunning() || !r.IsRunning() {
			logger.Info("Stopping queryMaj23Routine for peer")
			return
		}

		// Maybe send Height/Round/Prevotes
		{
			rs := r.conS.GetRoundState()
			prs := ps.GetRoundState()
			if rs.Height == prs.Height {
				if maj23, ok := rs.Votes.Prevotes(prs.Round).TwoThirdsMajority(); ok {
					peer.TrySend(StateChannel, MustEncode(&VoteSetMaj23Message{
						Height:  prs.Height,
						Round:   prs.Round,
						Type:    tmproto.PrevoteType,
						BlockID: maj23,
					}))
					time.Sleep(r.conS.config.PeerQueryMaj23SleepDuration)
				}
			}
		}

		// Maybe send Height/Round/Precommits
		{
			rs := r.conS.GetRoundState()
			prs := ps.GetRoundState()
			if rs.Height == prs.Height {
				if maj23, ok := rs.Votes.Precommits(prs.Round).TwoThirdsMajority(); ok {
					peer.TrySend(StateChannel, MustEncode(&VoteSetMaj23Message{
						Height:  prs.Height,
						Round:   prs.Round,
						Type:    tmproto.PrecommitType,
						BlockID: maj23,
					}))
					time.Sleep(r.conS.config.PeerQueryMaj23SleepDuration)
				}
			}
		}

		// Maybe send Height/Round/ProposalPOL
		{
			rs := r.conS.GetRoundState()
			prs := ps.GetRoundState()
			if rs.Height == prs.Height && prs.ProposalPOLRound >= 0 {
				if maj23, ok := rs.Votes.Prevotes(prs.ProposalPOLRound).TwoThirdsMajority(); ok {
					peer.TrySend(StateChannel, MustEncode(&VoteSetMaj23Message{
						Height:  prs.Height,
						Round:   prs.ProposalPOLRound,
						Type:    tmproto.PrevoteType,
						BlockID: maj23,
					}))
					time.Sleep(r.conS.config.PeerQueryMaj23SleepDuration)
				}
			}
		}

		// Little point sending LastCommitRound/LastCommit,
		// These are fleeting and non-blocking.

		// Maybe send Height/CatchupCommitRound/CatchupCommit.
		{
			prs := ps.GetRoundState()
			if prs.CatchupCommitRound != -1 && prs.Height > 0 && prs.Height <= r.conS.blockStore.Height() &&
				prs.Height >= r.conS.blockStore.Base() {
				if commit := r.conS.LoadCommit(prs.Height); commit != nil {
					peer.TrySend(StateChannel, MustEncode(&VoteSetMaj23Message{
						Height:  prs.Height,
						Round:   commit.Round,
						Type:    tmproto.PrecommitType,
						BlockID: commit.BlockID,
					}))
					time.Sleep(r.conS.config.PeerQueryMaj23SleepDuration)
				}
			}
		}

		time.Sleep(r.conS.config.PeerQueryMaj23SleepDuration)

		continue OUTER_LOOP
	}
}

func (r *Reactor) peerStatsRoutine() {
	for {
		if !r.IsRunning() {
			r.Logger.Info("Stopping peerStatsRoutine")
			return
		}

		select {
		case msg := <-r.conS.statsMsgQueue:
			// Get peer
			peer := r.Switch.Peers().Get(msg.PeerID)
			if peer == nil {
				r.Logger.Debug("Attempt to update stats for non-existent peer",
					"peer", msg.PeerID)
				continue
			}
			// Get peer state
			ps, ok := peer.Get(types.PeerStateKey).(*PeerState)
			if !ok {
				panic(fmt.Sprintf("Peer %v has no state", peer))
			}
			switch msg.Msg.(type) {
			case *VoteMessage:
				if numVotes := ps.RecordVote(); numVotes%votesToContributeToBecomeGoodPeer == 0 {
					r.Switch.MarkPeerAsGood(peer)
				}
			case *BlockPartMessage:
				if numParts := ps.RecordBlockPart(); numParts%blocksToContributeToBecomeGoodPeer == 0 {
					r.Switch.MarkPeerAsGood(peer)
				}
			}
		case <-r.conS.Quit():
			return

		case <-r.Quit():
			return
		}
	}
}

// String returns a string representation of the Reactor.
// NOTE: For now, it is just a hard-coded string to avoid accessing unprotected shared variables.
// TODO: improve!
func (r *Reactor) String() string {
	// better not to access shared variables
	return "ConsensusReactor" // r.StringIndented("")
}

// StringIndented returns an indented string representation of the Reactor
func (r *Reactor) StringIndented(indent string) string {
	s := "ConsensusReactor{\n"
	s += indent + "  " + r.conS.StringIndented(indent+"  ") + "\n"
	for _, peer := range r.Switch.Peers().List() {
		ps, ok := peer.Get(types.PeerStateKey).(*PeerState)
		if !ok {
			panic(fmt.Sprintf("Peer %v has no state", peer))
		}
		s += indent + "  " + ps.StringIndented(indent+"  ") + "\n"
	}
	s += indent + "}"
	return s
}

// ReactorMetrics sets the metrics
func ReactorMetrics(metrics *Metrics) ReactorOption {
	return func(r *Reactor) { r.Metrics = metrics }
}

//-----------------------------------------------------------------------------
// Messages

// Message is a message that can be sent and received on the Reactor
type Message interface {
	ValidateBasic() error
}

func init() {
	tmjson.RegisterType(&NewRoundStepMessage{}, "tendermint/NewRoundStepMessage")
	tmjson.RegisterType(&NewValidBlockMessage{}, "tendermint/NewValidBlockMessage")
	tmjson.RegisterType(&ProposalMessage{}, "tendermint/Proposal")
	tmjson.RegisterType(&ProposalPOLMessage{}, "tendermint/ProposalPOL")
	tmjson.RegisterType(&BlockPartMessage{}, "tendermint/BlockPart")
	tmjson.RegisterType(&VoteMessage{}, "tendermint/Vote")
	tmjson.RegisterType(&HasVoteMessage{}, "tendermint/HasVote")
	tmjson.RegisterType(&VoteSetMaj23Message{}, "tendermint/VoteSetMaj23")
	tmjson.RegisterType(&VoteSetBitsMessage{}, "tendermint/VoteSetBits")
}

func decodeMsg(bz []byte) (msg Message, err error) {
	pb := &tmcons.Message{}
	if err = proto.Unmarshal(bz, pb); err != nil {
		return msg, err
	}

	return MsgFromProto(pb)
}

//-------------------------------------

// NewRoundStepMessage is sent for every step taken in the ConsensusState.
// For every height/round/step transition
type NewRoundStepMessage struct {
	Height                int64
	Round                 int32
	Step                  cstypes.RoundStepType
	SecondsSinceStartTime int64
	LastCommitRound       int32
}

// ValidateBasic performs basic validation.
func (m *NewRoundStepMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	if m.Round < 0 {
		return errors.New("negative Round")
	}
	if !m.Step.IsValid() {
		return errors.New("invalid Step")
	}

	// NOTE: SecondsSinceStartTime may be negative

	// LastCommitRound will be -1 for the initial height, but we don't know what height this is
	// since it can be specified in genesis. The reactor will have to validate this via
	// ValidateHeight().
	if m.LastCommitRound < -1 {
		return errors.New("invalid LastCommitRound (cannot be < -1)")
	}

	return nil
}

// ValidateHeight validates the height given the chain's initial height.
func (m *NewRoundStepMessage) ValidateHeight(initialHeight int64) error {
	if m.Height < initialHeight {
		return fmt.Errorf("invalid Height %v (lower than initial height %v)",
			m.Height, initialHeight)
	}
	if m.Height == initialHeight && m.LastCommitRound != -1 {
		return fmt.Errorf("invalid LastCommitRound %v (must be -1 for initial height %v)",
			m.LastCommitRound, initialHeight)
	}
	if m.Height > initialHeight && m.LastCommitRound < 0 {
		return fmt.Errorf("LastCommitRound can only be negative for initial height %v", // nolint
			initialHeight)
	}
	return nil
}

// String returns a string representation.
func (m *NewRoundStepMessage) String() string {
	return fmt.Sprintf("[NewRoundStep H:%v R:%v S:%v LCR:%v]",
		m.Height, m.Round, m.Step, m.LastCommitRound)
}

//-------------------------------------

// NewValidBlockMessage is sent when a validator observes a valid block B in some round r,
// i.e., there is a Proposal for block B and 2/3+ prevotes for the block B in the round r.
// In case the block is also committed, then IsCommit flag is set to true.
type NewValidBlockMessage struct {
	Height             int64
	Round              int32
	BlockPartSetHeader types.PartSetHeader
	BlockParts         *bits.BitArray
	IsCommit           bool
}

// ValidateBasic performs basic validation.
func (m *NewValidBlockMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	if m.Round < 0 {
		return errors.New("negative Round")
	}
	if err := m.BlockPartSetHeader.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong BlockPartSetHeader: %v", err)
	}
	if m.BlockParts.Size() == 0 {
		return errors.New("empty blockParts")
	}
	if m.BlockParts.Size() != int(m.BlockPartSetHeader.Total) {
		return fmt.Errorf("blockParts bit array size %d not equal to BlockPartSetHeader.Total %d",
			m.BlockParts.Size(),
			m.BlockPartSetHeader.Total)
	}
	if m.BlockParts.Size() > int(types.MaxBlockPartsCount) {
		return fmt.Errorf("blockParts bit array is too big: %d, max: %d", m.BlockParts.Size(), types.MaxBlockPartsCount)
	}
	return nil
}

// String returns a string representation.
func (m *NewValidBlockMessage) String() string {
	return fmt.Sprintf("[ValidBlockMessage H:%v R:%v BP:%v BA:%v IsCommit:%v]",
		m.Height, m.Round, m.BlockPartSetHeader, m.BlockParts, m.IsCommit)
}

//-------------------------------------

// ProposalMessage is sent when a new block is proposed.
type ProposalMessage struct {
	Proposal *types.Proposal
}

// ValidateBasic performs basic validation.
func (m *ProposalMessage) ValidateBasic() error {
	return m.Proposal.ValidateBasic()
}

// String returns a string representation.
func (m *ProposalMessage) String() string {
	return fmt.Sprintf("[Proposal %v]", m.Proposal)
}

//-------------------------------------

// ProposalPOLMessage is sent when a previous proposal is re-proposed.
type ProposalPOLMessage struct {
	Height           int64
	ProposalPOLRound int32
	ProposalPOL      *bits.BitArray
}

// ValidateBasic performs basic validation.
func (m *ProposalPOLMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	if m.ProposalPOLRound < 0 {
		return errors.New("negative ProposalPOLRound")
	}
	if m.ProposalPOL.Size() == 0 {
		return errors.New("empty ProposalPOL bit array")
	}
	if m.ProposalPOL.Size() > types.MaxVotesCount {
		return fmt.Errorf("proposalPOL bit array is too big: %d, max: %d", m.ProposalPOL.Size(), types.MaxVotesCount)
	}
	return nil
}

// String returns a string representation.
func (m *ProposalPOLMessage) String() string {
	return fmt.Sprintf("[ProposalPOL H:%v POLR:%v POL:%v]", m.Height, m.ProposalPOLRound, m.ProposalPOL)
}

//-------------------------------------

// BlockPartMessage is sent when gossipping a piece of the proposed block.
type BlockPartMessage struct {
	Height int64
	Round  int32
	Part   *types.Part
}

// ValidateBasic performs basic validation.
func (m *BlockPartMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	if m.Round < 0 {
		return errors.New("negative Round")
	}
	if err := m.Part.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong Part: %v", err)
	}
	return nil
}

// String returns a string representation.
func (m *BlockPartMessage) String() string {
	return fmt.Sprintf("[BlockPart H:%v R:%v P:%v]", m.Height, m.Round, m.Part)
}

//-------------------------------------

// VoteMessage is sent when voting for a proposal (or lack thereof).
type VoteMessage struct {
	Vote *types.Vote
}

// ValidateBasic performs basic validation.
func (m *VoteMessage) ValidateBasic() error {
	return m.Vote.ValidateBasic()
}

// String returns a string representation.
func (m *VoteMessage) String() string {
	return fmt.Sprintf("[Vote %v]", m.Vote)
}

//-------------------------------------

// HasVoteMessage is sent to indicate that a particular vote has been received.
type HasVoteMessage struct {
	Height int64
	Round  int32
	Type   tmproto.SignedMsgType
	Index  int32
}

// ValidateBasic performs basic validation.
func (m *HasVoteMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	if m.Round < 0 {
		return errors.New("negative Round")
	}
	if !types.IsVoteTypeValid(m.Type) {
		return errors.New("invalid Type")
	}
	if m.Index < 0 {
		return errors.New("negative Index")
	}
	return nil
}

// String returns a string representation.
func (m *HasVoteMessage) String() string {
	return fmt.Sprintf("[HasVote VI:%v V:{%v/%02d/%v}]", m.Index, m.Height, m.Round, m.Type)
}

//-------------------------------------

// VoteSetMaj23Message is sent to indicate that a given BlockID has seen +2/3 votes.
type VoteSetMaj23Message struct {
	Height  int64
	Round   int32
	Type    tmproto.SignedMsgType
	BlockID types.BlockID
}

// ValidateBasic performs basic validation.
func (m *VoteSetMaj23Message) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	if m.Round < 0 {
		return errors.New("negative Round")
	}
	if !types.IsVoteTypeValid(m.Type) {
		return errors.New("invalid Type")
	}
	if err := m.BlockID.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong BlockID: %v", err)
	}
	return nil
}

// String returns a string representation.
func (m *VoteSetMaj23Message) String() string {
	return fmt.Sprintf("[VSM23 %v/%02d/%v %v]", m.Height, m.Round, m.Type, m.BlockID)
}

//-------------------------------------

// VoteSetBitsMessage is sent to communicate the bit-array of votes seen for the BlockID.
type VoteSetBitsMessage struct {
	Height  int64
	Round   int32
	Type    tmproto.SignedMsgType
	BlockID types.BlockID
	Votes   *bits.BitArray
}

// ValidateBasic performs basic validation.
func (m *VoteSetBitsMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("negative Height")
	}
	if !types.IsVoteTypeValid(m.Type) {
		return errors.New("invalid Type")
	}
	if err := m.BlockID.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong BlockID: %v", err)
	}
	// NOTE: Votes.Size() can be zero if the node does not have any
	if m.Votes.Size() > types.MaxVotesCount {
		return fmt.Errorf("votes bit array is too big: %d, max: %d", m.Votes.Size(), types.MaxVotesCount)
	}
	return nil
}

// String returns a string representation.
func (m *VoteSetBitsMessage) String() string {
	return fmt.Sprintf("[VSB %v/%02d/%v %v %v]", m.Height, m.Round, m.Type, m.BlockID, m.Votes)
}

//-------------------------------------
