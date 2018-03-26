package blockchain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	wire "github.com/tendermint/go-wire"

	cmn "github.com/tendermint/tmlibs/common"
	dbm "github.com/tendermint/tmlibs/db"

	"github.com/tendermint/tendermint/types"
)

/*
BlockStore is a simple low level store for blocks.

There are three types of information stored:
 - BlockMeta:   Meta information about each block
 - Block part:  Parts of each block, aggregated w/ PartSet
 - Commit:      The commit part of each block, for gossiping precommit votes

Currently the precommit signatures are duplicated in the Block parts as
well as the Commit.  In the future this may change, perhaps by moving
the Commit data outside the Block. (TODO)

// NOTE: BlockStore methods will panic if they encounter errors
// deserializing loaded data, indicating probable corruption on disk.
*/
type BlockStore struct {
	db dbm.DB

	mtx    sync.RWMutex
	height int64
}

// NewBlockStore returns a new BlockStore with the given DB,
// initialized to the last height that was committed to the DB.
func NewBlockStore(db dbm.DB) *BlockStore {
	bsjson := LoadBlockStoreStateJSON(db)
	return &BlockStore{
		height: bsjson.Height,
		db:     db,
	}
}

// Height returns the last known contiguous block height.
func (bs *BlockStore) Height() int64 {
	bs.mtx.RLock()
	defer bs.mtx.RUnlock()
	return bs.height
}

// GetReader returns the value associated with the given key wrapped in an io.Reader.
// If no value is found, it returns nil.
// It's mainly for use with wire.ReadBinary.
func (bs *BlockStore) GetReader(key []byte) io.Reader {
	bytez := bs.db.Get(key)
	if bytez == nil {
		return nil
	}
	return bytes.NewReader(bytez)
}

// LoadBlock returns the block with the given height.
// If no block is found for that height, it returns nil.
func (bs *BlockStore) LoadBlock(height int64) *types.Block {
	var n int
	var err error
	r := bs.GetReader(calcBlockMetaKey(height))
	if r == nil {
		return nil
	}
	blockMeta := wire.ReadBinary(&types.BlockMeta{}, r, 0, &n, &err).(*types.BlockMeta)
	if err != nil {
		panic(fmt.Sprintf("Error reading block meta: %v", err))
	}
	bytez := []byte{}
	for i := 0; i < blockMeta.BlockID.PartsHeader.Total; i++ {
		part := bs.LoadBlockPart(height, i)
		bytez = append(bytez, part.Bytes...)
	}
	block := wire.ReadBinary(&types.Block{}, bytes.NewReader(bytez), 0, &n, &err).(*types.Block)
	if err != nil {
		panic(fmt.Sprintf("Error reading block: %v", err))
	}
	return block
}

// LoadBlockPart returns the Part at the given index
// from the block at the given height.
// If no part is found for the given height and index, it returns nil.
func (bs *BlockStore) LoadBlockPart(height int64, index int) *types.Part {
	var n int
	var err error
	r := bs.GetReader(calcBlockPartKey(height, index))
	if r == nil {
		return nil
	}
	part := wire.ReadBinary(&types.Part{}, r, 0, &n, &err).(*types.Part)
	if err != nil {
		panic(fmt.Sprintf("Error reading block part: %v", err))
	}
	return part
}

// LoadBlockMeta returns the BlockMeta for the given height.
// If no block is found for the given height, it returns nil.
func (bs *BlockStore) LoadBlockMeta(height int64) *types.BlockMeta {
	var n int
	var err error
	r := bs.GetReader(calcBlockMetaKey(height))
	if r == nil {
		return nil
	}
	blockMeta := wire.ReadBinary(&types.BlockMeta{}, r, 0, &n, &err).(*types.BlockMeta)
	if err != nil {
		panic(fmt.Sprintf("Error reading block meta: %v", err))
	}
	return blockMeta
}

// LoadBlockCommit returns the Commit for the given height.
// This commit consists of the +2/3 and other Precommit-votes for block at `height`,
// and it comes from the block.LastCommit for `height+1`.
// If no commit is found for the given height, it returns nil.
func (bs *BlockStore) LoadBlockCommit(height int64) *types.Commit {
	var n int
	var err error
	r := bs.GetReader(calcBlockCommitKey(height))
	if r == nil {
		return nil
	}
	commit := wire.ReadBinary(&types.Commit{}, r, 0, &n, &err).(*types.Commit)
	if err != nil {
		panic(fmt.Sprintf("Error reading commit: %v", err))
	}
	return commit
}

// LoadSeenCommit returns the locally seen Commit for the given height.
// This is useful when we've seen a commit, but there has not yet been
// a new block at `height + 1` that includes this commit in its block.LastCommit.
func (bs *BlockStore) LoadSeenCommit(height int64) *types.Commit {
	var n int
	var err error
	r := bs.GetReader(calcSeenCommitKey(height))
	if r == nil {
		return nil
	}
	commit := wire.ReadBinary(&types.Commit{}, r, 0, &n, &err).(*types.Commit)
	if err != nil {
		panic(fmt.Sprintf("Error reading commit: %v", err))
	}
	return commit
}

// SaveBlock persists the given block, blockParts, and seenCommit to the underlying db.
// blockParts: Must be parts of the block
// seenCommit: The +2/3 precommits that were seen which committed at height.
//             If all the nodes restart after committing a block,
//             we need this to reload the precommits to catch-up nodes to the
//             most recent height.  Otherwise they'd stall at H-1.
func (bs *BlockStore) SaveBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit) {
	if block == nil {
		cmn.PanicSanity("BlockStore can only save a non-nil block")
	}
	height := block.Height
	if g, w := height, bs.Height()+1; g != w {
		cmn.PanicSanity(cmn.Fmt("BlockStore can only save contiguous blocks. Wanted %v, got %v", w, g))
	}
	if !blockParts.IsComplete() {
		cmn.PanicSanity(cmn.Fmt("BlockStore can only save complete block part sets"))
	}

	// Save block meta
	blockMeta := types.NewBlockMeta(block, blockParts)
	metaBytes := wire.BinaryBytes(blockMeta)
	bs.db.Set(calcBlockMetaKey(height), metaBytes)

	// Save block parts
	for i := 0; i < blockParts.Total(); i++ {
		bs.saveBlockPart(height, i, blockParts.GetPart(i))
	}

	// Save block commit (duplicate and separate from the Block)
	blockCommitBytes := wire.BinaryBytes(block.LastCommit)
	bs.db.Set(calcBlockCommitKey(height-1), blockCommitBytes)

	// Save seen commit (seen +2/3 precommits for block)
	// NOTE: we can delete this at a later height
	seenCommitBytes := wire.BinaryBytes(seenCommit)
	bs.db.Set(calcSeenCommitKey(height), seenCommitBytes)

	// Save new BlockStoreStateJSON descriptor
	BlockStoreStateJSON{Height: height}.Save(bs.db)

	// Done!
	bs.mtx.Lock()
	bs.height = height
	bs.mtx.Unlock()

	// Flush
	bs.db.SetSync(nil, nil)
}

func (bs *BlockStore) saveBlockPart(height int64, index int, part *types.Part) {
	if height != bs.Height()+1 {
		cmn.PanicSanity(cmn.Fmt("BlockStore can only save contiguous blocks. Wanted %v, got %v", bs.Height()+1, height))
	}
	partBytes := wire.BinaryBytes(part)
	bs.db.Set(calcBlockPartKey(height, index), partBytes)
}

//-----------------------------------------------------------------------------

func calcBlockMetaKey(height int64) []byte {
	return []byte(fmt.Sprintf("H:%v", height))
}

func calcBlockPartKey(height int64, partIndex int) []byte {
	return []byte(fmt.Sprintf("P:%v:%v", height, partIndex))
}

func calcBlockCommitKey(height int64) []byte {
	return []byte(fmt.Sprintf("C:%v", height))
}

func calcSeenCommitKey(height int64) []byte {
	return []byte(fmt.Sprintf("SC:%v", height))
}

//-----------------------------------------------------------------------------

var blockStoreKey = []byte("blockStore")

type BlockStoreStateJSON struct {
	Height int64
}

// Save persists the blockStore state to the database as JSON.
func (bsj BlockStoreStateJSON) Save(db dbm.DB) {
	bytes, err := json.Marshal(bsj)
	if err != nil {
		cmn.PanicSanity(cmn.Fmt("Could not marshal state bytes: %v", err))
	}
	db.SetSync(blockStoreKey, bytes)
}

// LoadBlockStoreStateJSON returns the BlockStoreStateJSON as loaded from disk.
// If no BlockStoreStateJSON was previously persisted, it returns the zero value.
func LoadBlockStoreStateJSON(db dbm.DB) BlockStoreStateJSON {
	bytes := db.Get(blockStoreKey)
	if len(bytes) == 0 {
		return BlockStoreStateJSON{
			Height: 0,
		}
	}
	bsj := BlockStoreStateJSON{}
	err := json.Unmarshal(bytes, &bsj)
	if err != nil {
		panic(fmt.Sprintf("Could not unmarshal bytes: %X", bytes))
	}
	return bsj
}
