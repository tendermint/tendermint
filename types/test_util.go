package types

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/tendermint/tendermint/crypto"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

func RandStateID() StateID {
	return StateID{
		Height:      rand.Int63(), // nolint:gosec
		LastAppHash: tmrand.Bytes(crypto.HashSize),
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
			VoteExtensions: []VoteExtension{
				{
					Type:      tmproto.VoteExtensionType_DEFAULT,
					Extension: []byte("default"),
				},
				{
					Type:      tmproto.VoteExtensionType_THRESHOLD_RECOVER,
					Extension: []byte("threshold"),
				},
			},
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
	for i, ext := range v.VoteExtensions {
		vote.VoteExtensions[i].Signature = ext.Signature
	}
	return voteSet.AddVote(vote)
}

// VoteExtSigns2BytesSlices returns a list of vote-extensions signatures
func VoteExtSigns2BytesSlices(exts []VoteExtension, onlyRecoverable bool) [][]byte {
	sigs := make([][]byte, 0, len(exts))
	for _, ext := range exts {
		if onlyRecoverable && !ext.IsRecoverable() {
			continue
		}
		sigs = append(sigs, ext.Signature)
	}
	return sigs
}
