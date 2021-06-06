package consensus

import (
	"bytes"
	"fmt"

	tmcon "github.com/tendermint/tendermint/consensus"
	cstypes "github.com/tendermint/tendermint/consensus/types"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

// MisbehaviorList encompasses a list of all possible behaviors
var MisbehaviorList = map[string]Misbehavior{
	"double-prevote": DoublePrevoteMisbehavior(),
}

type Misbehavior struct {
	Name string

	EnterPropose func(cs *State, height int64, round int32)

	EnterPrevote func(cs *State, height int64, round int32)

	EnterPrecommit func(cs *State, height int64, round int32)

	ReceivePrevote func(cs *State, prevote *types.Vote)

	ReceivePrecommit func(cs *State, precommit *types.Vote)

	ReceiveProposal func(cs *State, proposal *types.Proposal) error
}

// BEHAVIORS

func DefaultMisbehavior() Misbehavior {
	return Misbehavior{
		Name:             "default",
		EnterPropose:     defaultEnterPropose,
		EnterPrevote:     defaultEnterPrevote,
		EnterPrecommit:   defaultEnterPrecommit,
		ReceivePrevote:   defaultReceivePrevote,
		ReceivePrecommit: defaultReceivePrecommit,
		ReceiveProposal:  defaultReceiveProposal,
	}
}

// DoublePrevoteMisbehavior will make a node prevote both nil and a block in the same
// height and round.
func DoublePrevoteMisbehavior() Misbehavior {
	b := DefaultMisbehavior()
	b.Name = "double-prevote"
	b.EnterPrevote = func(cs *State, height int64, round int32) {
		fmt.Printf("entering prevote\n")
		// If a block is locked, prevote that.
		if cs.LockedBlock != nil {
			cs.Logger.Debug("enterPrevote: already locked on a block, prevoting locked block")
			cs.signAddVote(tmproto.PrevoteType, cs.LockedBlock.Hash(), cs.LockedBlockParts.Header())
			return
		}

		// If ProposalBlock is nil, prevote nil.
		if cs.ProposalBlock == nil {
			cs.Logger.Debug("enterPrevote: ProposalBlock is nil")
			cs.signAddVote(tmproto.PrevoteType, nil, types.PartSetHeader{})
			return
		}

		// Validate proposal block
		err := cs.blockExec.ValidateBlock(cs.state, cs.ProposalBlock)
		if err != nil {
			// ProposalBlock is invalid, prevote nil.
			cs.Logger.Error("enterPrevote: ProposalBlock is invalid", "err", err)
			cs.signAddVote(tmproto.PrevoteType, nil, types.PartSetHeader{})
			return
		}

		// Validate proposal block
		err = cs.blockExec.ValidateBlockChainLock(cs.state, cs.ProposalBlock)
		if err != nil {
			// ProposalBlock is invalid, prevote nil.
			cs.Logger.Error("enterPrevote: ProposalBlock chain lock is invalid", "err", err)
			cs.signAddVote(tmproto.PrevoteType, nil, types.PartSetHeader{})
			return
		}

		if cs.sw == nil {
			cs.Logger.Error("nil switch")
			return
		}

		prevote, err := cs.signVote(tmproto.PrevoteType, cs.ProposalBlock.Hash(), cs.ProposalBlockParts.Header())
		if err != nil {
			cs.Logger.Error("enterPrevote: Unable to sign block", "err", err)
		}

		nilPrevote, err := cs.signVote(tmproto.PrevoteType, nil, types.PartSetHeader{})
		if err != nil {
			cs.Logger.Error("enterPrevote: Unable to sign block", "err", err)
		}

		// fmt.Printf("%+v prevote\n", prevote.String())
		// fmt.Printf("%+v nilPrevote\n", nilPrevote.String())
		// a,_ := MsgToProto(&VoteMessage{prevote})
		// fmt.Printf("%+v encoded prevote\n", a)

		// fmt.Printf("%X proTxHash (%d)\n", prevote.ValidatorProTxHash.Bytes(), len(prevote.ValidatorProTxHash.Bytes()))
		// fmt.Printf("%X cs proTxHash (%d)\n", cs.privValidatorProTxHash.Bytes(), len(cs.privValidatorProTxHash))

		// add our own vote
		cs.sendInternalMessage(msgInfo{&tmcon.VoteMessage{Vote: prevote}, ""})

		cs.Logger.Info("Sending conflicting votes")
		peers := cs.sw.Peers().List()
		// there has to be at least two other peers connected else this behavior works normally
		for idx, peer := range peers {
			if idx%2 == 0 { // sign the proposal block
				peer.Send(VoteChannel, tmcon.MustEncode(&tmcon.VoteMessage{Vote: prevote}))
			} else { // sign a nil block
				peer.Send(VoteChannel, tmcon.MustEncode(&tmcon.VoteMessage{Vote: nilPrevote}))
			}
		}
	}
	return b
}

// DEFAULTS

func defaultEnterPropose(cs *State, height int64, round int32) {
	logger := cs.Logger.With("height", height, "round", round)
	// If we don't get the proposal and all block parts quick enough, enterPrevote
	cs.scheduleTimeout(cs.config.Propose(round), height, round, cstypes.RoundStepPropose)

	// Nothing more to do if we're not a validator
	if cs.privValidator == nil {
		logger.Debug("This node is not a validator")
		return
	}
	logger.Debug("This node is a validator")

	proTxHash, err := cs.privValidator.GetProTxHash()
	if err != nil {
		// If this node is a validator & proposer in the currentx round, it will
		// miss the opportunity to create a block.
		logger.Error("Error on retrieval of pubkey", "err", err)
		return
	}

	// if not a validator, we're done
	if !cs.Validators.HasProTxHash(proTxHash) {
		logger.Debug("This node is not a validator", "pro_tx_hash", proTxHash, "vals", cs.Validators)
		return
	}

	if cs.isProposer(proTxHash) {
		logger.Debug("enterPropose: our turn to propose",
			"proposer",
			proTxHash,
			"privValidator",
			cs.privValidator)
		cs.decideProposal(height, round)
	} else {
		logger.Debug("enterPropose: not our turn to propose",
			"proposer",
			cs.Validators.GetProposer().ProTxHash,
			"privValidator",
			cs.privValidator)
	}
}

func defaultEnterPrevote(cs *State, height int64, round int32) {
	logger := cs.Logger.With("height", height, "round", round)

	// If a block is locked, prevote that.
	if cs.LockedBlock != nil {
		logger.Debug("enterPrevote: already locked on a block, prevoting locked block")
		cs.signAddVote(tmproto.PrevoteType, cs.LockedBlock.Hash(), cs.LockedBlockParts.Header())
		return
	}

	// If ProposalBlock is nil, prevote nil.
	if cs.ProposalBlock == nil {
		logger.Debug("enterPrevote: ProposalBlock is nil")
		cs.signAddVote(tmproto.PrevoteType, nil, types.PartSetHeader{})
		return
	}

	// Validate proposal block
	err := cs.blockExec.ValidateBlock(cs.state, cs.ProposalBlock)
	if err != nil {
		// ProposalBlock is invalid, prevote nil.
		logger.Error("enterPrevote: ProposalBlock is invalid", "err", err)
		cs.signAddVote(tmproto.PrevoteType, nil, types.PartSetHeader{})
		return
	}

	// Validate proposal block time if we are not proposer
	if !bytes.Equal(cs.privValidatorProTxHash, cs.ProposalBlock.ProposerProTxHash) {
		err = cs.blockExec.ValidateBlockTime(cs.state, cs.ProposalBlock)
		if err != nil {
			// ProposalBlock is invalid, prevote nil.
			logger.Error("enterPrevote: ProposalBlock time is invalid", "err", err)
			cs.signAddVote(tmproto.PrevoteType, nil, types.PartSetHeader{})
			return
		}
	}

	// Validate proposal block chain lock
	err = cs.blockExec.ValidateBlockChainLock(cs.state, cs.ProposalBlock)
	if err != nil {
		// ProposalBlock is invalid, prevote nil.
		logger.Error("enterPrevote: ProposalBlock chain lock is invalid", "err", err)
		cs.signAddVote(tmproto.PrevoteType, nil, types.PartSetHeader{})
		return
	}

	// Prevote cs.ProposalBlock
	// NOTE: the proposal signature is validated when it is received,
	// and the proposal block parts are validated as they are received (against the merkle hash in the proposal)
	logger.Debug("enterPrevote: ProposalBlock is valid")
	cs.signAddVote(tmproto.PrevoteType, cs.ProposalBlock.Hash(), cs.ProposalBlockParts.Header())
}

func defaultEnterPrecommit(cs *State, height int64, round int32) {
	logger := cs.Logger.With("height", height, "round", round)

	// check for a polka
	blockID, ok := cs.Votes.Prevotes(round).TwoThirdsMajority()

	// If we don't have a polka, we must precommit nil.
	if !ok {
		if cs.LockedBlock != nil {
			logger.Debug("enterPrecommit: no +2/3 prevotes during enterPrecommit while we're locked; precommitting nil")
		} else {
			logger.Debug("enterPrecommit: no +2/3 prevotes during enterPrecommit; precommitting nil.")
		}
		cs.signAddVote(tmproto.PrecommitType, nil, types.PartSetHeader{})
		return
	}

	// At this point +2/3 prevoted for a particular block or nil.
	_ = cs.eventBus.PublishEventPolka(cs.RoundStateEvent())

	// the latest POLRound should be this round.
	polRound, _ := cs.Votes.POLInfo()
	if polRound < round {
		panic(fmt.Sprintf("This POLRound should be %v but got %v", round, polRound))
	}

	// +2/3 prevoted nil. Unlock and precommit nil.
	if len(blockID.Hash) == 0 {
		if cs.LockedBlock == nil {
			logger.Debug("enterPrecommit: +2/3 prevoted for nil")
		} else {
			logger.Debug("enterPrecommit: +2/3 prevoted for nil; unlocking")
			cs.LockedRound = -1
			cs.LockedBlock = nil
			cs.LockedBlockParts = nil
			_ = cs.eventBus.PublishEventUnlock(cs.RoundStateEvent())
		}
		cs.signAddVote(tmproto.PrecommitType, nil, types.PartSetHeader{})
		return
	}

	// At this point, +2/3 prevoted for a particular block.

	// If we're already locked on that block, precommit it, and update the LockedRound
	if cs.LockedBlock.HashesTo(blockID.Hash) {
		logger.Debug("enterPrecommit: +2/3 prevoted locked block; relocking")
		cs.LockedRound = round
		_ = cs.eventBus.PublishEventRelock(cs.RoundStateEvent())
		cs.signAddVote(tmproto.PrecommitType, blockID.Hash, blockID.PartSetHeader)
		return
	}

	// If +2/3 prevoted for proposal block, stage and precommit it
	if cs.ProposalBlock.HashesTo(blockID.Hash) {
		logger.Debug("enterPrecommit: +2/3 prevoted proposal block; locking", "hash", blockID.Hash)
		// Validate the block.
		if err := cs.blockExec.ValidateBlock(cs.state, cs.ProposalBlock); err != nil {
			panic(fmt.Sprintf("enterPrecommit: +2/3 prevoted for an invalid block: %v", err))
		}
		cs.LockedRound = round
		cs.LockedBlock = cs.ProposalBlock
		cs.LockedBlockParts = cs.ProposalBlockParts
		_ = cs.eventBus.PublishEventLock(cs.RoundStateEvent())
		cs.signAddVote(tmproto.PrecommitType, blockID.Hash, blockID.PartSetHeader)
		return
	}

	// There was a polka in this round for a block we don't have.
	// Fetch that block, unlock, and precommit nil.
	// The +2/3 prevotes for this round is the POL for our unlock.
	logger.Debug("enterPrecommit: +2/3 prevotes for a block we don't have; voting nil", "blockID", blockID)
	cs.LockedRound = -1
	cs.LockedBlock = nil
	cs.LockedBlockParts = nil
	if !cs.ProposalBlockParts.HasHeader(blockID.PartSetHeader) {
		cs.ProposalBlock = nil
		cs.ProposalBlockParts = types.NewPartSetFromHeader(blockID.PartSetHeader)
	}
	_ = cs.eventBus.PublishEventUnlock(cs.RoundStateEvent())
	cs.signAddVote(tmproto.PrecommitType, nil, types.PartSetHeader{})
}

func defaultReceivePrevote(cs *State, vote *types.Vote) {
	height := cs.Height
	prevotes := cs.Votes.Prevotes(vote.Round)

	// If +2/3 prevotes for a block or nil for *any* round:
	if blockID, ok := prevotes.TwoThirdsMajority(); ok {

		// There was a polka!
		// If we're locked but this is a recent polka, unlock.
		// If it matches our ProposalBlock, update the ValidBlock

		// Unlock if `cs.LockedRound < vote.Round <= cs.Round`
		// NOTE: If vote.Round > cs.Round, we'll deal with it when we get to vote.Round
		if (cs.LockedBlock != nil) &&
			(cs.LockedRound < vote.Round) &&
			(vote.Round <= cs.Round) &&
			!cs.LockedBlock.HashesTo(blockID.Hash) {

			cs.Logger.Info("Unlocking because of POL.", "lockedRound", cs.LockedRound, "POLRound", vote.Round)
			cs.LockedRound = -1
			cs.LockedBlock = nil
			cs.LockedBlockParts = nil
			_ = cs.eventBus.PublishEventUnlock(cs.RoundStateEvent())
		}

		// Update Valid* if we can.
		// NOTE: our proposal block may be nil or not what received a polka..
		if len(blockID.Hash) != 0 && (cs.ValidRound < vote.Round) && (vote.Round == cs.Round) {

			if cs.ProposalBlock.HashesTo(blockID.Hash) {
				cs.Logger.Info(
					"Updating ValidBlock because of POL.", "validRound", cs.ValidRound, "POLRound", vote.Round)
				cs.ValidRound = vote.Round
				cs.ValidBlock = cs.ProposalBlock
				cs.ValidBlockParts = cs.ProposalBlockParts
			} else {
				cs.Logger.Info(
					"valid block we do not know about; set ProposalBlock=nil",
					"proposal", cs.ProposalBlock.Hash(),
					"blockID", blockID.Hash,
				)

				// We're getting the wrong block.
				cs.ProposalBlock = nil
			}
			if !cs.ProposalBlockParts.HasHeader(blockID.PartSetHeader) {
				cs.ProposalBlockParts = types.NewPartSetFromHeader(blockID.PartSetHeader)
			}
			cs.evsw.FireEvent(types.EventValidBlock, &cs.RoundState)
			_ = cs.eventBus.PublishEventValidBlock(cs.RoundStateEvent())
		}
	}

	// If +2/3 prevotes for *anything* for future round:
	switch {
	case cs.Round < vote.Round && prevotes.HasTwoThirdsAny():
		// Round-skip if there is any 2/3+ of votes ahead of us
		cs.enterNewRound(height, vote.Round)
	case cs.Round == vote.Round && cstypes.RoundStepPrevote <= cs.Step: // current round
		blockID, ok := prevotes.TwoThirdsMajority()
		if ok && (cs.isProposalComplete() || len(blockID.Hash) == 0) {
			cs.enterPrecommit(height, vote.Round)
		} else if prevotes.HasTwoThirdsAny() {
			cs.enterPrevoteWait(height, vote.Round)
		}
	case cs.Proposal != nil && 0 <= cs.Proposal.POLRound && cs.Proposal.POLRound == vote.Round:
		// If the proposal is now complete, enter prevote of cs.Round.
		if cs.isProposalComplete() {
			cs.enterPrevote(height, cs.Round)
		}
	}

}

func defaultReceivePrecommit(cs *State, vote *types.Vote) {
	height := cs.Height
	precommits := cs.Votes.Precommits(vote.Round)
	cs.Logger.Info("Added to precommit", "vote", vote, "precommits", precommits.StringShort())

	blockID, ok := precommits.TwoThirdsMajority()
	if ok {
		// Executed as TwoThirdsMajority could be from a higher round
		cs.enterNewRound(height, vote.Round)
		cs.enterPrecommit(height, vote.Round)
		if len(blockID.Hash) != 0 {
			cs.enterCommit(height, vote.Round)
			if cs.config.SkipTimeoutCommit && precommits.HasAll() {
				cs.enterNewRound(cs.Height, 0)
			}
		} else {
			cs.enterPrecommitWait(height, vote.Round)
		}
	} else if cs.Round <= vote.Round && precommits.HasTwoThirdsAny() {
		cs.enterNewRound(height, vote.Round)
		cs.enterPrecommitWait(height, vote.Round)
	}
}

func defaultReceiveProposal(cs *State, proposal *types.Proposal) error {
	// Already have one
	// TODO: possibly catch double proposals
	if cs.Proposal != nil {
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

	proposer := cs.Validators.GetProposer()
	proposalBlockSignBytes := types.ProposalBlockSignBytes(cs.state.ChainID, p)
	// Verify signature
	if !proposer.PubKey.VerifySignature(proposalBlockSignBytes, proposal.Signature) {
		return fmt.Errorf("maverick error proposer %X verifying proposal signature %X at height %d with key %X blockSignBytes %X",
			proposer.ProTxHash, proposal.Signature, proposal.Height, proposer.PubKey.Bytes(), proposalBlockSignBytes)
	}

	proposal.Signature = p.Signature
	cs.Proposal = proposal
	// We don't update cs.ProposalBlockParts if it is already set.
	// This happens if we're already in cstypes.RoundStepApplyCommit or if there is a valid block in the current round.
	// TODO: We can check if Proposal is for a different block as this is a sign of misbehavior!
	if cs.ProposalBlockParts == nil {
		cs.ProposalBlockParts = types.NewPartSetFromHeader(proposal.BlockID.PartSetHeader)
	}

	cs.Logger.Info("received proposal", "proposal", proposal)
	return nil
}
