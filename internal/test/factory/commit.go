package factory

import (
	"context"
	"fmt"
	"github.com/dashevo/dashd-go/btcjson"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/log"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

func MakeCommit(blockID types.BlockID, height int64, round int32, voteSet *types.VoteSet,
	validatorSet *types.ValidatorSet, validators []types.PrivValidator, stateID types.StateID) (*types.Commit, error) {

	// all sign
	for i := 0; i < len(validators); i++ {
		proTxHash, err := validators[i].GetProTxHash(context.Background())
		if err != nil {
			return nil, fmt.Errorf("can't get pubkey: %w", err)
		}
		vote := &types.Vote{
			ValidatorProTxHash: proTxHash,
			ValidatorIndex:     int32(i),
			Height:             height,
			Round:              round,
			Type:               tmproto.PrecommitType,
			BlockID:            blockID,
		}
		_, err = signAddVote(validators[i], vote, voteSet, validatorSet.QuorumType, validatorSet.QuorumHash, stateID)
		if err != nil {
			return nil, err
		}
	}

	return voteSet.MakeCommit(), nil
}

func signAddVote(privVal types.PrivValidator, vote *types.Vote, voteSet *types.VoteSet, quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash, stateID types.StateID) (signed bool, err error) {
	v := vote.ToProto()
	err = privVal.SignVote(context.Background(), voteSet.ChainID(), quorumType, quorumHash, v, stateID, log.NewNopLogger())
	if err != nil {
		return false, err
	}
	return voteSet.AddVote(vote)
}
