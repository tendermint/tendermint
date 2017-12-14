package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	crypto "github.com/tendermint/go-crypto"
)

func TestValidateBlock(t *testing.T) {
	txs := []Tx{Tx("foo"), Tx("bar")}
	lastID := makeBlockID()
	valHash := []byte("val")
	appHash := []byte("app")
	consensusHash := []byte("consensus-params")
	h := int64(3)

	voteSet, _, vals := randVoteSet(h-1, 1, VoteTypePrecommit,
		10, 1)
	commit, err := makeCommit(lastID, h-1, 1, voteSet, vals)
	require.NoError(t, err)

	block, _ := MakeBlock(h, "hello", txs, 10, commit,
		lastID, valHash, appHash, consensusHash, 2)
	require.NotNil(t, block)

	// proper block must pass
	err = block.ValidateBasic("hello", h-1, 10, lastID, block.Time, appHash, consensusHash)
	require.NoError(t, err)

	// wrong chain fails
	err = block.ValidateBasic("other", h-1, 10, lastID, block.Time, appHash, consensusHash)
	require.Error(t, err)

	// wrong height fails
	err = block.ValidateBasic("hello", h+4, 10, lastID, block.Time, appHash, consensusHash)
	require.Error(t, err)

	// wrong total tx fails
	err = block.ValidateBasic("hello", h-1, 15, lastID, block.Time, appHash, consensusHash)
	require.Error(t, err)

	// wrong blockid fails
	err = block.ValidateBasic("hello", h-1, 10, makeBlockID(), block.Time, appHash, consensusHash)
	require.Error(t, err)

	// wrong app hash fails
	err = block.ValidateBasic("hello", h-1, 10, lastID, block.Time, []byte("bad-hash"), consensusHash)
	require.Error(t, err)

	// wrong consensus hash fails
	err = block.ValidateBasic("hello", h-1, 10, lastID, block.Time, appHash, []byte("wrong-params"))
	require.Error(t, err)
}

func makeBlockID() BlockID {
	blockHash, blockPartsHeader := crypto.CRandBytes(32), PartSetHeader{123, crypto.CRandBytes(32)}
	return BlockID{blockHash, blockPartsHeader}
}

func makeCommit(blockID BlockID, height int64, round int,
	voteSet *VoteSet,
	validators []*PrivValidatorFS) (*Commit, error) {

	voteProto := &Vote{
		ValidatorAddress: nil,
		ValidatorIndex:   -1,
		Height:           height,
		Round:            round,
		Type:             VoteTypePrecommit,
		BlockID:          blockID,
		Timestamp:        time.Now().UTC(),
	}

	// all sign
	for i := 0; i < len(validators); i++ {
		vote := withValidator(voteProto, validators[i].GetAddress(), i)
		_, err := signAddVote(validators[i], vote, voteSet)
		if err != nil {
			return nil, err
		}
	}

	return voteSet.MakeCommit(), nil
}
