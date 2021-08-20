package metadata_test

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
	"github.com/tendermint/tendermint/pkg/metadata"
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
		malleateCommit func(*metadata.Commit)
		expectErr      bool
	}{
		{"Random Commit", func(com *metadata.Commit) {}, false},
		{"Incorrect signature", func(com *metadata.Commit) { com.Signatures[0].Signature = []byte{0} }, false},
		{"Incorrect height", func(com *metadata.Commit) { com.Height = int64(-100) }, true},
		{"Incorrect round", func(com *metadata.Commit) { com.Round = -100 }, true},
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
		commit    *metadata.Commit
		expectErr bool
		errString string
	}{
		{
			"invalid height",
			&metadata.Commit{Height: -1},
			true, "negative Height",
		},
		{
			"invalid round",
			&metadata.Commit{Height: 1, Round: -1},
			true, "negative Round",
		},
		{
			"invalid block ID",
			&metadata.Commit{
				Height:  1,
				Round:   1,
				BlockID: metadata.BlockID{},
			},
			true, "commit cannot be for nil block",
		},
		{
			"no signatures",
			&metadata.Commit{
				Height: 1,
				Round:  1,
				BlockID: metadata.BlockID{
					Hash: make([]byte, tmhash.Size),
					PartSetHeader: metadata.PartSetHeader{
						Hash: make([]byte, tmhash.Size),
					},
				},
			},
			true, "no signatures in commit",
		},
		{
			"invalid signature",
			&metadata.Commit{
				Height: 1,
				Round:  1,
				BlockID: metadata.BlockID{
					Hash: make([]byte, tmhash.Size),
					PartSetHeader: metadata.PartSetHeader{
						Hash: make([]byte, tmhash.Size),
					},
				},
				Signatures: []metadata.CommitSig{
					{
						BlockIDFlag:      metadata.BlockIDFlagCommit,
						ValidatorAddress: make([]byte, crypto.AddressSize),
						Signature:        make([]byte, metadata.MaxSignatureSize+1),
					},
				},
			},
			true, "wrong CommitSig",
		},
		{
			"valid commit",
			&metadata.Commit{
				Height: 1,
				Round:  1,
				BlockID: metadata.BlockID{
					Hash: make([]byte, tmhash.Size),
					PartSetHeader: metadata.PartSetHeader{
						Hash: make([]byte, tmhash.Size),
					},
				},
				Signatures: []metadata.CommitSig{
					{
						BlockIDFlag:      metadata.BlockIDFlagCommit,
						ValidatorAddress: make([]byte, crypto.AddressSize),
						Signature:        make([]byte, metadata.MaxSignatureSize),
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

	cs := metadata.CommitSig{
		BlockIDFlag:      metadata.BlockIDFlagNil,
		ValidatorAddress: crypto.AddressHash([]byte("validator_address")),
		Timestamp:        timestamp,
		Signature:        crypto.CRandBytes(metadata.MaxSignatureSize),
	}

	pbSig := cs.ToProto()
	// test that a single commit sig doesn't exceed max commit sig bytes
	assert.EqualValues(t, metadata.MaxCommitSigBytes, pbSig.Size())

	// check size with a single commit
	commit := &metadata.Commit{
		Height: math.MaxInt64,
		Round:  math.MaxInt32,
		BlockID: metadata.BlockID{
			Hash: tmhash.Sum([]byte("blockID_hash")),
			PartSetHeader: metadata.PartSetHeader{
				Total: math.MaxInt32,
				Hash:  tmhash.Sum([]byte("blockID_part_set_header_hash")),
			},
		},
		Signatures: []metadata.CommitSig{cs},
	}

	pb := commit.ToProto()

	assert.EqualValues(t, metadata.MaxCommitBytes(1), int64(pb.Size()))

	// check the upper bound of the commit size
	for i := 1; i < consensus.MaxVotesCount; i++ {
		commit.Signatures = append(commit.Signatures, cs)
	}

	pb = commit.ToProto()

	assert.EqualValues(t, metadata.MaxCommitBytes(consensus.MaxVotesCount), int64(pb.Size()))
}

func TestCommitSig_ValidateBasic(t *testing.T) {
	testCases := []struct {
		name      string
		cs        metadata.CommitSig
		expectErr bool
		errString string
	}{
		{
			"invalid ID flag",
			metadata.CommitSig{BlockIDFlag: metadata.BlockIDFlag(0xFF)},
			true, "unknown BlockIDFlag",
		},
		{
			"BlockIDFlagAbsent validator address present",
			metadata.CommitSig{BlockIDFlag: metadata.BlockIDFlagAbsent, ValidatorAddress: crypto.Address("testaddr")},
			true, "validator address is present",
		},
		{
			"BlockIDFlagAbsent timestamp present",
			metadata.CommitSig{BlockIDFlag: metadata.BlockIDFlagAbsent, Timestamp: time.Now().UTC()},
			true, "time is present",
		},
		{
			"BlockIDFlagAbsent signatures present",
			metadata.CommitSig{BlockIDFlag: metadata.BlockIDFlagAbsent, Signature: []byte{0xAA}},
			true, "signature is present",
		},
		{
			"BlockIDFlagAbsent valid BlockIDFlagAbsent",
			metadata.CommitSig{BlockIDFlag: metadata.BlockIDFlagAbsent},
			false, "",
		},
		{
			"non-BlockIDFlagAbsent invalid validator address",
			metadata.CommitSig{BlockIDFlag: metadata.BlockIDFlagCommit, ValidatorAddress: make([]byte, 1)},
			true, "expected ValidatorAddress size",
		},
		{
			"non-BlockIDFlagAbsent invalid signature (zero)",
			metadata.CommitSig{
				BlockIDFlag:      metadata.BlockIDFlagCommit,
				ValidatorAddress: make([]byte, crypto.AddressSize),
				Signature:        make([]byte, 0),
			},
			true, "signature is missing",
		},
		{
			"non-BlockIDFlagAbsent invalid signature (too large)",
			metadata.CommitSig{
				BlockIDFlag:      metadata.BlockIDFlagCommit,
				ValidatorAddress: make([]byte, crypto.AddressSize),
				Signature:        make([]byte, metadata.MaxSignatureSize+1),
			},
			true, "signature is too big",
		},
		{
			"non-BlockIDFlagAbsent valid",
			metadata.CommitSig{
				BlockIDFlag:      metadata.BlockIDFlagCommit,
				ValidatorAddress: make([]byte, crypto.AddressSize),
				Signature:        make([]byte, metadata.MaxSignatureSize),
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
