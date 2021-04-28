package store

import (
	"bytes"
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/log"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmstore "github.com/tendermint/tendermint/proto/tendermint/store"
	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
	"github.com/tendermint/tendermint/version"
)

// A cleanupFunc cleans up any config / test files created for a particular
// test.
type cleanupFunc func()

// make a Commit with a single vote containing just the height and a timestamp
func makeTestCommit(height int64, timestamp time.Time) *types.Commit {
	commitSigs := []types.CommitSig{{
		BlockIDFlag:        types.BlockIDFlagCommit,
		ValidatorProTxHash: tmrand.Bytes(crypto.DefaultHashSize),
		BlockSignature:     []byte("BlockSignature"),
		StateSignature:     []byte("StateSignature"),
	}}
	return types.NewCommit(height, 0,
		types.BlockID{Hash: []byte(""), PartSetHeader: types.PartSetHeader{Hash: []byte(""), Total: 2}},
		types.StateID{LastAppHash: make([]byte, 32)}, commitSigs, crypto.RandQuorumHash(),
		commitSigs[0].BlockSignature, commitSigs[0].StateSignature)
}

func makeTxs(height int64) (txs []types.Tx) {
	for i := 0; i < 10; i++ {
		txs = append(txs, types.Tx([]byte{byte(height), byte(i)}))
	}
	return txs
}

func makeBlock(height int64, state sm.State, lastCommit *types.Commit) *types.Block {
	block, _ := state.MakeBlock(height, nil, makeTxs(height), lastCommit, nil, state.Validators.GetProposer().ProTxHash)
	return block
}

func makeStateAndBlockStore(logger log.Logger) (sm.State, *BlockStore, cleanupFunc) {
	config := cfg.ResetTestRoot("blockchain_reactor_test")
	// blockDB := dbm.NewDebugDB("blockDB", dbm.NewMemDB())
	// stateDB := dbm.NewDebugDB("stateDB", dbm.NewMemDB())
	blockDB := dbm.NewMemDB()
	stateDB := dbm.NewMemDB()
	stateStore := sm.NewStore(stateDB)
	state, err := stateStore.LoadFromDBOrGenesisFile(config.GenesisFile())
	if err != nil {
		panic(fmt.Errorf("error constructing state from genesis file: %w", err))
	}
	return state, NewBlockStore(blockDB), func() { os.RemoveAll(config.RootDir) }
}

func TestLoadBlockStoreState(t *testing.T) {

	type blockStoreTest struct {
		testName string
		bss      *tmstore.BlockStoreState
		want     tmstore.BlockStoreState
	}

	testCases := []blockStoreTest{
		{"success", &tmstore.BlockStoreState{Base: 100, Height: 1000},
			tmstore.BlockStoreState{Base: 100, Height: 1000}},
		{"empty", &tmstore.BlockStoreState{}, tmstore.BlockStoreState{}},
		{"no base", &tmstore.BlockStoreState{Height: 1000}, tmstore.BlockStoreState{Base: 1, Height: 1000}},
	}

	for _, tc := range testCases {
		db := dbm.NewMemDB()
		SaveBlockStoreState(tc.bss, db)
		retrBSJ := LoadBlockStoreState(db)
		assert.Equal(t, tc.want, retrBSJ, "expected the retrieved DBs to match: %s", tc.testName)
	}
}

func TestNewBlockStore(t *testing.T) {
	db := dbm.NewMemDB()
	bss := tmstore.BlockStoreState{Base: 100, Height: 10000}
	bz, _ := proto.Marshal(&bss)
	err := db.Set(blockStoreKey, bz)
	require.NoError(t, err)
	bs := NewBlockStore(db)
	require.Equal(t, int64(100), bs.Base(), "failed to properly parse blockstore")
	require.Equal(t, int64(10000), bs.Height(), "failed to properly parse blockstore")

	panicCausers := []struct {
		data    []byte
		wantErr string
	}{
		{[]byte("artful-doger"), "not unmarshal bytes"},
		{[]byte(" "), "unmarshal bytes"},
	}

	for i, tt := range panicCausers {
		tt := tt
		// Expecting a panic here on trying to parse an invalid blockStore
		_, _, panicErr := doFn(func() (interface{}, error) {
			err := db.Set(blockStoreKey, tt.data)
			require.NoError(t, err)
			_ = NewBlockStore(db)
			return nil, nil
		})
		require.NotNil(t, panicErr, "#%d panicCauser: %q expected a panic", i, tt.data)
		assert.Contains(t, fmt.Sprintf("%#v", panicErr), tt.wantErr, "#%d data: %q", i, tt.data)
	}

	err = db.Set(blockStoreKey, []byte{})
	require.NoError(t, err)
	bs = NewBlockStore(db)
	assert.Equal(t, bs.Height(), int64(0), "expecting empty bytes to be unmarshaled alright")
}

func freshBlockStore() (*BlockStore, dbm.DB) {
	db := dbm.NewMemDB()
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
	require.Equal(t, bs.Base(), int64(0), "initially the base should be zero")
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
	require.EqualValues(t, 1, bs.Base(), "expecting the new height to be changed")
	require.EqualValues(t, block.Header.Height, bs.Height(), "expecting the new height to be changed")

	incompletePartSet := types.NewPartSetFromHeader(types.PartSetHeader{Total: 2})
	uncontiguousPartSet := types.NewPartSetFromHeader(types.PartSetHeader{Total: 0})
	_, err := uncontiguousPartSet.AddPart(part2)
	require.Error(t, err)

	header1 := types.Header{
		Version:           tmversion.Consensus{Block: version.BlockProtocol},
		Height:            1,
		ChainID:           "block_test",
		Time:              tmtime.Now(),
		ProposerProTxHash: tmrand.Bytes(crypto.DefaultHashSize),
	}

	// End of setup, test data

	commitAtH10 := makeTestCommit(10, tmtime.Now())
	tuples := []struct {
		block      *types.Block
		parts      *types.PartSet
		seenCommit *types.Commit
		wantPanic  string
		wantErr    bool

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
			block: newBlock( // New block at height 5 in empty block store is fine
				types.Header{
					Version:           tmversion.Consensus{Block: version.BlockProtocol},
					Height:            5,
					ChainID:           "block_test",
					Time:              tmtime.Now(),
					ProposerProTxHash: tmrand.Bytes(crypto.DefaultHashSize)},
				makeTestCommit(5, tmtime.Now()),
			),
			parts:      validPartSet,
			seenCommit: makeTestCommit(5, tmtime.Now()),
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
			wantPanic:         "error reading block commit",
		},

		{
			block:            newBlock(header1, commitAtH10),
			parts:            validPartSet,
			seenCommit:       seenCommit1,
			wantPanic:        "unmarshal to tmproto.BlockMeta",
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
			wantPanic:             "error reading block seen commit",
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
		tuple := tuple
		bs, db := freshBlockStore()
		// SaveBlock
		res, err, panicErr := doFn(func() (interface{}, error) {
			bs.SaveBlock(tuple.block, tuple.parts, tuple.seenCommit)
			if tuple.block == nil {
				return nil, nil
			}

			if tuple.corruptBlockInDB {
				err := db.Set(calcBlockMetaKey(tuple.block.Height), []byte("block-bogus"))
				require.NoError(t, err)
			}
			bBlock := bs.LoadBlock(tuple.block.Height)
			bBlockMeta := bs.LoadBlockMeta(tuple.block.Height)

			if tuple.eraseSeenCommitInDB {
				err := db.Delete(calcSeenCommitKey(tuple.block.Height))
				require.NoError(t, err)
			}
			if tuple.corruptSeenCommitInDB {
				err := db.Set(calcSeenCommitKey(tuple.block.Height), []byte("bogus-seen-commit"))
				require.NoError(t, err)
			}
			bSeenCommit := bs.LoadSeenCommit(tuple.block.Height)

			commitHeight := tuple.block.Height - 1
			if tuple.eraseCommitInDB {
				err := db.Delete(calcBlockCommitKey(commitHeight))
				require.NoError(t, err)
			}
			if tuple.corruptCommitInDB {
				err := db.Set(calcBlockCommitKey(commitHeight), []byte("foo-bogus"))
				require.NoError(t, err)
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

func TestLoadBaseMeta(t *testing.T) {
	config := cfg.ResetTestRoot("blockchain_reactor_test")
	defer os.RemoveAll(config.RootDir)
	stateStore := sm.NewStore(dbm.NewMemDB())
	state, err := stateStore.LoadFromDBOrGenesisFile(config.GenesisFile())
	require.NoError(t, err)
	bs := NewBlockStore(dbm.NewMemDB())

	for h := int64(1); h <= 10; h++ {
		block := makeBlock(h, state, new(types.Commit))
		partSet := block.MakePartSet(2)
		seenCommit := makeTestCommit(h, tmtime.Now())
		bs.SaveBlock(block, partSet, seenCommit)
	}

	_, err = bs.PruneBlocks(4)
	require.NoError(t, err)

	baseBlock := bs.LoadBaseMeta()
	assert.EqualValues(t, 4, baseBlock.Header.Height)
	assert.EqualValues(t, 4, bs.Base())
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
	err := db.Set(calcBlockPartKey(height, index), []byte("Tendermint"))
	require.NoError(t, err)
	res, _, panicErr = doFn(loadPart)
	require.NotNil(t, panicErr, "expecting a non-nil panic")
	require.Contains(t, panicErr.Error(), "unmarshal to tmproto.Part failed")

	// 3. A good block serialized and saved to the DB should be retrievable
	pb1, err := part1.ToProto()
	require.NoError(t, err)
	err = db.Set(calcBlockPartKey(height, index), mustEncode(pb1))
	require.NoError(t, err)
	gotPart, _, panicErr := doFn(loadPart)
	require.Nil(t, panicErr, "an existent and proper block should not panic")
	require.Nil(t, res, "a properly saved block should return a proper block")
	require.Equal(t, gotPart.(*types.Part), part1,
		"expecting successful retrieval of previously saved block")
}

func TestPruneBlocks(t *testing.T) {
	config := cfg.ResetTestRoot("blockchain_reactor_test")
	defer os.RemoveAll(config.RootDir)
	stateStore := sm.NewStore(dbm.NewMemDB())
	state, err := stateStore.LoadFromDBOrGenesisFile(config.GenesisFile())
	require.NoError(t, err)
	db := dbm.NewMemDB()
	bs := NewBlockStore(db)
	assert.EqualValues(t, 0, bs.Base())
	assert.EqualValues(t, 0, bs.Height())
	assert.EqualValues(t, 0, bs.Size())

	// pruning an empty store should error, even when pruning to 0
	_, err = bs.PruneBlocks(1)
	require.Error(t, err)

	_, err = bs.PruneBlocks(0)
	require.Error(t, err)

	// make more than 1000 blocks, to test batch deletions
	for h := int64(1); h <= 1500; h++ {
		block := makeBlock(h, state, new(types.Commit))
		partSet := block.MakePartSet(2)
		seenCommit := makeTestCommit(h, tmtime.Now())
		bs.SaveBlock(block, partSet, seenCommit)
	}

	assert.EqualValues(t, 1, bs.Base())
	assert.EqualValues(t, 1500, bs.Height())
	assert.EqualValues(t, 1500, bs.Size())

	prunedBlock := bs.LoadBlock(1199)

	// Check that basic pruning works
	pruned, err := bs.PruneBlocks(1200)
	require.NoError(t, err)
	assert.EqualValues(t, 1199, pruned)
	assert.EqualValues(t, 1200, bs.Base())
	assert.EqualValues(t, 1500, bs.Height())
	assert.EqualValues(t, 301, bs.Size())
	assert.EqualValues(t, tmstore.BlockStoreState{
		Base:   1200,
		Height: 1500,
	}, LoadBlockStoreState(db))

	require.NotNil(t, bs.LoadBlock(1200))
	require.Nil(t, bs.LoadBlock(1199))
	require.Nil(t, bs.LoadBlockByHash(prunedBlock.Hash()))
	require.Nil(t, bs.LoadBlockCommit(1199))
	require.Nil(t, bs.LoadBlockMeta(1199))
	require.Nil(t, bs.LoadBlockPart(1199, 1))

	for i := int64(1); i < 1200; i++ {
		require.Nil(t, bs.LoadBlock(i))
	}
	for i := int64(1200); i <= 1500; i++ {
		require.NotNil(t, bs.LoadBlock(i))
	}

	// Pruning below the current base should error
	_, err = bs.PruneBlocks(1199)
	require.Error(t, err)

	// Pruning to the current base should work
	pruned, err = bs.PruneBlocks(1200)
	require.NoError(t, err)
	assert.EqualValues(t, 0, pruned)

	// Pruning again should work
	pruned, err = bs.PruneBlocks(1300)
	require.NoError(t, err)
	assert.EqualValues(t, 100, pruned)
	assert.EqualValues(t, 1300, bs.Base())

	// Pruning beyond the current height should error
	_, err = bs.PruneBlocks(1501)
	require.Error(t, err)

	// Pruning to the current height should work
	pruned, err = bs.PruneBlocks(1500)
	require.NoError(t, err)
	assert.EqualValues(t, 200, pruned)
	assert.Nil(t, bs.LoadBlock(1499))
	assert.NotNil(t, bs.LoadBlock(1500))
	assert.Nil(t, bs.LoadBlock(1501))
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
	err := db.Set(calcBlockMetaKey(height), []byte("Tendermint-Meta"))
	require.NoError(t, err)
	res, _, panicErr = doFn(loadMeta)
	require.NotNil(t, panicErr, "expecting a non-nil panic")
	require.Contains(t, panicErr.Error(), "unmarshal to tmproto.BlockMeta")

	// 3. A good blockMeta serialized and saved to the DB should be retrievable
	meta := &types.BlockMeta{Header: types.Header{
		Version: tmversion.Consensus{
			Block: version.BlockProtocol, App: 0}, Height: 1, ProposerProTxHash: tmrand.Bytes(crypto.DefaultHashSize)}}
	pbm := meta.ToProto()
	err = db.Set(calcBlockMetaKey(height), mustEncode(pbm))
	require.NoError(t, err)
	gotMeta, _, panicErr := doFn(loadMeta)
	require.Nil(t, panicErr, "an existent and proper block should not panic")
	require.Nil(t, res, "a properly saved blockMeta should return a proper blocMeta ")
	pbmeta := meta.ToProto()
	if gmeta, ok := gotMeta.(*types.BlockMeta); ok {
		pbgotMeta := gmeta.ToProto()
		require.Equal(t, mustEncode(pbmeta), mustEncode(pbgotMeta),
			"expecting successful retrieval of previously saved blockMeta")
	}
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
	b1, err := block.ToProto()
	require.NoError(t, err)
	b2, err := blockAtHeight.ToProto()
	require.NoError(t, err)
	bz1 := mustEncode(b1)
	bz2 := mustEncode(b2)
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
