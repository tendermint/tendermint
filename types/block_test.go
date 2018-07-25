package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	crypto "github.com/tendermint/tendermint/crypto"
	cmn "github.com/tendermint/tendermint/libs/common"
)

func TestBlockAddEvidence(t *testing.T) {
	txs := []Tx{Tx("foo"), Tx("bar")}
	lastID := makeBlockIDRandom()
	h := int64(3)

	voteSet, valSet, vals := randVoteSet(h-1, 1, VoteTypePrecommit, 10, 1)
	commit, err := MakeCommit(lastID, h-1, 1, voteSet, vals)
	require.NoError(t, err)

	ev := NewMockGoodEvidence(h, 0, valSet.Validators[0].Address)
	evList := []Evidence{ev}

	block := MakeBlock(h, txs, commit, evList)
	require.NotNil(t, block)
	require.Equal(t, 1, len(block.Evidence.Evidence))
	require.NotNil(t, block.EvidenceHash)
}

func TestBlockValidateBasic(t *testing.T) {
	require.Error(t, (*Block)(nil).ValidateBasic())

	txs := []Tx{Tx("foo"), Tx("bar")}
	lastID := makeBlockIDRandom()
	h := int64(3)

	voteSet, valSet, vals := randVoteSet(h-1, 1, VoteTypePrecommit, 10, 1)
	commit, err := MakeCommit(lastID, h-1, 1, voteSet, vals)
	require.NoError(t, err)

	ev := NewMockGoodEvidence(h, 0, valSet.Validators[0].Address)
	evList := []Evidence{ev}

	block := MakeBlock(h, txs, commit, evList)
	require.NotNil(t, block)

	// proper block must pass
	err = block.ValidateBasic()
	require.NoError(t, err)

	// tamper with NumTxs
	block = MakeBlock(h, txs, commit, evList)
	block.NumTxs++
	err = block.ValidateBasic()
	require.Error(t, err)

	// remove 1/2 the commits
	block = MakeBlock(h, txs, commit, evList)
	block.LastCommit.Precommits = commit.Precommits[:commit.Size()/2]
	block.LastCommit.hash = nil // clear hash or change wont be noticed
	err = block.ValidateBasic()
	require.Error(t, err)

	// tamper with LastCommitHash
	block = MakeBlock(h, txs, commit, evList)
	block.LastCommitHash = []byte("something else")
	err = block.ValidateBasic()
	require.Error(t, err)

	// tamper with data
	block = MakeBlock(h, txs, commit, evList)
	block.Data.Txs[0] = Tx("something else")
	block.Data.hash = nil // clear hash or change wont be noticed
	err = block.ValidateBasic()
	require.Error(t, err)

	// tamper with DataHash
	block = MakeBlock(h, txs, commit, evList)
	block.DataHash = cmn.RandBytes(len(block.DataHash))
	err = block.ValidateBasic()
	require.Error(t, err)

	// tamper with evidence
	block = MakeBlock(h, txs, commit, evList)
	block.EvidenceHash = []byte("something else")
	err = block.ValidateBasic()
	require.Error(t, err)
}

func TestBlockHash(t *testing.T) {
	assert.Nil(t, (*Block)(nil).Hash())
	assert.Nil(t, MakeBlock(int64(3), []Tx{Tx("Hello World")}, nil, nil).Hash())
}

func TestBlockMakePartSet(t *testing.T) {
	assert.Nil(t, (*Block)(nil).MakePartSet(2))

	partSet := MakeBlock(int64(3), []Tx{Tx("Hello World")}, nil, nil).MakePartSet(1024)
	assert.NotNil(t, partSet)
	assert.Equal(t, 1, partSet.Total())
}

func TestBlockHashesTo(t *testing.T) {
	assert.False(t, (*Block)(nil).HashesTo(nil))

	lastID := makeBlockIDRandom()
	h := int64(3)
	voteSet, valSet, vals := randVoteSet(h-1, 1, VoteTypePrecommit, 10, 1)
	commit, err := MakeCommit(lastID, h-1, 1, voteSet, vals)
	require.NoError(t, err)

	ev := NewMockGoodEvidence(h, 0, valSet.Validators[0].Address)
	evList := []Evidence{ev}

	block := MakeBlock(h, []Tx{Tx("Hello World")}, commit, evList)
	block.ValidatorsHash = valSet.Hash()
	assert.False(t, block.HashesTo([]byte{}))
	assert.False(t, block.HashesTo([]byte("something else")))
	assert.True(t, block.HashesTo(block.Hash()))
}

func TestBlockSize(t *testing.T) {
	size := MakeBlock(int64(3), []Tx{Tx("Hello World")}, nil, nil).Size()
	if size <= 0 {
		t.Fatal("Size of the block is zero or negative")
	}
}

func TestBlockString(t *testing.T) {
	assert.Equal(t, "nil-Block", (*Block)(nil).String())
	assert.Equal(t, "nil-Block", (*Block)(nil).StringIndented(""))
	assert.Equal(t, "nil-Block", (*Block)(nil).StringShort())

	block := MakeBlock(int64(3), []Tx{Tx("Hello World")}, nil, nil)
	assert.NotEqual(t, "nil-Block", block.String())
	assert.NotEqual(t, "nil-Block", block.StringIndented(""))
	assert.NotEqual(t, "nil-Block", block.StringShort())
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

func TestCommit(t *testing.T) {
	lastID := makeBlockIDRandom()
	h := int64(3)
	voteSet, _, vals := randVoteSet(h-1, 1, VoteTypePrecommit, 10, 1)
	commit, err := MakeCommit(lastID, h-1, 1, voteSet, vals)
	require.NoError(t, err)

	assert.NotNil(t, commit.FirstPrecommit())
	assert.Equal(t, h-1, commit.Height())
	assert.Equal(t, 1, commit.Round())
	assert.Equal(t, VoteTypePrecommit, commit.Type())
	if commit.Size() <= 0 {
		t.Fatalf("commit %v has a zero or negative size: %d", commit, commit.Size())
	}

	require.NotNil(t, commit.BitArray())
	assert.Equal(t, cmn.NewBitArray(10).Size(), commit.BitArray().Size())

	assert.Equal(t, voteSet.GetByIndex(0), commit.GetByIndex(0))
	assert.True(t, commit.IsCommit())
}

func TestCommitValidateBasic(t *testing.T) {
	commit := randCommit()
	assert.NoError(t, commit.ValidateBasic())

	// nil precommit is OK
	commit = randCommit()
	commit.Precommits[0] = nil
	assert.NoError(t, commit.ValidateBasic())

	// tamper with types
	commit = randCommit()
	commit.Precommits[0].Type = VoteTypePrevote
	assert.Error(t, commit.ValidateBasic())

	// tamper with height
	commit = randCommit()
	commit.Precommits[0].Height = int64(100)
	assert.Error(t, commit.ValidateBasic())

	// tamper with round
	commit = randCommit()
	commit.Precommits[0].Round = 100
	assert.Error(t, commit.ValidateBasic())
}

func randCommit() *Commit {
	lastID := makeBlockIDRandom()
	h := int64(3)
	voteSet, _, vals := randVoteSet(h-1, 1, VoteTypePrecommit, 10, 1)
	commit, err := MakeCommit(lastID, h-1, 1, voteSet, vals)
	if err != nil {
		panic(err)
	}
	return commit
}
