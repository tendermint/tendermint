package consensus

import (
	"io"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/state"
)

// Proof of lock.
// +2/3 of validators' (bare) votes for a given blockhash (or nil)
type POL struct {
	Height       uint32
	Round        uint16
	BlockHash    []byte      // Could be nil, which makes this a proof of unlock.
	Votes        []Signature // Vote signatures for height/round/hash
	Commits      []Signature // Commit signatures for height/hash
	CommitRounds []uint16    // Rounds of the commits, less than POL.Round.
}

func ReadPOL(r io.Reader, n *int64, err *error) *POL {
	return &POL{
		Height:       ReadUInt32(r, n, err),
		Round:        ReadUInt16(r, n, err),
		BlockHash:    ReadByteSlice(r, n, err),
		Votes:        ReadSignatures(r, n, err),
		Commits:      ReadSignatures(r, n, err),
		CommitRounds: ReadUInt16s(r, n, err),
	}
}

func (pol *POL) WriteTo(w io.Writer) (n int64, err error) {
	WriteUInt32(w, pol.Height, &n, &err)
	WriteUInt16(w, pol.Round, &n, &err)
	WriteByteSlice(w, pol.BlockHash, &n, &err)
	WriteSignatures(w, pol.Votes, &n, &err)
	WriteSignatures(w, pol.Commits, &n, &err)
	WriteUInt16s(w, pol.CommitRounds, &n, &err)
	return
}

// Returns whether +2/3 have voted/committed for BlockHash.
func (pol *POL) Verify(vset *state.ValidatorSet) error {

	talliedVotingPower := uint64(0)
	voteDoc := BinaryBytes(&Vote{Height: pol.Height, Round: pol.Round,
		Type: VoteTypeBare, BlockHash: pol.BlockHash})
	seenValidators := map[uint64]struct{}{}

	for _, sig := range pol.Votes {

		// Validate
		if _, seen := seenValidators[sig.SignerId]; seen {
			return Errorf("Duplicate validator for vote %v for POL %v", sig, pol)
		}
		_, val := vset.GetById(sig.SignerId)
		if val == nil {
			return Errorf("Invalid validator for vote %v for POL %v", sig, pol)
		}
		if !val.VerifyBytes(voteDoc, sig) {
			return Errorf("Invalid signature for vote %v for POL %v", sig, pol)
		}

		// Tally
		seenValidators[val.Id] = struct{}{}
		talliedVotingPower += val.VotingPower
	}

	for i, sig := range pol.Commits {
		round := pol.CommitRounds[i]

		// Validate
		if _, seen := seenValidators[sig.SignerId]; seen {
			return Errorf("Duplicate validator for commit %v for POL %v", sig, pol)
		}
		_, val := vset.GetById(sig.SignerId)
		if val == nil {
			return Errorf("Invalid validator for commit %v for POL %v", sig, pol)
		}
		if round >= pol.Round {
			return Errorf("Invalid commit round %v for POL %v", round, pol)
		}

		commitDoc := BinaryBytes(&Vote{Height: pol.Height, Round: round,
			Type: VoteTypeCommit, BlockHash: pol.BlockHash}) // TODO cache
		if !val.VerifyBytes(commitDoc, sig) {
			return Errorf("Invalid signature for commit %v for POL %v", sig, pol)
		}

		// Tally
		seenValidators[val.Id] = struct{}{}
		talliedVotingPower += val.VotingPower
	}

	if talliedVotingPower > vset.TotalVotingPower()*2/3 {
		return nil
	} else {
		return Errorf("Invalid POL, insufficient voting power %v, needed %v",
			talliedVotingPower, (vset.TotalVotingPower()*2/3 + 1))
	}

}
