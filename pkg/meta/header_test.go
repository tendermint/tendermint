package meta_test

import (
	"encoding/hex"
	"math"
	"reflect"
	"testing"
	"time"

	gogotypes "github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/crypto/tmhash"
	test "github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/pkg/meta"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
	"github.com/tendermint/tendermint/version"
)

var nilBytes []byte

func TestNilHeaderHashDoesntCrash(t *testing.T) {
	assert.Equal(t, nilBytes, []byte((*meta.Header)(nil).Hash()))
	assert.Equal(t, nilBytes, []byte((new(meta.Header)).Hash()))
}

func TestHeaderHash(t *testing.T) {
	testCases := []struct {
		desc       string
		header     *meta.Header
		expectHash bytes.HexBytes
	}{
		{"Generates expected hash", &meta.Header{
			Version:            version.Consensus{Block: 1, App: 2},
			ChainID:            "chainId",
			Height:             3,
			Time:               time.Date(2019, 10, 13, 16, 14, 44, 0, time.UTC),
			LastBlockID:        test.MakeBlockIDWithHash(make([]byte, tmhash.Size)),
			LastCommitHash:     tmhash.Sum([]byte("last_commit_hash")),
			DataHash:           tmhash.Sum([]byte("data_hash")),
			ValidatorsHash:     tmhash.Sum([]byte("validators_hash")),
			NextValidatorsHash: tmhash.Sum([]byte("next_validators_hash")),
			ConsensusHash:      tmhash.Sum([]byte("consensus_hash")),
			AppHash:            tmhash.Sum([]byte("app_hash")),
			LastResultsHash:    tmhash.Sum([]byte("last_results_hash")),
			EvidenceHash:       tmhash.Sum([]byte("evidence_hash")),
			ProposerAddress:    crypto.AddressHash([]byte("proposer_address")),
		}, hexBytesFromString("F740121F553B5418C3EFBD343C2DBFE9E007BB67B0D020A0741374BAB65242A4")},
		{"nil header yields nil", nil, nil},
		{"nil ValidatorsHash yields nil", &meta.Header{
			Version:            version.Consensus{Block: 1, App: 2},
			ChainID:            "chainId",
			Height:             3,
			Time:               time.Date(2019, 10, 13, 16, 14, 44, 0, time.UTC),
			LastBlockID:        test.MakeBlockIDWithHash(make([]byte, tmhash.Size)),
			LastCommitHash:     tmhash.Sum([]byte("last_commit_hash")),
			DataHash:           tmhash.Sum([]byte("data_hash")),
			ValidatorsHash:     nil,
			NextValidatorsHash: tmhash.Sum([]byte("next_validators_hash")),
			ConsensusHash:      tmhash.Sum([]byte("consensus_hash")),
			AppHash:            tmhash.Sum([]byte("app_hash")),
			LastResultsHash:    tmhash.Sum([]byte("last_results_hash")),
			EvidenceHash:       tmhash.Sum([]byte("evidence_hash")),
			ProposerAddress:    crypto.AddressHash([]byte("proposer_address")),
		}, nil},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.expectHash, tc.header.Hash())

			// We also make sure that all fields are hashed in struct order, and that all
			// fields in the test struct are non-zero.
			if tc.header != nil && tc.expectHash != nil {
				byteSlices := [][]byte{}

				s := reflect.ValueOf(*tc.header)
				for i := 0; i < s.NumField(); i++ {
					f := s.Field(i)

					assert.False(t, f.IsZero(), "Found zero-valued field %v",
						s.Type().Field(i).Name)

					switch f := f.Interface().(type) {
					case int64, bytes.HexBytes, string:
						byteSlices = append(byteSlices, meta.CdcEncode(f))
					case time.Time:
						bz, err := gogotypes.StdTimeMarshal(f)
						require.NoError(t, err)
						byteSlices = append(byteSlices, bz)
					case version.Consensus:
						pbc := tmversion.Consensus{
							Block: f.Block,
							App:   f.App,
						}
						bz, err := pbc.Marshal()
						require.NoError(t, err)
						byteSlices = append(byteSlices, bz)
					case meta.BlockID:
						pbbi := f.ToProto()
						bz, err := pbbi.Marshal()
						require.NoError(t, err)
						byteSlices = append(byteSlices, bz)
					default:
						t.Errorf("unknown type %T", f)
					}
				}
				assert.Equal(t,
					bytes.HexBytes(merkle.HashFromByteSlices(byteSlices)), tc.header.Hash())
			}
		})
	}
}

func TestMaxHeaderBytes(t *testing.T) {
	// Construct a UTF-8 string of MaxChainIDLen length using the supplementary
	// characters.
	// Each supplementary character takes 4 bytes.
	// http://www.i18nguy.com/unicode/supplementary-test.html
	maxChainID := ""
	for i := 0; i < meta.MaxChainIDLen; i++ {
		maxChainID += "𠜎"
	}

	// time is varint encoded so need to pick the max.
	// year int, month Month, day, hour, min, sec, nsec int, loc *Location
	timestamp := time.Date(math.MaxInt64, 0, 0, 0, 0, 0, math.MaxInt64, time.UTC)

	h := meta.Header{
		Version:            version.Consensus{Block: math.MaxInt64, App: math.MaxInt64},
		ChainID:            maxChainID,
		Height:             math.MaxInt64,
		Time:               timestamp,
		LastBlockID:        meta.BlockID{make([]byte, tmhash.Size), meta.PartSetHeader{math.MaxInt32, make([]byte, tmhash.Size)}},
		LastCommitHash:     tmhash.Sum([]byte("last_commit_hash")),
		DataHash:           tmhash.Sum([]byte("data_hash")),
		ValidatorsHash:     tmhash.Sum([]byte("validators_hash")),
		NextValidatorsHash: tmhash.Sum([]byte("next_validators_hash")),
		ConsensusHash:      tmhash.Sum([]byte("consensus_hash")),
		AppHash:            tmhash.Sum([]byte("app_hash")),
		LastResultsHash:    tmhash.Sum([]byte("last_results_hash")),
		EvidenceHash:       tmhash.Sum([]byte("evidence_hash")),
		ProposerAddress:    crypto.AddressHash([]byte("proposer_address")),
	}

	bz, err := h.ToProto().Marshal()
	require.NoError(t, err)

	assert.EqualValues(t, meta.MaxHeaderBytes, int64(len(bz)))
}

func randCommit(now time.Time) *meta.Commit {
	lastID := test.MakeBlockID()
	h := int64(3)
	voteSet, _, vals := test.RandVoteSet(h-1, 1, tmproto.PrecommitType, 10, 1)
	commit, err := test.MakeCommit(lastID, h-1, 1, voteSet, vals, now)
	if err != nil {
		panic(err)
	}
	return commit
}

func hexBytesFromString(s string) bytes.HexBytes {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return bytes.HexBytes(b)
}

func TestHeader_ValidateBasic(t *testing.T) {
	testCases := []struct {
		name      string
		header    meta.Header
		expectErr bool
		errString string
	}{
		{
			"invalid version block",
			meta.Header{Version: version.Consensus{Block: version.BlockProtocol + 1}},
			true, "block protocol is incorrect",
		},
		{
			"invalid chain ID length",
			meta.Header{
				Version: version.Consensus{Block: version.BlockProtocol},
				ChainID: string(make([]byte, meta.MaxChainIDLen+1)),
			},
			true, "chainID is too long",
		},
		{
			"invalid height (negative)",
			meta.Header{
				Version: version.Consensus{Block: version.BlockProtocol},
				ChainID: string(make([]byte, meta.MaxChainIDLen)),
				Height:  -1,
			},
			true, "negative Height",
		},
		{
			"invalid height (zero)",
			meta.Header{
				Version: version.Consensus{Block: version.BlockProtocol},
				ChainID: string(make([]byte, meta.MaxChainIDLen)),
				Height:  0,
			},
			true, "zero Height",
		},
		{
			"invalid block ID hash",
			meta.Header{
				Version: version.Consensus{Block: version.BlockProtocol},
				ChainID: string(make([]byte, meta.MaxChainIDLen)),
				Height:  1,
				LastBlockID: meta.BlockID{
					Hash: make([]byte, tmhash.Size+1),
				},
			},
			true, "wrong Hash",
		},
		{
			"invalid block ID parts header hash",
			meta.Header{
				Version: version.Consensus{Block: version.BlockProtocol},
				ChainID: string(make([]byte, meta.MaxChainIDLen)),
				Height:  1,
				LastBlockID: meta.BlockID{
					Hash: make([]byte, tmhash.Size),
					PartSetHeader: meta.PartSetHeader{
						Hash: make([]byte, tmhash.Size+1),
					},
				},
			},
			true, "wrong PartSetHeader",
		},
		{
			"invalid last commit hash",
			meta.Header{
				Version: version.Consensus{Block: version.BlockProtocol},
				ChainID: string(make([]byte, meta.MaxChainIDLen)),
				Height:  1,
				LastBlockID: meta.BlockID{
					Hash: make([]byte, tmhash.Size),
					PartSetHeader: meta.PartSetHeader{
						Hash: make([]byte, tmhash.Size),
					},
				},
				LastCommitHash: make([]byte, tmhash.Size+1),
			},
			true, "wrong LastCommitHash",
		},
		{
			"invalid data hash",
			meta.Header{
				Version: version.Consensus{Block: version.BlockProtocol},
				ChainID: string(make([]byte, meta.MaxChainIDLen)),
				Height:  1,
				LastBlockID: meta.BlockID{
					Hash: make([]byte, tmhash.Size),
					PartSetHeader: meta.PartSetHeader{
						Hash: make([]byte, tmhash.Size),
					},
				},
				LastCommitHash: make([]byte, tmhash.Size),
				DataHash:       make([]byte, tmhash.Size+1),
			},
			true, "wrong DataHash",
		},
		{
			"invalid evidence hash",
			meta.Header{
				Version: version.Consensus{Block: version.BlockProtocol},
				ChainID: string(make([]byte, meta.MaxChainIDLen)),
				Height:  1,
				LastBlockID: meta.BlockID{
					Hash: make([]byte, tmhash.Size),
					PartSetHeader: meta.PartSetHeader{
						Hash: make([]byte, tmhash.Size),
					},
				},
				LastCommitHash: make([]byte, tmhash.Size),
				DataHash:       make([]byte, tmhash.Size),
				EvidenceHash:   make([]byte, tmhash.Size+1),
			},
			true, "wrong EvidenceHash",
		},
		{
			"invalid proposer address",
			meta.Header{
				Version: version.Consensus{Block: version.BlockProtocol},
				ChainID: string(make([]byte, meta.MaxChainIDLen)),
				Height:  1,
				LastBlockID: meta.BlockID{
					Hash: make([]byte, tmhash.Size),
					PartSetHeader: meta.PartSetHeader{
						Hash: make([]byte, tmhash.Size),
					},
				},
				LastCommitHash:  make([]byte, tmhash.Size),
				DataHash:        make([]byte, tmhash.Size),
				EvidenceHash:    make([]byte, tmhash.Size),
				ProposerAddress: make([]byte, crypto.AddressSize+1),
			},
			true, "invalid ProposerAddress length",
		},
		{
			"invalid validator hash",
			meta.Header{
				Version: version.Consensus{Block: version.BlockProtocol},
				ChainID: string(make([]byte, meta.MaxChainIDLen)),
				Height:  1,
				LastBlockID: meta.BlockID{
					Hash: make([]byte, tmhash.Size),
					PartSetHeader: meta.PartSetHeader{
						Hash: make([]byte, tmhash.Size),
					},
				},
				LastCommitHash:  make([]byte, tmhash.Size),
				DataHash:        make([]byte, tmhash.Size),
				EvidenceHash:    make([]byte, tmhash.Size),
				ProposerAddress: make([]byte, crypto.AddressSize),
				ValidatorsHash:  make([]byte, tmhash.Size+1),
			},
			true, "wrong ValidatorsHash",
		},
		{
			"invalid next validator hash",
			meta.Header{
				Version: version.Consensus{Block: version.BlockProtocol},
				ChainID: string(make([]byte, meta.MaxChainIDLen)),
				Height:  1,
				LastBlockID: meta.BlockID{
					Hash: make([]byte, tmhash.Size),
					PartSetHeader: meta.PartSetHeader{
						Hash: make([]byte, tmhash.Size),
					},
				},
				LastCommitHash:     make([]byte, tmhash.Size),
				DataHash:           make([]byte, tmhash.Size),
				EvidenceHash:       make([]byte, tmhash.Size),
				ProposerAddress:    make([]byte, crypto.AddressSize),
				ValidatorsHash:     make([]byte, tmhash.Size),
				NextValidatorsHash: make([]byte, tmhash.Size+1),
			},
			true, "wrong NextValidatorsHash",
		},
		{
			"invalid consensus hash",
			meta.Header{
				Version: version.Consensus{Block: version.BlockProtocol},
				ChainID: string(make([]byte, meta.MaxChainIDLen)),
				Height:  1,
				LastBlockID: meta.BlockID{
					Hash: make([]byte, tmhash.Size),
					PartSetHeader: meta.PartSetHeader{
						Hash: make([]byte, tmhash.Size),
					},
				},
				LastCommitHash:     make([]byte, tmhash.Size),
				DataHash:           make([]byte, tmhash.Size),
				EvidenceHash:       make([]byte, tmhash.Size),
				ProposerAddress:    make([]byte, crypto.AddressSize),
				ValidatorsHash:     make([]byte, tmhash.Size),
				NextValidatorsHash: make([]byte, tmhash.Size),
				ConsensusHash:      make([]byte, tmhash.Size+1),
			},
			true, "wrong ConsensusHash",
		},
		{
			"invalid last results hash",
			meta.Header{
				Version: version.Consensus{Block: version.BlockProtocol},
				ChainID: string(make([]byte, meta.MaxChainIDLen)),
				Height:  1,
				LastBlockID: meta.BlockID{
					Hash: make([]byte, tmhash.Size),
					PartSetHeader: meta.PartSetHeader{
						Hash: make([]byte, tmhash.Size),
					},
				},
				LastCommitHash:     make([]byte, tmhash.Size),
				DataHash:           make([]byte, tmhash.Size),
				EvidenceHash:       make([]byte, tmhash.Size),
				ProposerAddress:    make([]byte, crypto.AddressSize),
				ValidatorsHash:     make([]byte, tmhash.Size),
				NextValidatorsHash: make([]byte, tmhash.Size),
				ConsensusHash:      make([]byte, tmhash.Size),
				LastResultsHash:    make([]byte, tmhash.Size+1),
			},
			true, "wrong LastResultsHash",
		},
		{
			"valid header",
			meta.Header{
				Version: version.Consensus{Block: version.BlockProtocol},
				ChainID: string(make([]byte, meta.MaxChainIDLen)),
				Height:  1,
				LastBlockID: meta.BlockID{
					Hash: make([]byte, tmhash.Size),
					PartSetHeader: meta.PartSetHeader{
						Hash: make([]byte, tmhash.Size),
					},
				},
				LastCommitHash:     make([]byte, tmhash.Size),
				DataHash:           make([]byte, tmhash.Size),
				EvidenceHash:       make([]byte, tmhash.Size),
				ProposerAddress:    make([]byte, crypto.AddressSize),
				ValidatorsHash:     make([]byte, tmhash.Size),
				NextValidatorsHash: make([]byte, tmhash.Size),
				ConsensusHash:      make([]byte, tmhash.Size),
				LastResultsHash:    make([]byte, tmhash.Size),
			},
			false, "",
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			err := tc.header.ValidateBasic()
			if tc.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errString)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestHeaderProto(t *testing.T) {
	h1 := test.MakeRandomHeader()
	tc := []struct {
		msg     string
		h1      *meta.Header
		expPass bool
	}{
		{"success", h1, true},
		{"failure empty Header", &meta.Header{}, false},
	}

	for _, tt := range tc {
		tt := tt
		t.Run(tt.msg, func(t *testing.T) {
			pb := tt.h1.ToProto()
			h, err := meta.HeaderFromProto(pb)
			if tt.expPass {
				require.NoError(t, err, tt.msg)
				require.Equal(t, tt.h1, &h, tt.msg)
			} else {
				require.Error(t, err, tt.msg)
			}

		})
	}
}

func TestHeaderHashVector(t *testing.T) {
	chainID := "test"
	h := meta.Header{
		Version:            version.Consensus{Block: 1, App: 1},
		ChainID:            chainID,
		Height:             50,
		Time:               time.Date(math.MaxInt64, 0, 0, 0, 0, 0, math.MaxInt64, time.UTC),
		LastBlockID:        meta.BlockID{},
		LastCommitHash:     []byte("f2564c78071e26643ae9b3e2a19fa0dc10d4d9e873aa0be808660123f11a1e78"),
		DataHash:           []byte("f2564c78071e26643ae9b3e2a19fa0dc10d4d9e873aa0be808660123f11a1e78"),
		ValidatorsHash:     []byte("f2564c78071e26643ae9b3e2a19fa0dc10d4d9e873aa0be808660123f11a1e78"),
		NextValidatorsHash: []byte("f2564c78071e26643ae9b3e2a19fa0dc10d4d9e873aa0be808660123f11a1e78"),
		ConsensusHash:      []byte("f2564c78071e26643ae9b3e2a19fa0dc10d4d9e873aa0be808660123f11a1e78"),
		AppHash:            []byte("f2564c78071e26643ae9b3e2a19fa0dc10d4d9e873aa0be808660123f11a1e78"),

		LastResultsHash: []byte("f2564c78071e26643ae9b3e2a19fa0dc10d4d9e873aa0be808660123f11a1e78"),

		EvidenceHash:    []byte("f2564c78071e26643ae9b3e2a19fa0dc10d4d9e873aa0be808660123f11a1e78"),
		ProposerAddress: []byte("2915b7b15f979e48ebc61774bb1d86ba3136b7eb"),
	}

	testCases := []struct {
		header   meta.Header
		expBytes string
	}{
		{header: h, expBytes: "87b6117ac7f827d656f178a3d6d30b24b205db2b6a3a053bae8baf4618570bfc"},
	}

	for _, tc := range testCases {
		hash := tc.header.Hash()
		require.Equal(t, tc.expBytes, hex.EncodeToString(hash))
	}
}
