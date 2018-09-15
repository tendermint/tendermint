package blockchain

import (
	"fmt"
	"sync"

	cmn "github.com/tendermint/tendermint/libs/common"
	dbm "github.com/tendermint/tendermint/libs/db"

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

// LoadBlock returns the block with the given height.
// If no block is found for that height, it returns nil.
func (bs *BlockStore) LoadBlock(height int64) *types.Block {
	var blockMeta = bs.LoadBlockMeta(height)
	if blockMeta == nil {
		return nil
	}

	var block = new(types.Block)
	buf := []byte{}
	for i := 0; i < blockMeta.BlockID.PartsHeader.Total; i++ {
		part := bs.LoadBlockPart(height, i)
		buf = append(buf, part.Bytes...)
	}
	err := cdc.UnmarshalBinary(buf, block)
	if err != nil {
		// NOTE: The existence of meta should imply the existence of the
		// block. So, make sure meta is only saved after blocks are saved.
		panic(cmn.ErrorWrap(err, "Error reading block"))
	}
	return block
}

// LoadBlockPart returns the Part at the given index
// from the block at the given height.
// If no part is found for the given height and index, it returns nil.
func (bs *BlockStore) LoadBlockPart(height int64, index int) *types.Part {
	var part = new(types.Part)
	bz := bs.db.Get(calcBlockPartKey(height, index))
	if len(bz) == 0 {
		return nil
	}
	err := cdc.UnmarshalBinaryBare(bz, part)
	if err != nil {
		panic(cmn.ErrorWrap(err, "Error reading block part"))
	}
	return part
}

// LoadBlockMeta returns the BlockMeta for the given height.
// If no block is found for the given height, it returns nil.
func (bs *BlockStore) LoadBlockMeta(height int64) *types.BlockMeta {
	var blockMeta = new(types.BlockMeta)
	bz := bs.db.Get(calcBlockMetaKey(height))
	if len(bz) == 0 {
		return nil
	}
	err := cdc.UnmarshalBinaryBare(bz, blockMeta)
	if err != nil {
		panic(cmn.ErrorWrap(err, "Error reading block meta"))
	}
	return blockMeta
}

// LoadBlockCommit returns the Commit for the given height.
// This commit consists of the +2/3 and other Precommit-votes for block at `height`,
// and it comes from the block.LastCommit for `height+1`.
// If no commit is found for the given height, it returns nil.
func (bs *BlockStore) LoadBlockCommit(height int64) *types.Commit {
	var commit = new(types.Commit)
	bz := bs.db.Get(calcBlockCommitKey(height))
	if len(bz) == 0 {
		return nil
	}
	err := cdc.UnmarshalBinaryBare(bz, commit)
	if err != nil {
		panic(cmn.ErrorWrap(err, "Error reading block commit"))
	}
	return commit
}

// LoadSeenCommit returns the locally seen Commit for the given height.
// This is useful when we've seen a commit, but there has not yet been
// a new block at `height + 1` that includes this commit in its block.LastCommit.
func (bs *BlockStore) LoadSeenCommit(height int64) *types.Commit {
	var commit = new(types.Commit)
	bz := bs.db.Get(calcSeenCommitKey(height))
	if len(bz) == 0 {
		return nil
	}
	err := cdc.UnmarshalBinaryBare(bz, commit)
	if err != nil {
		panic(cmn.ErrorWrap(err, "Error reading block seen commit"))
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
		cmn.PanicSanity(fmt.Sprintf("BlockStore can only save contiguous blocks. Wanted %v, got %v", w, g))
	}
	if !blockParts.IsComplete() {
		cmn.PanicSanity(fmt.Sprintf("BlockStore can only save complete block part sets"))
	}

	// Save block meta
	blockMeta := types.NewBlockMeta(block, blockParts)
	metaBytes := cdc.MustMarshalBinaryBare(blockMeta)
	bs.db.Set(calcBlockMetaKey(height), metaBytes)

	// Save block parts
	for i := 0; i < blockParts.Total(); i++ {
		part := blockParts.GetPart(i)
		bs.saveBlockPart(height, i, part)
	}

	// Save block commit (duplicate and separate from the Block)
	blockCommitBytes := cdc.MustMarshalBinaryBare(block.LastCommit)
	bs.db.Set(calcBlockCommitKey(height-1), blockCommitBytes)

	// Save seen commit (seen +2/3 precommits for block)
	// NOTE: we can delete this at a later height
	seenCommitBytes := cdc.MustMarshalBinaryBare(seenCommit)
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
		cmn.PanicSanity(fmt.Sprintf("BlockStore can only save contiguous blocks. Wanted %v, got %v", bs.Height()+1, height))
	}
	partBytes := cdc.MustMarshalBinaryBare(part)
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
	Height int64 `json:"height"`
}

// Save persists the blockStore state to the database as JSON.
func (bsj BlockStoreStateJSON) Save(db dbm.DB) {
	bytes, err := cdc.MarshalJSON(bsj)
	if err != nil {
		cmn.PanicSanity(fmt.Sprintf("Could not marshal state bytes: %v", err))
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
	err := cdc.UnmarshalJSON(bytes, &bsj)
	if err != nil {
		panic(fmt.Sprintf("Could not unmarshal bytes: %X", bytes))
	}
	return bsj
}
