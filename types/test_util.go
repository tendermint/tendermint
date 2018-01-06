package types

import (
	"time"
)

func MakeCommit(blockID BlockID, height int64, round int,
	voteSet *VoteSet,
	validators []TestSigner) (*Commit, error) {

	// all sign
	for i := 0; i < len(validators); i++ {

		vote := &Vote{
			ValidatorAddress: validators[i].Address(),
			ValidatorIndex:   i,
			Height:           height,
			Round:            round,
			Type:             VoteTypePrecommit,
			BlockID:          blockID,
			Timestamp:        time.Now().UTC(),
		}

		_, err := signAddVote(validators[i], vote, voteSet)
		if err != nil {
			return nil, err
		}
	}

	return voteSet.MakeCommit(), nil
}

func signAddVote(privVal TestSigner, vote *Vote, voteSet *VoteSet) (signed bool, err error) {
	vote.Signature, err = privVal.Sign(SignBytes(voteSet.ChainID(), vote))
	if err != nil {
		return false, err
	}
	return voteSet.AddVote(vote)
}
