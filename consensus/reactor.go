package consensus

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/pkg/errors"

	amino "github.com/tendermint/go-amino"
	cstypes "github.com/tendermint/tendermint/consensus/types"
	cmn "github.com/tendermint/tendermint/libs/common"
	tmevents "github.com/tendermint/tendermint/libs/events"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/p2p"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

const (
	StateChannel       = byte(0x20)
	DataChannel        = byte(0x21)
	VoteChannel        = byte(0x22)
	VoteSetBitsChannel = byte(0x23)

	maxMsgSize = 1048576 // 1MB; NOTE/TODO: keep in sync with types.PartSet sizes.

	blocksToContributeToBecomeGoodPeer = 10000
	votesToContributeToBecomeGoodPeer  = 10000
)

//-----------------------------------------------------------------------------

// ConsensusReactor defines a reactor for the consensus service.
type ConsensusReactor struct {
	p2p.BaseReactor // BaseService + p2p.Switch

	conS *ConsensusState

	mtx      sync.RWMutex
	fastSync bool
	eventBus *types.EventBus

	metrics *Metrics
}

type ReactorOption func(*ConsensusReactor)

// NewConsensusReactor returns a new ConsensusReactor with the given
// consensusState.
func NewConsensusReactor(consensusState *ConsensusState, fastSync bool, options ...ReactorOption) *ConsensusReactor {
	conR := &ConsensusReactor{
		conS:     consensusState,
		fastSync: fastSync,
		metrics:  NopMetrics(),
	}
	conR.updateFastSyncingMetric()
	conR.BaseReactor = *p2p.NewBaseReactor("ConsensusReactor", conR)

	for _, option := range options {
		option(conR)
	}

	return conR
}

// OnStart implements BaseService by subscribing to events, which later will be
// broadcasted to other peers and starting state if we're not in fast sync.
func (conR *ConsensusReactor) OnStart() error {
	conR.Logger.Info("ConsensusReactor ", "fastSync", conR.FastSync())

	// start routine that computes peer statistics for evaluating peer quality
	go conR.peerStatsRoutine()

	conR.subscribeToBroadcastEvents()

	if !conR.FastSync() {
		err := conR.conS.Start()
		if err != nil {
			return err
		}
	}

	return nil
}

// OnStop implements BaseService by unsubscribing from events and stopping
// state.
func (conR *ConsensusReactor) OnStop() {
	conR.unsubscribeFromBroadcastEvents()
	conR.conS.Stop()
	if !conR.FastSync() {
		conR.conS.Wait()
	}
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
	conR.metrics.FastSyncing.Set(0)

	if blocksSynced > 0 {
		// dont bother with the WAL if we fast synced
		conR.conS.doWALCatchup = false
	}
	err := conR.conS.Start()
	if err != nil {
		conR.Logger.Error("Error starting conS", "err", err)
		return
	}
}

// GetChannels implements Reactor
func (conR *ConsensusReactor) GetChannels() []*p2p.ChannelDescriptor {
	// TODO optimize
	return []*p2p.ChannelDescriptor{
		{
			ID:                  StateChannel,
			Priority:            5,
			SendQueueCapacity:   100,
			RecvMessageCapacity: maxMsgSize,
		},
		{
			ID:                  DataChannel, // maybe split between gossiping current block and catchup stuff
			Priority:            10,          // once we gossip the whole block there's nothing left to send until next height or round
			SendQueueCapacity:   100,
			RecvBufferCapacity:  50 * 4096,
			RecvMessageCapacity: maxMsgSize,
		},
		{
			ID:                  VoteChannel,
			Priority:            5,
			SendQueueCapacity:   100,
			RecvBufferCapacity:  100 * 100,
			RecvMessageCapacity: maxMsgSize,
		},
		{
			ID:                  VoteSetBitsChannel,
			Priority:            1,
			SendQueueCapacity:   2,
			RecvBufferCapacity:  1024,
			RecvMessageCapacity: maxMsgSize,
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
		conR.sendNewRoundStepMessage(peer)
	}
}

// RemovePeer implements Reactor
func (conR *ConsensusReactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	if !conR.IsRunning() {
		return
	}
	// TODO
	// ps, ok := peer.Get(PeerStateKey).(*PeerState)
	// if !ok {
	// 	panic(fmt.Sprintf("Peer %v has no state", peer))
	// }
	// ps.Disconnect()
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

	msg, err := decodeMsg(msgBytes)
	if err != nil {
		conR.Logger.Error("Error decoding message", "src", src, "chId", chID, "msg", msg, "err", err, "bytes", msgBytes)
		conR.Switch.StopPeerForError(src, err)
		return
	}

	if err = msg.ValidateBasic(); err != nil {
		conR.Logger.Error("Peer sent us invalid msg", "peer", src, "msg", msg, "err", err)
		conR.Switch.StopPeerForError(src, err)
		return
	}

	conR.Logger.Debug("Receive", "src", src, "chId", chID, "msg", msg)

	// Get peer states
	ps, ok := src.Get(types.PeerStateKey).(*PeerState)
	if !ok {
		panic(fmt.Sprintf("Peer %v has no state", src))
	}

	switch chID {
	case StateChannel:
		switch msg := msg.(type) {
		case *NewRoundStepMessage:
			ps.ApplyNewRoundStepMessage(msg)
		case *NewValidBlockMessage:
			ps.ApplyNewValidBlockMessage(msg)
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
			err := votes.SetPeerMaj23(msg.Round, msg.Type, ps.peer.ID(), msg.BlockID)
			if err != nil {
				conR.Switch.StopPeerForError(src, err)
				return
			}
			// Respond with a VoteSetBitsMessage showing which votes we have.
			// (and consequently shows which we don't have)
			var ourVotes *cmn.BitArray
			switch msg.Type {
			case types.PrevoteType:
				ourVotes = votes.Prevotes(msg.Round).BitArrayByBlockID(msg.BlockID)
			case types.PrecommitType:
				ourVotes = votes.Precommits(msg.Round).BitArrayByBlockID(msg.BlockID)
			default:
				panic("Bad VoteSetBitsMessage field Type. Forgot to add a check in ValidateBasic?")
			}
			src.TrySend(VoteSetBitsChannel, cdc.MustMarshalBinaryBare(&VoteSetBitsMessage{
				Height:  msg.Height,
				Round:   msg.Round,
				Type:    msg.Type,
				BlockID: msg.BlockID,
				Votes:   ourVotes,
			}))
		default:
			conR.Logger.Error(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
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
			conR.metrics.BlockParts.With("peer_id", string(src.ID())).Add(1)
			conR.conS.peerMsgQueue <- msgInfo{msg, src.ID()}
		default:
			conR.Logger.Error(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
		}

	case VoteChannel:
		if conR.FastSync() {
			conR.Logger.Info("Ignoring message received during fastSync", "msg", msg)
			return
		}
		switch msg := msg.(type) {
		case *VoteMessage:
			cs := conR.conS
			cs.mtx.RLock()
			height, valSize, lastCommitSize := cs.Height, cs.Validators.Size(), cs.LastCommit.Size()
			cs.mtx.RUnlock()
			ps.EnsureVoteBitArrays(height, valSize)
			ps.EnsureVoteBitArrays(height-1, lastCommitSize)
			ps.SetHasVote(msg.Vote)

			cs.peerMsgQueue <- msgInfo{msg, src.ID()}

		default:
			// don't punish (leave room for soft upgrades)
			conR.Logger.Error(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
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
				case types.PrevoteType:
					ourVotes = votes.Prevotes(msg.Round).BitArrayByBlockID(msg.BlockID)
				case types.PrecommitType:
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
			conR.Logger.Error(fmt.Sprintf("Unknown message type %v", reflect.TypeOf(msg)))
		}

	default:
		conR.Logger.Error(fmt.Sprintf("Unknown chId %X", chID))
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

// subscribeToBroadcastEvents subscribes for new round steps and votes
// using internal pubsub defined on state to broadcast
// them to peers upon receiving.
func (conR *ConsensusReactor) subscribeToBroadcastEvents() {
	const subscriber = "consensus-reactor"
	conR.conS.evsw.AddListenerForEvent(subscriber, types.EventNewRoundStep,
		func(data tmevents.EventData) {
			conR.broadcastNewRoundStepMessage(data.(*cstypes.RoundState))
		})

	conR.conS.evsw.AddListenerForEvent(subscriber, types.EventValidBlock,
		func(data tmevents.EventData) {
			conR.broadcastNewValidBlockMessage(data.(*cstypes.RoundState))
		})

	conR.conS.evsw.AddListenerForEvent(subscriber, types.EventVote,
		func(data tmevents.EventData) {
			conR.broadcastHasVoteMessage(data.(*types.Vote))
		})

}

func (conR *ConsensusReactor) unsubscribeFromBroadcastEvents() {
	const subscriber = "consensus-reactor"
	conR.conS.evsw.RemoveListener(subscriber)
}

func (conR *ConsensusReactor) broadcastNewRoundStepMessage(rs *cstypes.RoundState) {
	nrsMsg := makeRoundStepMessage(rs)
	conR.Switch.Broadcast(StateChannel, cdc.MustMarshalBinaryBare(nrsMsg))
}

func (conR *ConsensusReactor) broadcastNewValidBlockMessage(rs *cstypes.RoundState) {
	csMsg := &NewValidBlockMessage{
		Height:           rs.Height,
		Round:            rs.Round,
		BlockPartsHeader: rs.ProposalBlockParts.Header(),
		BlockParts:       rs.ProposalBlockParts.BitArray(),
		IsCommit:         rs.Step == cstypes.RoundStepCommit,
	}
	conR.Switch.Broadcast(StateChannel, cdc.MustMarshalBinaryBare(csMsg))
}

// Broadcasts HasVoteMessage to peers that care.
func (conR *ConsensusReactor) broadcastHasVoteMessage(vote *types.Vote) {
	msg := &HasVoteMessage{
		Height: vote.Height,
		Round:  vote.Round,
		Type:   vote.Type,
		Index:  vote.ValidatorIndex,
	}
	conR.Switch.Broadcast(StateChannel, cdc.MustMarshalBinaryBare(msg))
	/*
		// TODO: Make this broadcast more selective.
		for _, peer := range conR.Switch.Peers().List() {
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

func makeRoundStepMessage(rs *cstypes.RoundState) (nrsMsg *NewRoundStepMessage) {
	nrsMsg = &NewRoundStepMessage{
		Height:                rs.Height,
		Round:                 rs.Round,
		Step:                  rs.Step,
		SecondsSinceStartTime: int(time.Since(rs.StartTime).Seconds()),
		LastCommitRound:       rs.LastCommit.Round(),
	}
	return
}

func (conR *ConsensusReactor) sendNewRoundStepMessage(peer p2p.Peer) {
	rs := conR.conS.GetRoundState()
	nrsMsg := makeRoundStepMessage(rs)
	peer.Send(StateChannel, cdc.MustMarshalBinaryBare(nrsMsg))
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
				if peer.Send(DataChannel, cdc.MustMarshalBinaryBare(msg)) {
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
					cmn.PanicCrisis(fmt.Sprintf("Failed to load block %d when blockStore is at %d",
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
			time.Sleep(conR.conS.config.PeerGossipSleepDuration)
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
				if peer.Send(DataChannel, cdc.MustMarshalBinaryBare(msg)) {
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
				peer.Send(DataChannel, cdc.MustMarshalBinaryBare(msg))
			}
			continue OUTER_LOOP
		}

		// Nothing to do. Sleep.
		time.Sleep(conR.conS.config.PeerGossipSleepDuration)
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
			time.Sleep(conR.conS.config.PeerGossipSleepDuration)
			return
		} else if !blockMeta.BlockID.PartsHeader.Equals(prs.ProposalBlockPartsHeader) {
			logger.Info("Peer ProposalBlockPartsHeader mismatch, sleeping",
				"blockPartsHeader", blockMeta.BlockID.PartsHeader, "peerBlockPartsHeader", prs.ProposalBlockPartsHeader)
			time.Sleep(conR.conS.config.PeerGossipSleepDuration)
			return
		}
		// Load the part
		part := conR.conS.blockStore.LoadBlockPart(prs.Height, index)
		if part == nil {
			logger.Error("Could not load part", "index", index,
				"blockPartsHeader", blockMeta.BlockID.PartsHeader, "peerBlockPartsHeader", prs.ProposalBlockPartsHeader)
			time.Sleep(conR.conS.config.PeerGossipSleepDuration)
			return
		}
		// Send the part
		msg := &BlockPartMessage{
			Height: prs.Height, // Not our height, so it doesn't matter.
			Round:  prs.Round,  // Not our height, so it doesn't matter.
			Part:   part,
		}
		logger.Debug("Sending block part for catchup", "round", prs.Round, "index", index)
		if peer.Send(DataChannel, cdc.MustMarshalBinaryBare(msg)) {
			ps.SetHasProposalBlockPart(prs.Height, prs.Round, index)
		} else {
			logger.Debug("Sending block part for catchup failed")
		}
		return
	}
	//logger.Info("No parts to send in catch-up, sleeping")
	time.Sleep(conR.conS.config.PeerGossipSleepDuration)
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

		time.Sleep(conR.conS.config.PeerGossipSleepDuration)
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
					peer.TrySend(StateChannel, cdc.MustMarshalBinaryBare(&VoteSetMaj23Message{
						Height:  prs.Height,
						Round:   prs.Round,
						Type:    types.PrevoteType,
						BlockID: maj23,
					}))
					time.Sleep(conR.conS.config.PeerQueryMaj23SleepDuration)
				}
			}
		}

		// Maybe send Height/Round/Precommits
		{
			rs := conR.conS.GetRoundState()
			prs := ps.GetRoundState()
			if rs.Height == prs.Height {
				if maj23, ok := rs.Votes.Precommits(prs.Round).TwoThirdsMajority(); ok {
					peer.TrySend(StateChannel, cdc.MustMarshalBinaryBare(&VoteSetMaj23Message{
						Height:  prs.Height,
						Round:   prs.Round,
						Type:    types.PrecommitType,
						BlockID: maj23,
					}))
					time.Sleep(conR.conS.config.PeerQueryMaj23SleepDuration)
				}
			}
		}

		// Maybe send Height/Round/ProposalPOL
		{
			rs := conR.conS.GetRoundState()
			prs := ps.GetRoundState()
			if rs.Height == prs.Height && prs.ProposalPOLRound >= 0 {
				if maj23, ok := rs.Votes.Prevotes(prs.ProposalPOLRound).TwoThirdsMajority(); ok {
					peer.TrySend(StateChannel, cdc.MustMarshalBinaryBare(&VoteSetMaj23Message{
						Height:  prs.Height,
						Round:   prs.ProposalPOLRound,
						Type:    types.PrevoteType,
						BlockID: maj23,
					}))
					time.Sleep(conR.conS.config.PeerQueryMaj23SleepDuration)
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
				peer.TrySend(StateChannel, cdc.MustMarshalBinaryBare(&VoteSetMaj23Message{
					Height:  prs.Height,
					Round:   commit.Round(),
					Type:    types.PrecommitType,
					BlockID: commit.BlockID,
				}))
				time.Sleep(conR.conS.config.PeerQueryMaj23SleepDuration)
			}
		}

		time.Sleep(conR.conS.config.PeerQueryMaj23SleepDuration)

		continue OUTER_LOOP
	}
}

func (conR *ConsensusReactor) peerStatsRoutine() {
	for {
		if !conR.IsRunning() {
			conR.Logger.Info("Stopping peerStatsRoutine")
			return
		}

		select {
		case msg := <-conR.conS.statsMsgQueue:
			// Get peer
			peer := conR.Switch.Peers().Get(msg.PeerID)
			if peer == nil {
				conR.Logger.Debug("Attempt to update stats for non-existent peer",
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
					conR.Switch.MarkPeerAsGood(peer)
				}
			case *BlockPartMessage:
				if numParts := ps.RecordBlockPart(); numParts%blocksToContributeToBecomeGoodPeer == 0 {
					conR.Switch.MarkPeerAsGood(peer)
				}
			}
		case <-conR.conS.Quit():
			return

		case <-conR.Quit():
			return
		}
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
		ps, ok := peer.Get(types.PeerStateKey).(*PeerState)
		if !ok {
			panic(fmt.Sprintf("Peer %v has no state", peer))
		}
		s += indent + "  " + ps.StringIndented(indent+"  ") + "\n"
	}
	s += indent + "}"
	return s
}

func (conR *ConsensusReactor) updateFastSyncingMetric() {
	var fastSyncing float64
	if conR.fastSync {
		fastSyncing = 1
	} else {
		fastSyncing = 0
	}
	conR.metrics.FastSyncing.Set(fastSyncing)
}

// ReactorMetrics sets the metrics
func ReactorMetrics(metrics *Metrics) ReactorOption {
	return func(conR *ConsensusReactor) { conR.metrics = metrics }
}

//-----------------------------------------------------------------------------

var (
	ErrPeerStateHeightRegression = errors.New("Error peer state height regression")
	ErrPeerStateInvalidStartTime = errors.New("Error peer state invalid startTime")
)

// PeerState contains the known state of a peer, including its connection and
// threadsafe access to its PeerRoundState.
// NOTE: THIS GETS DUMPED WITH rpc/core/consensus.go.
// Be mindful of what you Expose.
type PeerState struct {
	peer   p2p.Peer
	logger log.Logger

	mtx   sync.Mutex             // NOTE: Modify below using setters, never directly.
	PRS   cstypes.PeerRoundState `json:"round_state"` // Exposed.
	Stats *peerStateStats        `json:"stats"`       // Exposed.
}

// peerStateStats holds internal statistics for a peer.
type peerStateStats struct {
	Votes      int `json:"votes"`
	BlockParts int `json:"block_parts"`
}

func (pss peerStateStats) String() string {
	return fmt.Sprintf("peerStateStats{votes: %d, blockParts: %d}",
		pss.Votes, pss.BlockParts)
}

// NewPeerState returns a new PeerState for the given Peer
func NewPeerState(peer p2p.Peer) *PeerState {
	return &PeerState{
		peer:   peer,
		logger: log.NewNopLogger(),
		PRS: cstypes.PeerRoundState{
			Round:              -1,
			ProposalPOLRound:   -1,
			LastCommitRound:    -1,
			CatchupCommitRound: -1,
		},
		Stats: &peerStateStats{},
	}
}

// SetLogger allows to set a logger on the peer state. Returns the peer state
// itself.
func (ps *PeerState) SetLogger(logger log.Logger) *PeerState {
	ps.logger = logger
	return ps
}

// GetRoundState returns an shallow copy of the PeerRoundState.
// There's no point in mutating it since it won't change PeerState.
func (ps *PeerState) GetRoundState() *cstypes.PeerRoundState {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	prs := ps.PRS // copy
	return &prs
}

// ToJSON returns a json of PeerState, marshalled using go-amino.
func (ps *PeerState) ToJSON() ([]byte, error) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	return cdc.MarshalJSON(ps)
}

// GetHeight returns an atomic snapshot of the PeerRoundState's height
// used by the mempool to ensure peers are caught up before broadcasting new txs
func (ps *PeerState) GetHeight() int64 {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return ps.PRS.Height
}

// SetHasProposal sets the given proposal as known for the peer.
func (ps *PeerState) SetHasProposal(proposal *types.Proposal) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.PRS.Height != proposal.Height || ps.PRS.Round != proposal.Round {
		return
	}

	if ps.PRS.Proposal {
		return
	}

	ps.PRS.Proposal = true

	// ps.PRS.ProposalBlockParts is set due to NewValidBlockMessage
	if ps.PRS.ProposalBlockParts != nil {
		return
	}

	ps.PRS.ProposalBlockPartsHeader = proposal.BlockID.PartsHeader
	ps.PRS.ProposalBlockParts = cmn.NewBitArray(proposal.BlockID.PartsHeader.Total)
	ps.PRS.ProposalPOLRound = proposal.POLRound
	ps.PRS.ProposalPOL = nil // Nil until ProposalPOLMessage received.
}

// InitProposalBlockParts initializes the peer's proposal block parts header and bit array.
func (ps *PeerState) InitProposalBlockParts(partsHeader types.PartSetHeader) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.PRS.ProposalBlockParts != nil {
		return
	}

	ps.PRS.ProposalBlockPartsHeader = partsHeader
	ps.PRS.ProposalBlockParts = cmn.NewBitArray(partsHeader.Total)
}

// SetHasProposalBlockPart sets the given block part index as known for the peer.
func (ps *PeerState) SetHasProposalBlockPart(height int64, round int, index int) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.PRS.Height != height || ps.PRS.Round != round {
		return
	}

	ps.PRS.ProposalBlockParts.SetIndex(index, true)
}

// PickSendVote picks a vote and sends it to the peer.
// Returns true if vote was sent.
func (ps *PeerState) PickSendVote(votes types.VoteSetReader) bool {
	if vote, ok := ps.PickVoteToSend(votes); ok {
		msg := &VoteMessage{vote}
		ps.logger.Debug("Sending vote message", "ps", ps, "vote", vote)
		if ps.peer.Send(VoteChannel, cdc.MustMarshalBinaryBare(msg)) {
			ps.SetHasVote(vote)
			return true
		}
		return false
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

	height, round, type_, size := votes.Height(), votes.Round(), types.SignedMsgType(votes.Type()), votes.Size()

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
		return votes.GetByIndex(index), true
	}
	return nil, false
}

func (ps *PeerState) getVoteBitArray(height int64, round int, type_ types.SignedMsgType) *cmn.BitArray {
	if !types.IsVoteTypeValid(type_) {
		return nil
	}

	if ps.PRS.Height == height {
		if ps.PRS.Round == round {
			switch type_ {
			case types.PrevoteType:
				return ps.PRS.Prevotes
			case types.PrecommitType:
				return ps.PRS.Precommits
			}
		}
		if ps.PRS.CatchupCommitRound == round {
			switch type_ {
			case types.PrevoteType:
				return nil
			case types.PrecommitType:
				return ps.PRS.CatchupCommit
			}
		}
		if ps.PRS.ProposalPOLRound == round {
			switch type_ {
			case types.PrevoteType:
				return ps.PRS.ProposalPOL
			case types.PrecommitType:
				return nil
			}
		}
		return nil
	}
	if ps.PRS.Height == height+1 {
		if ps.PRS.LastCommitRound == round {
			switch type_ {
			case types.PrevoteType:
				return nil
			case types.PrecommitType:
				return ps.PRS.LastCommit
			}
		}
		return nil
	}
	return nil
}

// 'round': A round for which we have a +2/3 commit.
func (ps *PeerState) ensureCatchupCommitRound(height int64, round int, numValidators int) {
	if ps.PRS.Height != height {
		return
	}
	/*
		NOTE: This is wrong, 'round' could change.
		e.g. if orig round is not the same as block LastCommit round.
		if ps.CatchupCommitRound != -1 && ps.CatchupCommitRound != round {
			cmn.PanicSanity(fmt.Sprintf("Conflicting CatchupCommitRound. Height: %v, Orig: %v, New: %v", height, ps.CatchupCommitRound, round))
		}
	*/
	if ps.PRS.CatchupCommitRound == round {
		return // Nothing to do!
	}
	ps.PRS.CatchupCommitRound = round
	if round == ps.PRS.Round {
		ps.PRS.CatchupCommit = ps.PRS.Precommits
	} else {
		ps.PRS.CatchupCommit = cmn.NewBitArray(numValidators)
	}
}

// EnsureVoteBitArrays ensures the bit-arrays have been allocated for tracking
// what votes this peer has received.
// NOTE: It's important to make sure that numValidators actually matches
// what the node sees as the number of validators for height.
func (ps *PeerState) EnsureVoteBitArrays(height int64, numValidators int) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	ps.ensureVoteBitArrays(height, numValidators)
}

func (ps *PeerState) ensureVoteBitArrays(height int64, numValidators int) {
	if ps.PRS.Height == height {
		if ps.PRS.Prevotes == nil {
			ps.PRS.Prevotes = cmn.NewBitArray(numValidators)
		}
		if ps.PRS.Precommits == nil {
			ps.PRS.Precommits = cmn.NewBitArray(numValidators)
		}
		if ps.PRS.CatchupCommit == nil {
			ps.PRS.CatchupCommit = cmn.NewBitArray(numValidators)
		}
		if ps.PRS.ProposalPOL == nil {
			ps.PRS.ProposalPOL = cmn.NewBitArray(numValidators)
		}
	} else if ps.PRS.Height == height+1 {
		if ps.PRS.LastCommit == nil {
			ps.PRS.LastCommit = cmn.NewBitArray(numValidators)
		}
	}
}

// RecordVote increments internal votes related statistics for this peer.
// It returns the total number of added votes.
func (ps *PeerState) RecordVote() int {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	ps.Stats.Votes++

	return ps.Stats.Votes
}

// VotesSent returns the number of blocks for which peer has been sending us
// votes.
func (ps *PeerState) VotesSent() int {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	return ps.Stats.Votes
}

// RecordBlockPart increments internal block part related statistics for this peer.
// It returns the total number of added block parts.
func (ps *PeerState) RecordBlockPart() int {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	ps.Stats.BlockParts++
	return ps.Stats.BlockParts
}

// BlockPartsSent returns the number of useful block parts the peer has sent us.
func (ps *PeerState) BlockPartsSent() int {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	return ps.Stats.BlockParts
}

// SetHasVote sets the given vote as known by the peer
func (ps *PeerState) SetHasVote(vote *types.Vote) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	ps.setHasVote(vote.Height, vote.Round, vote.Type, vote.ValidatorIndex)
}

func (ps *PeerState) setHasVote(height int64, round int, type_ types.SignedMsgType, index int) {
	logger := ps.logger.With("peerH/R", fmt.Sprintf("%d/%d", ps.PRS.Height, ps.PRS.Round), "H/R", fmt.Sprintf("%d/%d", height, round))
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
	if CompareHRS(msg.Height, msg.Round, msg.Step, ps.PRS.Height, ps.PRS.Round, ps.PRS.Step) <= 0 {
		return
	}

	// Just remember these values.
	psHeight := ps.PRS.Height
	psRound := ps.PRS.Round
	psCatchupCommitRound := ps.PRS.CatchupCommitRound
	psCatchupCommit := ps.PRS.CatchupCommit

	startTime := tmtime.Now().Add(-1 * time.Duration(msg.SecondsSinceStartTime) * time.Second)
	ps.PRS.Height = msg.Height
	ps.PRS.Round = msg.Round
	ps.PRS.Step = msg.Step
	ps.PRS.StartTime = startTime
	if psHeight != msg.Height || psRound != msg.Round {
		ps.PRS.Proposal = false
		ps.PRS.ProposalBlockPartsHeader = types.PartSetHeader{}
		ps.PRS.ProposalBlockParts = nil
		ps.PRS.ProposalPOLRound = -1
		ps.PRS.ProposalPOL = nil
		// We'll update the BitArray capacity later.
		ps.PRS.Prevotes = nil
		ps.PRS.Precommits = nil
	}
	if psHeight == msg.Height && psRound != msg.Round && msg.Round == psCatchupCommitRound {
		// Peer caught up to CatchupCommitRound.
		// Preserve psCatchupCommit!
		// NOTE: We prefer to use prs.Precommits if
		// pr.Round matches pr.CatchupCommitRound.
		ps.PRS.Precommits = psCatchupCommit
	}
	if psHeight != msg.Height {
		// Shift Precommits to LastCommit.
		if psHeight+1 == msg.Height && psRound == msg.LastCommitRound {
			ps.PRS.LastCommitRound = msg.LastCommitRound
			ps.PRS.LastCommit = ps.PRS.Precommits
		} else {
			ps.PRS.LastCommitRound = msg.LastCommitRound
			ps.PRS.LastCommit = nil
		}
		// We'll update the BitArray capacity later.
		ps.PRS.CatchupCommitRound = -1
		ps.PRS.CatchupCommit = nil
	}
}

// ApplyNewValidBlockMessage updates the peer state for the new valid block.
func (ps *PeerState) ApplyNewValidBlockMessage(msg *NewValidBlockMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.PRS.Height != msg.Height {
		return
	}

	if ps.PRS.Round != msg.Round && !msg.IsCommit {
		return
	}

	ps.PRS.ProposalBlockPartsHeader = msg.BlockPartsHeader
	ps.PRS.ProposalBlockParts = msg.BlockParts
}

// ApplyProposalPOLMessage updates the peer state for the new proposal POL.
func (ps *PeerState) ApplyProposalPOLMessage(msg *ProposalPOLMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.PRS.Height != msg.Height {
		return
	}
	if ps.PRS.ProposalPOLRound != msg.ProposalPOLRound {
		return
	}

	// TODO: Merge onto existing ps.PRS.ProposalPOL?
	// We might have sent some prevotes in the meantime.
	ps.PRS.ProposalPOL = msg.ProposalPOL
}

// ApplyHasVoteMessage updates the peer state for the new vote.
func (ps *PeerState) ApplyHasVoteMessage(msg *HasVoteMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.PRS.Height != msg.Height {
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
%s  Key        %v
%s  RoundState %v
%s  Stats      %v
%s}`,
		indent, ps.peer.ID(),
		indent, ps.PRS.StringIndented(indent+"  "),
		indent, ps.Stats,
		indent)
}

//-----------------------------------------------------------------------------
// Messages

// ConsensusMessage is a message that can be sent and received on the ConsensusReactor
type ConsensusMessage interface {
	ValidateBasic() error
}

func RegisterConsensusMessages(cdc *amino.Codec) {
	cdc.RegisterInterface((*ConsensusMessage)(nil), nil)
	cdc.RegisterConcrete(&NewRoundStepMessage{}, "tendermint/NewRoundStepMessage", nil)
	cdc.RegisterConcrete(&NewValidBlockMessage{}, "tendermint/NewValidBlockMessage", nil)
	cdc.RegisterConcrete(&ProposalMessage{}, "tendermint/Proposal", nil)
	cdc.RegisterConcrete(&ProposalPOLMessage{}, "tendermint/ProposalPOL", nil)
	cdc.RegisterConcrete(&BlockPartMessage{}, "tendermint/BlockPart", nil)
	cdc.RegisterConcrete(&VoteMessage{}, "tendermint/Vote", nil)
	cdc.RegisterConcrete(&HasVoteMessage{}, "tendermint/HasVote", nil)
	cdc.RegisterConcrete(&VoteSetMaj23Message{}, "tendermint/VoteSetMaj23", nil)
	cdc.RegisterConcrete(&VoteSetBitsMessage{}, "tendermint/VoteSetBits", nil)
}

func decodeMsg(bz []byte) (msg ConsensusMessage, err error) {
	if len(bz) > maxMsgSize {
		return msg, fmt.Errorf("Msg exceeds max size (%d > %d)", len(bz), maxMsgSize)
	}
	err = cdc.UnmarshalBinaryBare(bz, &msg)
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

// ValidateBasic performs basic validation.
func (m *NewRoundStepMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("Negative Height")
	}
	if m.Round < 0 {
		return errors.New("Negative Round")
	}
	if !m.Step.IsValid() {
		return errors.New("Invalid Step")
	}

	// NOTE: SecondsSinceStartTime may be negative

	if (m.Height == 1 && m.LastCommitRound != -1) ||
		(m.Height > 1 && m.LastCommitRound < -1) { // TODO: #2737 LastCommitRound should always be >= 0 for heights > 1
		return errors.New("Invalid LastCommitRound (for 1st block: -1, for others: >= 0)")
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
//i.e., there is a Proposal for block B and 2/3+ prevotes for the block B in the round r.
// In case the block is also committed, then IsCommit flag is set to true.
type NewValidBlockMessage struct {
	Height           int64
	Round            int
	BlockPartsHeader types.PartSetHeader
	BlockParts       *cmn.BitArray
	IsCommit         bool
}

// ValidateBasic performs basic validation.
func (m *NewValidBlockMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("Negative Height")
	}
	if m.Round < 0 {
		return errors.New("Negative Round")
	}
	if err := m.BlockPartsHeader.ValidateBasic(); err != nil {
		return fmt.Errorf("Wrong BlockPartsHeader: %v", err)
	}
	if m.BlockParts.Size() != m.BlockPartsHeader.Total {
		return fmt.Errorf("BlockParts bit array size %d not equal to BlockPartsHeader.Total %d",
			m.BlockParts.Size(),
			m.BlockPartsHeader.Total)
	}
	return nil
}

// String returns a string representation.
func (m *NewValidBlockMessage) String() string {
	return fmt.Sprintf("[ValidBlockMessage H:%v R:%v BP:%v BA:%v IsCommit:%v]",
		m.Height, m.Round, m.BlockPartsHeader, m.BlockParts, m.IsCommit)
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
	ProposalPOLRound int
	ProposalPOL      *cmn.BitArray
}

// ValidateBasic performs basic validation.
func (m *ProposalPOLMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("Negative Height")
	}
	if m.ProposalPOLRound < 0 {
		return errors.New("Negative ProposalPOLRound")
	}
	if m.ProposalPOL.Size() == 0 {
		return errors.New("Empty ProposalPOL bit array")
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
	Round  int
	Part   *types.Part
}

// ValidateBasic performs basic validation.
func (m *BlockPartMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("Negative Height")
	}
	if m.Round < 0 {
		return errors.New("Negative Round")
	}
	if err := m.Part.ValidateBasic(); err != nil {
		return fmt.Errorf("Wrong Part: %v", err)
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
	Round  int
	Type   types.SignedMsgType
	Index  int
}

// ValidateBasic performs basic validation.
func (m *HasVoteMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("Negative Height")
	}
	if m.Round < 0 {
		return errors.New("Negative Round")
	}
	if !types.IsVoteTypeValid(m.Type) {
		return errors.New("Invalid Type")
	}
	if m.Index < 0 {
		return errors.New("Negative Index")
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
	Round   int
	Type    types.SignedMsgType
	BlockID types.BlockID
}

// ValidateBasic performs basic validation.
func (m *VoteSetMaj23Message) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("Negative Height")
	}
	if m.Round < 0 {
		return errors.New("Negative Round")
	}
	if !types.IsVoteTypeValid(m.Type) {
		return errors.New("Invalid Type")
	}
	if err := m.BlockID.ValidateBasic(); err != nil {
		return fmt.Errorf("Wrong BlockID: %v", err)
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
	Round   int
	Type    types.SignedMsgType
	BlockID types.BlockID
	Votes   *cmn.BitArray
}

// ValidateBasic performs basic validation.
func (m *VoteSetBitsMessage) ValidateBasic() error {
	if m.Height < 0 {
		return errors.New("Negative Height")
	}
	if m.Round < 0 {
		return errors.New("Negative Round")
	}
	if !types.IsVoteTypeValid(m.Type) {
		return errors.New("Invalid Type")
	}
	if err := m.BlockID.ValidateBasic(); err != nil {
		return fmt.Errorf("Wrong BlockID: %v", err)
	}
	// NOTE: Votes.Size() can be zero if the node does not have any
	return nil
}

// String returns a string representation.
func (m *VoteSetBitsMessage) String() string {
	return fmt.Sprintf("[VSB %v/%02d/%v %v %v]", m.Height, m.Round, m.Type, m.BlockID, m.Votes)
}

//-------------------------------------
