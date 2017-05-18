package blockchain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	. "github.com/tendermint/tmlibs/common"
	dbm "github.com/tendermint/tmlibs/db"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/types"
)

/*
Simple low level store for blocks.

There are three types of information stored:
 - BlockMeta:   Meta information about each block
 - Block part:  Parts of each block, aggregated w/ PartSet
 - Commit:      The commit part of each block, for gossiping precommit votes

Currently the precommit signatures are duplicated in the Block parts as
well as the Commit.  In the future this may change, perhaps by moving
the Commit data outside the Block.

Panics indicate probable corruption in the data
*/
type BlockStore struct {
	db dbm.DB

	mtx    sync.RWMutex
	height int
}

func NewBlockStore(db dbm.DB) *BlockStore {
	bsjson := LoadBlockStoreStateJSON(db)
	return &BlockStore{
		height: bsjson.Height,
		db:     db,
	}
}

// Height() returns the last known contiguous block height.
func (bs *BlockStore) Height() int {
	bs.mtx.RLock()
	defer bs.mtx.RUnlock()
	return bs.height
}

func (bs *BlockStore) GetReader(key []byte) io.Reader {
	bytez := bs.db.Get(key)
	if bytez == nil {
		return nil
	}
	return bytes.NewReader(bytez)
}

func (bs *BlockStore) LoadBlock(height int) *types.Block {
	var n int
	var err error
	r := bs.GetReader(calcBlockMetaKey(height))
	if r == nil {
		return nil
	}
	blockMeta := wire.ReadBinary(&types.BlockMeta{}, r, 0, &n, &err).(*types.BlockMeta)
	if err != nil {
		PanicCrisis(Fmt("Error reading block meta: %v", err))
	}
	bytez := []byte{}
	for i := 0; i < blockMeta.BlockID.PartsHeader.Total; i++ {
		part := bs.LoadBlockPart(height, i)
		bytez = append(bytez, part.Bytes...)
	}
	block := wire.ReadBinary(&types.Block{}, bytes.NewReader(bytez), 0, &n, &err).(*types.Block)
	if err != nil {
		PanicCrisis(Fmt("Error reading block: %v", err))
	}
	return block
}

func (bs *BlockStore) LoadBlockPart(height int, index int) *types.Part {
	var n int
	var err error
	r := bs.GetReader(calcBlockPartKey(height, index))
	if r == nil {
		return nil
	}
	part := wire.ReadBinary(&types.Part{}, r, 0, &n, &err).(*types.Part)
	if err != nil {
		PanicCrisis(Fmt("Error reading block part: %v", err))
	}
	return part
}

func (bs *BlockStore) LoadBlockMeta(height int) *types.BlockMeta {
	var n int
	var err error
	r := bs.GetReader(calcBlockMetaKey(height))
	if r == nil {
		return nil
	}
	blockMeta := wire.ReadBinary(&types.BlockMeta{}, r, 0, &n, &err).(*types.BlockMeta)
	if err != nil {
		PanicCrisis(Fmt("Error reading block meta: %v", err))
	}
	return blockMeta
}

// The +2/3 and other Precommit-votes for block at `height`.
// This Commit comes from block.LastCommit for `height+1`.
func (bs *BlockStore) LoadBlockCommit(height int) *types.Commit {
	var n int
	var err error
	r := bs.GetReader(calcBlockCommitKey(height))
	if r == nil {
		return nil
	}
	commit := wire.ReadBinary(&types.Commit{}, r, 0, &n, &err).(*types.Commit)
	if err != nil {
		PanicCrisis(Fmt("Error reading commit: %v", err))
	}
	return commit
}

// NOTE: the Precommit-vote heights are for the block at `height`
func (bs *BlockStore) LoadSeenCommit(height int) *types.Commit {
	var n int
	var err error
	r := bs.GetReader(calcSeenCommitKey(height))
	if r == nil {
		return nil
	}
	commit := wire.ReadBinary(&types.Commit{}, r, 0, &n, &err).(*types.Commit)
	if err != nil {
		PanicCrisis(Fmt("Error reading commit: %v", err))
	}
	return commit
}

// blockParts: Must be parts of the block
// seenCommit: The +2/3 precommits that were seen which committed at height.
//             If all the nodes restart after committing a block,
//             we need this to reload the precommits to catch-up nodes to the
//             most recent height.  Otherwise they'd stall at H-1.
func (bs *BlockStore) SaveBlock(block *types.Block, blockParts *types.PartSet, seenCommit *types.Commit) {
	height := block.Height
	if height != bs.Height()+1 {
		PanicSanity(Fmt("BlockStore can only save contiguous blocks. Wanted %v, got %v", bs.Height()+1, height))
	}
	if !blockParts.IsComplete() {
		PanicSanity(Fmt("BlockStore can only save complete block part sets"))
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

func (bs *BlockStore) saveBlockPart(height int, index int, part *types.Part) {
	if height != bs.Height()+1 {
		PanicSanity(Fmt("BlockStore can only save contiguous blocks. Wanted %v, got %v", bs.Height()+1, height))
	}
	partBytes := wire.BinaryBytes(part)
	bs.db.Set(calcBlockPartKey(height, index), partBytes)
}

//-----------------------------------------------------------------------------

func calcBlockMetaKey(height int) []byte {
	return []byte(fmt.Sprintf("H:%v", height))
}

func calcBlockPartKey(height int, partIndex int) []byte {
	return []byte(fmt.Sprintf("P:%v:%v", height, partIndex))
}

func calcBlockCommitKey(height int) []byte {
	return []byte(fmt.Sprintf("C:%v", height))
}

func calcSeenCommitKey(height int) []byte {
	return []byte(fmt.Sprintf("SC:%v", height))
}

//-----------------------------------------------------------------------------

var blockStoreKey = []byte("blockStore")

type BlockStoreStateJSON struct {
	Height int
}

func (bsj BlockStoreStateJSON) Save(db dbm.DB) {
	bytes, err := json.Marshal(bsj)
	if err != nil {
		PanicSanity(Fmt("Could not marshal state bytes: %v", err))
	}
	db.SetSync(blockStoreKey, bytes)
}

func LoadBlockStoreStateJSON(db dbm.DB) BlockStoreStateJSON {
	bytes := db.Get(blockStoreKey)
	if bytes == nil {
		return BlockStoreStateJSON{
			Height: 0,
		}
	}
	bsj := BlockStoreStateJSON{}
	err := json.Unmarshal(bytes, &bsj)
	if err != nil {
		PanicCrisis(Fmt("Could not unmarshal bytes: %X", bytes))
	}
	return bsj
}
