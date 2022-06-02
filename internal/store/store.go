package store

import (
	"bytes"
	"errors"
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
		panic(err)
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
		panic(err)
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
		panic(fmt.Errorf("error reading block: %w", err))
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

// LoadBlockMetaByHash returns the blockmeta who's header corresponds to the given
// hash. If none is found, returns nil.
func (bs *BlockStore) LoadBlockMetaByHash(hash []byte) *types.BlockMeta {
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
	return bs.LoadBlockMeta(height)
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
		panic(fmt.Errorf("error reading block part: %w", err))
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
		panic(fmt.Errorf("converting commit to proto: %w", err))
	}
	return commit
}

// LoadExtendedCommit returns the ExtendedCommit for the given height.
// The extended commit is not guaranteed to contain the same +2/3 precommits data
// as the commit in the block.
func (bs *BlockStore) LoadBlockExtendedCommit(height int64) *types.ExtendedCommit {
	pbec := new(tmproto.ExtendedCommit)
	bz, err := bs.db.Get(extCommitKey(height))
	if err != nil {
		panic(fmt.Errorf("fetching extended commit: %w", err))
	}
	if len(bz) == 0 {
		return nil
	}
	err = proto.Unmarshal(bz, pbec)
	if err != nil {
		panic(fmt.Errorf("decoding extended commit: %w", err))
	}
	extCommit, err := types.ExtendedCommitFromProto(pbec)
	if err != nil {
		panic(fmt.Errorf("converting extended commit: %w", err))
	}
	return extCommit
}

// LoadSeenCommit returns the last locally seen Commit before being
// cannonicalized. This is useful when we've seen a commit, but there
// has not yet been a new block at `height + 1` that includes this
// commit in its block.LastCommit.
func (bs *BlockStore) LoadSeenCommit() *types.Commit {
	var pbc = new(tmproto.Commit)
	bz, err := bs.db.Get(seenCommitKey())
	if err != nil {
		panic(err)
	}
	if len(bz) == 0 {
		return nil
	}
	err = proto.Unmarshal(bz, pbc)
	if err != nil {
		panic(fmt.Errorf("error reading block seen commit: %w", err))
	}

	commit, err := types.CommitFromProto(pbc)
	if err != nil {
		panic(fmt.Errorf("converting seen commit: %w", err))
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
	removeBlockHash := func(key, value []byte, batch dbm.Batch) error {
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
	pruned, err := bs.pruneRange(blockMetaKey(0), blockMetaKey(height), removeBlockHash)
	if err != nil {
		return pruned, err
	}

	if _, err := bs.pruneRange(blockPartKey(0, 0), blockPartKey(height, 0), nil); err != nil {
		return pruned, err
	}

	if _, err := bs.pruneRange(blockCommitKey(0), blockCommitKey(height), nil); err != nil {
		return pruned, err
	}

	return pruned, nil
}

// pruneRange is a generic function for deleting a range of values based on the lowest
// height up to but excluding retainHeight. For each key/value pair, an optional hook can be
// executed before the deletion itself is made. pruneRange will use batch delete to delete
// keys in batches of at most 1000 keys.
func (bs *BlockStore) pruneRange(
	start []byte,
	end []byte,
	preDeletionHook func(key, value []byte, batch dbm.Batch) error,
) (uint64, error) {
	var (
		err         error
		pruned      uint64
		totalPruned uint64
	)

	batch := bs.db.NewBatch()
	defer batch.Close()

	pruned, start, err = bs.batchDelete(batch, start, end, preDeletionHook)
	if err != nil {
		return totalPruned, err
	}

	// loop until we have finished iterating over all the keys by writing, opening a new batch
	// and incrementing through the next range of keys.
	for !bytes.Equal(start, end) {
		if err := batch.Write(); err != nil {
			return totalPruned, err
		}

		totalPruned += pruned

		if err := batch.Close(); err != nil {
			return totalPruned, err
		}

		batch = bs.db.NewBatch()

		pruned, start, err = bs.batchDelete(batch, start, end, preDeletionHook)
		if err != nil {
			return totalPruned, err
		}
	}

	// once we looped over all keys we do a final flush to disk
	if err := batch.WriteSync(); err != nil {
		return totalPruned, err
	}
	totalPruned += pruned
	return totalPruned, nil
}

// batchDelete runs an iterator over a set of keys, first preforming a pre deletion hook before adding it to the batch.
// The function ends when either 1000 keys have been added to the batch or the iterator has reached the end.
func (bs *BlockStore) batchDelete(
	batch dbm.Batch,
	start, end []byte,
	preDeletionHook func(key, value []byte, batch dbm.Batch) error,
) (uint64, []byte, error) {
	var pruned uint64
	iter, err := bs.db.Iterator(start, end)
	if err != nil {
		return pruned, start, err
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		key := iter.Key()
		if preDeletionHook != nil {
			if err := preDeletionHook(key, iter.Value(), batch); err != nil {
				return 0, start, fmt.Errorf("pruning error at key %X: %w", iter.Key(), err)
			}
		}

		if err := batch.Delete(key); err != nil {
			return 0, start, fmt.Errorf("pruning error at key %X: %w", iter.Key(), err)
		}

		pruned++
		if pruned == 1000 {
			return pruned, iter.Key(), iter.Error()
		}
	}

	return pruned, end, iter.Error()
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
	batch := bs.db.NewBatch()
	if err := bs.saveBlockToBatch(batch, block, blockParts, seenCommit); err != nil {
		panic(err)
	}

	if err := batch.WriteSync(); err != nil {
		panic(err)
	}

	if err := batch.Close(); err != nil {
		panic(err)
	}
}

// SaveBlockWithExtendedCommit persists the given block, blockParts, and
// seenExtendedCommit to the underlying db. seenExtendedCommit is stored under
// two keys in the database: as the seenCommit and as the ExtendedCommit data for the
// height. This allows the vote extension data to be persisted for all blocks
// that are saved.
func (bs *BlockStore) SaveBlockWithExtendedCommit(block *types.Block, blockParts *types.PartSet, seenExtendedCommit *types.ExtendedCommit) {
	if block == nil {
		panic("BlockStore can only save a non-nil block")
	}
	if err := seenExtendedCommit.EnsureExtensions(); err != nil {
		panic(fmt.Errorf("saving block with extensions: %w", err))
	}
	batch := bs.db.NewBatch()
	if err := bs.saveBlockToBatch(batch, block, blockParts, seenExtendedCommit.ToCommit()); err != nil {
		panic(err)
	}
	height := block.Height

	pbec := seenExtendedCommit.ToProto()
	extCommitBytes := mustEncode(pbec)
	if err := batch.Set(extCommitKey(height), extCommitBytes); err != nil {
		panic(err)
	}

	if err := batch.WriteSync(); err != nil {
		panic(err)
	}

	if err := batch.Close(); err != nil {
		panic(err)
	}
}

func (bs *BlockStore) saveBlockToBatch(batch dbm.Batch, block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit) error {
	if block == nil {
		panic("BlockStore can only save a non-nil block")
	}

	height := block.Height
	hash := block.Hash()

	if g, w := height, bs.Height()+1; bs.Base() > 0 && g != w {
		return fmt.Errorf("BlockStore can only save contiguous blocks. Wanted %v, got %v", w, g)
	}
	if !blockParts.IsComplete() {
		return errors.New("BlockStore can only save complete block part sets")
	}
	if height != seenCommit.Height {
		return fmt.Errorf("BlockStore cannot save seen commit of a different height (block: %d, commit: %d)", height, seenCommit.Height)
	}

	// Save block parts. This must be done before the block meta, since callers
	// typically load the block meta first as an indication that the block exists
	// and then go on to load block parts - we must make sure the block is
	// complete as soon as the block meta is written.
	for i := 0; i < int(blockParts.Total()); i++ {
		part := blockParts.GetPart(i)
		bs.saveBlockPart(height, i, part, batch)
	}

	blockMeta := types.NewBlockMeta(block, blockParts)
	pbm := blockMeta.ToProto()
	if pbm == nil {
		return errors.New("nil blockmeta")
	}

	metaBytes := mustEncode(pbm)
	if err := batch.Set(blockMetaKey(height), metaBytes); err != nil {
		return err
	}

	if err := batch.Set(blockHashKey(hash), []byte(fmt.Sprintf("%d", height))); err != nil {
		return err
	}

	pbc := block.LastCommit.ToProto()
	blockCommitBytes := mustEncode(pbc)
	if err := batch.Set(blockCommitKey(height-1), blockCommitBytes); err != nil {
		return err
	}

	// Save seen commit (seen +2/3 precommits for block)
	pbsc := seenCommit.ToProto()
	seenCommitBytes := mustEncode(pbsc)
	if err := batch.Set(seenCommitKey(), seenCommitBytes); err != nil {
		return err
	}

	return nil
}

func (bs *BlockStore) saveBlockPart(height int64, index int, part *types.Part, batch dbm.Batch) {
	pbp, err := part.ToProto()
	if err != nil {
		panic(fmt.Errorf("unable to make part into proto: %w", err))
	}
	partBytes := mustEncode(pbp)
	if err := batch.Set(blockPartKey(height, index), partBytes); err != nil {
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
	return bs.db.Set(seenCommitKey(), seenCommitBytes)
}

func (bs *BlockStore) SaveSignedHeader(sh *types.SignedHeader, blockID types.BlockID) error {
	// first check that the block store doesn't already have the block
	bz, err := bs.db.Get(blockMetaKey(sh.Height))
	if err != nil {
		return err
	}
	if bz != nil {
		return fmt.Errorf("block at height %d already saved", sh.Height)
	}

	// FIXME: saving signed headers although necessary for proving evidence,
	// doesn't have complete parity with block meta's thus block size and num
	// txs are filled with negative numbers. We should aim to find a solution to
	// this.
	blockMeta := &types.BlockMeta{
		BlockID:   blockID,
		BlockSize: -1,
		Header:    *sh.Header,
		NumTxs:    -1,
	}

	batch := bs.db.NewBatch()

	pbm := blockMeta.ToProto()
	metaBytes := mustEncode(pbm)
	if err := batch.Set(blockMetaKey(sh.Height), metaBytes); err != nil {
		return fmt.Errorf("unable to save block meta: %w", err)
	}

	pbc := sh.Commit.ToProto()
	blockCommitBytes := mustEncode(pbc)
	if err := batch.Set(blockCommitKey(sh.Height), blockCommitBytes); err != nil {
		return fmt.Errorf("unable to save commit: %w", err)
	}

	if err := batch.WriteSync(); err != nil {
		return err
	}

	return batch.Close()
}

func (bs *BlockStore) Close() error {
	return bs.db.Close()
}

//---------------------------------- KEY ENCODING -----------------------------------------

// key prefixes
// NB: Before modifying these, cross-check them with those in
// * internal/store/store.go    [0..4, 13]
// * internal/state/store.go    [5..8, 14]
// * internal/evidence/pool.go  [9..10]
// * light/store/db/db.go       [11..12]
// TODO(thane): Move all these to their own package.
// TODO: what about these (they already collide):
// * scripts/scmigrate/migrate.go [3] --> Looks OK, as it is also called "SeenCommit"
// * internal/p2p/peermanager.go  [1]
const (
	// prefixes are unique across all tm db's
	prefixBlockMeta   = int64(0)
	prefixBlockPart   = int64(1)
	prefixBlockCommit = int64(2)
	prefixSeenCommit  = int64(3)
	prefixBlockHash   = int64(4)
	prefixExtCommit   = int64(13)
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

func seenCommitKey() []byte {
	key, err := orderedcode.Append(nil, prefixSeenCommit)
	if err != nil {
		panic(err)
	}
	return key
}

func extCommitKey(height int64) []byte {
	key, err := orderedcode.Append(nil, prefixExtCommit, height)
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
