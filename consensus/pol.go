package consensus

import (
	"fmt"

	"github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

// Each signature of a POL (proof-of-lock, see whitepaper) is
// either a prevote or a commit.
// Commits require an additional round which is strictly less than
// the POL round.  Prevote rounds are equal to the POL round.
type POLVoteSignature struct {
	Round     uint                     `json:"round"`
	Signature account.SignatureEd25519 `json:"signature"`
}

// Proof of lock.
// +2/3 of validators' prevotes for a given blockhash (or nil)
type POL struct {
	Height     uint                `json:"height"`
	Round      uint                `json:"round"`
	BlockHash  []byte              `json:"block_hash"`  // Could be nil, which makes this a proof of unlock.
	BlockParts types.PartSetHeader `json:"block_parts"` // When BlockHash is nil, this is zero.
	Votes      []POLVoteSignature  `json:"votes"`       // Prevote and commit signatures in ValidatorSet order.
}

// Returns whether +2/3 have prevoted/committed for BlockHash.
func (pol *POL) Verify(valSet *sm.ValidatorSet) error {

	if uint(len(pol.Votes)) != valSet.Size() {
		return fmt.Errorf("Invalid POL votes count: Expected %v, got %v",
			valSet.Size(), len(pol.Votes))
	}

	talliedVotingPower := uint64(0)
	prevoteDoc := account.SignBytes(config.GetString("chain_id"), &types.Vote{
		Height: pol.Height, Round: pol.Round, Type: types.VoteTypePrevote,
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
			voteDoc = account.SignBytes(config.GetString("chain_id"), &types.Vote{
				Height: pol.Height, Round: vote.Round, Type: types.VoteTypeCommit,
				BlockHash:  pol.BlockHash,
				BlockParts: pol.BlockParts,
			})
		} else if vote.Round > pol.Round {
			return fmt.Errorf("Invalid commit round %v for POL %v", vote.Round, pol)
		}

		// Validate
		if _, seen := seenValidators[string(val.Address)]; seen {
			return fmt.Errorf("Duplicate validator for vote %v for POL %v", vote, pol)
		}

		if !val.PubKey.VerifyBytes(voteDoc, vote.Signature) {
			return fmt.Errorf("Invalid signature for vote %v for POL %v", vote, pol)
		}

		// Tally
		seenValidators[string(val.Address)] = struct{}{}
		talliedVotingPower += val.VotingPower
	}

	if talliedVotingPower > valSet.TotalVotingPower()*2/3 {
		return nil
	} else {
		return fmt.Errorf("Invalid POL, insufficient voting power %v, needed %v",
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

func (pol *POL) MakePartSet() *types.PartSet {
	return types.NewPartSetFromData(binary.BinaryBytes(pol))
}
