package types

import (
	tmtime "github.com/tendermint/tendermint/types/time"
)

func MakeCommit(blockID BlockID, height int64, round int,
	voteSet *VoteSet,
	validators []PrivValidator) (*Commit, error) {

	// all sign
	for i := 0; i < len(validators); i++ {

		vote := &Vote{
			ValidatorAddress: validators[i].GetAddress(),
			ValidatorIndex:   i,
			Height:           height,
			Round:            round,
			Type:             VoteTypePrecommit,
			BlockID:          blockID,
			Timestamp:        tmtime.Now(),
		}

		_, err := signAddVote(validators[i], vote, voteSet)
		if err != nil {
			return nil, err
		}
	}

	return voteSet.MakeCommit(), nil
}

func signAddVote(privVal PrivValidator, vote *Vote, voteSet *VoteSet) (signed bool, err error) {
	err = privVal.SignVote(voteSet.ChainID(), vote)
	if err != nil {
		return false, err
	}
	return voteSet.AddVote(vote)
}
