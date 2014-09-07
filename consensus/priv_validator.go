package consensus

import (
	. "github.com/tendermint/tendermint/blocks"
	db_ "github.com/tendermint/tendermint/db"
	. "github.com/tendermint/tendermint/state"
)

//-----------------------------------------------------------------------------

type PrivValidator struct {
	PrivAccount
	db *db_.LevelDB
}

// Returns new signed blockParts.
// If signatures already exist in proposal BlockParts,
// e.g. a locked proposal from some prior round,
// those signatures are overwritten.
// Double signing (signing multiple proposals for the same height&round) results in an error.
func (pv *PrivValidator) SignProposal(round uint16, proposal *BlockPartSet) (err error) {
	//TODO: prevent double signing.
	blockParts := proposal.BlockParts()
	for i, part := range blockParts {
		partHash := part.Hash()
		doc := GenBlockPartDocument(
			proposal.Height(), round, uint16(i), uint16(len(blockParts)), partHash)
		part.Signature = pv.Sign([]byte(doc))
	}
	return nil
}

// Modifies the vote object in memory.
// Double signing results in an error.
func (pv *PrivValidator) SignVote(vote *Vote) error {
	//TODO: prevent double signing.
	doc := GenVoteDocument(vote.Type, vote.Height, vote.Round, vote.Hash)
	vote.Signature = pv.Sign([]byte(doc))
	return nil
}
