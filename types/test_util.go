package types

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/tendermint/tendermint/crypto/tmhash"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

func RandStateID() StateID {
	return StateID{
		Height:      rand.Int63(), // nolint:gosec
		LastAppHash: tmrand.Bytes(tmhash.Size),
	}
}

func makeCommit(
	ctx context.Context,
	blockID BlockID,
	stateID StateID,
	height int64,
	round int32,
	voteSet *VoteSet,
	validators []PrivValidator,
	now time.Time,
) (*Commit, error) {

	// all sign
	for i := 0; i < len(validators); i++ {
		proTxHash, err := validators[i].GetProTxHash(ctx)
		if err != nil {
			return nil, fmt.Errorf("can't get proTxHash: %w", err)
		}
		vote := &Vote{
			ValidatorProTxHash: proTxHash,
			ValidatorIndex:     int32(i),
			Height:             height,
			Round:              round,
			Type:               tmproto.PrecommitType,
			BlockID:            blockID,
		}

		_, err = signAddVote(ctx, validators[i], vote, voteSet)
		if err != nil {
			return nil, err
		}
	}

	return voteSet.MakeCommit(), nil
}

// signAddVote signs a vote using StateID configured inside voteSet, and adds it to that voteSet
func signAddVote(ctx context.Context, privVal PrivValidator, vote *Vote, voteSet *VoteSet) (signed bool, err error) {
	return signAddVoteForStateID(ctx, privVal, vote, voteSet, voteSet.stateID)
}

func signAddVoteForStateID(ctx context.Context, privVal PrivValidator, vote *Vote, voteSet *VoteSet,
	stateID StateID) (signed bool, err error) {
	v := vote.ToProto()
	err = privVal.SignVote(ctx, voteSet.ChainID(), voteSet.valSet.QuorumType, voteSet.valSet.QuorumHash,
		v, stateID, nil)
	if err != nil {
		return false, err
	}
	vote.BlockSignature = v.BlockSignature
	vote.StateSignature = v.StateSignature
	vote.ExtensionSignature = v.ExtensionSignature
	return voteSet.AddVote(vote)
}

// Votes constructed from commits don't have extensions, because we don't store
// the extensions themselves in the commit. This method is used to construct a
// copy of a vote, but nil its extension and signature.
func voteWithoutExtension(v *Vote) *Vote {
	vc := v.Copy()
	vc.Extension = nil
	vc.ExtensionSignature = nil
	return vc
}
