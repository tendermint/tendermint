package blockchain

import (
	"bytes"
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cfg "github.com/tendermint/tendermint/config"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/db"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/libs/log"
	sm "github.com/tendermint/tendermint/state"

	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

// A cleanupFunc cleans up any config / test files created for a particular
// test.
type cleanupFunc func()

// make a Commit with a single vote containing just the height and a timestamp
func makeTestCommit(height int64, timestamp time.Time) *types.Commit {
	commitSigs := []*types.CommitSig{{Height: height, Timestamp: timestamp}}
	return types.NewCommit(types.BlockID{}, commitSigs)
}

func makeStateAndBlockStore(logger log.Logger) (sm.State, *BlockStore, cleanupFunc) {
	config := cfg.ResetTestRoot("blockchain_reactor_test")
	// blockDB := dbm.NewDebugDB("blockDB", dbm.NewMemDB())
	// stateDB := dbm.NewDebugDB("stateDB", dbm.NewMemDB())
	blockDB := dbm.NewMemDB()
	stateDB := dbm.NewMemDB()
	state, err := sm.LoadStateFromDBOrGenesisFile(stateDB, config.GenesisFile())
	if err != nil {
		panic(cmn.ErrorWrap(err, "error constructing state from genesis file"))
	}
	return state, NewBlockStore(blockDB), func() { os.RemoveAll(config.RootDir) }
}

func TestLoadBlockStoreStateJSON(t *testing.T) {
	db := db.NewMemDB()

	bsj := &BlockStoreStateJSON{Height: 1000}
	bsj.Save(db)

	retrBSJ := LoadBlockStoreStateJSON(db)

	assert.Equal(t, *bsj, retrBSJ, "expected the retrieved DBs to match")
}

func TestNewBlockStore(t *testing.T) {
	db := db.NewMemDB()
	db.Set(blockStoreKey, []byte(`{"height": "10000"}`))
	bs := NewBlockStore(db)
	require.Equal(t, int64(10000), bs.Height(), "failed to properly parse blockstore")

	panicCausers := []struct {
		data    []byte
		wantErr string
	}{
		{[]byte("artful-doger"), "not unmarshal bytes"},
		{[]byte(" "), "unmarshal bytes"},
	}

	for i, tt := range panicCausers {
		// Expecting a panic here on trying to parse an invalid blockStore
		_, _, panicErr := doFn(func() (interface{}, error) {
			db.Set(blockStoreKey, tt.data)
			_ = NewBlockStore(db)
			return nil, nil
		})
		require.NotNil(t, panicErr, "#%d panicCauser: %q expected a panic", i, tt.data)
		assert.Contains(t, fmt.Sprintf("%#v", panicErr), tt.wantErr, "#%d data: %q", i, tt.data)
	}

	db.Set(blockStoreKey, nil)
	bs = NewBlockStore(db)
	assert.Equal(t, bs.Height(), int64(0), "expecting nil bytes to be unmarshaled alright")
}

func freshBlockStore() (*BlockStore, db.DB) {
	db := db.NewMemDB()
	return NewBlockStore(db), db
}

var (
	state       sm.State
	block       *types.Block
	partSet     *types.PartSet
	part1       *types.Part
	part2       *types.Part
	seenCommit1 *types.Commit
)

func TestMain(m *testing.M) {
	var cleanup cleanupFunc
	state, _, cleanup = makeStateAndBlockStore(log.NewTMLogger(new(bytes.Buffer)))
	block = makeBlock(1, state, new(types.Commit))
	partSet = block.MakePartSet(2)
	part1 = partSet.GetPart(0)
	part2 = partSet.GetPart(1)
	seenCommit1 = makeTestCommit(10, tmtime.Now())
	code := m.Run()
	cleanup()
	os.Exit(code)
}

// TODO: This test should be simplified ...

func TestBlockStoreSaveLoadBlock(t *testing.T) {
	state, bs, cleanup := makeStateAndBlockStore(log.NewTMLogger(new(bytes.Buffer)))
	defer cleanup()
	require.Equal(t, bs.Height(), int64(0), "initially the height should be zero")

	// check there are no blocks at various heights
	noBlockHeights := []int64{0, -1, 100, 1000, 2}
	for i, height := range noBlockHeights {
		if g := bs.LoadBlock(height); g != nil {
			t.Errorf("#%d: height(%d) got a block; want nil", i, height)
		}
	}

	// save a block
	block := makeBlock(bs.Height()+1, state, new(types.Commit))
	validPartSet := block.MakePartSet(2)
	seenCommit := makeTestCommit(10, tmtime.Now())
	bs.SaveBlock(block, partSet, seenCommit)
	require.Equal(t, bs.Height(), block.Header.Height, "expecting the new height to be changed")

	incompletePartSet := types.NewPartSetFromHeader(types.PartSetHeader{Total: 2})
	uncontiguousPartSet := types.NewPartSetFromHeader(types.PartSetHeader{Total: 0})
	uncontiguousPartSet.AddPart(part2)

	header1 := types.Header{
		Height:  1,
		NumTxs:  100,
		ChainID: "block_test",
		Time:    tmtime.Now(),
	}
	header2 := header1
	header2.Height = 4

	// End of setup, test data

	commitAtH10 := makeTestCommit(10, tmtime.Now())
	tuples := []struct {
		block      *types.Block
		parts      *types.PartSet
		seenCommit *types.Commit
		wantErr    bool
		wantPanic  string

		corruptBlockInDB      bool
		corruptCommitInDB     bool
		corruptSeenCommitInDB bool
		eraseCommitInDB       bool
		eraseSeenCommitInDB   bool
	}{
		{
			block:      newBlock(header1, commitAtH10),
			parts:      validPartSet,
			seenCommit: seenCommit1,
		},

		{
			block:     nil,
			wantPanic: "only save a non-nil block",
		},

		{
			block:     newBlock(header2, commitAtH10),
			parts:     uncontiguousPartSet,
			wantPanic: "only save contiguous blocks", // and incomplete and uncontiguous parts
		},

		{
			block:     newBlock(header1, commitAtH10),
			parts:     incompletePartSet,
			wantPanic: "only save complete block", // incomplete parts
		},

		{
			block:             newBlock(header1, commitAtH10),
			parts:             validPartSet,
			seenCommit:        seenCommit1,
			corruptCommitInDB: true, // Corrupt the DB's commit entry
			wantPanic:         "unmarshal to types.Commit failed",
		},

		{
			block:            newBlock(header1, commitAtH10),
			parts:            validPartSet,
			seenCommit:       seenCommit1,
			wantPanic:        "unmarshal to types.BlockMeta failed",
			corruptBlockInDB: true, // Corrupt the DB's block entry
		},

		{
			block:      newBlock(header1, commitAtH10),
			parts:      validPartSet,
			seenCommit: seenCommit1,

			// Expecting no error and we want a nil back
			eraseSeenCommitInDB: true,
		},

		{
			block:      newBlock(header1, commitAtH10),
			parts:      validPartSet,
			seenCommit: seenCommit1,

			corruptSeenCommitInDB: true,
			wantPanic:             "unmarshal to types.Commit failed",
		},

		{
			block:      newBlock(header1, commitAtH10),
			parts:      validPartSet,
			seenCommit: seenCommit1,

			// Expecting no error and we want a nil back
			eraseCommitInDB: true,
		},
	}

	type quad struct {
		block  *types.Block
		commit *types.Commit
		meta   *types.BlockMeta

		seenCommit *types.Commit
	}

	for i, tuple := range tuples {
		bs, db := freshBlockStore()
		// SaveBlock
		res, err, panicErr := doFn(func() (interface{}, error) {
			bs.SaveBlock(tuple.block, tuple.parts, tuple.seenCommit)
			if tuple.block == nil {
				return nil, nil
			}

			if tuple.corruptBlockInDB {
				db.Set(calcBlockMetaKey(tuple.block.Height), []byte("block-bogus"))
			}
			bBlock := bs.LoadBlock(tuple.block.Height)
			bBlockMeta := bs.LoadBlockMeta(tuple.block.Height)

			if tuple.eraseSeenCommitInDB {
				db.Delete(calcSeenCommitKey(tuple.block.Height))
			}
			if tuple.corruptSeenCommitInDB {
				db.Set(calcSeenCommitKey(tuple.block.Height), []byte("bogus-seen-commit"))
			}
			bSeenCommit := bs.LoadSeenCommit(tuple.block.Height)

			commitHeight := tuple.block.Height - 1
			if tuple.eraseCommitInDB {
				db.Delete(calcBlockCommitKey(commitHeight))
			}
			if tuple.corruptCommitInDB {
				db.Set(calcBlockCommitKey(commitHeight), []byte("foo-bogus"))
			}
			bCommit := bs.LoadBlockCommit(commitHeight)
			return &quad{block: bBlock, seenCommit: bSeenCommit, commit: bCommit,
				meta: bBlockMeta}, nil
		})

		if subStr := tuple.wantPanic; subStr != "" {
			if panicErr == nil {
				t.Errorf("#%d: want a non-nil panic", i)
			} else if got := fmt.Sprintf("%#v", panicErr); !strings.Contains(got, subStr) {
				t.Errorf("#%d:\n\tgotErr: %q\nwant substring: %q", i, got, subStr)
			}
			continue
		}

		if tuple.wantErr {
			if err == nil {
				t.Errorf("#%d: got nil error", i)
			}
			continue
		}

		assert.Nil(t, panicErr, "#%d: unexpected panic", i)
		assert.Nil(t, err, "#%d: expecting a non-nil error", i)
		qua, ok := res.(*quad)
		if !ok || qua == nil {
			t.Errorf("#%d: got nil quad back; gotType=%T", i, res)
			continue
		}
		if tuple.eraseSeenCommitInDB {
			assert.Nil(t, qua.seenCommit,
				"erased the seenCommit in the DB hence we should get back a nil seenCommit")
		}
		if tuple.eraseCommitInDB {
			assert.Nil(t, qua.commit,
				"erased the commit in the DB hence we should get back a nil commit")
		}
	}
}

func TestLoadBlockPart(t *testing.T) {
	bs, db := freshBlockStore()
	height, index := int64(10), 1
	loadPart := func() (interface{}, error) {
		part := bs.LoadBlockPart(height, index)
		return part, nil
	}

	// Initially no contents.
	// 1. Requesting for a non-existent block shouldn't fail
	res, _, panicErr := doFn(loadPart)
	require.Nil(t, panicErr, "a non-existent block part shouldn't cause a panic")
	require.Nil(t, res, "a non-existent block part should return nil")

	// 2. Next save a corrupted block then try to load it
	db.Set(calcBlockPartKey(height, index), []byte("Tendermint"))
	res, _, panicErr = doFn(loadPart)
	require.NotNil(t, panicErr, "expecting a non-nil panic")
	require.Contains(t, panicErr.Error(), "unmarshal to types.Part failed")

	// 3. A good block serialized and saved to the DB should be retrievable
	db.Set(calcBlockPartKey(height, index), cdc.MustMarshalBinaryBare(part1))
	gotPart, _, panicErr := doFn(loadPart)
	require.Nil(t, panicErr, "an existent and proper block should not panic")
	require.Nil(t, res, "a properly saved block should return a proper block")
	require.Equal(t, gotPart.(*types.Part), part1,
		"expecting successful retrieval of previously saved block")
}

func TestLoadBlockMeta(t *testing.T) {
	bs, db := freshBlockStore()
	height := int64(10)
	loadMeta := func() (interface{}, error) {
		meta := bs.LoadBlockMeta(height)
		return meta, nil
	}

	// Initially no contents.
	// 1. Requesting for a non-existent blockMeta shouldn't fail
	res, _, panicErr := doFn(loadMeta)
	require.Nil(t, panicErr, "a non-existent blockMeta shouldn't cause a panic")
	require.Nil(t, res, "a non-existent blockMeta should return nil")

	// 2. Next save a corrupted blockMeta then try to load it
	db.Set(calcBlockMetaKey(height), []byte("Tendermint-Meta"))
	res, _, panicErr = doFn(loadMeta)
	require.NotNil(t, panicErr, "expecting a non-nil panic")
	require.Contains(t, panicErr.Error(), "unmarshal to types.BlockMeta")

	// 3. A good blockMeta serialized and saved to the DB should be retrievable
	meta := &types.BlockMeta{}
	db.Set(calcBlockMetaKey(height), cdc.MustMarshalBinaryBare(meta))
	gotMeta, _, panicErr := doFn(loadMeta)
	require.Nil(t, panicErr, "an existent and proper block should not panic")
	require.Nil(t, res, "a properly saved blockMeta should return a proper blocMeta ")
	require.Equal(t, cdc.MustMarshalBinaryBare(meta), cdc.MustMarshalBinaryBare(gotMeta),
		"expecting successful retrieval of previously saved blockMeta")
}

func TestBlockFetchAtHeight(t *testing.T) {
	state, bs, cleanup := makeStateAndBlockStore(log.NewTMLogger(new(bytes.Buffer)))
	defer cleanup()
	require.Equal(t, bs.Height(), int64(0), "initially the height should be zero")
	block := makeBlock(bs.Height()+1, state, new(types.Commit))

	partSet := block.MakePartSet(2)
	seenCommit := makeTestCommit(10, tmtime.Now())
	bs.SaveBlock(block, partSet, seenCommit)
	require.Equal(t, bs.Height(), block.Header.Height, "expecting the new height to be changed")

	blockAtHeight := bs.LoadBlock(bs.Height())
	bz1 := cdc.MustMarshalBinaryBare(block)
	bz2 := cdc.MustMarshalBinaryBare(blockAtHeight)
	require.Equal(t, bz1, bz2)
	require.Equal(t, block.Hash(), blockAtHeight.Hash(),
		"expecting a successful load of the last saved block")

	blockAtHeightPlus1 := bs.LoadBlock(bs.Height() + 1)
	require.Nil(t, blockAtHeightPlus1, "expecting an unsuccessful load of Height()+1")
	blockAtHeightPlus2 := bs.LoadBlock(bs.Height() + 2)
	require.Nil(t, blockAtHeightPlus2, "expecting an unsuccessful load of Height()+2")
}

func doFn(fn func() (interface{}, error)) (res interface{}, err error, panicErr error) {
	defer func() {
		if r := recover(); r != nil {
			switch e := r.(type) {
			case error:
				panicErr = e
			case string:
				panicErr = fmt.Errorf("%s", e)
			default:
				if st, ok := r.(fmt.Stringer); ok {
					panicErr = fmt.Errorf("%s", st)
				} else {
					panicErr = fmt.Errorf("%s", debug.Stack())
				}
			}
		}
	}()

	res, err = fn()
	return res, err, panicErr
}

func newBlock(hdr types.Header, lastCommit *types.Commit) *types.Block {
	return &types.Block{
		Header:     hdr,
		LastCommit: lastCommit,
	}
}
