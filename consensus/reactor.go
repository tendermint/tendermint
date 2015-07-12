package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tendermint/tendermint/binary"
	bc "github.com/tendermint/tendermint/blockchain"
	. "github.com/tendermint/tendermint/common"
	. "github.com/tendermint/tendermint/consensus/types"
	"github.com/tendermint/tendermint/events"
	"github.com/tendermint/tendermint/p2p"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

const (
	StateChannel = byte(0x20)
	DataChannel  = byte(0x21)
	VoteChannel  = byte(0x22)

	PeerStateKey = "ConsensusReactor.peerState"

	peerGossipSleepDuration = 100 * time.Millisecond // Time to sleep if there's nothing to send.
)

//-----------------------------------------------------------------------------

type ConsensusReactor struct {
	sw      *p2p.Switch
	running uint32
	quit    chan struct{}

	blockStore *bc.BlockStore
	conS       *ConsensusState
	fastSync   bool

	evsw events.Fireable
}

func NewConsensusReactor(consensusState *ConsensusState, blockStore *bc.BlockStore, fastSync bool) *ConsensusReactor {
	conR := &ConsensusReactor{
		quit:       make(chan struct{}),
		blockStore: blockStore,
		conS:       consensusState,
		fastSync:   fastSync,
	}
	return conR
}

// Implements Reactor
func (conR *ConsensusReactor) Start(sw *p2p.Switch) {
	if atomic.CompareAndSwapUint32(&conR.running, 0, 1) {
		log.Info("Starting ConsensusReactor", "fastSync", conR.fastSync)
		conR.sw = sw
		if !conR.fastSync {
			conR.conS.Start()
		}
		go conR.broadcastNewRoundStepRoutine()
	}
}

// Implements Reactor
func (conR *ConsensusReactor) Stop() {
	if atomic.CompareAndSwapUint32(&conR.running, 1, 0) {
		log.Info("Stopping ConsensusReactor")
		conR.conS.Stop()
		close(conR.quit)
	}
}

func (conR *ConsensusReactor) IsRunning() bool {
	return atomic.LoadUint32(&conR.running) == 1
}

// Implements Reactor
func (conR *ConsensusReactor) GetChannels() []*p2p.ChannelDescriptor {
	// TODO optimize
	return []*p2p.ChannelDescriptor{
		&p2p.ChannelDescriptor{
			Id:                StateChannel,
			Priority:          5,
			SendQueueCapacity: 100,
		},
		&p2p.ChannelDescriptor{
			Id:                DataChannel,
			Priority:          5,
			SendQueueCapacity: 2,
		},
		&p2p.ChannelDescriptor{
			Id:                VoteChannel,
			Priority:          5,
			SendQueueCapacity: 40,
		},
	}
}

// Implements Reactor
func (conR *ConsensusReactor) AddPeer(peer *p2p.Peer) {
	if !conR.IsRunning() {
		return
	}

	// Create peerState for peer
	peerState := NewPeerState(peer)
	peer.Data.Set(PeerStateKey, peerState)

	// Begin gossip routines for this peer.
	go conR.gossipDataRoutine(peer, peerState)
	go conR.gossipVotesRoutine(peer, peerState)

	// Send our state to peer.
	// If we're fast_syncing, broadcast a RoundStepMessage later upon SwitchToConsensus().
	if !conR.fastSync {
		conR.sendNewRoundStepMessage(peer)
	}
}

// Implements Reactor
func (conR *ConsensusReactor) RemovePeer(peer *p2p.Peer, reason interface{}) {
	if !conR.IsRunning() {
		return
	}
	// TODO
	//peer.Data.Get(PeerStateKey).(*PeerState).Disconnect()
}

// Implements Reactor
// NOTE: We process these messages even when we're fast_syncing.
func (conR *ConsensusReactor) Receive(chId byte, peer *p2p.Peer, msgBytes []byte) {
	log.Debug("Receive", "channel", chId, "peer", peer, "bytes", msgBytes)
	if !conR.IsRunning() {
		return
	}

	// Get round state
	rs := conR.conS.GetRoundState()
	ps := peer.Data.Get(PeerStateKey).(*PeerState)
	_, msg_, err := DecodeMessage(msgBytes)
	if err != nil {
		log.Warn("Error decoding message", "channel", chId, "peer", peer, "msg", msg_, "error", err, "bytes", msgBytes)
		return
	}
	log.Debug("Receive", "channel", chId, "peer", peer, "msg", msg_, "rsHeight", rs.Height) //, "bytes", msgBytes)

	switch chId {
	case StateChannel:
		switch msg := msg_.(type) {
		case *NewRoundStepMessage:
			ps.ApplyNewRoundStepMessage(msg, rs)
		case *CommitStepMessage:
			ps.ApplyCommitStepMessage(msg)
		case *HasVoteMessage:
			ps.ApplyHasVoteMessage(msg)
		default:
			log.Warn(Fmt("Unknown message type %v", reflect.TypeOf(msg)))
		}

	case DataChannel:
		if conR.fastSync {
			log.Warn("Ignoring message received during fastSync", "msg", msg_)
			return
		}
		switch msg := msg_.(type) {
		case *ProposalMessage:
			ps.SetHasProposal(msg.Proposal)
			err = conR.conS.SetProposal(msg.Proposal)
		case *ProposalPOLMessage:
			ps.ApplyProposalPOLMessage(msg)
		case *BlockPartMessage:
			ps.SetHasProposalBlockPart(msg.Height, msg.Round, msg.Part.Proof.Index)
			_, err = conR.conS.AddProposalBlockPart(msg.Height, msg.Part)
		default:
			log.Warn(Fmt("Unknown message type %v", reflect.TypeOf(msg)))
		}

	case VoteChannel:
		if conR.fastSync {
			log.Warn("Ignoring message received during fastSync", "msg", msg_)
			return
		}
		switch msg := msg_.(type) {
		case *VoteMessage:
			vote := msg.Vote
			var validators *sm.ValidatorSet
			if rs.Height == vote.Height {
				validators = rs.Validators
			} else if rs.Height == vote.Height+1 {
				if !(rs.Step == RoundStepNewHeight && vote.Type == types.VoteTypePrecommit) {
					return // Wrong height, not a LastCommit straggler commit.
				}
				validators = rs.LastValidators
			} else {
				return // Wrong height. Not necessarily a bad peer.
			}

			// We have vote/validators.  Height may not be rs.Height

			address, _ := validators.GetByIndex(msg.ValidatorIndex)
			added, index, err := conR.conS.AddVote(address, vote, peer.Key)
			if err != nil {
				// If conflicting sig, broadcast evidence tx for slashing. Else punish peer.
				if errDupe, ok := err.(*types.ErrVoteConflictingSignature); ok {
					log.Warn("Found conflicting vote. Publish evidence")
					evidenceTx := &types.DupeoutTx{
						Address: address,
						VoteA:   *errDupe.VoteA,
						VoteB:   *errDupe.VoteB,
					}
					conR.conS.mempoolReactor.BroadcastTx(evidenceTx) // shouldn't need to check returned err
				} else {
					// Probably an invalid signature. Bad peer.
					log.Warn("Error attempting to add vote", "error", err)
					// TODO: punish peer
				}
			}
			ps.EnsureVoteBitArrays(rs.Height, rs.Validators.Size())
			ps.EnsureVoteBitArrays(rs.Height-1, rs.LastCommit.Size())
			ps.SetHasVote(vote, index)
			if added {
				// If rs.Height == vote.Height && rs.Round < vote.Round,
				// the peer is sending us CatchupCommit precommits.
				// We could make note of this and help filter in broadcastHasVoteMessage().
				conR.broadcastHasVoteMessage(vote, index)
			}

		default:
			log.Warn(Fmt("Unknown message type %v", reflect.TypeOf(msg)))
		}
	default:
		log.Warn(Fmt("Unknown channel %X", chId))
	}

	if err != nil {
		log.Warn("Error in Receive()", "error", err)
	}
}

// Broadcasts HasVoteMessage to peers that care.
func (conR *ConsensusReactor) broadcastHasVoteMessage(vote *types.Vote, index int) {
	msg := &HasVoteMessage{
		Height: vote.Height,
		Round:  vote.Round,
		Type:   vote.Type,
		Index:  index,
	}
	conR.sw.Broadcast(StateChannel, msg)
	/*
		// TODO: Make this broadcast more selective.
		for _, peer := range conR.sw.Peers().List() {
			ps := peer.Data.Get(PeerStateKey).(*PeerState)
			prs := ps.GetRoundState()
			if prs.Height == vote.Height {
				// TODO: Also filter on round?
				peer.TrySend(StateChannel, msg)
			} else {
				// Height doesn't match
				// TODO: check a field, maybe CatchupCommitRound?
				// TODO: But that requires changing the struct field comment.
			}
		}
	*/
}

// Sets our private validator account for signing votes.
func (conR *ConsensusReactor) SetPrivValidator(priv *sm.PrivValidator) {
	conR.conS.SetPrivValidator(priv)
}

// Switch from the fast_sync to the consensus:
// reset the state, turn off fast_sync, start the consensus-state-machine
func (conR *ConsensusReactor) SwitchToConsensus(state *sm.State) {
	log.Info("SwitchToConsensus")
	// NOTE: The line below causes broadcastNewRoundStepRoutine() to
	// broadcast a NewRoundStepMessage.
	conR.conS.updateToState(state, false)
	conR.fastSync = false
	conR.conS.Start()
}

// implements events.Eventable
func (conR *ConsensusReactor) SetFireable(evsw events.Fireable) {
	conR.evsw = evsw
	conR.conS.SetFireable(evsw)
}

//--------------------------------------

func makeRoundStepMessages(rs *RoundState) (nrsMsg *NewRoundStepMessage, csMsg *CommitStepMessage) {
	nrsMsg = &NewRoundStepMessage{
		Height: rs.Height,
		Round:  rs.Round,
		Step:   rs.Step,
		SecondsSinceStartTime: int(time.Now().Sub(rs.StartTime).Seconds()),
		LastCommitRound:       rs.LastCommit.Round(),
	}
	if rs.Step == RoundStepCommit {
		csMsg = &CommitStepMessage{
			Height:           rs.Height,
			BlockPartsHeader: rs.ProposalBlockParts.Header(),
			BlockParts:       rs.ProposalBlockParts.BitArray(),
		}
	}
	return
}

// Listens for changes to the ConsensusState.Step by pulling
// on conR.conS.NewStepCh().
func (conR *ConsensusReactor) broadcastNewRoundStepRoutine() {
	for {
		// Get RoundState with new Step or quit.
		var rs *RoundState
		select {
		case rs = <-conR.conS.NewStepCh():
		case <-conR.quit:
			return
		}

		nrsMsg, csMsg := makeRoundStepMessages(rs)
		if nrsMsg != nil {
			conR.sw.Broadcast(StateChannel, nrsMsg)
		}
		if csMsg != nil {
			conR.sw.Broadcast(StateChannel, csMsg)
		}
	}
}

func (conR *ConsensusReactor) sendNewRoundStepMessage(peer *p2p.Peer) {
	rs := conR.conS.GetRoundState()
	nrsMsg, csMsg := makeRoundStepMessages(rs)
	if nrsMsg != nil {
		peer.Send(StateChannel, nrsMsg)
	}
	if csMsg != nil {
		peer.Send(StateChannel, csMsg)
	}
}

func (conR *ConsensusReactor) gossipDataRoutine(peer *p2p.Peer, ps *PeerState) {
	log := log.New("peer", peer.Key)

OUTER_LOOP:
	for {
		// Manage disconnects from self or peer.
		if !peer.IsRunning() || !conR.IsRunning() {
			log.Info(Fmt("Stopping gossipDataRoutine for %v.", peer))
			return
		}
		rs := conR.conS.GetRoundState()
		prs := ps.GetRoundState()

		// Send proposal Block parts?
		if rs.ProposalBlockParts.HasHeader(prs.ProposalBlockPartsHeader) {
			//log.Debug("ProposalBlockParts matched", "blockParts", prs.ProposalBlockParts)
			if index, ok := rs.ProposalBlockParts.BitArray().Sub(prs.ProposalBlockParts.Copy()).PickRandom(); ok {
				part := rs.ProposalBlockParts.GetPart(index)
				msg := &BlockPartMessage{
					Height: rs.Height, // This tells peer that this part applies to us.
					Round:  rs.Round,  // This tells peer that this part applies to us.
					Part:   part,
				}
				peer.Send(DataChannel, msg)
				ps.SetHasProposalBlockPart(prs.Height, prs.Round, index)
				continue OUTER_LOOP
			}
		}

		// If the peer is on a previous height, help catch up.
		if (0 < prs.Height) && (prs.Height < rs.Height) {
			//log.Debug("Data catchup", "height", rs.Height, "peerHeight", prs.Height, "peerProposalBlockParts", prs.ProposalBlockParts)
			if index, ok := prs.ProposalBlockParts.Not().PickRandom(); ok {
				// Ensure that the peer's PartSetHeader is correct
				blockMeta := conR.blockStore.LoadBlockMeta(prs.Height)
				if !blockMeta.PartsHeader.Equals(prs.ProposalBlockPartsHeader) {
					log.Debug("Peer ProposalBlockPartsHeader mismatch, sleeping",
						"peerHeight", prs.Height, "blockPartsHeader", blockMeta.PartsHeader, "peerBlockPartsHeader", prs.ProposalBlockPartsHeader)
					time.Sleep(peerGossipSleepDuration)
					continue OUTER_LOOP
				}
				// Load the part
				part := conR.blockStore.LoadBlockPart(prs.Height, index)
				if part == nil {
					log.Warn("Could not load part", "index", index,
						"peerHeight", prs.Height, "blockPartsHeader", blockMeta.PartsHeader, "peerBlockPartsHeader", prs.ProposalBlockPartsHeader)
					time.Sleep(peerGossipSleepDuration)
					continue OUTER_LOOP
				}
				// Send the part
				msg := &BlockPartMessage{
					Height: prs.Height, // Not our height, so it doesn't matter.
					Round:  prs.Round,  // Not our height, so it doesn't matter.
					Part:   part,
				}
				peer.Send(DataChannel, msg)
				ps.SetHasProposalBlockPart(prs.Height, prs.Round, index)
				continue OUTER_LOOP
			} else {
				//log.Debug("No parts to send in catch-up, sleeping")
				time.Sleep(peerGossipSleepDuration)
				continue OUTER_LOOP
			}
		}

		// If height and round don't match, sleep.
		if (rs.Height != prs.Height) || (rs.Round != prs.Round) {
			//log.Debug("Peer Height|Round mismatch, sleeping", "peerHeight", prs.Height, "peerRound", prs.Round, "peer", peer)
			time.Sleep(peerGossipSleepDuration)
			continue OUTER_LOOP
		}

		// By here, height and round match.
		// Proposal block parts were already matched and sent if any were wanted.
		// (These can match on hash so the round doesn't matter)
		// Now consider sending other things, like the Proposal itself.

		// Send Proposal && ProposalPOL BitArray?
		if rs.Proposal != nil && !prs.Proposal {
			// Proposal
			{
				msg := &ProposalMessage{Proposal: rs.Proposal}
				peer.Send(DataChannel, msg)
				ps.SetHasProposal(rs.Proposal)
			}
			// ProposalPOL.
			// Peer must receive ProposalMessage first.
			// rs.Proposal was validated, so rs.Proposal.POLRound <= rs.Round,
			// so we definitely have rs.Votes.Prevotes(rs.Proposal.POLRound).
			if 0 <= rs.Proposal.POLRound {
				msg := &ProposalPOLMessage{
					Height:           rs.Height,
					ProposalPOLRound: rs.Proposal.POLRound,
					ProposalPOL:      rs.Votes.Prevotes(rs.Proposal.POLRound).BitArray(),
				}
				peer.Send(DataChannel, msg)
			}
			continue OUTER_LOOP
		}

		// Nothing to do. Sleep.
		time.Sleep(peerGossipSleepDuration)
		continue OUTER_LOOP
	}
}

func (conR *ConsensusReactor) gossipVotesRoutine(peer *p2p.Peer, ps *PeerState) {
	log := log.New("peer", peer.Key)

	// Simple hack to throttle logs upon sleep.
	var sleeping = 0

OUTER_LOOP:
	for {
		// Manage disconnects from self or peer.
		if !peer.IsRunning() || !conR.IsRunning() {
			log.Info(Fmt("Stopping gossipVotesRoutine for %v.", peer))
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

		log.Debug("gossipVotesRoutine", "rsHeight", rs.Height, "rsRound", rs.Round,
			"prsHeight", prs.Height, "prsRound", prs.Round, "prsStep", prs.Step)

		// If height matches, then send LastCommit, Prevotes, Precommits.
		if rs.Height == prs.Height {
			// If there are lastCommits to send...
			if prs.Step == RoundStepNewHeight {
				if ps.PickSendVote(rs.LastCommit) {
					log.Debug("Picked rs.LastCommit to send")
					continue OUTER_LOOP
				}
			}
			// If there are prevotes to send...
			if rs.Round == prs.Round && prs.Step <= RoundStepPrevote {
				if ps.PickSendVote(rs.Votes.Prevotes(rs.Round)) {
					log.Debug("Picked rs.Prevotes(rs.Round) to send")
					continue OUTER_LOOP
				}
			}
			// If there are precommits to send...
			if rs.Round == prs.Round && prs.Step <= RoundStepPrecommit {
				if ps.PickSendVote(rs.Votes.Precommits(rs.Round)) {
					log.Debug("Picked rs.Precommits(rs.Round) to send")
					continue OUTER_LOOP
				}
			}
			// If there are prevotes to send for the last round...
			if rs.Round == prs.Round+1 && prs.Step <= RoundStepPrevote {
				if ps.PickSendVote(rs.Votes.Prevotes(prs.Round)) {
					log.Debug("Picked rs.Prevotes(prs.Round) to send")
					continue OUTER_LOOP
				}
			}
			// If there are precommits to send for the last round...
			if rs.Round == prs.Round+1 && prs.Step <= RoundStepPrecommit {
				if ps.PickSendVote(rs.Votes.Precommits(prs.Round)) {
					log.Debug("Picked rs.Precommits(prs.Round) to send")
					continue OUTER_LOOP
				}
			}
			// If there are POLPrevotes to send...
			if 0 <= prs.ProposalPOLRound {
				if polPrevotes := rs.Votes.Prevotes(prs.ProposalPOLRound); polPrevotes != nil {
					if ps.PickSendVote(polPrevotes) {
						log.Debug("Picked rs.Prevotes(prs.ProposalPOLRound) to send")
						continue OUTER_LOOP
					}
				}
			}
		}

		// Special catchup logic.
		// If peer is lagging by height 1, send LastCommit.
		if prs.Height != 0 && rs.Height == prs.Height+1 {
			if ps.PickSendVote(rs.LastCommit) {
				log.Debug("Picked rs.LastCommit to send")
				continue OUTER_LOOP
			}
		}

		// Catchup logic
		// If peer is lagging by more than 1, send Validation.
		if prs.Height != 0 && rs.Height >= prs.Height+2 {
			// Load the block validation for prs.Height,
			// which contains precommit signatures for prs.Height.
			validation := conR.blockStore.LoadBlockValidation(prs.Height)
			log.Debug("Loaded BlockValidation for catch-up", "height", prs.Height, "validation", validation)
			if ps.PickSendVote(validation) {
				log.Debug("Picked Catchup validation to send")
				continue OUTER_LOOP
			}
		}

		if sleeping == 0 {
			// We sent nothing. Sleep...
			sleeping = 1
			log.Debug("No votes to send, sleeping", "peer", peer,
				"localPV", rs.Votes.Prevotes(rs.Round).BitArray(), "peerPV", prs.Prevotes,
				"localPC", rs.Votes.Precommits(rs.Round).BitArray(), "peerPC", prs.Precommits)
		} else if sleeping == 2 {
			// Continued sleep...
			sleeping = 1
		}

		time.Sleep(peerGossipSleepDuration)
		continue OUTER_LOOP
	}
}

//-----------------------------------------------------------------------------

// Read only when returned by PeerState.GetRoundState().
type PeerRoundState struct {
	Height                   int                 // Height peer is at
	Round                    int                 // Round peer is at
	Step                     RoundStepType       // Step peer is at
	StartTime                time.Time           // Estimated start of round 0 at this height
	Proposal                 bool                // True if peer has proposal for this round
	ProposalBlockPartsHeader types.PartSetHeader //
	ProposalBlockParts       *BitArray           //
	ProposalPOLRound         int                 // -1 if none
	ProposalPOL              *BitArray           // nil until ProposalPOLMessage received.
	Prevotes                 *BitArray           // All votes peer has for this round
	Precommits               *BitArray           // All precommits peer has for this round
	LastCommitRound          int                 // Round of commit for last height.
	LastCommit               *BitArray           // All commit precommits of commit for last height.
	CatchupCommitRound       int                 // Round that we believe commit round is.
	CatchupCommit            *BitArray           // All commit precommits peer has for this height
}

//-----------------------------------------------------------------------------

var (
	ErrPeerStateHeightRegression = errors.New("Error peer state height regression")
	ErrPeerStateInvalidStartTime = errors.New("Error peer state invalid startTime")
)

type PeerState struct {
	Peer *p2p.Peer

	mtx sync.Mutex
	PeerRoundState
}

func NewPeerState(peer *p2p.Peer) *PeerState {
	return &PeerState{Peer: peer}
}

// Returns an atomic snapshot of the PeerRoundState.
// There's no point in mutating it since it won't change PeerState.
func (ps *PeerState) GetRoundState() *PeerRoundState {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	prs := ps.PeerRoundState // copy
	return &prs
}

func (ps *PeerState) SetHasProposal(proposal *Proposal) {
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
	ps.ProposalBlockParts = NewBitArray(proposal.BlockPartsHeader.Total)
	ps.ProposalPOLRound = proposal.POLRound
	ps.ProposalPOL = nil // Nil until ProposalPOLMessage received.
}

func (ps *PeerState) SetHasProposalBlockPart(height int, round int, index int) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != height || ps.Round != round {
		return
	}

	ps.ProposalBlockParts.SetIndex(index, true)
}

// Convenience function to send vote to peer.
// Returns true if vote was sent.
func (ps *PeerState) PickSendVote(votes types.VoteSetReader) (ok bool) {
	if index, vote, ok := ps.PickVoteToSend(votes); ok {
		msg := &VoteMessage{index, vote}
		ps.Peer.Send(VoteChannel, msg)
		return true
	}
	return false
}

// votes: Must be the correct Size() for the Height().
func (ps *PeerState) PickVoteToSend(votes types.VoteSetReader) (index int, vote *types.Vote, ok bool) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if votes.Size() == 0 {
		return 0, nil, false
	}

	height, round, type_, size := votes.Height(), votes.Round(), votes.Type(), votes.Size()

	// Lazily set data using 'votes'.
	if votes.IsCommit() {
		ps.ensureCatchupCommitRound(height, round, size)
	}
	ps.ensureVoteBitArrays(height, size)

	psVotes := ps.getVoteBitArray(height, round, type_)
	if psVotes == nil {
		return 0, nil, false // Not something worth sending
	}
	if index, ok := votes.BitArray().Sub(psVotes).PickRandom(); ok {
		ps.setHasVote(height, round, type_, index)
		return index, votes.GetByIndex(index), true
	}
	return 0, nil, false
}

func (ps *PeerState) getVoteBitArray(height, round int, type_ byte) *BitArray {
	if ps.Height == height {
		if ps.Round == round {
			switch type_ {
			case types.VoteTypePrevote:
				return ps.Prevotes
			case types.VoteTypePrecommit:
				return ps.Precommits
			default:
				panic(Fmt("Unexpected vote type %X", type_))
			}
		}
		if ps.CatchupCommitRound == round {
			switch type_ {
			case types.VoteTypePrevote:
				return nil
			case types.VoteTypePrecommit:
				return ps.CatchupCommit
			default:
				panic(Fmt("Unexpected vote type %X", type_))
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
			default:
				panic(Fmt("Unexpected vote type %X", type_))
			}
		}
		return nil
	}
	return nil
}

// NOTE: 'round' is what we know to be the commit round for height.
func (ps *PeerState) ensureCatchupCommitRound(height, round int, numValidators int) {
	if ps.Height != height {
		return
	}
	if ps.CatchupCommitRound != -1 && ps.CatchupCommitRound != round {
		panic(Fmt("Conflicting CatchupCommitRound. Height: %v, Orig: %v, New: %v", height, ps.CatchupCommitRound, round))
	}
	if ps.CatchupCommitRound == round {
		return // Nothing to do!
	}
	ps.CatchupCommitRound = round
	if round == ps.Round {
		ps.CatchupCommit = ps.Precommits
	} else {
		ps.CatchupCommit = NewBitArray(numValidators)
	}
}

// NOTE: It's important to make sure that numValidators actually matches
// what the node sees as the number of validators for height.
func (ps *PeerState) EnsureVoteBitArrays(height int, numValidators int) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	ps.ensureVoteBitArrays(height, numValidators)
}

func (ps *PeerState) ensureVoteBitArrays(height int, numValidators int) {
	if ps.Height == height {
		if ps.Prevotes == nil {
			ps.Prevotes = NewBitArray(numValidators)
		}
		if ps.Precommits == nil {
			ps.Precommits = NewBitArray(numValidators)
		}
		if ps.CatchupCommit == nil {
			ps.CatchupCommit = NewBitArray(numValidators)
		}
		if ps.ProposalPOL == nil {
			ps.ProposalPOL = NewBitArray(numValidators)
		}
	} else if ps.Height == height+1 {
		if ps.LastCommit == nil {
			ps.LastCommit = NewBitArray(numValidators)
		}
	}
}

func (ps *PeerState) SetHasVote(vote *types.Vote, index int) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	ps.setHasVote(vote.Height, vote.Round, vote.Type, index)
}

func (ps *PeerState) setHasVote(height int, round int, type_ byte, index int) {
	log := log.New("peer", ps.Peer.Key, "peerRound", ps.Round, "height", height, "round", round)
	if type_ != types.VoteTypePrevote && type_ != types.VoteTypePrecommit {
		panic("Invalid vote type") // SANITY
	}

	if ps.Height == height {
		if ps.Round == round {
			switch type_ {
			case types.VoteTypePrevote:
				ps.Prevotes.SetIndex(index, true)
				log.Debug("SetHasVote(round-match)", "prevotes", ps.Prevotes, "index", index)
			case types.VoteTypePrecommit:
				ps.Precommits.SetIndex(index, true)
				log.Debug("SetHasVote(round-match)", "precommits", ps.Precommits, "index", index)
			}
		} else if ps.CatchupCommitRound == round {
			switch type_ {
			case types.VoteTypePrevote:
			case types.VoteTypePrecommit:
				ps.CatchupCommit.SetIndex(index, true)
				log.Debug("SetHasVote(CatchupCommit)", "precommits", ps.Precommits, "index", index)
			}
		} else if ps.ProposalPOLRound == round {
			switch type_ {
			case types.VoteTypePrevote:
				ps.ProposalPOL.SetIndex(index, true)
				log.Debug("SetHasVote(ProposalPOL)", "prevotes", ps.Prevotes, "index", index)
			case types.VoteTypePrecommit:
			}
		}
	} else if ps.Height == height+1 {
		if ps.LastCommitRound == round {
			switch type_ {
			case types.VoteTypePrevote:
			case types.VoteTypePrecommit:
				ps.LastCommit.SetIndex(index, true)
				log.Debug("setHasVote(LastCommit)", "lastCommit", ps.LastCommit, "index", index)
			}
		}
	} else {
		// Does not apply.
	}
}

func (ps *PeerState) ApplyNewRoundStepMessage(msg *NewRoundStepMessage, rs *RoundState) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	// Ignore duplicate messages.
	if ps.Height == msg.Height && ps.Round == msg.Round && ps.Step == msg.Step {
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

func (ps *PeerState) ApplyCommitStepMessage(msg *CommitStepMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != msg.Height {
		return
	}

	ps.ProposalBlockPartsHeader = msg.BlockPartsHeader
	ps.ProposalBlockParts = msg.BlockParts
}

func (ps *PeerState) ApplyHasVoteMessage(msg *HasVoteMessage) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	if ps.Height != msg.Height {
		return
	}

	ps.setHasVote(msg.Height, msg.Round, msg.Type, msg.Index)
}

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
)

type ConsensusMessage interface{}

var _ = binary.RegisterInterface(
	struct{ ConsensusMessage }{},
	binary.ConcreteType{&NewRoundStepMessage{}, msgTypeNewRoundStep},
	binary.ConcreteType{&CommitStepMessage{}, msgTypeCommitStep},
	binary.ConcreteType{&ProposalMessage{}, msgTypeProposal},
	binary.ConcreteType{&ProposalPOLMessage{}, msgTypeProposalPOL},
	binary.ConcreteType{&BlockPartMessage{}, msgTypeBlockPart},
	binary.ConcreteType{&VoteMessage{}, msgTypeVote},
	binary.ConcreteType{&HasVoteMessage{}, msgTypeHasVote},
)

// TODO: check for unnecessary extra bytes at the end.
func DecodeMessage(bz []byte) (msgType byte, msg ConsensusMessage, err error) {
	msgType = bz[0]
	n := new(int64)
	r := bytes.NewReader(bz)
	msg = binary.ReadBinary(struct{ ConsensusMessage }{}, r, n, &err).(struct{ ConsensusMessage }).ConsensusMessage
	return
}

//-------------------------------------

// For every height/round/step transition
type NewRoundStepMessage struct {
	Height                int
	Round                 int
	Step                  RoundStepType
	SecondsSinceStartTime int
	LastCommitRound       int
}

func (m *NewRoundStepMessage) String() string {
	return fmt.Sprintf("[NewRoundStep H:%v R:%v S:%v LCR:%v]",
		m.Height, m.Round, m.Step, m.LastCommitRound)
}

//-------------------------------------

type CommitStepMessage struct {
	Height           int
	BlockPartsHeader types.PartSetHeader
	BlockParts       *BitArray
}

func (m *CommitStepMessage) String() string {
	return fmt.Sprintf("[CommitStep H:%v BP:%v BA:%v]", m.Height, m.BlockPartsHeader, m.BlockParts)
}

//-------------------------------------

type ProposalMessage struct {
	Proposal *Proposal
}

func (m *ProposalMessage) String() string {
	return fmt.Sprintf("[Proposal %v]", m.Proposal)
}

//-------------------------------------

type ProposalPOLMessage struct {
	Height           int
	ProposalPOLRound int
	ProposalPOL      *BitArray
}

func (m *ProposalPOLMessage) String() string {
	return fmt.Sprintf("[ProposalPOL H:%v POLR:%v POL:%v]", m.Height, m.ProposalPOLRound, m.ProposalPOL)
}

//-------------------------------------

type BlockPartMessage struct {
	Height int
	Round  int
	Part   *types.Part
}

func (m *BlockPartMessage) String() string {
	return fmt.Sprintf("[BlockPart H:%v R:%v P:%v]", m.Height, m.Round, m.Part)
}

//-------------------------------------

type VoteMessage struct {
	ValidatorIndex int
	Vote           *types.Vote
}

func (m *VoteMessage) String() string {
	return fmt.Sprintf("[Vote VI:%v V:%v VI:%v]", m.ValidatorIndex, m.Vote, m.ValidatorIndex)
}

//-------------------------------------

type HasVoteMessage struct {
	Height int
	Round  int
	Type   byte
	Index  int
}

func (m *HasVoteMessage) String() string {
	return fmt.Sprintf("[HasVote VI:%v V:{%v/%02d/%v} VI:%v]", m.Index, m.Height, m.Round, m.Type, m.Index)
}
