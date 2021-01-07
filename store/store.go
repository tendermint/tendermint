package store

import (
	"fmt"
	"strconv"

	"github.com/gogo/protobuf/proto"
	"github.com/google/orderedcode"
	dbm "github.com/tendermint/tm-db"

	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
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
}

// NewBlockStore returns a new BlockStore with the given DB,
// initialized to the last height that was committed to the DB.
func NewBlockStore(db dbm.DB) *BlockStore {
	return &BlockStore{db}
}

// Base returns the first known contiguous block height, or 0 for empty block stores.
func (bs *BlockStore) Base() int64 {
	iter, err := bs.db.Iterator(
		blockMetaKey(1),
		blockMetaKey(1<<63-1),
	)
	if err != nil {
		return 0
	}
	defer iter.Close()

	if iter.Valid() {
		height, err := decodeBlockMetaKey(iter.Key())
		if err == nil {
			return height
		}
	}
	if err := iter.Error(); err != nil {
		panic(err)
	}

	return 0
}

// Height returns the last known contiguous block height, or 0 for empty block stores.
func (bs *BlockStore) Height() int64 {
	iter, err := bs.db.ReverseIterator(
		blockMetaKey(1),
		blockMetaKey(1<<63-1),
	)
	if err != nil {
		return 0
	}
	defer iter.Close()

	if iter.Valid() {
		height, err := decodeBlockMetaKey(iter.Key())
		if err == nil {
			return height
		}
	}
	if err := iter.Error(); err != nil {
		panic(err)
	}

	return 0
}

// Size returns the number of blocks in the block store.
func (bs *BlockStore) Size() int64 {
	height := bs.Height()
	if height == 0 {
		return 0
	}
	return height + 1 - bs.Base()
}

// LoadBase atomically loads the base block meta, or returns nil if no base is found.
func (bs *BlockStore) LoadBaseMeta() *types.BlockMeta {
	iter, err := bs.db.Iterator(
		blockMetaKey(1),
		blockMetaKey(1<<63-1),
	)
	if err != nil {
		return nil
	}
	defer iter.Close()

	if iter.Valid() {
		var pbbm = new(tmproto.BlockMeta)
		err = proto.Unmarshal(iter.Value(), pbbm)
		if err != nil {
			panic(fmt.Errorf("unmarshal to tmproto.BlockMeta: %w", err))
		}

		blockMeta, err := types.BlockMetaFromProto(pbbm)
		if err != nil {
			panic(fmt.Errorf("error from proto blockMeta: %w", err))
		}

		return blockMeta
	}

	return nil
}

// LoadBlock returns the block with the given height.
// If no block is found for that height, it returns nil.
func (bs *BlockStore) LoadBlock(height int64) *types.Block {
	var blockMeta = bs.LoadBlockMeta(height)
	if blockMeta == nil {
		return nil
	}

	pbb := new(tmproto.Block)
	buf := []byte{}
	for i := 0; i < int(blockMeta.BlockID.PartSetHeader.Total); i++ {
		part := bs.LoadBlockPart(height, i)
		// If the part is missing (e.g. since it has been deleted after we
		// loaded the block meta) we consider the whole block to be missing.
		if part == nil {
			return nil
		}
		buf = append(buf, part.Bytes...)
	}
	err := proto.Unmarshal(buf, pbb)
	if err != nil {
		// NOTE: The existence of meta should imply the existence of the
		// block. So, make sure meta is only saved after blocks are saved.
		panic(fmt.Sprintf("Error reading block: %v", err))
	}

	block, err := types.BlockFromProto(pbb)
	if err != nil {
		panic(fmt.Errorf("error from proto block: %w", err))
	}

	return block
}

// LoadBlockByHash returns the block with the given hash.
// If no block is found for that hash, it returns nil.
// Panics if it fails to parse height associated with the given hash.
func (bs *BlockStore) LoadBlockByHash(hash []byte) *types.Block {
	bz, err := bs.db.Get(blockHashKey(hash))
	if err != nil {
		panic(err)
	}
	if len(bz) == 0 {
		return nil
	}

	s := string(bz)
	height, err := strconv.ParseInt(s, 10, 64)

	if err != nil {
		panic(fmt.Sprintf("failed to extract height from %s: %v", s, err))
	}
	return bs.LoadBlock(height)
}

// LoadBlockPart returns the Part at the given index
// from the block at the given height.
// If no part is found for the given height and index, it returns nil.
func (bs *BlockStore) LoadBlockPart(height int64, index int) *types.Part {
	var pbpart = new(tmproto.Part)

	bz, err := bs.db.Get(blockPartKey(height, index))
	if err != nil {
		panic(err)
	}
	if len(bz) == 0 {
		return nil
	}

	err = proto.Unmarshal(bz, pbpart)
	if err != nil {
		panic(fmt.Errorf("unmarshal to tmproto.Part failed: %w", err))
	}
	part, err := types.PartFromProto(pbpart)
	if err != nil {
		panic(fmt.Sprintf("Error reading block part: %v", err))
	}

	return part
}

// LoadBlockMeta returns the BlockMeta for the given height.
// If no block is found for the given height, it returns nil.
func (bs *BlockStore) LoadBlockMeta(height int64) *types.BlockMeta {
	var pbbm = new(tmproto.BlockMeta)
	bz, err := bs.db.Get(blockMetaKey(height))

	if err != nil {
		panic(err)
	}

	if len(bz) == 0 {
		return nil
	}

	err = proto.Unmarshal(bz, pbbm)
	if err != nil {
		panic(fmt.Errorf("unmarshal to tmproto.BlockMeta: %w", err))
	}

	blockMeta, err := types.BlockMetaFromProto(pbbm)
	if err != nil {
		panic(fmt.Errorf("error from proto blockMeta: %w", err))
	}

	return blockMeta
}

// LoadBlockCommit returns the Commit for the given height.
// This commit consists of the +2/3 and other Precommit-votes for block at `height`,
// and it comes from the block.LastCommit for `height+1`.
// If no commit is found for the given height, it returns nil.
func (bs *BlockStore) LoadBlockCommit(height int64) *types.Commit {
	var pbc = new(tmproto.Commit)
	bz, err := bs.db.Get(blockCommitKey(height))
	if err != nil {
		panic(err)
	}
	if len(bz) == 0 {
		return nil
	}
	err = proto.Unmarshal(bz, pbc)
	if err != nil {
		panic(fmt.Errorf("error reading block commit: %w", err))
	}
	commit, err := types.CommitFromProto(pbc)
	if err != nil {
		panic(fmt.Sprintf("Error reading block commit: %v", err))
	}
	return commit
}

// LoadSeenCommit returns the locally seen Commit for the given height.
// This is useful when we've seen a commit, but there has not yet been
// a new block at `height + 1` that includes this commit in its block.LastCommit.
func (bs *BlockStore) LoadSeenCommit(height int64) *types.Commit {
	var pbc = new(tmproto.Commit)
	bz, err := bs.db.Get(seenCommitKey(height))
	if err != nil {
		panic(err)
	}
	if len(bz) == 0 {
		return nil
	}
	err = proto.Unmarshal(bz, pbc)
	if err != nil {
		panic(fmt.Sprintf("error reading block seen commit: %v", err))
	}

	commit, err := types.CommitFromProto(pbc)
	if err != nil {
		panic(fmt.Errorf("error from proto commit: %w", err))
	}
	return commit
}

// PruneBlocks removes block up to (but not including) a height. It returns the number of blocks pruned.
func (bs *BlockStore) PruneBlocks(height int64) (uint64, error) {
	if height <= 0 {
		return 0, fmt.Errorf("height must be greater than 0")
	}

	if height > bs.Height() {
		return 0, fmt.Errorf("height must be equal to or less than the latest height %d", bs.Height())
	}

	// when removing the block meta, use the hash to remove the hash key at the same time
	blockHashClause := func(key, value []byte, batch dbm.Batch) error {
		// unmarshal block meta
		var pbbm = new(tmproto.BlockMeta)
		err := proto.Unmarshal(value, pbbm)
		if err != nil {
			return fmt.Errorf("unmarshal to tmproto.BlockMeta: %w", err)
		}

		blockMeta, err := types.BlockMetaFromProto(pbbm)
		if err != nil {
			return fmt.Errorf("error from proto blockMeta: %w", err)
		}

		// delete the hash key corresponding to the block meta's hash
		if err := batch.Delete(blockHashKey(blockMeta.BlockID.Hash)); err != nil {
			return fmt.Errorf("failed to delete hash key: %X: %w", blockHashKey(blockMeta.BlockID.Hash), err)
		}

		return nil
	}

	// remove block meta first as this is used to indicate whether the block exists.
	// For this reason, we also use ony block meta as a measure of the amount of blocks pruned
	pruned, err := bs.batchDelete(blockMetaKey(0), blockMetaKey(height), blockHashClause)
	if err != nil {
		return pruned, err
	}

	// for the rest of the block data we just want to remove it
	emptyClause := func(key, value []byte, batch dbm.Batch) error { return nil }

	if _, err := bs.batchDelete(blockPartKey(0, 0), blockPartKey(height, 0), emptyClause); err != nil {
		return pruned, err
	}

	if _, err := bs.batchDelete(blockCommitKey(0), blockCommitKey(height), emptyClause); err != nil {
		return pruned, err
	}

	if _, err := bs.batchDelete(seenCommitKey(0), seenCommitKey(height), emptyClause); err != nil {
		return pruned, err
	}

	return pruned, nil
}

// batchDelete is a generic function for deleting a range of values based on the lowest
// height up to but excluding retainHeight
func (bs *BlockStore) batchDelete(
	start []byte,
	end []byte,
	clause func(key, value []byte, batch dbm.Batch) error) (uint64, error) {
	iter, err := bs.db.Iterator(start, end)
	if err != nil {
		panic(err)
	}
	defer iter.Close()

	batch := bs.db.NewBatch()
	defer batch.Close()

	pruned := uint64(0)
	flushed := pruned
	for iter.Valid() {
		key := iter.Key()
		if err := clause(key, iter.Value(), batch); err != nil {
			return flushed, err
		}

		if err := batch.Delete(key); err != nil {
			return flushed, fmt.Errorf("pruning error at key %X: %w", iter.Key(), err)
		}

		pruned++
		// avoid batches growing too large by flushing to database regularly
		if pruned%1000 == 0 {
			if err := iter.Error(); err != nil {
				return flushed, err
			}
			if err := iter.Close(); err != nil {
				return flushed, err
			}

			err := batch.Write()
			if err != nil {
				return flushed, fmt.Errorf("pruning error at key %X: %w", iter.Key(), err)
			}
			if err := batch.Close(); err != nil {
				return flushed, err
			}
			flushed = pruned

			iter, err = bs.db.Iterator(start, end)
			if err != nil {
				panic(err)
			}
			defer iter.Close()

			batch = bs.db.NewBatch()
			defer batch.Close()
		} else {
			iter.Next()
		}
	}
	flushed = pruned
	if err := iter.Error(); err != nil {
		return flushed, err
	}

	err = batch.WriteSync()
	if err != nil {
		return flushed, fmt.Errorf("pruning error at key %X: %w", iter.Key(), err)
	}

	return flushed, nil
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
		panic("BlockStore can only save complete block part sets")
	}

	// Save block parts. This must be done before the block meta, since callers
	// typically load the block meta first as an indication that the block exists
	// and then go on to load block parts - we must make sure the block is
	// complete as soon as the block meta is written.
	for i := 0; i < int(blockParts.Total()); i++ {
		part := blockParts.GetPart(i)
		bs.saveBlockPart(height, i, part)
	}

	// Save block meta
	blockMeta := types.NewBlockMeta(block, blockParts)
	pbm := blockMeta.ToProto()
	if pbm == nil {
		panic("nil blockmeta")
	}
	metaBytes := mustEncode(pbm)
	if err := bs.db.Set(blockMetaKey(height), metaBytes); err != nil {
		panic(err)
	}
	if err := bs.db.Set(blockHashKey(hash), []byte(fmt.Sprintf("%d", height))); err != nil {
		panic(err)
	}

	// Save block commit (duplicate and separate from the Block)
	pbc := block.LastCommit.ToProto()
	blockCommitBytes := mustEncode(pbc)
	if err := bs.db.Set(blockCommitKey(height-1), blockCommitBytes); err != nil {
		panic(err)
	}

	// Save seen commit (seen +2/3 precommits for block)
	// NOTE: we can delete this at a later height
	pbsc := seenCommit.ToProto()
	seenCommitBytes := mustEncode(pbsc)
	if err := bs.db.SetSync(seenCommitKey(height), seenCommitBytes); err != nil {
		panic(err)
	}

}

func (bs *BlockStore) saveBlockPart(height int64, index int, part *types.Part) {
	pbp, err := part.ToProto()
	if err != nil {
		panic(fmt.Errorf("unable to make part into proto: %w", err))
	}
	partBytes := mustEncode(pbp)
	if err := bs.db.Set(blockPartKey(height, index), partBytes); err != nil {
		panic(err)
	}
}

// SaveSeenCommit saves a seen commit, used by e.g. the state sync reactor when bootstrapping node.
func (bs *BlockStore) SaveSeenCommit(height int64, seenCommit *types.Commit) error {
	pbc := seenCommit.ToProto()
	seenCommitBytes, err := proto.Marshal(pbc)
	if err != nil {
		return fmt.Errorf("unable to marshal commit: %w", err)
	}
	return bs.db.Set(seenCommitKey(height), seenCommitBytes)
}

//---------------------------------- KEY ENCODING -----------------------------------------

// key prefixes
const (
	// prefixes are unique across all tm db's
	prefixBlockMeta   = int64(0)
	prefixBlockPart   = int64(1)
	prefixBlockCommit = int64(2)
	prefixSeenCommit  = int64(3)
	prefixBlockHash   = int64(4)
)

func blockMetaKey(height int64) []byte {
	key, err := orderedcode.Append(nil, prefixBlockMeta, height)
	if err != nil {
		panic(err)
	}
	return key
}

func decodeBlockMetaKey(key []byte) (height int64, err error) {
	var prefix int64
	remaining, err := orderedcode.Parse(string(key), &prefix, &height)
	if err != nil {
		return
	}
	if len(remaining) != 0 {
		return -1, fmt.Errorf("expected complete key but got remainder: %s", remaining)
	}
	if prefix != prefixBlockMeta {
		return -1, fmt.Errorf("incorrect prefix. Expected %v, got %v", prefixBlockMeta, prefix)
	}
	return
}

func blockPartKey(height int64, partIndex int) []byte {
	key, err := orderedcode.Append(nil, prefixBlockPart, height, int64(partIndex))
	if err != nil {
		panic(err)
	}
	return key
}

func blockCommitKey(height int64) []byte {
	key, err := orderedcode.Append(nil, prefixBlockCommit, height)
	if err != nil {
		panic(err)
	}
	return key
}

func seenCommitKey(height int64) []byte {
	key, err := orderedcode.Append(nil, prefixSeenCommit, height)
	if err != nil {
		panic(err)
	}
	return key
}

func blockHashKey(hash []byte) []byte {
	key, err := orderedcode.Append(nil, prefixBlockHash, string(hash))
	if err != nil {
		panic(err)
	}
	return key
}

//-----------------------------------------------------------------------------

// mustEncode proto encodes a proto.message and panics if fails
func mustEncode(pb proto.Message) []byte {
	bz, err := proto.Marshal(pb)
	if err != nil {
		panic(fmt.Errorf("unable to marshal: %w", err))
	}
	return bz
}
