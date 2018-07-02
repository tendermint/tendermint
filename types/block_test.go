package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	crypto "github.com/tendermint/tendermint/crypto"
	cmn "github.com/tendermint/tendermint/libs/common"
)

func TestValidateBlock(t *testing.T) {
	txs := []Tx{Tx("foo"), Tx("bar")}
	lastID := makeBlockIDRandom()
	h := int64(3)

	voteSet, _, vals := randVoteSet(h-1, 1, VoteTypePrecommit, 10, 1)
	commit, err := MakeCommit(lastID, h-1, 1, voteSet, vals)
	require.NoError(t, err)

	block := MakeBlock(h, txs, commit)
	require.NotNil(t, block)

	// proper block must pass
	err = block.ValidateBasic()
	require.NoError(t, err)

	// tamper with NumTxs
	block = MakeBlock(h, txs, commit)
	block.NumTxs++
	err = block.ValidateBasic()
	require.Error(t, err)

	// remove 1/2 the commits
	block = MakeBlock(h, txs, commit)
	block.LastCommit.Precommits = commit.Precommits[:commit.Size()/2]
	block.LastCommit.hash = nil // clear hash or change wont be noticed
	err = block.ValidateBasic()
	require.Error(t, err)

	// tamper with LastCommitHash
	block = MakeBlock(h, txs, commit)
	block.LastCommitHash = []byte("something else")
	err = block.ValidateBasic()
	require.Error(t, err)

	// tamper with data
	block = MakeBlock(h, txs, commit)
	block.Data.Txs[0] = Tx("something else")
	block.Data.hash = nil // clear hash or change wont be noticed
	err = block.ValidateBasic()
	require.Error(t, err)

	// tamper with DataHash
	block = MakeBlock(h, txs, commit)
	block.DataHash = cmn.RandBytes(len(block.DataHash))
	err = block.ValidateBasic()
	require.Error(t, err)
}

func makeBlockIDRandom() BlockID {
	blockHash, blockPartsHeader := crypto.CRandBytes(32), PartSetHeader{123, crypto.CRandBytes(32)}
	return BlockID{blockHash, blockPartsHeader}
}

func makeBlockID(hash string, partSetSize int, partSetHash string) BlockID {
	return BlockID{
		Hash: []byte(hash),
		PartsHeader: PartSetHeader{
			Total: partSetSize,
			Hash:  []byte(partSetHash),
		},
	}

}

var nilBytes []byte

func TestNilHeaderHashDoesntCrash(t *testing.T) {
	assert.Equal(t, []byte((*Header)(nil).Hash()), nilBytes)
	assert.Equal(t, []byte((new(Header)).Hash()), nilBytes)
}

func TestNilDataHashDoesntCrash(t *testing.T) {
	assert.Equal(t, []byte((*Data)(nil).Hash()), nilBytes)
	assert.Equal(t, []byte(new(Data).Hash()), nilBytes)
}
