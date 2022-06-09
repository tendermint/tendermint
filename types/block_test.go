package types

import (
	// it is ok to use math/rand here: we do not need a cryptographically secure random
	// number generator here and we can run the tests a bit faster
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	mrand "math/rand"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/dashevo/dashd-go/btcjson"
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	"github.com/tendermint/tendermint/crypto/merkle"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
	"github.com/tendermint/tendermint/version"
)

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}

func TestBlockAddEvidence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	txs := []Tx{Tx("foo"), Tx("bar")}
	lastID := makeBlockIDRandom()

	h := int64(3)
	stateID := RandStateID().WithHeight(h - 2)

	coreChainLock := NewMockChainLock(1)

	voteSet, valSet, vals := randVoteSet(ctx, t, h-1, 1, tmproto.PrecommitType, 10, stateID)
	commit, err := makeCommit(ctx, lastID, stateID, h-1, 1, voteSet, vals)
	require.NoError(t, err)

	ev, err := NewMockDuplicateVoteEvidenceWithValidator(ctx, h, time.Now(), vals[0], "block-test-chain", valSet.QuorumType,
		valSet.QuorumHash)
	require.NoError(t, err)
	evList := []Evidence{ev}

	block := MakeBlock(h, coreChainLock.CoreBlockHeight, &coreChainLock, txs, commit, evList, 0)
	require.NotNil(t, block)
	require.Equal(t, 1, len(block.Evidence))
	require.NotNil(t, block.EvidenceHash)
}

func TestBlockValidateBasic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	require.Error(t, (*Block)(nil).ValidateBasic())

	txs := []Tx{Tx("foo"), Tx("bar")}
	lastID := makeBlockIDRandom()
	h := int64(3)

	stateID := RandStateID().WithHeight(h - 2)

	voteSet, valSet, vals := randVoteSet(ctx, t, h-1, 1, tmproto.PrecommitType, 10, stateID)
	commit, err := makeCommit(ctx, lastID, stateID, h-1, 1, voteSet, vals)
	require.NoError(t, err)

	ev, err := NewMockDuplicateVoteEvidenceWithValidator(ctx, h, time.Now(), vals[0], "block-test-chain", valSet.QuorumType,
		valSet.QuorumHash)
	require.NoError(t, err)
	evList := []Evidence{ev}

	testCases := []struct {
		testName      string
		malleateBlock func(*Block)
		expErr        bool
	}{
		{"Make Block", func(blk *Block) {}, false},
		{"Make Block w/ proposer pro_tx_hash", func(blk *Block) {
			blk.ProposerProTxHash =
				valSet.GetProposer().ProTxHash
		}, false},
		{"Negative Height", func(blk *Block) { blk.Height = -1 }, true},
		{"Modify the last Commit", func(blk *Block) {
			blk.LastCommit.BlockID = BlockID{}
			blk.LastCommit.hash = nil // clear hash or change wont be noticed
		}, true},
		{"Remove LastCommitHash", func(blk *Block) {
			blk.LastCommitHash =
				[]byte("something else")
		}, true},
		{"Tampered Data", func(blk *Block) {
			blk.Data.Txs[0] = Tx("something else")
			blk.Data.hash = nil // clear hash or change wont be noticed
		}, true},
		{"Tampered DataHash", func(blk *Block) {
			blk.DataHash = tmrand.Bytes(len(blk.DataHash))
		}, true},
		{"Tampered EvidenceHash", func(blk *Block) {
			blk.EvidenceHash = tmrand.Bytes(len(blk.EvidenceHash))
		}, true},
		{"Incorrect block protocol version", func(blk *Block) {
			blk.Version.Block = 1
		}, true},
		{"Missing LastCommit", func(blk *Block) {
			blk.LastCommit = nil
		}, true},
		{"Invalid LastCommit", func(blk *Block) {
			blk.LastCommit = NewCommit(-1, 0, *voteSet.maj23, StateID{}, nil, nil, nil)
		}, true},
		{"Invalid Evidence", func(blk *Block) {
			emptyEv := &DuplicateVoteEvidence{}
			blk.Evidence = []Evidence{emptyEv}
		}, true},
	}

	for i, tc := range testCases {
		tcRun := tc
		j := i
		t.Run(tcRun.testName, func(t *testing.T) {
			block := MakeBlock(h, 0, nil, txs, commit, evList, 0)
			block.ProposerProTxHash = valSet.GetProposer().ProTxHash
			tcRun.malleateBlock(block)
			err = block.ValidateBasic()
			assert.Equal(t, tcRun.expErr, err != nil, "#%d: %v", j, err)
		})
	}
}

func TestBlockHash(t *testing.T) {
	assert.Nil(t, (*Block)(nil).Hash())
	assert.Nil(t, MakeBlock(int64(3), 0, nil, []Tx{Tx("Hello World")}, nil, nil, 0).Hash())
}

func TestBlockMakePartSet(t *testing.T) {
	bps, err := (*Block)(nil).MakePartSet(2)
	assert.Error(t, err)
	assert.Nil(t, bps)

	partSet, err := MakeBlock(int64(3), 0, nil, []Tx{Tx("Hello World")}, nil, nil, 0).MakePartSet(1024)
	require.NoError(t, err)

	assert.NotNil(t, partSet)
	assert.EqualValues(t, 1, partSet.Total())
}

func TestBlockMakePartSetWithEvidence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bps, err := (*Block)(nil).MakePartSet(2)
	assert.Error(t, err)
	assert.Nil(t, bps)

	lastID := makeBlockIDRandom()
	h := int64(3)
	stateID := RandStateID().WithHeight(h - 2)

	voteSet, valSet, vals := randVoteSet(ctx, t, h-1, 1, tmproto.PrecommitType, 10, stateID)
	commit, err := makeCommit(ctx, lastID, stateID, h-1, 1, voteSet, vals)
	require.NoError(t, err)

	ev, err := NewMockDuplicateVoteEvidenceWithValidator(ctx, h, time.Now(), vals[0], "block-test-chain", valSet.QuorumType,
		valSet.QuorumHash)
	require.NoError(t, err)
	evList := []Evidence{ev}

	partSet, err := MakeBlock(h, 0, nil, []Tx{Tx("Hello World")}, commit, evList, 0).MakePartSet(512)
	require.NoError(t, err)

	// The part set can be either 3 or 4 parts, this is because of variance in sizes due to the non second part of
	// timestamps marshaling to different sizes
	assert.True(t, partSet.Total() == 3)
}

func TestBlockHashesTo(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	assert.False(t, (*Block)(nil).HashesTo(nil))

	lastID := makeBlockIDRandom()
	h := int64(3)
	stateID := RandStateID().WithHeight(h - 2)

	voteSet, valSet, vals := randVoteSet(ctx, t, h-1, 1, tmproto.PrecommitType, 10, stateID)
	commit, err := makeCommit(ctx, lastID, stateID, h-1, 1, voteSet, vals)
	require.NoError(t, err)

	ev, err := NewMockDuplicateVoteEvidenceWithValidator(ctx, h, time.Now(), vals[0], "block-test-chain", valSet.QuorumType,
		valSet.QuorumHash)
	require.NoError(t, err)
	evList := []Evidence{ev}

	block := MakeBlock(h, 0, nil, []Tx{Tx("Hello World")}, commit, evList, 0)
	block.ValidatorsHash = valSet.Hash()
	assert.False(t, block.HashesTo([]byte{}))
	assert.False(t, block.HashesTo([]byte("something else")))
	assert.True(t, block.HashesTo(block.Hash()))
}

func TestBlockSize(t *testing.T) {
	size := MakeBlock(int64(3), 0, nil, []Tx{Tx("Hello World")}, nil, nil, 0).Size()
	if size <= 0 {
		t.Fatal("Size of the block is zero or negative")
	}
}

func TestBlockString(t *testing.T) {
	assert.Equal(t, "nil-Block", (*Block)(nil).String())
	assert.Equal(t, "nil-Block", (*Block)(nil).StringIndented(""))
	assert.Equal(t, "nil-Block", (*Block)(nil).StringShort())

	block := MakeBlock(int64(3), 0, nil, []Tx{Tx("Hello World")}, nil, nil, 0)
	assert.NotEqual(t, "nil-Block", block.String())
	assert.NotEqual(t, "nil-Block", block.StringIndented(""))
	assert.NotEqual(t, "nil-Block", block.StringShort())
}

func makeBlockIDRandom() BlockID {
	var (
		blockHash   = make([]byte, crypto.HashSize)
		partSetHash = make([]byte, crypto.HashSize)
	)
	rand.Read(blockHash)   //nolint: errcheck // ignore errcheck for read
	rand.Read(partSetHash) //nolint: errcheck // ignore errcheck for read
	return BlockID{blockHash, PartSetHeader{123, partSetHash}}
}

func makeBlockID(hash []byte, partSetSize uint32, partSetHash []byte) BlockID {
	var (
		h   = make([]byte, crypto.HashSize)
		psH = make([]byte, crypto.HashSize)
	)
	copy(h, hash)
	copy(psH, partSetHash)
	return BlockID{
		Hash: h,
		PartSetHeader: PartSetHeader{
			Total: partSetSize,
			Hash:  psH,
		},
	}
}

var nilBytes []byte

// This follows RFC-6962, i.e. `echo -n '' | sha256sum`
var emptyBytes = []byte{0xe3, 0xb0, 0xc4, 0x42, 0x98, 0xfc, 0x1c, 0x14, 0x9a, 0xfb, 0xf4, 0xc8,
	0x99, 0x6f, 0xb9, 0x24, 0x27, 0xae, 0x41, 0xe4, 0x64, 0x9b, 0x93, 0x4c, 0xa4, 0x95, 0x99, 0x1b,
	0x78, 0x52, 0xb8, 0x55}

func TestNilHeaderHashDoesntCrash(t *testing.T) {
	assert.Equal(t, nilBytes, []byte((*Header)(nil).Hash()))
	assert.Equal(t, nilBytes, []byte((new(Header)).Hash()))
}

func TestNilDataHashDoesntCrash(t *testing.T) {
	assert.Equal(t, emptyBytes, []byte((*Data)(nil).Hash()))
	assert.Equal(t, emptyBytes, []byte(new(Data).Hash()))
}

func TestCommit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lastID := makeBlockIDRandom()
	h := int64(3)
	stateID := RandStateID().WithHeight(h - 2)

	voteSet, _, vals := randVoteSet(ctx, t, h-1, 1, tmproto.PrecommitType, 10, stateID)
	commit, err := makeCommit(ctx, lastID, stateID, h-1, 1, voteSet, vals)
	require.NoError(t, err)

	assert.Equal(t, h-1, commit.Height)
	assert.EqualValues(t, 1, commit.Round)
	assert.Equal(t, tmproto.PrecommitType, tmproto.SignedMsgType(commit.Type()))

	require.NotNil(t, commit.ThresholdBlockSignature)
	require.NotNil(t, commit.ThresholdStateSignature)
	// TODO replace an assertion with a correct one
	//assert.Equal(t, voteWithoutExtension(voteSet.GetByIndex(0)), commit.GetByIndex(0))
	assert.True(t, commit.IsCommit())
}

func TestCommitValidateBasic(t *testing.T) {
	const height int64 = 5
	testCases := []struct {
		testName       string
		malleateCommit func(*Commit)
		expectErr      bool
	}{
		{"Random Commit", func(com *Commit) {}, false},
		{"Incorrect block signature", func(com *Commit) { com.ThresholdBlockSignature = []byte{0} }, true},
		{"Incorrect state signature", func(com *Commit) { com.ThresholdStateSignature = []byte{0} }, true},
		{"Incorrect height", func(com *Commit) { com.Height = int64(-100) }, true},
		{"Incorrect round", func(com *Commit) { com.Round = -100 }, true},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			stateID := RandStateID().WithHeight(height - 1)

			com := randCommit(ctx, t, stateID)

			tc.malleateCommit(com)
			assert.Equal(t, tc.expectErr, com.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestMaxCommitBytes(t *testing.T) {
	// check size with a single commit
	commit := &Commit{
		Height: math.MaxInt64,
		Round:  math.MaxInt32,
		BlockID: BlockID{
			Hash: crypto.Checksum([]byte("blockID_hash")),
			PartSetHeader: PartSetHeader{
				Total: math.MaxInt32,
				Hash:  crypto.Checksum([]byte("blockID_part_set_header_hash")),
			},
		},
		StateID: StateID{
			LastAppHash: crypto.Checksum([]byte("stateID_hash")),
		},
		ThresholdBlockSignature: crypto.CRandBytes(SignatureSize),
		ThresholdStateSignature: crypto.CRandBytes(SignatureSize),
	}

	pb := commit.ToProto()
	pbSize := int64(pb.Size())
	assert.EqualValues(t, MaxCommitOverheadBytes, pbSize)

	pb = commit.ToProto()

	assert.EqualValues(t, MaxCommitOverheadBytes, int64(pb.Size()))
}

func TestHeaderHash(t *testing.T) {
	testCases := []struct {
		desc       string
		header     *Header
		expectHash tmbytes.HexBytes
	}{
		{"Generates expected hash", &Header{
			Version:               version.Consensus{Block: 1, App: 2},
			ChainID:               "chainId",
			Height:                3,
			CoreChainLockedHeight: 1,
			Time:                  time.Date(2019, 10, 13, 16, 14, 44, 0, time.UTC),
			LastBlockID:           makeBlockID(make([]byte, crypto.HashSize), 6, make([]byte, crypto.HashSize)),
			LastCommitHash:        crypto.Checksum([]byte("last_commit_hash")),
			DataHash:              crypto.Checksum([]byte("data_hash")),
			ValidatorsHash:        crypto.Checksum([]byte("validators_hash")),
			NextValidatorsHash:    crypto.Checksum([]byte("next_validators_hash")),
			ConsensusHash:         crypto.Checksum([]byte("consensus_hash")),
			AppHash:               crypto.Checksum([]byte("app_hash")),
			LastResultsHash:       crypto.Checksum([]byte("last_results_hash")),
			EvidenceHash:          crypto.Checksum([]byte("evidence_hash")),
			ProposerProTxHash:     crypto.ProTxHashFromSeedBytes([]byte("proposer_pro_tx_hash")),
			ProposedAppVersion:    1,
		}, hexBytesFromString(t, "74EEFDA2F09ACE19D46DE191EC2745CE14B42F7DE48AF86E6D65B17939B08D3E")},
		{"nil header yields nil", nil, nil},
		{"nil ValidatorsHash yields nil", &Header{
			Version:               version.Consensus{Block: 1, App: 2},
			ChainID:               "chainId",
			Height:                3,
			CoreChainLockedHeight: 1,
			Time:                  time.Date(2019, 10, 13, 16, 14, 44, 0, time.UTC),
			LastBlockID:           makeBlockID(make([]byte, crypto.HashSize), 6, make([]byte, crypto.HashSize)),
			LastCommitHash:        crypto.Checksum([]byte("last_commit_hash")),
			DataHash:              crypto.Checksum([]byte("data_hash")),
			ValidatorsHash:        nil,
			NextValidatorsHash:    crypto.Checksum([]byte("next_validators_hash")),
			ConsensusHash:         crypto.Checksum([]byte("consensus_hash")),
			AppHash:               crypto.Checksum([]byte("app_hash")),
			LastResultsHash:       crypto.Checksum([]byte("last_results_hash")),
			EvidenceHash:          crypto.Checksum([]byte("evidence_hash")),
			ProposerProTxHash:     crypto.ProTxHashFromSeedBytes([]byte("proposer_pro_tx_hash")),
			ProposedAppVersion:    1,
		}, nil},
	}
	for _, tc := range testCases {
		tcRun := tc
		t.Run(tcRun.desc, func(t *testing.T) {
			assert.Equal(t, tcRun.expectHash, tcRun.header.Hash())

			// We also make sure that all fields are hashed in struct order, and that all
			// fields in the test struct are non-zero.
			if tcRun.header != nil && tcRun.expectHash != nil {
				byteSlices := [][]byte{}

				s := reflect.ValueOf(*tcRun.header)
				for i := 0; i < s.NumField(); i++ {
					f := s.Field(i)

					assert.False(t, f.IsZero(), "Found zero-valued field %v",
						s.Type().Field(i).Name)

					switch f := f.Interface().(type) {
					case int64, uint32, uint64, tmbytes.HexBytes, string:
						byteSlices = append(byteSlices, cdcEncode(f))
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
					case BlockID:
						pbbi := f.ToProto()
						bz, err := pbbi.Marshal()
						require.NoError(t, err)
						byteSlices = append(byteSlices, bz)
					default:
						t.Errorf("unknown type %T", f)
					}
				}
				assert.Equal(t,
					tmbytes.HexBytes(merkle.HashFromByteSlices(byteSlices)), tcRun.header.Hash())
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
	for i := 0; i < MaxChainIDLen; i++ {
		maxChainID += "𠜎"
	}

	// time is varint encoded so need to pick the max.
	// year int, month Month, day, hour, min, sec, nsec int, loc *Location
	timestamp := time.Date(math.MaxInt64, 0, 0, 0, 0, 0, math.MaxInt64, time.UTC)

	h := Header{
		Version:            version.Consensus{Block: math.MaxInt64, App: math.MaxInt64},
		ChainID:            maxChainID,
		Height:             math.MaxInt64,
		Time:               timestamp,
		LastBlockID:        makeBlockID(make([]byte, crypto.HashSize), math.MaxInt32, make([]byte, crypto.HashSize)),
		LastCommitHash:     crypto.Checksum([]byte("last_commit_hash")),
		DataHash:           crypto.Checksum([]byte("data_hash")),
		ValidatorsHash:     crypto.Checksum([]byte("validators_hash")),
		NextValidatorsHash: crypto.Checksum([]byte("next_validators_hash")),
		ConsensusHash:      crypto.Checksum([]byte("consensus_hash")),
		AppHash:            crypto.Checksum([]byte("app_hash")),
		LastResultsHash:    crypto.Checksum([]byte("last_results_hash")),
		EvidenceHash:       crypto.Checksum([]byte("evidence_hash")),

		CoreChainLockedHeight: math.MaxUint32,
		ProposerProTxHash:     crypto.ProTxHashFromSeedBytes([]byte("proposer_pro_tx_hash")),
	}

	bz, err := h.ToProto().Marshal()
	require.NoError(t, err)

	assert.EqualValues(t, MaxHeaderBytes, int64(len(bz)))
}

func randCommit(ctx context.Context, t *testing.T, stateID StateID) *Commit {
	t.Helper()
	lastID := makeBlockIDRandom()
	height := stateID.Height + 1
	voteSet, _, vals := randVoteSet(ctx, t, height, 1, tmproto.PrecommitType, 10, stateID)
	commit, err := makeCommit(ctx, lastID, stateID, height, 1, voteSet, vals)

	require.NoError(t, err)

	return commit
}

func hexBytesFromString(t *testing.T, s string) tmbytes.HexBytes {
	t.Helper()

	b, err := hex.DecodeString(s)
	require.NoError(t, err)

	return b
}

func TestBlockMaxDataBytes(t *testing.T) {
	testCases := []struct {
		maxBytes      int64
		keyType       crypto.KeyType
		valsCount     int
		evidenceBytes int64
		panics        bool
		result        int64
	}{
		0: {-10, crypto.BLS12381, 1, 0, true, 0},
		1: {10, crypto.BLS12381, 1, 0, true, 0},
		2: {1114, crypto.BLS12381, 1, 0, true, 0},
		3: {1118, crypto.BLS12381, 1, 0, false, 0},
		4: {1119, crypto.BLS12381, 1, 0, false, 1},
		5: {1119, crypto.BLS12381, 2, 0, false, 1},
		6: {1218, crypto.BLS12381, 2, 100, false, 0},
	}
	// An extra 33 bytes (32 for sig, 1 for proto encoding are needed for BLS compared to edwards per validator

	for i, tc := range testCases {
		tcRun := tc
		j := i
		t.Run(fmt.Sprintf("%d", tcRun.maxBytes), func(t *testing.T) {
			if tcRun.panics {
				assert.Panics(t, func() {
					MaxDataBytes(tcRun.maxBytes, tcRun.keyType, tcRun.evidenceBytes, tcRun.valsCount)
				}, "#%v", j)
			} else {
				assert.Equal(t,
					tcRun.result,
					MaxDataBytes(tcRun.maxBytes, tcRun.keyType, tcRun.evidenceBytes, tcRun.valsCount),
					"#%v", j)
			}
		})
	}
}

func TestBlockMaxDataBytesNoEvidence(t *testing.T) {
	testCases := []struct {
		maxBytes    int64
		maxEvidence uint32
		keyType     crypto.KeyType
		valsCount   int
		panics      bool
		result      int64
	}{
		0: {-10, 1, crypto.BLS12381, 1, true, 0},
		1: {10, 1, crypto.BLS12381, 1, true, 0},
		2: {1114, 1, crypto.BLS12381, 1, true, 0},
		3: {1118, 1, crypto.BLS12381, 1, false, 0},
		4: {1119, 1, crypto.BLS12381, 1, false, 1},
	}

	for i, tc := range testCases {
		tcRun := tc
		j := i
		t.Run(fmt.Sprintf("%d", tcRun.maxBytes), func(t *testing.T) {
			if tcRun.panics {
				assert.Panics(t, func() {
					MaxDataBytesNoEvidence(tcRun.maxBytes)
				}, "#%v", j)
			} else {
				assert.Equal(t,
					tcRun.result,
					MaxDataBytesNoEvidence(tcRun.maxBytes),
					"#%v", j)
			}
		})
	}
}

func TestCommitToVoteSetWithVotesForNilBlock(t *testing.T) {
	blockID := makeBlockID([]byte("blockhash"), 1000, []byte("partshash"))

	const (
		height = int64(3)
		round  = 0
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// all votes below use height - 1, so state is at height - 2
	stateID := RandStateID().WithHeight(height - 2)

	type commitVoteTest struct {
		blockIDs      []BlockID
		numVotes      []int // must sum to numValidators
		numValidators int
		valid         bool
	}

	testCases := []commitVoteTest{
		{[]BlockID{blockID, {}}, []int{67, 33}, 100, true},
	}

	for _, tc := range testCases {
		voteSet, valSet, vals := randVoteSet(ctx, t, height-1, round, tmproto.PrecommitType, tc.numValidators, stateID)

		vi := int32(0)
		for n := range tc.blockIDs {
			for i := 0; i < tc.numVotes[n]; i++ {
				proTxHash, err := vals[vi].GetProTxHash(ctx)
				require.NoError(t, err)
				vote := &Vote{
					ValidatorProTxHash: proTxHash,
					ValidatorIndex:     vi,
					Height:             height - 1,
					Round:              round,
					Type:               tmproto.PrecommitType,
					BlockID:            tc.blockIDs[n],
				}

				added, err := signAddVote(ctx, vals[vi], vote, voteSet)
				assert.NoError(t, err)
				assert.True(t, added)

				vi++
			}
		}

		if tc.valid {
			commit := voteSet.MakeCommit() // panics without > 2/3 valid votes
			assert.NotNil(t, commit)
			err := valSet.VerifyCommit(voteSet.ChainID(), blockID, stateID, height-1, commit)
			assert.NoError(t, err)
		} else {
			assert.Panics(t, func() { voteSet.MakeCommit() })
		}
	}
}

func TestBlockIDValidateBasic(t *testing.T) {
	validBlockID := BlockID{
		Hash: tmbytes.HexBytes{},
		PartSetHeader: PartSetHeader{
			Total: 1,
			Hash:  tmbytes.HexBytes{},
		},
	}

	invalidBlockID := BlockID{
		Hash: []byte{0},
		PartSetHeader: PartSetHeader{
			Total: 1,
			Hash:  []byte{0},
		},
	}

	testCases := []struct {
		testName             string
		blockIDHash          tmbytes.HexBytes
		blockIDPartSetHeader PartSetHeader
		expectErr            bool
	}{
		{"Valid BlockID", validBlockID.Hash, validBlockID.PartSetHeader, false},
		{"Invalid BlockID", invalidBlockID.Hash, validBlockID.PartSetHeader, true},
		{"Invalid BlockID", validBlockID.Hash, invalidBlockID.PartSetHeader, true},
	}

	for _, tc := range testCases {
		tcRun := tc
		t.Run(tcRun.testName, func(t *testing.T) {
			blockID := BlockID{
				Hash:          tcRun.blockIDHash,
				PartSetHeader: tcRun.blockIDPartSetHeader,
			}
			assert.Equal(t, tcRun.expectErr, blockID.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestBlockProtoBuf(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	h := mrand.Int63()
	stateID := RandStateID().WithHeight(h - 1)
	c1 := randCommit(ctx, t, stateID)
	b1 := MakeBlock(h, 0, nil, []Tx{Tx([]byte{1})}, &Commit{}, []Evidence{}, 0)
	b1.ProposerProTxHash = tmrand.Bytes(crypto.DefaultHashSize)

	b2 := MakeBlock(h, 0, nil, []Tx{Tx([]byte{1})}, c1, []Evidence{}, 0)
	b2.ProposerProTxHash = tmrand.Bytes(crypto.DefaultHashSize)
	evidenceTime := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	evi, err := NewMockDuplicateVoteEvidence(
		ctx,
		h,
		evidenceTime,
		"block-test-chain",
		btcjson.LLMQType_5_60,
		crypto.RandQuorumHash(),
	)
	require.NoError(t, err)
	b2.Evidence = EvidenceList{evi}
	b2.EvidenceHash = b2.Evidence.Hash()

	b3 := MakeBlock(h, 0, nil, []Tx{}, c1, []Evidence{}, 0)
	b3.ProposerProTxHash = tmrand.Bytes(crypto.DefaultHashSize)
	testCases := []struct {
		msg      string
		b1       *Block
		expPass  bool
		expPass2 bool
	}{
		{"nil block", nil, false, false},
		{"b1", b1, true, true},
		{"b2", b2, true, true},
		{"b3", b3, true, true},
	}
	for _, tc := range testCases {
		pb, err := tc.b1.ToProto()
		if tc.expPass {
			require.NoError(t, err, tc.msg)
		} else {
			require.Error(t, err, tc.msg)
		}

		block, err := BlockFromProto(pb)
		if tc.expPass2 {
			require.NoError(t, err, tc.msg)
			require.EqualValues(t, tc.b1.Header, block.Header, tc.msg)
			require.EqualValues(t, tc.b1.Data, block.Data, tc.msg)
			require.EqualValues(t, tc.b1.Evidence, block.Evidence, tc.msg)
			require.EqualValues(t, *tc.b1.LastCommit, *block.LastCommit, tc.msg)
		} else {
			require.Error(t, err, tc.msg)
		}
	}
}

func TestDataProtoBuf(t *testing.T) {
	data := &Data{Txs: Txs{Tx([]byte{1}), Tx([]byte{2}), Tx([]byte{3})}}
	data2 := &Data{Txs: Txs{}}
	testCases := []struct {
		msg     string
		data1   *Data
		expPass bool
	}{
		{"success", data, true},
		{"success data2", data2, true},
	}
	for _, tc := range testCases {
		protoData := tc.data1.ToProto()
		d, err := DataFromProto(&protoData)
		if tc.expPass {
			require.NoError(t, err, tc.msg)
			require.EqualValues(t, tc.data1, &d, tc.msg)
		} else {
			require.Error(t, err, tc.msg)
		}
	}
}

// exposed for testing
func MakeRandHeader() Header {
	chainID := "test"
	t := time.Now()
	height := mrand.Int63()
	randBytes := tmrand.Bytes(crypto.HashSize)
	randProTxHash := tmrand.Bytes(crypto.DefaultHashSize)
	h := Header{
		Version:            version.Consensus{Block: version.BlockProtocol, App: 1},
		ChainID:            chainID,
		Height:             height,
		Time:               t,
		LastBlockID:        BlockID{},
		LastCommitHash:     randBytes,
		DataHash:           randBytes,
		ValidatorsHash:     randBytes,
		NextValidatorsHash: randBytes,
		ConsensusHash:      randBytes,
		AppHash:            randBytes,

		LastResultsHash: randBytes,

		EvidenceHash:      randBytes,
		ProposerProTxHash: randProTxHash,
	}

	return h
}

func TestHeaderProto(t *testing.T) {
	h1 := MakeRandHeader()
	tc := []struct {
		msg     string
		h1      *Header
		expPass bool
	}{
		{"success", &h1, true},
		{"failure empty Header", &Header{}, false},
	}

	for _, tt := range tc {
		tt := tt
		t.Run(tt.msg, func(t *testing.T) {
			pb := tt.h1.ToProto()
			h, err := HeaderFromProto(pb)
			if tt.expPass {
				require.NoError(t, err, tt.msg)
				require.Equal(t, tt.h1, &h, tt.msg)
			} else {
				require.Error(t, err, tt.msg)
			}

		})
	}
}

func TestBlockIDProtoBuf(t *testing.T) {
	blockID := makeBlockID([]byte("hash"), 2, []byte("part_set_hash"))
	testCases := []struct {
		msg     string
		bid1    *BlockID
		expPass bool
	}{
		{"success", &blockID, true},
		{"success empty", &BlockID{}, true},
		{"failure BlockID nil", nil, false},
	}
	for _, tc := range testCases {
		protoBlockID := tc.bid1.ToProto()

		bi, err := BlockIDFromProto(&protoBlockID)
		if tc.expPass {
			require.NoError(t, err)
			require.Equal(t, tc.bid1, bi, tc.msg)
		} else {
			require.NotEqual(t, tc.bid1, bi, tc.msg)
		}
	}
}

func TestSignedHeaderProtoBuf(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stateID := RandStateID()

	commit := randCommit(ctx, t, stateID)

	h := MakeRandHeader()

	sh := SignedHeader{Header: &h, Commit: commit}

	testCases := []struct {
		msg     string
		sh1     *SignedHeader
		expPass bool
	}{
		{"empty SignedHeader 2", &SignedHeader{}, true},
		{"success", &sh, true},
		{"failure nil", nil, false},
	}
	for _, tc := range testCases {
		protoSignedHeader := tc.sh1.ToProto()

		sh, err := SignedHeaderFromProto(protoSignedHeader)

		if tc.expPass {
			require.NoError(t, err, tc.msg)
			require.Equal(t, tc.sh1, sh, tc.msg)
		} else {
			require.Error(t, err, tc.msg)
		}
	}
}

func TestBlockIDEquals(t *testing.T) {
	var (
		blockID          = makeBlockID([]byte("hash"), 2, []byte("part_set_hash"))
		blockIDDuplicate = makeBlockID([]byte("hash"), 2, []byte("part_set_hash"))
		blockIDDifferent = makeBlockID([]byte("different_hash"), 2, []byte("part_set_hash"))
		blockIDEmpty     = BlockID{}
	)

	assert.True(t, blockID.Equals(blockIDDuplicate))
	assert.False(t, blockID.Equals(blockIDDifferent))
	assert.False(t, blockID.Equals(blockIDEmpty))
	assert.True(t, blockIDEmpty.Equals(blockIDEmpty)) //nolint: gocritic
	assert.False(t, blockIDEmpty.Equals(blockIDDifferent))
}

// StateID tests

// TODO: Move to separate file

func TestStateID_Copy(t *testing.T) {
	state1 := RandStateID()
	state2 := state1.Copy()
	assert.Equal(t, state1, state2)

	state2.LastAppHash[5] = 0x12
	assert.NotEqual(t, state1, state2)
}

func TestStateID_Equals(t *testing.T) {
	tests := []struct {
		state1 StateID
		state2 StateID
		equal  bool
	}{
		{RandStateID(), RandStateID(), false},
		{
			StateID{12, []byte("12345678901234567890123456789012")},
			StateID{12, []byte("12345678901234567890123456789012")},
			true,
		},
		{
			StateID{11, []byte("12345678901234567890123456789012")},
			StateID{12, []byte("12345678901234567890123456789012")},
			false,
		},
		{
			StateID{12, []byte("12345678901234567890123456789012")},
			StateID{12, []byte("1234567890123456789012345678901")},
			false,
		},
	}
	//nolint:scopelint
	for tcID, tc := range tests {
		t.Run(strconv.Itoa(tcID), func(t *testing.T) {
			assert.Equal(t, tc.equal, tc.state1.Equals(tc.state2))
		})
	}
}

func TestHeader_ValidateBasic(t *testing.T) {
	testCases := []struct {
		name      string
		header    Header
		expectErr bool
		errString string
	}{
		{
			"invalid version block",
			Header{Version: version.Consensus{Block: version.BlockProtocol + 1}},
			true, "block protocol is incorrect",
		},
		{
			"invalid chain ID length",
			Header{
				Version: version.Consensus{Block: version.BlockProtocol},
				ChainID: string(make([]byte, MaxChainIDLen+1)),
			},
			true, "chainID is too long",
		},
		{
			"invalid height (negative)",
			Header{
				Version: version.Consensus{Block: version.BlockProtocol},
				ChainID: string(make([]byte, MaxChainIDLen)),
				Height:  -1,
			},
			true, "negative Height",
		},
		{
			"invalid height (zero)",
			Header{
				Version: version.Consensus{Block: version.BlockProtocol},
				ChainID: string(make([]byte, MaxChainIDLen)),
				Height:  0,
			},
			true, "zero Height",
		},
		{
			"invalid block ID hash",
			Header{
				Version: version.Consensus{Block: version.BlockProtocol},
				ChainID: string(make([]byte, MaxChainIDLen)),
				Height:  1,
				LastBlockID: BlockID{
					Hash: make([]byte, crypto.HashSize+1),
				},
			},
			true, "wrong Hash",
		},
		{
			"invalid block ID parts header hash",
			Header{
				Version: version.Consensus{Block: version.BlockProtocol},
				ChainID: string(make([]byte, MaxChainIDLen)),
				Height:  1,
				LastBlockID: BlockID{
					Hash: make([]byte, crypto.HashSize),
					PartSetHeader: PartSetHeader{
						Hash: make([]byte, crypto.HashSize+1),
					},
				},
			},
			true, "wrong PartSetHeader",
		},
		{
			"invalid last commit hash",
			Header{
				Version: version.Consensus{Block: version.BlockProtocol},
				ChainID: string(make([]byte, MaxChainIDLen)),
				Height:  1,
				LastBlockID: BlockID{
					Hash: make([]byte, crypto.HashSize),
					PartSetHeader: PartSetHeader{
						Hash: make([]byte, crypto.HashSize),
					},
				},
				LastCommitHash: make([]byte, crypto.HashSize+1),
			},
			true, "wrong LastCommitHash",
		},
		{
			"invalid data hash",
			Header{
				Version: version.Consensus{Block: version.BlockProtocol},
				ChainID: string(make([]byte, MaxChainIDLen)),
				Height:  1,
				LastBlockID: BlockID{
					Hash: make([]byte, crypto.HashSize),
					PartSetHeader: PartSetHeader{
						Hash: make([]byte, crypto.HashSize),
					},
				},
				LastCommitHash: make([]byte, crypto.HashSize),
				DataHash:       make([]byte, crypto.HashSize+1),
			},
			true, "wrong DataHash",
		},
		{
			"invalid evidence hash",
			Header{
				Version: version.Consensus{Block: version.BlockProtocol},
				ChainID: string(make([]byte, MaxChainIDLen)),
				Height:  1,
				LastBlockID: BlockID{
					Hash: make([]byte, crypto.HashSize),
					PartSetHeader: PartSetHeader{
						Hash: make([]byte, crypto.HashSize),
					},
				},
				LastCommitHash: make([]byte, crypto.HashSize),
				DataHash:       make([]byte, crypto.HashSize),
				EvidenceHash:   make([]byte, crypto.HashSize+1),
			},
			true, "wrong EvidenceHash",
		},
		{
			"invalid proposer protxhash",
			Header{
				Version: version.Consensus{Block: version.BlockProtocol},
				ChainID: string(make([]byte, MaxChainIDLen)),
				Height:  1,
				LastBlockID: BlockID{
					Hash: make([]byte, crypto.HashSize),
					PartSetHeader: PartSetHeader{
						Hash: make([]byte, crypto.HashSize),
					},
				},
				LastCommitHash: make([]byte, crypto.HashSize),
				DataHash:       make([]byte, crypto.HashSize),
				EvidenceHash:   make([]byte, crypto.HashSize),

				ProposerProTxHash: make([]byte, crypto.ProTxHashSize+1),
			},
			true, "invalid ProposerProTxHash length; got: 33, expected: 32",
		},
		{
			"invalid validator hash",
			Header{
				Version: version.Consensus{Block: version.BlockProtocol},
				ChainID: string(make([]byte, MaxChainIDLen)),
				Height:  1,
				LastBlockID: BlockID{
					Hash: make([]byte, crypto.HashSize),
					PartSetHeader: PartSetHeader{
						Hash: make([]byte, crypto.HashSize),
					},
				},
				LastCommitHash:    make([]byte, crypto.HashSize),
				DataHash:          make([]byte, crypto.HashSize),
				EvidenceHash:      make([]byte, crypto.HashSize),
				ProposerProTxHash: make([]byte, crypto.ProTxHashSize),
				ValidatorsHash:    make([]byte, crypto.HashSize+1),
			},
			true, "wrong ValidatorsHash",
		},
		{
			"invalid next validator hash",
			Header{
				Version: version.Consensus{Block: version.BlockProtocol},
				ChainID: string(make([]byte, MaxChainIDLen)),
				Height:  1,
				LastBlockID: BlockID{
					Hash: make([]byte, crypto.HashSize),
					PartSetHeader: PartSetHeader{
						Hash: make([]byte, crypto.HashSize),
					},
				},
				LastCommitHash:     make([]byte, crypto.HashSize),
				DataHash:           make([]byte, crypto.HashSize),
				EvidenceHash:       make([]byte, crypto.HashSize),
				ProposerProTxHash:  make([]byte, crypto.ProTxHashSize),
				ValidatorsHash:     make([]byte, crypto.HashSize),
				NextValidatorsHash: make([]byte, crypto.HashSize+1),
			},
			true, "wrong NextValidatorsHash",
		},
		{
			"invalid consensus hash",
			Header{
				Version: version.Consensus{Block: version.BlockProtocol},
				ChainID: string(make([]byte, MaxChainIDLen)),
				Height:  1,
				LastBlockID: BlockID{
					Hash: make([]byte, crypto.HashSize),
					PartSetHeader: PartSetHeader{
						Hash: make([]byte, crypto.HashSize),
					},
				},
				LastCommitHash:     make([]byte, crypto.HashSize),
				DataHash:           make([]byte, crypto.HashSize),
				EvidenceHash:       make([]byte, crypto.HashSize),
				ProposerProTxHash:  make([]byte, crypto.ProTxHashSize),
				ValidatorsHash:     make([]byte, crypto.HashSize),
				NextValidatorsHash: make([]byte, crypto.HashSize),
				ConsensusHash:      make([]byte, crypto.HashSize+1),
			},
			true, "wrong ConsensusHash",
		},
		{
			"invalid last results hash",
			Header{
				Version: version.Consensus{Block: version.BlockProtocol},
				ChainID: string(make([]byte, MaxChainIDLen)),
				Height:  1,
				LastBlockID: BlockID{
					Hash: make([]byte, crypto.HashSize),
					PartSetHeader: PartSetHeader{
						Hash: make([]byte, crypto.HashSize),
					},
				},
				LastCommitHash:     make([]byte, crypto.HashSize),
				DataHash:           make([]byte, crypto.HashSize),
				EvidenceHash:       make([]byte, crypto.HashSize),
				ProposerProTxHash:  make([]byte, crypto.ProTxHashSize),
				ValidatorsHash:     make([]byte, crypto.HashSize),
				NextValidatorsHash: make([]byte, crypto.HashSize),
				ConsensusHash:      make([]byte, crypto.HashSize),
				LastResultsHash:    make([]byte, crypto.HashSize+1),
			},
			true, "wrong LastResultsHash",
		},
		{
			"valid header",
			Header{
				Version:               version.Consensus{Block: version.BlockProtocol},
				ChainID:               string(make([]byte, MaxChainIDLen)),
				Height:                1,
				CoreChainLockedHeight: 1,
				LastBlockID: BlockID{
					Hash: make([]byte, crypto.HashSize),
					PartSetHeader: PartSetHeader{
						Hash: make([]byte, crypto.HashSize),
					},
				},
				LastCommitHash:     make([]byte, crypto.HashSize),
				DataHash:           make([]byte, crypto.HashSize),
				EvidenceHash:       make([]byte, crypto.HashSize),
				ProposerProTxHash:  make([]byte, crypto.ProTxHashSize),
				ValidatorsHash:     make([]byte, crypto.HashSize),
				NextValidatorsHash: make([]byte, crypto.HashSize),
				ConsensusHash:      make([]byte, crypto.HashSize),
				LastResultsHash:    make([]byte, crypto.HashSize),
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

func TestStateID_ValidateBasic(t *testing.T) {
	type fields struct {
		Height      int64
		LastAppHash tmbytes.HexBytes
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{name: "negative height", fields: fields{-1, nil}, wantErr: true},
		{name: "zero height - allowed for genesis block", fields: fields{0, nil}, wantErr: false},
		{name: "nil apphash", fields: fields{12, nil}, wantErr: false},
		{name: "empty apphash", fields: fields{12, []byte{}}, wantErr: false},
		{name: "apphash too short", fields: fields{12, []byte{0x1, 0x2, 0x3}}, wantErr: true},
		{name: "apphash too short 2", fields: fields{12, tmrand.Bytes(crypto.SmallAppHashSize - 1)}, wantErr: true},
		{name: "apphash small", fields: fields{12, tmrand.Bytes(crypto.SmallAppHashSize)}, wantErr: false},
		{name: "apphash default", fields: fields{12, tmrand.Bytes(crypto.DefaultAppHashSize)}, wantErr: false},
		{name: "apphash large", fields: fields{12, tmrand.Bytes(crypto.LargeAppHashSize)}, wantErr: false},
		{name: "apphash too large", fields: fields{12, tmrand.Bytes(crypto.LargeAppHashSize + 1)}, wantErr: true},
	}
	//nolint:scopelint
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateID := StateID{
				Height:      tt.fields.Height,
				LastAppHash: tt.fields.LastAppHash,
			}
			if err := stateID.ValidateBasic(); (err != nil) != tt.wantErr {
				t.Errorf("StateID.ValidateBasic() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCommit_ValidateBasic(t *testing.T) {
	testCases := []struct {
		name      string
		commit    *Commit
		expectErr bool
		errString string
	}{
		{
			"invalid height",
			&Commit{Height: -1},
			true, "negative Height",
		},
		{
			"invalid round",
			&Commit{Height: 1, Round: -1},
			true, "negative Round",
		},
		{
			"invalid block ID",
			&Commit{
				Height:  1,
				Round:   1,
				BlockID: BlockID{},
			},
			true, "commit cannot be for nil block",
		},
		{
			"invalid block signature",
			&Commit{
				Height: 1,
				Round:  1,
				BlockID: BlockID{
					Hash: make([]byte, crypto.HashSize),
					PartSetHeader: PartSetHeader{
						Hash: make([]byte, crypto.HashSize),
					},
				},
				ThresholdBlockSignature: make([]byte, bls12381.SignatureSize+1),
				ThresholdStateSignature: make([]byte, bls12381.SignatureSize),
			},
			true, "block threshold signature is wrong size",
		},
		{
			"invalid state signature",
			&Commit{
				Height: 1,
				Round:  1,
				BlockID: BlockID{
					Hash: make([]byte, crypto.HashSize),
					PartSetHeader: PartSetHeader{
						Hash: make([]byte, crypto.HashSize),
					},
				},
				ThresholdBlockSignature: make([]byte, bls12381.SignatureSize),
				ThresholdStateSignature: make([]byte, bls12381.SignatureSize+1),
			},
			true, "state threshold signature is wrong size",
		},
		{
			"valid commit",
			&Commit{
				Height: 1,
				Round:  1,
				BlockID: BlockID{
					Hash: make([]byte, crypto.HashSize),
					PartSetHeader: PartSetHeader{
						Hash: make([]byte, crypto.HashSize),
					},
				},
				ThresholdBlockSignature: make([]byte, bls12381.SignatureSize),
				ThresholdStateSignature: make([]byte, bls12381.SignatureSize),
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

func TestStateID_WithHeight(t *testing.T) {
	stateID := RandStateID()
	height := stateID.Height
	stateIDWithHeight := stateID.WithHeight(height + 1)

	assert.Equal(t, height, stateIDWithHeight.Height-1)
}

func TestHeaderHashVector(t *testing.T) {
	chainID := "test"
	h := Header{
		Version:            version.Consensus{Block: 1, App: 1},
		ChainID:            chainID,
		Height:             50,
		Time:               time.Date(math.MaxInt64, 0, 0, 0, 0, 0, math.MaxInt64, time.UTC),
		LastBlockID:        BlockID{},
		LastCommitHash:     []byte("f2564c78071e26643ae9b3e2a19fa0dc10d4d9e873aa0be808660123f11a1e78"),
		DataHash:           []byte("f2564c78071e26643ae9b3e2a19fa0dc10d4d9e873aa0be808660123f11a1e78"),
		ValidatorsHash:     []byte("f2564c78071e26643ae9b3e2a19fa0dc10d4d9e873aa0be808660123f11a1e78"),
		NextValidatorsHash: []byte("f2564c78071e26643ae9b3e2a19fa0dc10d4d9e873aa0be808660123f11a1e78"),
		ConsensusHash:      []byte("f2564c78071e26643ae9b3e2a19fa0dc10d4d9e873aa0be808660123f11a1e78"),
		AppHash:            []byte("f2564c78071e26643ae9b3e2a19fa0dc10d4d9e873aa0be808660123f11a1e78"),

		LastResultsHash: []byte("f2564c78071e26643ae9b3e2a19fa0dc10d4d9e873aa0be808660123f11a1e78"),

		EvidenceHash:      []byte("f2564c78071e26643ae9b3e2a19fa0dc10d4d9e873aa0be808660123f11a1e78"),
		ProposerProTxHash: []byte("f2564c78071e26643ae9b3e2a19fa0dc10d4d9e873aa0be808660123f11a1e78"),
	}

	testCases := []struct {
		header   Header
		expBytes string
	}{
		{header: h, expBytes: "8df5f303af2ae303adaaa56d8f5645247dd8fd12dc5ebdff55444649f653452f"},
	}

	for _, tc := range testCases {
		hash := tc.header.Hash()
		require.Equal(t, tc.expBytes, hex.EncodeToString(hash))
	}
}
