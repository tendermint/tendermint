package consensus

import (
	"fmt"

	cstypes "github.com/tendermint/tendermint/consensus/types"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

type Behavior interface {
	String() string

	EnterPropose(cs *State, height int64, round int32)

	EnterPrevote(cs *State, height int64, round int32)

	EnterPrecommit(cs *State, height int64, round int32)

	ReceivePrevote(cs *State, prevote *types.Vote)

	ReceivePrecommit(cs *State, precommit *types.Vote)

	ReceiveProposal(cs *State, proposal *types.Proposal) error
}

func DefaultEnterPropose(cs *State, height int64, round int32) {
	logger := cs.Logger.With("height", height, "round", round)
	// If we don't get the proposal and all block parts quick enough, enterPrevote
	cs.scheduleTimeout(cs.config.Propose(round), height, round, cstypes.RoundStepPropose)

	// Nothing more to do if we're not a validator
	if cs.privValidator == nil {
		logger.Debug("This node is not a validator")
		return
	}
	logger.Debug("This node is a validator")

	pubKey, err := cs.privValidator.GetPubKey()
	if err != nil {
		// If this node is a validator & proposer in the currentx round, it will
		// miss the opportunity to create a block.
		logger.Error("Error on retrival of pubkey", "err", err)
		return
	}
	address := pubKey.Address()

	// if not a validator, we're done
	if !cs.Validators.HasAddress(address) {
		logger.Debug("This node is not a validator", "addr", address, "vals", cs.Validators)
		return
	}

	if cs.isProposer(address) {
		logger.Info("enterPropose: Our turn to propose",
			"proposer",
			address,
			"privValidator",
			cs.privValidator)
		cs.decideProposal(height, round)
	} else {
		logger.Info("enterPropose: Not our turn to propose",
			"proposer",
			cs.Validators.GetProposer().Address,
			"privValidator",
			cs.privValidator)
	}
}

func DefaultEnterPrevote(cs *State, height int64, round int32) {
	logger := cs.Logger.With("height", height, "round", round)

	// If a block is locked, prevote that.
	if cs.LockedBlock != nil {
		logger.Info("enterPrevote: Already locked on a block, prevoting locked block")
		cs.signAddVote(tmproto.PrevoteType, cs.LockedBlock.Hash(), cs.LockedBlockParts.Header())
		return
	}

	// If ProposalBlock is nil, prevote nil.
	if cs.ProposalBlock == nil {
		logger.Info("enterPrevote: ProposalBlock is nil")
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

	// Prevote cs.ProposalBlock
	// NOTE: the proposal signature is validated when it is received,
	// and the proposal block parts are validated as they are received (against the merkle hash in the proposal)
	logger.Info("enterPrevote: ProposalBlock is valid")
	cs.signAddVote(tmproto.PrevoteType, cs.ProposalBlock.Hash(), cs.ProposalBlockParts.Header())
}

func DefaultEnterPrecommit(cs *State, height int64, round int32) {
	logger := cs.Logger.With("height", height, "round", round)
	// check for a polka
	blockID, ok := cs.Votes.Prevotes(round).TwoThirdsMajority()

	// If we don't have a polka, we must precommit nil.
	if !ok {
		if cs.LockedBlock != nil {
			logger.Info("enterPrecommit: No +2/3 prevotes during enterPrecommit while we're locked. Precommitting nil")
		} else {
			logger.Info("enterPrecommit: No +2/3 prevotes during enterPrecommit. Precommitting nil.")
		}
		cs.signAddVote(tmproto.PrecommitType, nil, types.PartSetHeader{})
		return
	}

	// At this point +2/3 prevoted for a particular block or nil.
	cs.eventBus.PublishEventPolka(cs.RoundStateEvent())

	// the latest POLRound should be this round.
	polRound, _ := cs.Votes.POLInfo()
	if polRound < round {
		panic(fmt.Sprintf("This POLRound should be %v but got %v", round, polRound))
	}

	// +2/3 prevoted nil. Unlock and precommit nil.
	if len(blockID.Hash) == 0 {
		if cs.LockedBlock == nil {
			logger.Info("enterPrecommit: +2/3 prevoted for nil.")
		} else {
			logger.Info("enterPrecommit: +2/3 prevoted for nil. Unlocking")
			cs.LockedRound = -1
			cs.LockedBlock = nil
			cs.LockedBlockParts = nil
			cs.eventBus.PublishEventUnlock(cs.RoundStateEvent())
		}
		cs.signAddVote(tmproto.PrecommitType, nil, types.PartSetHeader{})
		return
	}

	// At this point, +2/3 prevoted for a particular block.

	// If we're already locked on that block, precommit it, and update the LockedRound
	if cs.LockedBlock.HashesTo(blockID.Hash) {
		logger.Info("enterPrecommit: +2/3 prevoted locked block. Relocking")
		cs.LockedRound = round
		cs.eventBus.PublishEventRelock(cs.RoundStateEvent())
		cs.signAddVote(tmproto.PrecommitType, blockID.Hash, blockID.PartSetHeader)
		return
	}

	// If +2/3 prevoted for proposal block, stage and precommit it
	if cs.ProposalBlock.HashesTo(blockID.Hash) {
		logger.Info("enterPrecommit: +2/3 prevoted proposal block. Locking", "hash", blockID.Hash)
		// Validate the block.
		if err := cs.blockExec.ValidateBlock(cs.state, cs.ProposalBlock); err != nil {
			panic(fmt.Sprintf("enterPrecommit: +2/3 prevoted for an invalid block: %v", err))
		}
		cs.LockedRound = round
		cs.LockedBlock = cs.ProposalBlock
		cs.LockedBlockParts = cs.ProposalBlockParts
		cs.eventBus.PublishEventLock(cs.RoundStateEvent())
		cs.signAddVote(tmproto.PrecommitType, blockID.Hash, blockID.PartSetHeader)
		return
	}

	// There was a polka in this round for a block we don't have.
	// Fetch that block, unlock, and precommit nil.
	// The +2/3 prevotes for this round is the POL for our unlock.
	logger.Info("enterPrecommit: +2/3 prevotes for a block we don't have. Voting nil", "blockID", blockID)
	cs.LockedRound = -1
	cs.LockedBlock = nil
	cs.LockedBlockParts = nil
	if !cs.ProposalBlockParts.HasHeader(blockID.PartSetHeader) {
		cs.ProposalBlock = nil
		cs.ProposalBlockParts = types.NewPartSetFromHeader(blockID.PartSetHeader)
	}
	cs.eventBus.PublishEventUnlock(cs.RoundStateEvent())
	cs.signAddVote(tmproto.PrecommitType, nil, types.PartSetHeader{})
}

func DefaultReceivePrevote(cs *State, vote *types.Vote) {
	height := cs.Height
	prevotes := cs.Votes.Prevotes(vote.Round)
	cs.Logger.Info("Added to prevote", "vote", vote, "prevotes", prevotes.StringShort())

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
			cs.eventBus.PublishEventUnlock(cs.RoundStateEvent())
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
					"Valid block we don't know about. Set ProposalBlock=nil",
					"proposal", cs.ProposalBlock.Hash(), "blockID", blockID.Hash)
				// We're getting the wrong block.
				cs.ProposalBlock = nil
			}
			if !cs.ProposalBlockParts.HasHeader(blockID.PartSetHeader) {
				cs.ProposalBlockParts = types.NewPartSetFromHeader(blockID.PartSetHeader)
			}
			cs.evsw.FireEvent(types.EventValidBlock, &cs.RoundState)
			cs.eventBus.PublishEventValidBlock(cs.RoundStateEvent())
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

func DefaultReceivePrecommit(cs *State, vote *types.Vote) {
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

func DefaultReceiveProposal(cs *State, proposal *types.Proposal) error {
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
	// Verify signature
	if !cs.Validators.GetProposer().PubKey.VerifySignature(types.ProposalSignBytes(cs.state.ChainID, p), proposal.Signature) {
		return ErrInvalidProposalSignature
	}

	proposal.Signature = p.Signature
	cs.Proposal = proposal
	// We don't update cs.ProposalBlockParts if it is already set.
	// This happens if we're already in cstypes.RoundStepCommit or if there is a valid block in the current round.
	// TODO: We can check if Proposal is for a different block as this is a sign of misbehavior!
	if cs.ProposalBlockParts == nil {
		cs.ProposalBlockParts = types.NewPartSetFromHeader(proposal.BlockID.PartSetHeader)
	}
	cs.Logger.Info("Received proposal", "proposal", proposal)
	return nil
}

type DefaultBehavior struct{}

var _ Behavior = &DefaultBehavior{}

func (d *DefaultBehavior) String() string { return "DefaultBehavior" }

func (d *DefaultBehavior) EnterPropose(cs *State, height int64, round int32) {
	DefaultEnterPropose(cs, height, round)
}

func (d *DefaultBehavior) EnterPrevote(cs *State, height int64, round int32) {
	DefaultEnterPrevote(cs, height, round)
}

func (d *DefaultBehavior) EnterPrecommit(cs *State, height int64, round int32) {
	DefaultEnterPrecommit(cs, height, round)
}

func (d *DefaultBehavior) ReceivePrevote(cs *State, prevote *types.Vote) {
	DefaultReceivePrevote(cs, prevote)
}

func (d *DefaultBehavior) ReceivePrecommit(cs *State, precommit *types.Vote) {
	DefaultReceivePrecommit(cs, precommit)
}

func (d *DefaultBehavior) ReceiveProposal(cs *State, proposal *types.Proposal) error {
	return DefaultReceiveProposal(cs, proposal)
}

type EquivocationBehavior struct{}

var _ Behavior = &EquivocationBehavior{}

func (e *EquivocationBehavior) String() string { return "EquivocationBehavior" }

func (e *EquivocationBehavior) EnterPropose(cs *State, height int64, round int32) {
	DefaultEnterPropose(cs, height, round)
}

func (e *EquivocationBehavior) EnterPrevote(cs *State, height int64, round int32) {
	logger := cs.Logger.With("height", height, "round", round)

	// If a block is locked, prevote that.
	if cs.LockedBlock != nil {
		logger.Info("enterPrevote: Already locked on a block, prevoting locked block")
		cs.signAddVote(tmproto.PrevoteType, cs.LockedBlock.Hash(), cs.LockedBlockParts.Header())
		return
	}

	// If ProposalBlock is nil, prevote nil.
	if cs.ProposalBlock == nil {
		logger.Info("enterPrevote: ProposalBlock is nil")
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

	peers := cs.sw.Peers().List()
	for _, peer := range peers {
		scenario := tmrand.Intn(4)
		var prevote *types.Vote
		switch scenario {
		case 0: // sign the proposal block
			prevote, err = cs.signVote(tmproto.PrevoteType, cs.ProposalBlock.Hash(), cs.ProposalBlockParts.Header())
			if err != nil {
				logger.Error("enterPrevote: Unable to sign block", "err", err)
			}
		case 1: // sign a nil block
			prevote, err = cs.signVote(tmproto.PrevoteType, nil, types.PartSetHeader{})
			if err != nil {
				logger.Error("enterPrevote: Unable to sign block", "err", err)
			}
		case 2: // sign a different block
			block, blockParts := cs.createProposalBlock()
			prevote, err = cs.signVote(tmproto.PrevoteType, block.Hash(), blockParts.Header())
			if err != nil {
				logger.Error("enterPrevote: Unable to sign block", "err", err)
			}
		case 3: // send multiple votes for different blocks
			numVotes := tmrand.Intn(3) + 2
			for i := 0; i < numVotes; i++ {
				block, blockParts := cs.createProposalBlock()
				prevote, err = cs.signVote(tmproto.PrevoteType, block.Hash(), blockParts.Header())
				if err != nil {
					logger.Error("enterPrevote: Unable to sign block", "err", err)
				}
				if i < numVotes-1 {
					peer.Send(VoteChannel, MustEncode(&VoteMessage{prevote}))
				}
			}
		default:
			prevote, err = cs.signVote(tmproto.PrevoteType, cs.ProposalBlock.Hash(), cs.ProposalBlockParts.Header())
			if err != nil {
				logger.Error("enterPrevote: Unable to sign block", "err", err)
			}
		}
		peer.Send(VoteChannel, MustEncode(&VoteMessage{prevote}))

	}

	// Prevote cs.ProposalBlock
	// NOTE: the proposal signature is validated when it is received,
	// and the proposal block parts are validated as they are received (against the merkle hash in the proposal)
	// logger.Info("enterPrevote: ProposalBlock is valid")
	// cs.signAddVote(tmproto.PrevoteType, cs.ProposalBlock.Hash(), cs.ProposalBlockParts.Header())
	// cs.signAddVote(tmproto.PrevoteType, nil, types.PartSetHeader{})
}

func (e *EquivocationBehavior) EnterPrecommit(cs *State, height int64, round int32) {
	logger := cs.Logger.With("height", height, "round", round)
	// check for a polka
	blockID, ok := cs.Votes.Prevotes(round).TwoThirdsMajority()

	// If we don't have a polka, we must precommit nil.
	if !ok {
		if cs.LockedBlock != nil {
			logger.Info("enterPrecommit: No +2/3 prevotes during enterPrecommit while we're locked. Precommitting nil")
		} else {
			logger.Info("enterPrecommit: No +2/3 prevotes during enterPrecommit. Precommitting nil.")
		}
		cs.signAddVote(tmproto.PrecommitType, nil, types.PartSetHeader{})
		return
	}

	// At this point +2/3 prevoted for a particular block or nil.
	cs.eventBus.PublishEventPolka(cs.RoundStateEvent())

	// the latest POLRound should be this round.
	polRound, _ := cs.Votes.POLInfo()
	if polRound < round {
		panic(fmt.Sprintf("This POLRound should be %v but got %v", round, polRound))
	}

	// +2/3 prevoted nil. Unlock and precommit nil.
	if len(blockID.Hash) == 0 {
		if cs.LockedBlock == nil {
			logger.Info("enterPrecommit: +2/3 prevoted for nil.")
		} else {
			logger.Info("enterPrecommit: +2/3 prevoted for nil. Unlocking")
			cs.LockedRound = -1
			cs.LockedBlock = nil
			cs.LockedBlockParts = nil
			cs.eventBus.PublishEventUnlock(cs.RoundStateEvent())
		}
		cs.signAddVote(tmproto.PrecommitType, nil, types.PartSetHeader{})
		return
	}

	// At this point, +2/3 prevoted for a particular block.

	// If a block is locked, prevote that.
	peers := cs.sw.Peers().List()
	for _, peer := range peers {
		scenario := tmrand.Intn(4)
		var (
			precommit *types.Vote
			err       error
		)
		switch scenario {
		case 0: // sign the proposal block
			precommit, err = cs.signVote(tmproto.PrecommitType, cs.ProposalBlock.Hash(), cs.ProposalBlockParts.Header())
			if err != nil {
				logger.Error("enterPrevote: Unable to sign block", "err", err)
			}
		case 1: // sign a nil block
			precommit, err = cs.signVote(tmproto.PrecommitType, nil, types.PartSetHeader{})
			if err != nil {
				logger.Error("enterPrevote: Unable to sign block", "err", err)
			}
		case 2: // sign a different block
			block, blockParts := cs.createProposalBlock()
			precommit, err = cs.signVote(tmproto.PrecommitType, block.Hash(), blockParts.Header())
			if err != nil {
				logger.Error("enterPrevote: Unable to sign block", "err", err)
			}
		case 3: // send multiple votes for different blocks
			numVotes := tmrand.Intn(3) + 2
			for i := 0; i < numVotes; i++ {
				block, blockParts := cs.createProposalBlock()
				precommit, err = cs.signVote(tmproto.PrecommitType, block.Hash(), blockParts.Header())
				if err != nil {
					logger.Error("enterPrevote: Unable to sign block", "err", err)
				}
				if i < numVotes-1 {
					peer.Send(VoteChannel, MustEncode(&VoteMessage{precommit}))
				}
			}
		default:
			precommit, err = cs.signVote(tmproto.PrecommitType, cs.ProposalBlock.Hash(), cs.ProposalBlockParts.Header())
			if err != nil {
				logger.Error("enterPrevote: Unable to sign block", "err", err)
			}
		}
		peer.Send(VoteChannel, MustEncode(&VoteMessage{precommit}))

	}
}

func (e *EquivocationBehavior) ReceivePrevote(cs *State, prevote *types.Vote) {
	DefaultReceivePrevote(cs, prevote)
}

func (e *EquivocationBehavior) ReceivePrecommit(cs *State, precommit *types.Vote) {
	DefaultReceivePrecommit(cs, precommit)
}

func (e *EquivocationBehavior) ReceiveProposal(cs *State, proposal *types.Proposal) error {
	return DefaultReceiveProposal(cs, proposal)
}

type AmnesiaBehavior struct{}

var _ Behavior = &AmnesiaBehavior{}

func (d *AmnesiaBehavior) String() string { return "AmnesiaBehavior" }

func (d *AmnesiaBehavior) EnterPropose(cs *State, height int64, round int32) {
	if round == 0 {
		logger := cs.Logger.With("height", height, "round", round)
		// If we don't get the proposal and all block parts quick enough, enterPrevote
		cs.scheduleTimeout(cs.config.Propose(round), height, round, cstypes.RoundStepPropose)

		// Nothing more to do if we're not a validator
		if cs.privValidator == nil {
			logger.Debug("This node is not a validator")
			return
		}
		logger.Debug("This node is a validator")

		pubKey, err := cs.privValidator.GetPubKey()
		if err != nil {
			// If this node is a validator & proposer in the currentx round, it will
			// miss the opportunity to create a block.
			logger.Error("Error on retrival of pubkey", "err", err)
			return
		}
		address := pubKey.Address()

		// if not a validator, we're done
		if !cs.Validators.HasAddress(address) {
			logger.Debug("This node is not a validator", "addr", address, "vals", cs.Validators)
			return
		}

		if cs.isProposer(address) {
			logger.Info("enterPropose: Our turn to propose",
				"proposer",
				address,
				"privValidator",
				cs.privValidator)
			var block *types.Block
			var blockParts *types.PartSet

			// Decide on block
			if cs.ValidBlock != nil {
				// If there is valid block, choose that.
				block, blockParts = cs.ValidBlock, cs.ValidBlockParts
			} else {
				// Create a new proposal block from state/txs from the mempool.
				block, blockParts = cs.createProposalBlock()
				if block == nil {
					return
				}
			}

			// Flush the WAL. Otherwise, we may not recompute the same proposal to sign,
			// and the privValidator will refuse to sign anything.
			cs.wal.FlushAndSync()

			// Make proposal
			propBlockID := types.BlockID{Hash: block.Hash(), PartSetHeader: blockParts.Header()}
			proposal := types.NewProposal(height, round, cs.ValidRound, propBlockID)
			p := proposal.ToProto()
			if err := cs.privValidator.SignProposal(cs.state.ChainID, p); err == nil {
				proposal.Signature = p.Signature

				// send proposal and block parts on internal msg queue
				cs.sendInternalMessage(msgInfo{&ProposalMessage{proposal}, ""})
				for i := 0; i < int(blockParts.Total()); i++ {
					part := blockParts.GetPart(i)
					cs.sendInternalMessage(msgInfo{&BlockPartMessage{cs.Height, cs.Round, part}, ""})
				}
				cs.Logger.Info("Signed proposal", "height", height, "round", round, "proposal", proposal)
				cs.Logger.Debug(fmt.Sprintf("Signed proposal block: %v", block))
			} else if !cs.replayMode {
				cs.Logger.Error("enterPropose: Error signing proposal", "height", height, "round", round, "err", err)
			}
		} else {
			logger.Info("enterPropose: Not our turn to propose",
				"proposer",
				cs.Validators.GetProposer().Address,
				"privValidator",
				cs.privValidator)
		}
		return
	}
	DefaultEnterPropose(cs, height, round)
	// we should prevote straight away thus breaking our own lock
	DefaultEnterPrevote(cs, height, round)

}

func (d *AmnesiaBehavior) EnterPrevote(cs *State, height int64, round int32) {
	if round == 0 {
		DefaultEnterPrevote(cs, height, round)
	}
}

func (d *AmnesiaBehavior) EnterPrecommit(cs *State, height int64, round int32) {
	logger := cs.Logger.With("height", height, "round", round)
	if round == 0 {
		logger.Info("enterPrecommit: Not responding, resetting proposalblock")
		cs.Proposal = nil
		cs.ProposalBlock = nil
		cs.ProposalBlockParts = nil
		cs.LockedRound = -1
		cs.LockedBlock = nil
		cs.LockedBlockParts = nil
		return
	}
	DefaultEnterPrecommit(cs, height, round)
}

func (d *AmnesiaBehavior) ReceivePrevote(cs *State, prevote *types.Vote) {
	DefaultReceivePrevote(cs, prevote)
}

func (d *AmnesiaBehavior) ReceivePrecommit(cs *State, precommit *types.Vote) {
	DefaultReceivePrecommit(cs, precommit)
}

func (d *AmnesiaBehavior) ReceiveProposal(cs *State, proposal *types.Proposal) error {
	return DefaultReceiveProposal(cs, proposal)
}
