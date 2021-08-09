package meta_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
	test "github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/libs/bits"
	"github.com/tendermint/tendermint/pkg/consensus"
	"github.com/tendermint/tendermint/pkg/meta"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

func TestCommit(t *testing.T) {
	lastID := test.MakeBlockID()
	h := int64(3)
	voteSet, _, vals := test.RandVoteSet(h-1, 1, tmproto.PrecommitType, 10, 1)
	commit, err := test.MakeCommit(lastID, h-1, 1, voteSet, vals, time.Now())
	require.NoError(t, err)

	assert.Equal(t, h-1, commit.Height)
	assert.EqualValues(t, 1, commit.Round)
	assert.Equal(t, tmproto.PrecommitType, tmproto.SignedMsgType(commit.Type()))
	if commit.Size() <= 0 {
		t.Fatalf("commit %v has a zero or negative size: %d", commit, commit.Size())
	}

	require.NotNil(t, commit.BitArray())
	assert.Equal(t, bits.NewBitArray(10).Size(), commit.BitArray().Size())

	assert.Equal(t, voteSet.GetByIndex(0), consensus.GetVoteFromCommit(commit, 0))
	assert.True(t, commit.IsCommit())
}

func TestCommitValidateBasic(t *testing.T) {
	testCases := []struct {
		testName       string
		malleateCommit func(*meta.Commit)
		expectErr      bool
	}{
		{"Random Commit", func(com *meta.Commit) {}, false},
		{"Incorrect signature", func(com *meta.Commit) { com.Signatures[0].Signature = []byte{0} }, false},
		{"Incorrect height", func(com *meta.Commit) { com.Height = int64(-100) }, true},
		{"Incorrect round", func(com *meta.Commit) { com.Round = -100 }, true},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			com := test.MakeRandomCommit(time.Now())
			tc.malleateCommit(com)
			assert.Equal(t, tc.expectErr, com.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestCommit_ValidateBasic(t *testing.T) {
	testCases := []struct {
		name      string
		commit    *meta.Commit
		expectErr bool
		errString string
	}{
		{
			"invalid height",
			&meta.Commit{Height: -1},
			true, "negative Height",
		},
		{
			"invalid round",
			&meta.Commit{Height: 1, Round: -1},
			true, "negative Round",
		},
		{
			"invalid block ID",
			&meta.Commit{
				Height:  1,
				Round:   1,
				BlockID: meta.BlockID{},
			},
			true, "commit cannot be for nil block",
		},
		{
			"no signatures",
			&meta.Commit{
				Height: 1,
				Round:  1,
				BlockID: meta.BlockID{
					Hash: make([]byte, tmhash.Size),
					PartSetHeader: meta.PartSetHeader{
						Hash: make([]byte, tmhash.Size),
					},
				},
			},
			true, "no signatures in commit",
		},
		{
			"invalid signature",
			&meta.Commit{
				Height: 1,
				Round:  1,
				BlockID: meta.BlockID{
					Hash: make([]byte, tmhash.Size),
					PartSetHeader: meta.PartSetHeader{
						Hash: make([]byte, tmhash.Size),
					},
				},
				Signatures: []meta.CommitSig{
					{
						BlockIDFlag:      meta.BlockIDFlagCommit,
						ValidatorAddress: make([]byte, crypto.AddressSize),
						Signature:        make([]byte, meta.MaxSignatureSize+1),
					},
				},
			},
			true, "wrong CommitSig",
		},
		{
			"valid commit",
			&meta.Commit{
				Height: 1,
				Round:  1,
				BlockID: meta.BlockID{
					Hash: make([]byte, tmhash.Size),
					PartSetHeader: meta.PartSetHeader{
						Hash: make([]byte, tmhash.Size),
					},
				},
				Signatures: []meta.CommitSig{
					{
						BlockIDFlag:      meta.BlockIDFlagCommit,
						ValidatorAddress: make([]byte, crypto.AddressSize),
						Signature:        make([]byte, meta.MaxSignatureSize),
					},
				},
			},
			false, "",
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			err := tc.commit.ValidateBasic()
			if tc.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errString)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMaxCommitBytes(t *testing.T) {
	// time is varint encoded so need to pick the max.
	// year int, month Month, day, hour, min, sec, nsec int, loc *Location
	timestamp := time.Date(math.MaxInt64, 0, 0, 0, 0, 0, math.MaxInt64, time.UTC)

	cs := meta.CommitSig{
		BlockIDFlag:      meta.BlockIDFlagNil,
		ValidatorAddress: crypto.AddressHash([]byte("validator_address")),
		Timestamp:        timestamp,
		Signature:        crypto.CRandBytes(meta.MaxSignatureSize),
	}

	pbSig := cs.ToProto()
	// test that a single commit sig doesn't exceed max commit sig bytes
	assert.EqualValues(t, meta.MaxCommitSigBytes, pbSig.Size())

	// check size with a single commit
	commit := &meta.Commit{
		Height: math.MaxInt64,
		Round:  math.MaxInt32,
		BlockID: meta.BlockID{
			Hash: tmhash.Sum([]byte("blockID_hash")),
			PartSetHeader: meta.PartSetHeader{
				Total: math.MaxInt32,
				Hash:  tmhash.Sum([]byte("blockID_part_set_header_hash")),
			},
		},
		Signatures: []meta.CommitSig{cs},
	}

	pb := commit.ToProto()

	assert.EqualValues(t, meta.MaxCommitBytes(1), int64(pb.Size()))

	// check the upper bound of the commit size
	for i := 1; i < consensus.MaxVotesCount; i++ {
		commit.Signatures = append(commit.Signatures, cs)
	}

	pb = commit.ToProto()

	assert.EqualValues(t, meta.MaxCommitBytes(consensus.MaxVotesCount), int64(pb.Size()))
}

func TestCommitSig_ValidateBasic(t *testing.T) {
	testCases := []struct {
		name      string
		cs        meta.CommitSig
		expectErr bool
		errString string
	}{
		{
			"invalid ID flag",
			meta.CommitSig{BlockIDFlag: meta.BlockIDFlag(0xFF)},
			true, "unknown BlockIDFlag",
		},
		{
			"BlockIDFlagAbsent validator address present",
			meta.CommitSig{BlockIDFlag: meta.BlockIDFlagAbsent, ValidatorAddress: crypto.Address("testaddr")},
			true, "validator address is present",
		},
		{
			"BlockIDFlagAbsent timestamp present",
			meta.CommitSig{BlockIDFlag: meta.BlockIDFlagAbsent, Timestamp: time.Now().UTC()},
			true, "time is present",
		},
		{
			"BlockIDFlagAbsent signatures present",
			meta.CommitSig{BlockIDFlag: meta.BlockIDFlagAbsent, Signature: []byte{0xAA}},
			true, "signature is present",
		},
		{
			"BlockIDFlagAbsent valid BlockIDFlagAbsent",
			meta.CommitSig{BlockIDFlag: meta.BlockIDFlagAbsent},
			false, "",
		},
		{
			"non-BlockIDFlagAbsent invalid validator address",
			meta.CommitSig{BlockIDFlag: meta.BlockIDFlagCommit, ValidatorAddress: make([]byte, 1)},
			true, "expected ValidatorAddress size",
		},
		{
			"non-BlockIDFlagAbsent invalid signature (zero)",
			meta.CommitSig{
				BlockIDFlag:      meta.BlockIDFlagCommit,
				ValidatorAddress: make([]byte, crypto.AddressSize),
				Signature:        make([]byte, 0),
			},
			true, "signature is missing",
		},
		{
			"non-BlockIDFlagAbsent invalid signature (too large)",
			meta.CommitSig{
				BlockIDFlag:      meta.BlockIDFlagCommit,
				ValidatorAddress: make([]byte, crypto.AddressSize),
				Signature:        make([]byte, meta.MaxSignatureSize+1),
			},
			true, "signature is too big",
		},
		{
			"non-BlockIDFlagAbsent valid",
			meta.CommitSig{
				BlockIDFlag:      meta.BlockIDFlagCommit,
				ValidatorAddress: make([]byte, crypto.AddressSize),
				Signature:        make([]byte, meta.MaxSignatureSize),
			},
			false, "",
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			err := tc.cs.ValidateBasic()
			if tc.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errString)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
