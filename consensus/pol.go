package consensus

import (
	"fmt"

	"github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/block"
	. "github.com/tendermint/tendermint/common"
	sm "github.com/tendermint/tendermint/state"
)

// Each signature of a POL (proof-of-lock, see whitepaper) is
// either a prevote or a commit.
// Commits require an additional round which is strictly less than
// the POL round.  Prevote rounds are equal to the POL round.
type POLVoteSignature struct {
	Round     uint
	Signature account.SignatureEd25519
}

// Proof of lock.
// +2/3 of validators' prevotes for a given blockhash (or nil)
type POL struct {
	Height     uint
	Round      uint
	BlockHash  []byte              // Could be nil, which makes this a proof of unlock.
	BlockParts block.PartSetHeader // When BlockHash is nil, this is zero.
	Votes      []POLVoteSignature  // Prevote and commit signatures in ValidatorSet order.
}

// Returns whether +2/3 have prevoted/committed for BlockHash.
func (pol *POL) Verify(valSet *sm.ValidatorSet) error {

	if uint(len(pol.Votes)) != valSet.Size() {
		return Errorf("Invalid POL votes count: Expected %v, got %v",
			valSet.Size(), len(pol.Votes))
	}

	talliedVotingPower := uint64(0)
	prevoteDoc := account.SignBytes(&block.Vote{
		Height: pol.Height, Round: pol.Round, Type: block.VoteTypePrevote,
		BlockHash:  pol.BlockHash,
		BlockParts: pol.BlockParts,
	})
	seenValidators := map[string]struct{}{}

	for idx, vote := range pol.Votes {
		// vote may be zero, in which case skip.
		if vote.Signature.IsZero() {
			continue
		}
		voteDoc := prevoteDoc
		_, val := valSet.GetByIndex(uint(idx))

		// Commit vote?
		if vote.Round < pol.Round {
			voteDoc = account.SignBytes(&block.Vote{
				Height: pol.Height, Round: vote.Round, Type: block.VoteTypeCommit,
				BlockHash:  pol.BlockHash,
				BlockParts: pol.BlockParts,
			})
		} else if vote.Round > pol.Round {
			return Errorf("Invalid commit round %v for POL %v", vote.Round, pol)
		}

		// Validate
		if _, seen := seenValidators[string(val.Address)]; seen {
			return Errorf("Duplicate validator for vote %v for POL %v", vote, pol)
		}

		if !val.PubKey.VerifyBytes(voteDoc, vote.Signature) {
			return Errorf("Invalid signature for vote %v for POL %v", vote, pol)
		}

		// Tally
		seenValidators[string(val.Address)] = struct{}{}
		talliedVotingPower += val.VotingPower
	}

	if talliedVotingPower > valSet.TotalVotingPower()*2/3 {
		return nil
	} else {
		return Errorf("Invalid POL, insufficient voting power %v, needed %v",
			talliedVotingPower, (valSet.TotalVotingPower()*2/3 + 1))
	}

}

func (pol *POL) StringShort() string {
	if pol == nil {
		return "nil-POL"
	} else {
		return fmt.Sprintf("POL{H:%v R:%v BH:%X}", pol.Height, pol.Round,
			Fingerprint(pol.BlockHash), pol.BlockParts)
	}
}
