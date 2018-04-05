package consensus

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/pkg/errors"

	wire "github.com/tendermint/go-wire"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"

	cstypes "github.com/tendermint/tendermint/consensus/types"
	"github.com/tendermint/tendermint/p2p"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

const (
	StateChannel       = byte(0x20)
	DataChannel        = byte(0x21)
	VoteChannel        = byte(0x22)
	VoteSetBitsChannel = byte(0x23)

	maxConsensusMessageSize = 1048576 // 1MB; NOTE/TODO: keep in sync with types.PartSet sizes.

	blocksToContributeToBecomeGoodPeer = 10000
)

//-----------------------------------------------------------------------------

// ConsensusReactor defines a reactor for the consensus service.
type ConsensusReactor struct {
	p2p.BaseReactor // BaseService + p2p.Switch

	conS *ConsensusState

	mtx      sync.RWMutex
	fastSync bool
	eventBus *types.EventBus
}

// NewConsensusReactor returns a new ConsensusReactor with the given consensusState.
func NewConsensusReactor(consensusState *ConsensusState, fastSync bool) *ConsensusReactor {
	conR := &ConsensusReactor{
		conS:     consensusState,
		fastSync: fastSync,
	}
	conR.BaseReactor = *p2p.NewBaseReactor("ConsensusReactor", conR)
	return conR
}

// OnStart implements BaseService.
func (conR *ConsensusReactor) OnStart() error {
	conR.Logger.Info("ConsensusReactor ", "fastSync", conR.FastSync())
	if err := conR.BaseReactor.OnStart(); err != nil {
		return err
	}

	err := conR.startBroadcastRoutine()
	if err != nil {
		return err
	}

	if !conR.FastSync() {
		err := conR.conS.Start()
		if err != nil {
			return err
		}
	}

	return nil
}

// OnStop implements BaseService
func (conR *ConsensusReactor) OnStop() {
	conR.BaseReactor.OnStop()
	conR.conS.Stop()
}

// SwitchToConsensus switches from fast_sync mode to consensus mode.
// It resets the state, turns off fast_sync, and starts the consensus state-machine
func (conR *ConsensusReactor) SwitchToConsensus(state sm.State, blocksSynced int) {
	conR.Logger.Info("SwitchToConsensus")
	conR.conS.reconstructLastCommit(state)
	// NOTE: The line below causes broadcastNewRoundStepRoutine() to
	// broadcast a NewRoundStepMessage.
	conR.conS.updateToState(state)

	conR.mtx.Lock()
	conR.fastSync = false
	conR.mtx.Unlock()

	if blocksSynced > 0 {
		// dont bother with the WAL if we fast synced
		conR.conS.doWALCatchup = false
	}
	err := conR.conS.Start()
	if err != nil {
		conR.Logger.Error("Error starting conS", "err", err)
	}
}

// GetChannels implements Reactor
func (conR *ConsensusReactor) GetChannels() []*p2p.ChannelDescriptor {
	// TODO optimize
	return []*p2p.ChannelDescriptor{
		{
			ID:                StateChannel,
			Priority:          5,
			SendQueueCapacity: 100,
		},
		{
			ID:                 DataChannel, // maybe split between gossiping current block and catchup stuff
			Priority:           10,          // once we gossip the whole block there's nothing left to send until next height or round
			SendQueueCapacity:  100,
			RecvBufferCapacity: 50 * 4096,
		},
		{
			ID:                 VoteChannel,
			Priority:           5,
			SendQueueCapacity:  100,
			RecvBufferCapacity: 100 * 100,
		},
		{
			ID:                 VoteSetBitsChannel,
			Priority:           1,
			SendQueueCapacity:  2,
			RecvBufferCapacity: 1024,
		},
	}
}

// AddPeer implements Reactor
func (conR *ConsensusReactor) AddPeer(peer p2p.Peer) {
	if !conR.IsRunning() {
		return
	}

	// Create peerState for peer
	peerState := NewPeerState(peer).SetLogger(conR.Logger)
	peer.Set(types.PeerStateKey, peerState)

	// Begin routines for this peer.
	go conR.gossipDataRoutine(peer, peerState)
	go conR.gossipVotesRoutine(peer, peerState)
	go conR.queryMaj23Routine(peer, peerState)

	// Send our state to peer.
	// If we're fast_syncing, broadcast a RoundStepMessage later upon SwitchToConsensus().
	if !conR.FastSync() {
		conR.sendNewRoundStepMessages(peer)
	}
}

// RemovePeer implements Reactor
func (conR *ConsensusReactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	if !conR.IsRunning() {
		return
	}
	// TODO
	//peer.Get(PeerStateKey).(*PeerState).Disconnect()
}

// Receive implements Reactor
// NOTE: We process these messages even when we're fast_syncing.
// Messages affect either a peer state or the consensus state.
// Peer state updates can happen in parallel, but processing of
// proposals, block parts, and votes are ordered by the receiveRoutine
// NOTE: blocks on consensus state for proposals, block parts, and votes
func (conR *ConsensusReactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	if !conR.IsRunning() {
		conR.Logger.Debug("Receive", "src", src, "chId", chID, "bytes", msgBytes)
		return
	}

	_, msg, err := DecodeMessage(msgBytes)
	if err != nil {
		conR.Logger.Error("Error decoding message", "src", src, "chId", chID, "msg", msg, "err", err, "bytes", msgBytes)
		conR.Switch.StopPeerForError(src, err)
		return
	}
	conR.Logger.Debug("Receive", "src", src, "chId", chID, "msg", msg)

	// Get peer states
	ps := src.Get(types.PeerStateKey).(*PeerState)

	switch chID {
	case StateChannel:
		switch msg := msg.(type) {
		case *NewRoundStepMessage:
			ps.ApplyNewRoundStepMessage(msg)
		case *CommitStepMessage:
			ps.ApplyCommitStepMessage(msg)
		case *HasVoteMessage:
			ps.ApplyHasVoteMessage(msg)
		case *VoteSetMaj23Message:
			cs := conR.conS
			cs.mtx.Lock()
			height, votes := cs.Height, cs.Votes
			cs.mtx.Unlock()
			if height != msg.Height {
				return
			}
			// Peer claims to have a maj23 for some BlockID at H,R,S,
			err := votes.SetPeerMaj23(msg.Round, msg.Type, ps.Peer.ID(), msg.BlockID)
			if err != nil {
				conR.Switch.StopPeerForError(src, err)
				return
			}
			// Respond with a VoteSetBitsMessage showing which votes we have.
			// (and consequently shows which we don't have)
			var ourVotes *cmn.BitArray
			switch msg.Type {
			case types.VoteTypePrevote:
				ourVotes = votes.Prevotes(msg.Round).BitArrayByBlockID(msg.BlockID)
			case types.VoteTypePrecommit:
				ourVotes = votes.Precommits(msg.Round).BitArrayByBlockID(msg.BlockID)
			default:
				conR.Logger.Error("Bad VoteSetBitsMessage field Type")
				return
			}
			src.TrySend(VoteSetBitsChannel, struct{ ConsensusMessage }{&VoteSetBitsMessage{
				Height:  msg.Height,
				Round:   msg.Round,
				Type:    msg.Type,
				BlockID: msg.BlockID,
				Votes:   ourVotes,
			}})
		case *ProposalHeartbeatMessage:
			hb := msg.Heartbeat
			conR.Logger.Debug("Received proposal heartbeat message",
				"height", hb.Height, "round", hb.Round, "sequence", hb.Sequence,
				"valIdx", hb.ValidatorIndex, "valAddr", hb.ValidatorAddress)
		default:
			conR.Logger.Error(cmn.Fmt("Unknown message type %v", reflect.TypeOf(msg)))
		}

	case DataChannel:
		if conR.FastSync() {
			conR.Logger.Info("Ignoring message received during fastSync", "msg", msg)
			return
		}
		switch msg := msg.(type) {
		case *ProposalMessage:
			ps.SetHasProposal(msg.Proposal)
			conR.conS.peerMsgQueue <- msgInfo{msg, src.ID()}
		case *ProposalPOLMessage:
			ps.ApplyProposalPOLMessage(msg)
		case *BlockPartMessage:
			ps.SetHasProposalBlockPart(msg.Height, msg.Round, msg.Part.Index)
			if numBlocks := ps.RecordBlockPart(msg); numBlocks%blocksToContributeToBecomeGoodPeer == 0 {
				conR.Switch.MarkPeerAsGood(src)
			}
			conR.conS.peerMsgQueue <- msgInfo{msg, src.ID()}
		default:
			conR.Logger.Error(cmn.Fmt("Unknown message type %v", reflect.TypeOf(msg)))
		}

	case VoteChannel:
		if conR.FastSync() {
			conR.Logger.Info("Ignoring message received during fastSync", "msg", msg)
			return
		}
		switch msg := msg.(type) {
		case *VoteMessage:
			cs := conR.conS
			cs.mtx.Lock()
			height, valSize, lastCommitSize := cs.Height, cs.Validators.Size(), cs.LastCommit.Size()
			cs.mtx.Unlock()
			ps.EnsureVoteBitArrays(height, valSize)
			ps.EnsureVoteBitArrays(height-1, lastCommitSize)
			ps.SetHasVote(msg.Vote)
			if blocks := ps.RecordVote(msg.Vote); blocks%blocksToContributeToBecomeGoodPeer == 0 {
				conR.Switch.MarkPeerAsGood(src)
			}

			cs.peerMsgQueue <- msgInfo{msg, src.ID()}

		default:
			// don't punish (leave room for soft upgrades)
			conR.Logger.Error(cmn.Fmt("Unknown message type %v", reflect.TypeOf(msg)))
		}

	case VoteSetBitsChannel:
		if conR.FastSync() {
			conR.Logger.Info("Ignoring message received during fastSync", "msg", msg)
			return
		}
		switch msg := msg.(type) {
		case *VoteSetBitsMessage:
			cs := conR.conS
			cs.mtx.Lock()
			height, votes := cs.Height, cs.Votes
			cs.mtx.Unlock()

			if height == msg.Height {
				var ourVotes *cmn.BitArray
				switch msg.Type {
				case types.VoteTypePrevote:
					ourVotes = votes.Prevotes(msg.Round).BitArrayByBlockID(msg.BlockID)
				case types.VoteTypePrecommit:
					ourVotes = votes.Precommits(msg.Round).BitArrayByBlockID(msg.BlockID)
				default:
					conR.Logger.Error("Bad VoteSetBitsMessage field Type")
					return
				}
				ps.ApplyVoteSetBitsMessage(msg, ourVotes)
			} else {
				ps.ApplyVoteSetBitsMessage(msg, nil)
			}
		default:
			// don't punish (leave room for soft upgrades)
			conR.Logger.Error(cmn.Fmt("Unknown message type %v", reflect.TypeOf(msg)))
		}

	default:
		conR.Logger.Error(cmn.Fmt("Unknown chId %X", chID))
	}

	if err != nil {
		conR.Logger.Error("Error in Receive()", "err", err)
	}
}

// SetEventBus sets event bus.
func (conR *ConsensusReactor) SetEventBus(b *types.EventBus) {
	conR.eventBus = b
	conR.conS.SetEventBus(b)
}

// FastSync returns whether the consensus reactor is in fast-sync mode.
func (conR *ConsensusReactor) FastSync() bool {
	conR.mtx.RLock()
	defer conR.mtx.RUnlock()
	return conR.fastSync
}

//--------------------------------------

// startBroadcastRoutine subscribes for new round steps, votes and proposal
// heartbeats using the event bus and starts a go routine to broadcasts events
// to peers upon receiving them.
func (conR *ConsensusReactor) startBroadcastRoutine() error {
	const subscriber = "consensus-reactor"
	ctx := context.Background()

	// new round steps
	stepsCh := make(chan interface{})
	err := conR.eventBus.Subscribe(ctx, subscriber, types.EventQueryNewRoundStep, stepsCh)
	if err != nil {
		return errors.Wrapf(err, "failed to subscribe %s to %s", subscriber, types.EventQueryNewRoundStep)
	}

	// votes
	votesCh := make(chan interface{})
	err = conR.eventBus.Subscribe(ctx, subscriber, types.EventQueryVote, votesCh)
	if err != nil {
		return errors.Wrapf(err, "failed to subscribe %s to %s", subscriber, types.EventQueryVote)
	}

	// proposal heartbeats
	heartbeatsCh := make(chan interface{})
	err = conR.eventBus.Subscribe(ctx, subscriber, types.EventQueryProposalHeartbeat, heartbeatsCh)
	if err != nil {
		return errors.Wrapf(err, "failed to subscribe %s to %s", subscriber, types.EventQueryProposalHeartbeat)
	}

	go func() {
		var data interface{}
		var ok bool
		for {
			select {
			case data, ok = <-stepsCh:
				if ok { // a receive from a closed channel returns the zero value immediately
					edrs := data.(types.TMEventData).Unwrap().(types.EventDataRoundState)
					conR.broadcastNewRoundStep(edrs.RoundState.(*cstypes.RoundState))
				}
			case data, ok = <-votesCh:
				if ok {
					edv := data.(types.TMEventData).Unwrap().(types.EventDataVote)
					conR.broadcastHasVoteMessage(edv.Vote)
				}
			case data, ok = <-heartbeatsCh:
				if ok {
					edph := data.(types.TMEventData).Unwrap().(types.EventDataProposalHeartbeat)
					conR.broadcastProposalHeartbeatMessage(edph)
				}
			case <-conR.Quit():
				conR.eventBus.UnsubscribeAll(ctx, subscriber)
				return
			}
			if !ok {
				conR.eventBus.UnsubscribeAll(ctx, subscriber)
				return
			}
		}
	}()

	return nil
}

func (conR *ConsensusReactor) broadcastProposalHeartbeatMessage(heartbeat types.EventDataProposalHeartbeat) {
	hb := heartbeat.Heartbeat
	conR.Logger.Debug("Broadcasting proposal heartbeat message",
		"height", hb.Height, "round", hb.Round, "sequence", hb.Sequence)
	msg := &ProposalHeartbeatMessage{hb}
	conR.Switch.Broadcast(StateChannel, struct{ ConsensusMessage }{msg})
}

func (conR *ConsensusReactor) broadcastNewRoundStep(rs *cstypes.RoundState) {
	nrsMsg, csMsg := makeRoundStepMessages(rs)
	if nrsMsg != nil {
		conR.Switch.Broadcast(StateChannel, struct{ ConsensusMessage }{nrsMsg})
	}
	if csMsg != nil {
		conR.Switch.Broadcast(StateChannel, struct{ ConsensusMessage }{csMsg})
	}
}

// Broadcasts HasVoteMessage to peers that care.
func (conR *ConsensusReactor) broadcastHasVoteMessage(vote *types.Vote) {
	msg := &HasVoteMessage{
		Height: vote.Height,
		Round:  vote.Round,
		Type:   vote.Type,
		Index:  vote.ValidatorIndex,
	}
	conR.Switch.Broadcast(StateChannel, struct{ ConsensusMessage }{msg})
	/*
		// TODO: Make this broadcast more selective.
		for _, peer := range conR.Switch.Peers().List() {
			ps := peer.Get(PeerStateKey).(*PeerState)
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

func makeRoundStepMessages(rs *cstypes.RoundState) (nrsMsg *NewRoundStepMessage, csMsg *CommitStepMessage) {
	nrsMsg = &NewRoundStepMessage{
		Height: rs.Height,
		Round:  rs.Round,
		Step:   rs.Step,
		SecondsSinceStartTime: int(time.Since(rs.StartTime).Seconds()),
		LastCommitRound:       rs.LastCommit.Round(),
	}
	if rs.Step == cstypes.RoundStepCommit {
		csMsg = &CommitStepMessage{
			Height:           rs.Height,
			BlockPartsHeader: rs.ProposalBlockParts.Header(),
			BlockParts:       rs.ProposalBlockParts.BitArray(),
		}
	}
	return
}

func (conR *ConsensusReactor) sendNewRoundStepMessages(peer p2p.Peer) {
	rs := conR.conS.GetRoundState()
	nrsMsg, csMsg := makeRoundStepMessages(rs)
	if nrsMsg != nil {
		peer.Send(StateChannel, struct{ ConsensusMessage }{nrsMsg})
	}
	if csMsg != nil {
		peer.Send(StateChannel, struct{ ConsensusMessage }{csMsg})
	}
}

func (conR *ConsensusReactor) gossipDataRoutine(peer p2p.Peer, ps *PeerState) {
	logger := conR.Logger.With("peer", peer)

OUTER_LOOP:
	for {
		// Manage disconnects from self or peer.
		if !peer.IsRunning() || !conR.IsRunning() {
			logger.Info("Stopping gossipDataRoutine for peer")
			return
		}
		rs := conR.conS.GetRoundState()
		prs := ps.GetRoundState()

		// Send proposal Block parts?
		if rs.ProposalBlockParts.HasHeader(prs.ProposalBlockPartsHeader) {
			if index, ok := rs.ProposalBlockParts.BitArray().Sub(prs.ProposalBlockParts.Copy()).PickRandom(); ok {
				part := rs.ProposalBlockParts.GetPart(index)
				msg := &BlockPartMessage{
					Height: rs.Height, // This tells peer that this part applies to us.
					Round:  rs.Round,  // This tells peer that this part applies to us.
					Part:   part,
				}
				logger.Debug("Sending block part", "height", prs.Height, "round", prs.Round)
				if peer.Send(DataChannel, struct{ ConsensusMessage }{msg}) {
					ps.SetHasProposalBlockPart(prs.Height, prs.Round, index)
				}
				continue OUTER_LOOP
			}
		}

		// If the peer is on a previous height, help catch up.
		if (0 < prs.Height) && (prs.Height < rs.Height) {
			heightLogger := logger.With("height", prs.Height)

			// if we never received the commit message from the peer, the block parts wont be initialized
			if prs.ProposalBlockParts == nil {
				blockMeta := conR.conS.blockStore.LoadBlockMeta(prs.Height)
				if blockMeta == nil {
					cmn.PanicCrisis(cmn.Fmt("Failed to load block %d when blockStore is at %d",
						prs.Height, conR.conS.blockStore.Height()))
				}
				ps.InitProposalBlockParts(blockMeta.BlockID.PartsHeader)
				// continue the loop since prs is a copy and not effected by this initialization
				continue OUTER_LOOP
			}
			conR.gossipDataForCatchup(heightLogger, rs, prs, ps, peer)
			continue OUTER_LOOP
		}

		// If height and round don't match, sleep.
		if (rs.Height != prs.Height) || (rs.Round != prs.Round) {
			//logger.Info("Peer Height|Round mismatch, sleeping", "peerHeight", prs.Height, "peerRound", prs.Round, "peer", peer)
			time.Sleep(conR.conS.config.PeerGossipSleep())
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
				if peer.Send(DataChannel, struct{ ConsensusMessage }{msg}) {
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
				peer.Send(DataChannel, struct{ ConsensusMessage }{msg})
			}
			continue OUTER_LOOP
		}

		// Nothing to do. Sleep.
		time.Sleep(conR.conS.config.PeerGossipSleep())
		continue OUTER_LOOP
	}
}

func (conR *ConsensusReactor) gossipDataForCatchup(logger log.Logger, rs *cstypes.RoundState,
	prs *cstypes.PeerRoundState, ps *PeerState, peer p2p.Peer) {

	if index, ok := prs.ProposalBlockParts.Not().PickRandom(); ok {
		// Ensure that the peer's PartSetHeader is correct
		blockMeta := conR.conS.blockStore.LoadBlockMeta(prs.Height)
		if blockMeta == nil {
			logger.Error("Failed to load block meta",
				"ourHeight", rs.Height, "blockstoreHeight", conR.conS.blockStore.Height())
			time.Sleep(conR.conS.config.PeerGossipSleep())
			return
		} else if !blockMeta.BlockID.PartsHeader.Equals(prs.ProposalBlockPartsHeader) {
			logger.Info("Peer ProposalBlockPartsHeader mismatch, sleeping",
				"blockPartsHeader", blockMeta.BlockID.PartsHeader, "peerBlockPartsHeader", prs.ProposalBlockPartsHeader)
			time.Sleep(conR.conS.config.PeerGossipSleep())
			return
		}
		// Load the part
		part := conR.conS.blockStore.LoadBlockPart(prs.Height, index)
		if part == nil {
			logger.Error("Could not load part", "index", index,
				"blockPartsHeader", blockMeta.BlockID.PartsHeader, "peerBlockPartsHeader", prs.ProposalBlockPartsHeader)
			time.Sleep(conR.conS.config.PeerGossipSleep())
			return
		}
		// Send the part
		msg := &BlockPartMessage{
			Height: prs.Height, // Not our height, so it doesn't matter.
			Round:  prs.Round,  // Not our height, so it doesn't matter.
			Part:   part,
		}
		logger.Debug("Sending block part for catchup", "round", prs.Round, "index", index)
		if peer.Send(DataChannel, struct{ ConsensusMessage }{msg}) {
			ps.SetHasProposalBlockPart(prs.Height, prs.Round, index)
		} else {
			logger.Debug("Sending block part for catchup failed")
		}
		return
	}
	//logger.Info("No parts to send in catch-up, sleeping")
	time.Sleep(conR.conS.config.PeerGossipSleep())
}

func (conR *ConsensusReactor) gossipVotesRoutine(peer p2p.Peer, ps *PeerState) {
	logger := conR.Logger.With("peer", peer)

	// Simple hack to throttle logs upon sleep.
	var sleeping = 0

OUTER_LOOP:
	for {
		// Manage disconnects from self or peer.
		if !peer.IsRunning() || !conR.IsRunning() {
			logger.Info("Stopping gossipVotesRoutine for peer")
			return
		}
		rs := conR.conS.GetRoundState()
		prs := ps.GetRoundState()

		switch sleeping {
		case 1: // First sleep
			sleeping = 2
		case 2: // No more sleep
			sleeping = 0
		}

		//logger.Debug("gossipVotesRoutine", "rsHeight", rs.Height, "rsRound", rs.Round,
		//	"prsHeight", prs.Height, "prsRound", prs.Round, "prsStep", prs.Step)

		// If height matches, then send LastCommit, Prevotes, Precommits.
		if rs.Height == prs.Height {
			heightLogger := logger.With("height", prs.Height)
			if conR.gossipVotesForHeight(heightLogger, rs, prs, ps) {
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
		if prs.Height != 0 && rs.Height >= prs.Height+2 {
			// Load the block commit for prs.Height,
			// which contains precommit signatures for prs.Height.
			commit := conR.conS.blockStore.LoadBlockCommit(prs.Height)
			if ps.PickSendVote(commit) {
				logger.Debug("Picked Catchup commit to send", "height", prs.Height)
				continue OUTER_LOOP
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

		time.Sleep(conR.conS.config.PeerGossipSleep())
		continue OUTER_LOOP
	}
}

func (conR *ConsensusReactor) gossipVotesForHeight(logger log.Logger, rs *cstypes.RoundState, prs *cstypes.PeerRoundState, ps *PeerState) bool {

	// If there are lastCommits to send...
	if prs.Step == cstypes.RoundStepNewHeight {
		if ps.PickSendVote(rs.LastCommit) {
			logger.Debug("Picked rs.LastCommit to send")
			return true
		}
	}
	// If there are prevotes to send...
	if prs.Step <= cstypes.RoundStepPrevote && prs.Round != -1 && prs.Round <= rs.Round {
		if ps.PickSendVote(rs.Votes.Prevotes(prs.Round)) {
			logger.Debug("Picked rs.Prevotes(prs.Round) to send", "round", prs.Round)
			return true
		}
	}
	// If there are precommits to send...
	if prs.Step <= cstypes.RoundStepPrecommit && prs.Round != -1 && prs.Round <= rs.Round {
		if ps.PickSendVote(rs.Votes.Precommits(prs.Round)) {
			logger.Debug("Picked rs.Precommits(prs.Round) to send", "round", prs.Round)
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
func (conR *ConsensusReactor) queryMaj23Routine(peer p2p.Peer, ps *PeerState) {
	logger := conR.Logger.With("peer", peer)

OUTER_LOOP:
	for {
		// Manage disconnects from self or peer.
		if !peer.IsRunning() || !conR.IsRunning() {
			logger.Info("Stopping queryMaj23Routine for peer")
			return
		}

		// Maybe send Height/Round/Prevotes
		{
			rs := conR.conS.GetRoundState()
			prs := ps.GetRoundState()
			if rs.Height == prs.Height {
				if maj23, ok := rs.Votes.Prevotes(prs.Round).TwoThirdsMajority(); ok {
					peer.TrySend(StateChannel, struct{ ConsensusMessage }{&VoteSetMaj23Message{
						Height:  prs.Height,
						Round:   prs.Round,
						Type:    types.VoteTypePrevote,
						BlockID: maj23,
					}})
					time.Sleep(conR.conS.config.PeerQueryMaj23Sleep())
				}
			}
		}

		// Maybe send Height/Round/Precommits
		{
			rs := conR.conS.GetRoundState()
			prs := ps.GetRoundState()
			if rs.Height == prs.Height {
				if maj23, ok := rs.Votes.Precommits(prs.Round).TwoThirdsMajority(); ok {
					peer.TrySend(StateChannel, struct{ ConsensusMessage }{&VoteSetMaj23Message{
						Height:  prs.Height,
						Round:   prs.Round,
						Type:    types.VoteTypePrecommit,
						BlockID: maj23,
					}})
					time.Sleep(conR.conS.config.PeerQueryMaj23Sleep())
				}
			}
		}

		// Maybe send Height/Round/ProposalPOL
		{
			rs := conR.conS.GetRoundState()
			prs := ps.GetRoundState()
			if rs.Height == prs.Height && prs.ProposalPOLRound >= 0 {
				if maj23, ok := rs.Votes.Prevotes(prs.ProposalPOLRound).TwoThirdsMajority(); ok {
					peer.TrySend(StateChannel, struct{ ConsensusMessage }{&VoteSetMaj23Message{
						Height:  prs.Height,
						Round:   prs.ProposalPOLRound,
						Type:    types.VoteTypePrevote,
						BlockID: maj23,
					}})
					time.Sleep(conR.conS.config.PeerQueryMaj23Sleep())
				}
			}
		}

		// Little point sending LastCommitRound/LastCommit,
		// These are fleeting and non-blocking.

		// Maybe send Height/CatchupCommitRound/CatchupCommit.
		{
			prs := ps.GetRoundState()
			if prs.CatchupCommitRound != -1 && 0 < prs.Height && prs.Height <= conR.conS.blockStore.Height() {
				commit := conR.conS.LoadCommit(prs.Height)
				peer.TrySend(StateChannel, struct{ ConsensusMessage }{&VoteSetMaj23Message{
					Height:  prs.Height,
					Round:   commit.Round(),
					Type:    types.VoteTypePrecommit,
					BlockID: commit.BlockID,
				}})
				time.Sleep(conR.conS.config.PeerQueryMaj23Sleep())
			}
		}

		time.Sleep(conR.conS.config.PeerQueryMaj23Sleep())

		continue OUTER_LOOP
	}
}

// String returns a string representation of the ConsensusReactor.
// NOTE: For now, it is just a hard-coded string to avoid accessing unprotected shared variables.
// TODO: improve!
func (conR *ConsensusReactor) String() string {
	// better not to access shared variables
	return "ConsensusReactor" // conR.StringIndented("")
}

// StringIndented returns an indented string representation of the ConsensusReactor
func (conR *ConsensusReactor) StringIndented(indent string) string {
	s := "ConsensusReactor{\n"
	s += indent + "  " + conR.conS.StringIndented(indent+"  ") + "\n"
	for _, peer := range conR.Switch.Peers().List() {
		ps := peer.Get(types.PeerStateKey).(*PeerState)
		s += indent + "  " + ps.StringIndented(indent+"  ") + "\n"
	}
	s += indent + "}"
	return s
}

//-----------------------------------------------------------------------------

var (
	ErrPeerStateHeightRegression = errors.New("Error peer state height regression")
	ErrPeerStateInvalidStartTime = errors.New("Error peer state invalid startTime")
)

// PeerState contains the known state of a peer, including its connection
// and threadsafe access to its PeerRoundState.
type PeerState struct {
	Peer   p2p.Peer
	logger log.Logger

	mtx sync.Mutex
	cstypes.PeerRoundState

	stats *peerStateStats
}

// peerStateStats holds internal statistics for a peer.
type peerStateStats struct {
	lastVoteHeight int64
	votes          int

	lastBlockPartHeight int64
	blockParts          int
}

func (pss peerStateStats) String() string {
	return fmt.Sprintf("peerStateStats{votes: %d, blockParts: %d}", pss.votes, pss.blockParts)
}

// NewPeerState returns a new PeerState for the given Peer
func NewPeerState(peer p2p.Peer) *PeerState {
	return &PeerState{
		Peer:   peer,
		logger: log.NewNopLogger(),
		PeerRoundState: cstypes.PeerRoundState{
			Round:              -1,
			ProposalPOLRound:   -1,
			LastCommitRound:    -1,
			CatchupCommitRound: -1,
		},
		stats: &peerStateStats{},
	}
}

func (ps *PeerState) SetLogger(logger log.Logger) *PeerState {
	ps.logger = logger
	return ps
}

// GetRoundState returns an atomic snapshot of the PeerRoundState.
// There's no point in mutating it since it won't change PeerState.
func (ps *PeerState) GetRoundState() *cstypes.PeerRoundState {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	prs := ps.PeerRoundState // copy
	return &prs
}

// GetHeight returns an atomic snapshot of the PeerRoundState's height
// used by the mempool to ensure peers are caught up before broadcasting new txs
func (ps *PeerState) GetHeight() int64 {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return ps.PeerRoundState.Height
}

// SetHasProposal sets the given proposal as known for the peer.
func (ps *PeerState) SetHasProposal(proposal *types.Proposal) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != proposal.Height || ps.Round != proposal.Round {
		return
	}
	if ps.Proposal {
		return
	}

	ps.Proposal = true
	ps.ProposalBlockPartsHeader = proposal.BlockPartsHeader
	ps.ProposalBlockParts = cmn.NewBitArray(proposal.BlockPartsHeader.Total)
	ps.ProposalPOLRound = proposal.POLRound
	ps.ProposalPOL = nil // Nil until ProposalPOLMessage received.
}

// InitProposalBlockParts initializes the peer's proposal block parts header and bit array.
func (ps *PeerState) InitProposalBlockParts(partsHeader types.PartSetHeader) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.ProposalBlockParts != nil {
		return
	}

	ps.ProposalBlockPartsHeader = partsHeader
	ps.ProposalBlockParts = cmn.NewBitArray(partsHeader.Total)
}

// SetHasProposalBlockPart sets the given block part index as known for the peer.
func (ps *PeerState) SetHasProposalBlockPart(height int64, round int, index int) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != height || ps.Round != round {
		return
	}

	ps.ProposalBlockParts.SetIndex(index, true)
}

// PickSendVote picks a vote and sends it to the peer.
// Returns true if vote was sent.
func (ps *PeerState) PickSendVote(votes types.VoteSetReader) bool {
	if vote, ok := ps.PickVoteToSend(votes); ok {
		msg := &VoteMessage{vote}
		ps.logger.Debug("Sending vote message", "ps", ps, "vote", vote)
		return ps.Peer.Send(VoteChannel, struct{ ConsensusMessage }{msg})
	}
	return false
}

// PickVoteToSend picks a vote to send to the peer.
// Returns true if a vote was picked.
// NOTE: `votes` must be the correct Size() for the Height().
func (ps *PeerState) PickVoteToSend(votes types.VoteSetReader) (vote *types.Vote, ok bool) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if votes.Size() == 0 {
		return nil, false
	}

	height, round, type_, size := votes.Height(), votes.Round(), votes.Type(), votes.Size()

	// Lazily set data using 'votes'.
	if votes.IsCommit() {
		ps.ensureCatchupCommitRound(height, round, size)
	}
	ps.ensureVoteBitArrays(height, size)

	psVotes := ps.getVoteBitArray(height, round, type_)
	if psVotes == nil {
		return nil, false // Not something worth sending
	}
	if index, ok := votes.BitArray().Sub(psVotes).PickRandom(); ok {
		ps.setHasVote(height, round, type_, index)
		return votes.GetByIndex(index), true
	}
	return nil, false
}

func (ps *PeerState) getVoteBitArray(height int64, round int, type_ byte) *cmn.BitArray {
	if !types.IsVoteTypeValid(type_) {
		return nil
	}

	if ps.Height == height {
		if ps.Round == round {
			switch type_ {
			case types.VoteTypePrevote:
				return ps.Prevotes
			case types.VoteTypePrecommit:
				return ps.Precommits
			}
		}
		if ps.CatchupCommitRound == round {
			switch type_ {
			case types.VoteTypePrevote:
				return nil
			case types.VoteTypePrecommit:
				return ps.CatchupCommit
			}
		}
		if ps.ProposalPOLRound == round {
			switch type_ {
			case types.VoteTypePrevote:
				return ps.ProposalPOL
			case types.VoteTypePrecommit:
				return nil
			}
		}
		return nil
	}
	if ps.Height == height+1 {
		if ps.LastCommitRound == round {
			switch type_ {
			case types.VoteTypePrevote:
				return nil
			case types.VoteTypePrecommit:
				return ps.LastCommit
			}
		}
		return nil
	}
	return nil
}

// 'round': A round for which we have a +2/3 commit.
func (ps *PeerState) ensureCatchupCommitRound(height int64, round int, numValidators int) {
	if ps.Height != height {
		return
	}
	/*
		NOTE: This is wrong, 'round' could change.
		e.g. if orig round is not the same as block LastCommit round.
		if ps.CatchupCommitRound != -1 && ps.CatchupCommitRound != round {
			cmn.PanicSanity(cmn.Fmt("Conflicting CatchupCommitRound. Height: %v, Orig: %v, New: %v", height, ps.CatchupCommitRound, round))
		}
	*/
	if ps.CatchupCommitRound == round {
		return // Nothing to do!
	}
	ps.CatchupCommitRound = round
	if round == ps.Round {
		ps.CatchupCommit = ps.Precommits
	} else {
		ps.CatchupCommit = cmn.NewBitArray(numValidators)
	}
}

// EnsureVoteVitArrays ensures the bit-arrays have been allocated for tracking
// what votes this peer has received.
// NOTE: It's important to make sure that numValidators actually matches
// what the node sees as the number of validators for height.
func (ps *PeerState) EnsureVoteBitArrays(height int64, numValidators int) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	ps.ensureVoteBitArrays(height, numValidators)
}

func (ps *PeerState) ensureVoteBitArrays(height int64, numValidators int) {
	if ps.Height == height {
		if ps.Prevotes == nil {
			ps.Prevotes = cmn.NewBitArray(numValidators)
		}
		if ps.Precommits == nil {
			ps.Precommits = cmn.NewBitArray(numValidators)
		}
		if ps.CatchupCommit == nil {
			ps.CatchupCommit = cmn.NewBitArray(numValidators)
		}
		if ps.ProposalPOL == nil {
			ps.ProposalPOL = cmn.NewBitArray(numValidators)
		}
	} else if ps.Height == height+1 {
		if ps.LastCommit == nil {
			ps.LastCommit = cmn.NewBitArray(numValidators)
		}
	}
}

// RecordVote updates internal statistics for this peer by recording the vote.
// It returns the total number of votes (1 per block). This essentially means
// the number of blocks for which peer has been sending us votes.
func (ps *PeerState) RecordVote(vote *types.Vote) int {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.stats.lastVoteHeight >= vote.Height {
		return ps.stats.votes
	}
	ps.stats.lastVoteHeight = vote.Height
	ps.stats.votes++
	return ps.stats.votes
}

// VotesSent returns the number of blocks for which peer has been sending us
// votes.
func (ps *PeerState) VotesSent() int {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	return ps.stats.votes
}

// RecordBlockPart updates internal statistics for this peer by recording the
// block part. It returns the total number of block parts (1 per block). This
// essentially means the number of blocks for which peer has been sending us
// block parts.
func (ps *PeerState) RecordBlockPart(bp *BlockPartMessage) int {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.stats.lastBlockPartHeight >= bp.Height {
		return ps.stats.blockParts
	}

	ps.stats.lastBlockPartHeight = bp.Height
	ps.stats.blockParts++
	return ps.stats.blockParts
}

// BlockPartsSent returns the number of blocks for which peer has been sending
// us block parts.
func (ps *PeerState) BlockPartsSent() int {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	return ps.stats.blockParts
}

// SetHasVote sets the given vote as known by the peer
func (ps *PeerState) SetHasVote(vote *types.Vote) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	ps.setHasVote(vote.Height, vote.Round, vote.Type, vote.ValidatorIndex)
}

func (ps *PeerState) setHasVote(height int64, round int, type_ byte, index int) {
	logger := ps.logger.With("peerH/R", cmn.Fmt("%d/%d", ps.Height, ps.Round), "H/R", cmn.Fmt("%d/%d", height, round))
	logger.Debug("setHasVote", "type", type_, "index", index)

	// NOTE: some may be nil BitArrays -> no side effects.
	psVotes := ps.getVoteBitArray(height, round, type_)
	if psVotes != nil {
		psVotes.SetIndex(index, true)
	}
}

// ApplyNewRoundStepMessage updates the peer state for the new round.
func (ps *PeerState) ApplyNewRoundStepMessage(msg *NewRoundStepMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	// Ignore duplicates or decreases
	if CompareHRS(msg.Height, msg.Round, msg.Step, ps.Height, ps.Round, ps.Step) <= 0 {
		return
	}

	// Just remember these values.
	psHeight := ps.Height
	psRound := ps.Round
	//psStep := ps.Step
	psCatchupCommitRound := ps.CatchupCommitRound
	psCatchupCommit := ps.CatchupCommit

	startTime := time.Now().Add(-1 * time.Duration(msg.SecondsSinceStartTime) * time.Second)
	ps.Height = msg.Height
	ps.Round = msg.Round
	ps.Step = msg.Step
	ps.StartTime = startTime
	if psHeight != msg.Height || psRound != msg.Round {
		ps.Proposal = false
		ps.ProposalBlockPartsHeader = types.PartSetHeader{}
		ps.ProposalBlockParts = nil
		ps.ProposalPOLRound = -1
		ps.ProposalPOL = nil
		// We'll update the BitArray capacity later.
		ps.Prevotes = nil
		ps.Precommits = nil
	}
	if psHeight == msg.Height && psRound != msg.Round && msg.Round == psCatchupCommitRound {
		// Peer caught up to CatchupCommitRound.
		// Preserve psCatchupCommit!
		// NOTE: We prefer to use prs.Precommits if
		// pr.Round matches pr.CatchupCommitRound.
		ps.Precommits = psCatchupCommit
	}
	if psHeight != msg.Height {
		// Shift Precommits to LastCommit.
		if psHeight+1 == msg.Height && psRound == msg.LastCommitRound {
			ps.LastCommitRound = msg.LastCommitRound
			ps.LastCommit = ps.Precommits
		} else {
			ps.LastCommitRound = msg.LastCommitRound
			ps.LastCommit = nil
		}
		// We'll update the BitArray capacity later.
		ps.CatchupCommitRound = -1
		ps.CatchupCommit = nil
	}
}

// ApplyCommitStepMessage updates the peer state for the new commit.
func (ps *PeerState) ApplyCommitStepMessage(msg *CommitStepMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != msg.Height {
		return
	}

	ps.ProposalBlockPartsHeader = msg.BlockPartsHeader
	ps.ProposalBlockParts = msg.BlockParts
}

// ApplyProposalPOLMessage updates the peer state for the new proposal POL.
func (ps *PeerState) ApplyProposalPOLMessage(msg *ProposalPOLMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != msg.Height {
		return
	}
	if ps.ProposalPOLRound != msg.ProposalPOLRound {
		return
	}

	// TODO: Merge onto existing ps.ProposalPOL?
	// We might have sent some prevotes in the meantime.
	ps.ProposalPOL = msg.ProposalPOL
}

// ApplyHasVoteMessage updates the peer state for the new vote.
func (ps *PeerState) ApplyHasVoteMessage(msg *HasVoteMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != msg.Height {
		return
	}

	ps.setHasVote(msg.Height, msg.Round, msg.Type, msg.Index)
}

// ApplyVoteSetBitsMessage updates the peer state for the bit-array of votes
// it claims to have for the corresponding BlockID.
// `ourVotes` is a BitArray of votes we have for msg.BlockID
// NOTE: if ourVotes is nil (e.g. msg.Height < rs.Height),
// we conservatively overwrite ps's votes w/ msg.Votes.
func (ps *PeerState) ApplyVoteSetBitsMessage(msg *VoteSetBitsMessage, ourVotes *cmn.BitArray) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	votes := ps.getVoteBitArray(msg.Height, msg.Round, msg.Type)
	if votes != nil {
		if ourVotes == nil {
			votes.Update(msg.Votes)
		} else {
			otherVotes := votes.Sub(ourVotes)
			hasVotes := otherVotes.Or(msg.Votes)
			votes.Update(hasVotes)
		}
	}
}

// String returns a string representation of the PeerState
func (ps *PeerState) String() string {
	return ps.StringIndented("")
}

// StringIndented returns a string representation of the PeerState
func (ps *PeerState) StringIndented(indent string) string {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return fmt.Sprintf(`PeerState{
%s  Key   %v
%s  PRS   %v
%s  Stats %v
%s}`,
		indent, ps.Peer.ID(),
		indent, ps.PeerRoundState.StringIndented(indent+"  "),
		indent, ps.stats,
		indent)
}

//-----------------------------------------------------------------------------
// Messages

const (
	msgTypeNewRoundStep = byte(0x01)
	msgTypeCommitStep   = byte(0x02)
	msgTypeProposal     = byte(0x11)
	msgTypeProposalPOL  = byte(0x12)
	msgTypeBlockPart    = byte(0x13) // both block & POL
	msgTypeVote         = byte(0x14)
	msgTypeHasVote      = byte(0x15)
	msgTypeVoteSetMaj23 = byte(0x16)
	msgTypeVoteSetBits  = byte(0x17)

	msgTypeProposalHeartbeat = byte(0x20)
)

// ConsensusMessage is a message that can be sent and received on the ConsensusReactor
type ConsensusMessage interface{}

var _ = wire.RegisterInterface(
	struct{ ConsensusMessage }{},
	wire.ConcreteType{&NewRoundStepMessage{}, msgTypeNewRoundStep},
	wire.ConcreteType{&CommitStepMessage{}, msgTypeCommitStep},
	wire.ConcreteType{&ProposalMessage{}, msgTypeProposal},
	wire.ConcreteType{&ProposalPOLMessage{}, msgTypeProposalPOL},
	wire.ConcreteType{&BlockPartMessage{}, msgTypeBlockPart},
	wire.ConcreteType{&VoteMessage{}, msgTypeVote},
	wire.ConcreteType{&HasVoteMessage{}, msgTypeHasVote},
	wire.ConcreteType{&VoteSetMaj23Message{}, msgTypeVoteSetMaj23},
	wire.ConcreteType{&VoteSetBitsMessage{}, msgTypeVoteSetBits},
	wire.ConcreteType{&ProposalHeartbeatMessage{}, msgTypeProposalHeartbeat},
)

// DecodeMessage decodes the given bytes into a ConsensusMessage.
// TODO: check for unnecessary extra bytes at the end.
func DecodeMessage(bz []byte) (msgType byte, msg ConsensusMessage, err error) {
	msgType = bz[0]
	n := new(int)
	r := bytes.NewReader(bz)
	msgI := wire.ReadBinary(struct{ ConsensusMessage }{}, r, maxConsensusMessageSize, n, &err)
	msg = msgI.(struct{ ConsensusMessage }).ConsensusMessage
	return
}

//-------------------------------------

// NewRoundStepMessage is sent for every step taken in the ConsensusState.
// For every height/round/step transition
type NewRoundStepMessage struct {
	Height                int64
	Round                 int
	Step                  cstypes.RoundStepType
	SecondsSinceStartTime int
	LastCommitRound       int
}

// String returns a string representation.
func (m *NewRoundStepMessage) String() string {
	return fmt.Sprintf("[NewRoundStep H:%v R:%v S:%v LCR:%v]",
		m.Height, m.Round, m.Step, m.LastCommitRound)
}

//-------------------------------------

// CommitStepMessage is sent when a block is committed.
type CommitStepMessage struct {
	Height           int64
	BlockPartsHeader types.PartSetHeader
	BlockParts       *cmn.BitArray
}

// String returns a string representation.
func (m *CommitStepMessage) String() string {
	return fmt.Sprintf("[CommitStep H:%v BP:%v BA:%v]", m.Height, m.BlockPartsHeader, m.BlockParts)
}

//-------------------------------------

// ProposalMessage is sent when a new block is proposed.
type ProposalMessage struct {
	Proposal *types.Proposal
}

// String returns a string representation.
func (m *ProposalMessage) String() string {
	return fmt.Sprintf("[Proposal %v]", m.Proposal)
}

//-------------------------------------

// ProposalPOLMessage is sent when a previous proposal is re-proposed.
type ProposalPOLMessage struct {
	Height           int64
	ProposalPOLRound int
	ProposalPOL      *cmn.BitArray
}

// String returns a string representation.
func (m *ProposalPOLMessage) String() string {
	return fmt.Sprintf("[ProposalPOL H:%v POLR:%v POL:%v]", m.Height, m.ProposalPOLRound, m.ProposalPOL)
}

//-------------------------------------

// BlockPartMessage is sent when gossipping a piece of the proposed block.
type BlockPartMessage struct {
	Height int64
	Round  int
	Part   *types.Part
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

// String returns a string representation.
func (m *VoteMessage) String() string {
	return fmt.Sprintf("[Vote %v]", m.Vote)
}

//-------------------------------------

// HasVoteMessage is sent to indicate that a particular vote has been received.
type HasVoteMessage struct {
	Height int64
	Round  int
	Type   byte
	Index  int
}

// String returns a string representation.
func (m *HasVoteMessage) String() string {
	return fmt.Sprintf("[HasVote VI:%v V:{%v/%02d/%v}]", m.Index, m.Height, m.Round, m.Type)
}

//-------------------------------------

// VoteSetMaj23Message is sent to indicate that a given BlockID has seen +2/3 votes.
type VoteSetMaj23Message struct {
	Height  int64
	Round   int
	Type    byte
	BlockID types.BlockID
}

// String returns a string representation.
func (m *VoteSetMaj23Message) String() string {
	return fmt.Sprintf("[VSM23 %v/%02d/%v %v]", m.Height, m.Round, m.Type, m.BlockID)
}

//-------------------------------------

// VoteSetBitsMessage is sent to communicate the bit-array of votes seen for the BlockID.
type VoteSetBitsMessage struct {
	Height  int64
	Round   int
	Type    byte
	BlockID types.BlockID
	Votes   *cmn.BitArray
}

// String returns a string representation.
func (m *VoteSetBitsMessage) String() string {
	return fmt.Sprintf("[VSB %v/%02d/%v %v %v]", m.Height, m.Round, m.Type, m.BlockID, m.Votes)
}

//-------------------------------------

// ProposalHeartbeatMessage is sent to signal that a node is alive and waiting for transactions for a proposal.
type ProposalHeartbeatMessage struct {
	Heartbeat *types.Heartbeat
}

// String returns a string representation.
func (m *ProposalHeartbeatMessage) String() string {
	return fmt.Sprintf("[HEARTBEAT %v]", m.Heartbeat)
}
