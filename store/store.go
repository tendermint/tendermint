package store

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/pkg/errors"

	db "github.com/tendermint/tm-db"
	dbm "github.com/tendermint/tm-db"

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

The store can be assumed to contain all contiguous blocks between base and height (inclusive).

// NOTE: BlockStore methods will panic if they encounter errors
// deserializing loaded data, indicating probable corruption on disk.
*/
type BlockStore struct {
	db dbm.DB

	mtx    sync.RWMutex
	base   int64
	height int64
}

// NewBlockStore returns a new BlockStore with the given DB,
// initialized to the last height that was committed to the DB.
func NewBlockStore(db dbm.DB) *BlockStore {
	bsjson := LoadBlockStoreStateJSON(db)
	return &BlockStore{
		base:   bsjson.Base,
		height: bsjson.Height,
		db:     db,
	}
}

// Base returns the first known contiguous block height, or 0 for empty block stores.
func (bs *BlockStore) Base() int64 {
	bs.mtx.RLock()
	defer bs.mtx.RUnlock()
	return bs.base
}

// Height returns the last known contiguous block height, or 0 for empty block stores.
func (bs *BlockStore) Height() int64 {
	bs.mtx.RLock()
	defer bs.mtx.RUnlock()
	return bs.height
}

// Size returns the number of blocks in the block store.
func (bs *BlockStore) Size() int64 {
	bs.mtx.RLock()
	defer bs.mtx.RUnlock()
	if bs.height == 0 {
		return 0
	}
	return bs.height - bs.base + 1
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
	err := cdc.UnmarshalBinaryLengthPrefixed(buf, block)
	if err != nil {
		// NOTE: The existence of meta should imply the existence of the
		// block. So, make sure meta is only saved after blocks are saved.
		panic(errors.Wrap(err, "Error reading block"))
	}
	return block
}

// LoadBlockByHash returns the block with the given hash.
// If no block is found for that hash, it returns nil.
// Panics if it fails to parse height associated with the given hash.
func (bs *BlockStore) LoadBlockByHash(hash []byte) *types.Block {
	bz, err := bs.db.Get(calcBlockHashKey(hash))
	if err != nil {
		panic(err)
	}
	if len(bz) == 0 {
		return nil
	}

	s := string(bz)
	height, err := strconv.ParseInt(s, 10, 64)

	if err != nil {
		panic(errors.Wrapf(err, "failed to extract height from %s", s))
	}
	return bs.LoadBlock(height)
}

// LoadBlockPart returns the Part at the given index
// from the block at the given height.
// If no part is found for the given height and index, it returns nil.
func (bs *BlockStore) LoadBlockPart(height int64, index int) *types.Part {
	var part = new(types.Part)
	bz, err := bs.db.Get(calcBlockPartKey(height, index))
	if err != nil {
		panic(err)
	}
	if len(bz) == 0 {
		return nil
	}
	err = cdc.UnmarshalBinaryBare(bz, part)
	if err != nil {
		panic(errors.Wrap(err, "Error reading block part"))
	}
	return part
}

// LoadBlockMeta returns the BlockMeta for the given height.
// If no block is found for the given height, it returns nil.
func (bs *BlockStore) LoadBlockMeta(height int64) *types.BlockMeta {
	var blockMeta = new(types.BlockMeta)
	bz, err := bs.db.Get(calcBlockMetaKey(height))
	if err != nil {
		panic(err)
	}
	if len(bz) == 0 {
		return nil
	}
	err = cdc.UnmarshalBinaryBare(bz, blockMeta)
	if err != nil {
		panic(errors.Wrap(err, "Error reading block meta"))
	}
	return blockMeta
}

// LoadBlockCommit returns the Commit for the given height.
// This commit consists of the +2/3 and other Precommit-votes for block at `height`,
// and it comes from the block.LastCommit for `height+1`.
// If no commit is found for the given height, it returns nil.
func (bs *BlockStore) LoadBlockCommit(height int64) *types.Commit {
	var commit = new(types.Commit)
	bz, err := bs.db.Get(calcBlockCommitKey(height))
	if err != nil {
		panic(err)
	}
	if len(bz) == 0 {
		return nil
	}
	err = cdc.UnmarshalBinaryBare(bz, commit)
	if err != nil {
		panic(errors.Wrap(err, "Error reading block commit"))
	}
	return commit
}

// LoadSeenCommit returns the locally seen Commit for the given height.
// This is useful when we've seen a commit, but there has not yet been
// a new block at `height + 1` that includes this commit in its block.LastCommit.
func (bs *BlockStore) LoadSeenCommit(height int64) *types.Commit {
	var commit = new(types.Commit)
	bz, err := bs.db.Get(calcSeenCommitKey(height))
	if err != nil {
		panic(err)
	}
	if len(bz) == 0 {
		return nil
	}
	err = cdc.UnmarshalBinaryBare(bz, commit)
	if err != nil {
		panic(errors.Wrap(err, "Error reading block seen commit"))
	}
	return commit
}

// PruneBlocks removes block up to (but not including) a height. It returns number of blocks pruned.
func (bs *BlockStore) PruneBlocks(height int64) (uint64, error) {
	if height <= 0 {
		return 0, fmt.Errorf("height must be greater than 0")
	}
	bs.mtx.RLock()
	if height > bs.height {
		bs.mtx.RUnlock()
		return 0, fmt.Errorf("cannot prune beyond the latest height %v", bs.height)
	}
	base := bs.base
	bs.mtx.RUnlock()
	if height < base {
		return 0, fmt.Errorf("cannot prune to height %v, it is lower than base height %v",
			height, base)
	}

	pruned := uint64(0)
	batch := bs.db.NewBatch()
	defer batch.Close()
	flush := func(batch db.Batch, base int64) error {
		// We can't trust batches to be atomic, so update base first to make sure noone
		// tries to access missing blocks.
		bs.mtx.Lock()
		bs.base = base
		bs.mtx.Unlock()
		bs.saveState()

		err := batch.WriteSync()
		if err != nil {
			return fmt.Errorf("failed to prune up to height %v: %w", base, err)
		}
		batch.Close()
		return nil
	}

	for h := base; h < height; h++ {
		meta := bs.LoadBlockMeta(h)
		if meta == nil { // assume already deleted
			continue
		}
		batch.Delete(calcBlockMetaKey(h))
		batch.Delete(calcBlockHashKey(meta.BlockID.Hash))
		batch.Delete(calcBlockCommitKey(h))
		batch.Delete(calcSeenCommitKey(h))
		for p := 0; p < meta.BlockID.PartsHeader.Total; p++ {
			batch.Delete(calcBlockPartKey(h, p))
		}
		pruned++

		// flush every 1000 blocks to avoid batches becoming too large
		if pruned%1000 == 0 && pruned > 0 {
			err := flush(batch, h)
			if err != nil {
				return 0, err
			}
			batch = bs.db.NewBatch()
			defer batch.Close()
		}
	}

	err := flush(batch, height)
	if err != nil {
		return 0, err
	}
	return pruned, nil
}

// SaveBlock persists the given block, blockParts, and seenCommit to the underlying db.
// blockParts: Must be parts of the block
// seenCommit: The +2/3 precommits that were seen which committed at height.
//             If all the nodes restart after committing a block,
//             we need this to reload the precommits to catch-up nodes to the
//             most recent height.  Otherwise they'd stall at H-1.
func (bs *BlockStore) SaveBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit) {
	if block == nil {
		panic("BlockStore can only save a non-nil block")
	}

	height := block.Height
	hash := block.Hash()

	if g, w := height, bs.Height()+1; bs.Base() > 0 && g != w {
		panic(fmt.Sprintf("BlockStore can only save contiguous blocks. Wanted %v, got %v", w, g))
	}
	if !blockParts.IsComplete() {
		panic(fmt.Sprintf("BlockStore can only save complete block part sets"))
	}

	// Save block meta
	blockMeta := types.NewBlockMeta(block, blockParts)
	metaBytes := cdc.MustMarshalBinaryBare(blockMeta)
	bs.db.Set(calcBlockMetaKey(height), metaBytes)
	bs.db.Set(calcBlockHashKey(hash), []byte(fmt.Sprintf("%d", height)))

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

	// Done!
	bs.mtx.Lock()
	bs.height = height
	if bs.base == 0 {
		bs.base = height
	}
	bs.mtx.Unlock()

	// Save new BlockStoreStateJSON descriptor
	bs.saveState()

	// Flush
	bs.db.SetSync(nil, nil)
}

func (bs *BlockStore) saveBlockPart(height int64, index int, part *types.Part) {
	partBytes := cdc.MustMarshalBinaryBare(part)
	bs.db.Set(calcBlockPartKey(height, index), partBytes)
}

func (bs *BlockStore) saveState() {
	bs.mtx.RLock()
	bsJSON := BlockStoreStateJSON{
		Base:   bs.base,
		Height: bs.height,
	}
	bs.mtx.RUnlock()
	bsJSON.Save(bs.db)
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

func calcBlockHashKey(hash []byte) []byte {
	return []byte(fmt.Sprintf("BH:%x", hash))
}

//-----------------------------------------------------------------------------

var blockStoreKey = []byte("blockStore")

// BlockStoreStateJSON is the block store state JSON structure.
type BlockStoreStateJSON struct {
	Base   int64 `json:"base"`
	Height int64 `json:"height"`
}

// Save persists the blockStore state to the database as JSON.
func (bsj BlockStoreStateJSON) Save(db dbm.DB) {
	bytes, err := cdc.MarshalJSON(bsj)
	if err != nil {
		panic(fmt.Sprintf("Could not marshal state bytes: %v", err))
	}
	db.SetSync(blockStoreKey, bytes)
}

// LoadBlockStoreStateJSON returns the BlockStoreStateJSON as loaded from disk.
// If no BlockStoreStateJSON was previously persisted, it returns the zero value.
func LoadBlockStoreStateJSON(db dbm.DB) BlockStoreStateJSON {
	bytes, err := db.Get(blockStoreKey)
	if err != nil {
		panic(err)
	}
	if len(bytes) == 0 {
		return BlockStoreStateJSON{
			Base:   0,
			Height: types.GetStartBlockHeight(),
		}
	}
	bsj := BlockStoreStateJSON{}
	err = cdc.UnmarshalJSON(bytes, &bsj)
	if err != nil {
		panic(fmt.Sprintf("Could not unmarshal bytes: %X", bytes))
	}
	// Backwards compatibility with persisted data from before Base existed.
	if bsj.Height > 0 && bsj.Base == 0 {
		bsj.Base = 1
	}
	return bsj
}
