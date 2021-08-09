package block_test

import (
	"encoding/hex"
	"math"
	mrand "math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	test "github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/libs/bytes"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/pkg/block"
	"github.com/tendermint/tendermint/pkg/evidence"
	"github.com/tendermint/tendermint/pkg/mempool"
	"github.com/tendermint/tendermint/pkg/meta"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}

func TestBlockAddEvidence(t *testing.T) {
	txs := []mempool.Tx{mempool.Tx("foo"), mempool.Tx("bar")}
	lastID := test.MakeBlockID()
	h := int64(3)

	voteSet, _, vals := test.RandVoteSet(h-1, 1, tmproto.PrecommitType, 10, 1)
	commit, err := test.MakeCommit(lastID, h-1, 1, voteSet, vals, time.Now())
	require.NoError(t, err)

	ev := evidence.NewMockDuplicateVoteEvidenceWithValidator(h, time.Now(), vals[0], "block-test-chain")
	evList := []evidence.Evidence{ev}

	b := block.MakeBlock(h, txs, commit, evList)
	require.NotNil(t, b)
	require.Equal(t, 1, len(b.Evidence.Evidence))
	require.NotNil(t, b.EvidenceHash)
}

func TestBlockValidateBasic(t *testing.T) {
	require.Error(t, (*block.Block)(nil).ValidateBasic())

	txs := []mempool.Tx{mempool.Tx("foo"), mempool.Tx("bar")}
	lastID := test.MakeBlockID()
	h := int64(3)

	voteSet, valSet, vals := test.RandVoteSet(h-1, 1, tmproto.PrecommitType, 10, 1)
	commit, err := test.MakeCommit(lastID, h-1, 1, voteSet, vals, time.Now())
	require.NoError(t, err)

	ev := evidence.NewMockDuplicateVoteEvidenceWithValidator(h, time.Now(), vals[0], "block-test-chain")
	evList := []evidence.Evidence{ev}

	testCases := []struct {
		testName      string
		malleateBlock func(*block.Block)
		expErr        bool
	}{
		{"Make Block", func(blk *block.Block) {}, false},
		{"Make Block w/ proposer Addr", func(blk *block.Block) { blk.ProposerAddress = valSet.GetProposer().Address }, false},
		{"Negative Height", func(blk *block.Block) { blk.Height = -1 }, true},
		{"Remove 1/2 the commits", func(blk *block.Block) {
			blk.LastCommit.Signatures = commit.Signatures[:commit.Size()/2]
			blk.LastCommit.ClearCache() // clear hash or change wont be noticed
		}, true},
		{"Remove LastCommitHash", func(blk *block.Block) { blk.LastCommitHash = []byte("something else") }, true},
		{"Tampered Data", func(blk *block.Block) {
			blk.Data.Txs[0] = mempool.Tx("something else")
			blk.Data.ClearCache() // clear hash or change wont be noticed
		}, true},
		{"Tampered DataHash", func(blk *block.Block) {
			blk.DataHash = tmrand.Bytes(len(blk.DataHash))
		}, true},
		{"Tampered EvidenceHash", func(blk *block.Block) {
			blk.EvidenceHash = tmrand.Bytes(len(blk.EvidenceHash))
		}, true},
		{"Incorrect block protocol version", func(blk *block.Block) {
			blk.Version.Block = 1
		}, true},
		{"Missing LastCommit", func(blk *block.Block) {
			blk.LastCommit = nil
		}, true},
		{"Invalid LastCommit", func(blk *block.Block) {
			blk.LastCommit = meta.NewCommit(-1, 0, test.MakeBlockID(), nil)
		}, true},
		{"Invalid Evidence", func(blk *block.Block) {
			emptyEv := &evidence.DuplicateVoteEvidence{}
			blk.Evidence = block.EvidenceData{Evidence: []evidence.Evidence{emptyEv}}
		}, true},
	}
	for i, tc := range testCases {
		tc := tc
		i := i
		t.Run(tc.testName, func(t *testing.T) {
			block := block.MakeBlock(h, txs, commit, evList)
			block.ProposerAddress = valSet.GetProposer().Address
			tc.malleateBlock(block)
			err = block.ValidateBasic()
			t.Log(err)
			assert.Equal(t, tc.expErr, err != nil, "#%d: %v", i, err)
		})
	}
}

func TestBlockHash(t *testing.T) {
	assert.Nil(t, (*block.Block)(nil).Hash())
	assert.Nil(t, block.MakeBlock(int64(3), []mempool.Tx{mempool.Tx("Hello World")}, nil, nil).Hash())
}

func TestBlockMakePartSet(t *testing.T) {
	assert.Nil(t, (*block.Block)(nil).MakePartSet(2))

	partSet := block.MakeBlock(int64(3), []mempool.Tx{mempool.Tx("Hello World")}, nil, nil).MakePartSet(1024)
	assert.NotNil(t, partSet)
	assert.EqualValues(t, 1, partSet.Total())
}

func TestBlockMakePartSetWithEvidence(t *testing.T) {
	assert.Nil(t, (*block.Block)(nil).MakePartSet(2))

	lastID := test.MakeBlockID()
	h := int64(3)

	voteSet, _, vals := test.RandVoteSet(h-1, 1, tmproto.PrecommitType, 10, 1)
	commit, err := test.MakeCommit(lastID, h-1, 1, voteSet, vals, time.Now())
	require.NoError(t, err)

	ev := evidence.NewMockDuplicateVoteEvidenceWithValidator(h, time.Now(), vals[0], "block-test-chain")
	evList := []evidence.Evidence{ev}

	partSet := block.MakeBlock(h, []mempool.Tx{mempool.Tx("Hello World")}, commit, evList).MakePartSet(512)
	assert.NotNil(t, partSet)
	assert.EqualValues(t, 4, partSet.Total())
}

func TestBlockHashesTo(t *testing.T) {
	assert.False(t, (*block.Block)(nil).HashesTo(nil))

	lastID := test.MakeBlockID()
	h := int64(3)
	voteSet, valSet, vals := test.RandVoteSet(h-1, 1, tmproto.PrecommitType, 10, 1)
	commit, err := test.MakeCommit(lastID, h-1, 1, voteSet, vals, time.Now())
	require.NoError(t, err)

	ev := evidence.NewMockDuplicateVoteEvidenceWithValidator(h, time.Now(), vals[0], "block-test-chain")
	evList := []evidence.Evidence{ev}

	block := block.MakeBlock(h, []mempool.Tx{mempool.Tx("Hello World")}, commit, evList)
	block.ValidatorsHash = valSet.Hash()
	assert.False(t, block.HashesTo([]byte{}))
	assert.False(t, block.HashesTo([]byte("something else")))
	assert.True(t, block.HashesTo(block.Hash()))
}

func TestBlockSize(t *testing.T) {
	size := block.MakeBlock(int64(3), []mempool.Tx{mempool.Tx("Hello World")}, nil, nil).Size()
	if size <= 0 {
		t.Fatal("Size of the block is zero or negative")
	}
}

func TestBlockString(t *testing.T) {
	assert.Equal(t, "nil-Block", (*block.Block)(nil).String())
	assert.Equal(t, "nil-Block", (*block.Block)(nil).StringIndented(""))
	assert.Equal(t, "nil-Block", (*block.Block)(nil).StringShort())

	block := block.MakeBlock(int64(3), []mempool.Tx{mempool.Tx("Hello World")}, nil, nil)
	assert.NotEqual(t, "nil-Block", block.String())
	assert.NotEqual(t, "nil-Block", block.StringIndented(""))
	assert.NotEqual(t, "nil-Block", block.StringShort())
}

func hexBytesFromString(s string) bytes.HexBytes {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return bytes.HexBytes(b)
}

func TestBlockMaxDataBytes(t *testing.T) {
	testCases := []struct {
		maxBytes      int64
		valsCount     int
		evidenceBytes int64
		panics        bool
		result        int64
	}{
		0: {-10, 1, 0, true, 0},
		1: {10, 1, 0, true, 0},
		2: {841, 1, 0, true, 0},
		3: {842, 1, 0, false, 0},
		4: {843, 1, 0, false, 1},
		5: {954, 2, 0, false, 1},
		6: {1053, 2, 100, false, 0},
	}

	for i, tc := range testCases {
		tc := tc
		if tc.panics {
			assert.Panics(t, func() {
				block.MaxDataBytes(tc.maxBytes, tc.evidenceBytes, tc.valsCount)
			}, "#%v", i)
		} else {
			assert.Equal(t,
				tc.result,
				block.MaxDataBytes(tc.maxBytes, tc.evidenceBytes, tc.valsCount),
				"#%v", i)
		}
	}
}

func TestBlockMaxDataBytesNoEvidence(t *testing.T) {
	testCases := []struct {
		maxBytes  int64
		valsCount int
		panics    bool
		result    int64
	}{
		0: {-10, 1, true, 0},
		1: {10, 1, true, 0},
		2: {841, 1, true, 0},
		3: {842, 1, false, 0},
		4: {843, 1, false, 1},
	}

	for i, tc := range testCases {
		tc := tc
		if tc.panics {
			assert.Panics(t, func() {
				block.MaxDataBytesNoEvidence(tc.maxBytes, tc.valsCount)
			}, "#%v", i)
		} else {
			assert.Equal(t,
				tc.result,
				block.MaxDataBytesNoEvidence(tc.maxBytes, tc.valsCount),
				"#%v", i)
		}
	}
}

func TestBlockProtoBuf(t *testing.T) {
	h := mrand.Int63()
	c1 := test.MakeRandomCommit(time.Now())
	b1 := block.MakeBlock(h, []mempool.Tx{mempool.Tx([]byte{1})}, &meta.Commit{Signatures: []meta.CommitSig{}}, []evidence.Evidence{})
	b1.ProposerAddress = tmrand.Bytes(crypto.AddressSize)

	b2 := block.MakeBlock(h, []mempool.Tx{mempool.Tx([]byte{1})}, c1, []evidence.Evidence{})
	b2.ProposerAddress = tmrand.Bytes(crypto.AddressSize)
	evidenceTime := time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)
	evi := evidence.NewMockDuplicateVoteEvidence(h, evidenceTime, "block-test-chain")
	b2.Evidence = block.EvidenceData{Evidence: evidence.EvidenceList{evi}}
	b2.EvidenceHash = b2.Evidence.Hash()

	b3 := block.MakeBlock(h, []mempool.Tx{}, c1, []evidence.Evidence{})
	b3.ProposerAddress = tmrand.Bytes(crypto.AddressSize)
	testCases := []struct {
		msg      string
		b1       *block.Block
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

		block, err := block.BlockFromProto(pb)
		if tc.expPass2 {
			require.NoError(t, err, tc.msg)
			require.EqualValues(t, tc.b1.Header, block.Header, tc.msg)
			require.EqualValues(t, tc.b1.Data, block.Data, tc.msg)
			require.EqualValues(t, tc.b1.Evidence.Evidence, block.Evidence.Evidence, tc.msg)
			require.EqualValues(t, *tc.b1.LastCommit, *block.LastCommit, tc.msg)
		} else {
			require.Error(t, err, tc.msg)
		}
	}
}

func TestDataProtoBuf(t *testing.T) {
	data := &block.Data{Txs: mempool.Txs{mempool.Tx([]byte{1}), mempool.Tx([]byte{2}), mempool.Tx([]byte{3})}}
	data2 := &block.Data{Txs: mempool.Txs{}}
	testCases := []struct {
		msg     string
		data1   *block.Data
		expPass bool
	}{
		{"success", data, true},
		{"success data2", data2, true},
	}
	for _, tc := range testCases {
		protoData := tc.data1.ToProto()
		d, err := block.DataFromProto(&protoData)
		if tc.expPass {
			require.NoError(t, err, tc.msg)
			require.EqualValues(t, tc.data1, &d, tc.msg)
		} else {
			require.Error(t, err, tc.msg)
		}
	}
}

// TestEvidenceDataProtoBuf ensures parity in converting to and from proto.
func TestEvidenceDataProtoBuf(t *testing.T) {
	const chainID = "mychain"
	ev := evidence.NewMockDuplicateVoteEvidence(math.MaxInt64, time.Now(), chainID)
	data := &block.EvidenceData{Evidence: evidence.EvidenceList{ev}}
	_ = data.ByteSize()
	testCases := []struct {
		msg      string
		data1    *block.EvidenceData
		expPass1 bool
		expPass2 bool
	}{
		{"success", data, true, true},
		{"empty evidenceData", &block.EvidenceData{Evidence: evidence.EvidenceList{}}, true, true},
		{"fail nil Data", nil, false, false},
	}

	for _, tc := range testCases {
		protoData, err := tc.data1.ToProto()
		if tc.expPass1 {
			require.NoError(t, err, tc.msg)
		} else {
			require.Error(t, err, tc.msg)
		}

		eviD := new(block.EvidenceData)
		err = eviD.FromProto(protoData)
		if tc.expPass2 {
			require.NoError(t, err, tc.msg)
			require.Equal(t, tc.data1, eviD, tc.msg)
		} else {
			require.Error(t, err, tc.msg)
		}
	}
}

// This follows RFC-6962, i.e. `echo -n '' | sha256sum`
var emptyBytes = []byte{0xe3, 0xb0, 0xc4, 0x42, 0x98, 0xfc, 0x1c, 0x14, 0x9a, 0xfb, 0xf4, 0xc8,
	0x99, 0x6f, 0xb9, 0x24, 0x27, 0xae, 0x41, 0xe4, 0x64, 0x9b, 0x93, 0x4c, 0xa4, 0x95, 0x99, 0x1b,
	0x78, 0x52, 0xb8, 0x55}

func TestNilDataHashDoesntCrash(t *testing.T) {
	assert.Equal(t, emptyBytes, []byte((*block.Data)(nil).Hash()))
	assert.Equal(t, emptyBytes, []byte(new(block.Data).Hash()))
}
