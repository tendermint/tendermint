package blockchain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	dbm "github.com/tendermint/tendermint/db"
	"github.com/tendermint/tendermint/types"
)

/*
Simple low level store for blocks.

There are three types of information stored:
 - BlockMeta:   Meta information about each block
 - Block part:  Parts of each block, aggregated w/ PartSet
 - Validation:  The Validation part of each block, for gossiping precommit votes

Currently the precommit signatures are duplicated in the Block parts as
well as the Validation.  In the future this may change, perhaps by moving
the Validation data outside the Block.
*/
type BlockStore struct {
	height uint
	db     dbm.DB
}

func NewBlockStore(db dbm.DB) *BlockStore {
	bsjson := LoadBlockStoreStateJSON(db)
	return &BlockStore{
		height: bsjson.Height,
		db:     db,
	}
}

// Height() returns the last known contiguous block height.
func (bs *BlockStore) Height() uint {
	return bs.height
}

func (bs *BlockStore) GetReader(key []byte) io.Reader {
	bytez := bs.db.Get(key)
	if bytez == nil {
		return nil
	}
	return bytes.NewReader(bytez)
}

func (bs *BlockStore) LoadBlock(height uint) *types.Block {
	var n int64
	var err error
	r := bs.GetReader(calcBlockMetaKey(height))
	if r == nil {
		panic(Fmt("Block does not exist at height %v", height))
	}
	meta := binary.ReadBinary(&types.BlockMeta{}, r, &n, &err).(*types.BlockMeta)
	if err != nil {
		panic(Fmt("Error reading block meta: %v", err))
	}
	bytez := []byte{}
	for i := uint(0); i < meta.Parts.Total; i++ {
		part := bs.LoadBlockPart(height, i)
		bytez = append(bytez, part.Bytes...)
	}
	block := binary.ReadBinary(&types.Block{}, bytes.NewReader(bytez), &n, &err).(*types.Block)
	if err != nil {
		panic(Fmt("Error reading block: %v", err))
	}
	return block
}

func (bs *BlockStore) LoadBlockPart(height uint, index uint) *types.Part {
	var n int64
	var err error
	r := bs.GetReader(calcBlockPartKey(height, index))
	if r == nil {
		panic(Fmt("BlockPart does not exist for height %v index %v", height, index))
	}
	part := binary.ReadBinary(&types.Part{}, r, &n, &err).(*types.Part)
	if err != nil {
		panic(Fmt("Error reading block part: %v", err))
	}
	return part
}

func (bs *BlockStore) LoadBlockMeta(height uint) *types.BlockMeta {
	var n int64
	var err error
	r := bs.GetReader(calcBlockMetaKey(height))
	if r == nil {
		panic(Fmt("BlockMeta does not exist for height %v", height))
	}
	meta := binary.ReadBinary(&types.BlockMeta{}, r, &n, &err).(*types.BlockMeta)
	if err != nil {
		panic(Fmt("Error reading block meta: %v", err))
	}
	return meta
}

// NOTE: the Precommit-vote heights are for the block at `height-1`
// Since these are included in the subsequent block, the height
// is off by 1.
func (bs *BlockStore) LoadBlockValidation(height uint) *types.Validation {
	var n int64
	var err error
	r := bs.GetReader(calcBlockValidationKey(height))
	if r == nil {
		panic(Fmt("BlockValidation does not exist for height %v", height))
	}
	validation := binary.ReadBinary(&types.Validation{}, r, &n, &err).(*types.Validation)
	if err != nil {
		panic(Fmt("Error reading validation: %v", err))
	}
	return validation
}

// NOTE: the Precommit-vote heights are for the block at `height`
func (bs *BlockStore) LoadSeenValidation(height uint) *types.Validation {
	var n int64
	var err error
	r := bs.GetReader(calcSeenValidationKey(height))
	if r == nil {
		panic(Fmt("SeenValidation does not exist for height %v", height))
	}
	validation := binary.ReadBinary(&types.Validation{}, r, &n, &err).(*types.Validation)
	if err != nil {
		panic(Fmt("Error reading validation: %v", err))
	}
	return validation
}

// blockParts:     Must be parts of the block
// seenValidation: The +2/3 precommits that were seen which committed at height.
//                 If all the nodes restart after committing a block,
//                 we need this to reload the precommits to catch-up nodes to the
//                 most recent height.  Otherwise they'd stall at H-1.
func (bs *BlockStore) SaveBlock(block *types.Block, blockParts *types.PartSet, seenValidation *types.Validation) {
	height := block.Height
	if height != bs.height+1 {
		panic(Fmt("BlockStore can only save contiguous blocks. Wanted %v, got %v", bs.height+1, height))
	}
	if !blockParts.IsComplete() {
		panic(Fmt("BlockStore can only save complete block part sets"))
	}

	// Save block meta
	meta := types.NewBlockMeta(block, blockParts)
	metaBytes := binary.BinaryBytes(meta)
	bs.db.Set(calcBlockMetaKey(height), metaBytes)

	// Save block parts
	for i := uint(0); i < blockParts.Total(); i++ {
		bs.saveBlockPart(height, i, blockParts.GetPart(i))
	}

	// Save block validation (duplicate and separate from the Block)
	blockValidationBytes := binary.BinaryBytes(block.Validation)
	bs.db.Set(calcBlockValidationKey(height), blockValidationBytes)

	// Save seen validation (seen +2/3 precommits for block)
	seenValidationBytes := binary.BinaryBytes(seenValidation)
	bs.db.Set(calcSeenValidationKey(height), seenValidationBytes)

	// Save new BlockStoreStateJSON descriptor
	BlockStoreStateJSON{Height: height}.Save(bs.db)

	// Done!
	bs.height = height
}

func (bs *BlockStore) saveBlockPart(height uint, index uint, part *types.Part) {
	if height != bs.height+1 {
		panic(Fmt("BlockStore can only save contiguous blocks. Wanted %v, got %v", bs.height+1, height))
	}
	partBytes := binary.BinaryBytes(part)
	bs.db.Set(calcBlockPartKey(height, index), partBytes)
}

//-----------------------------------------------------------------------------

func calcBlockMetaKey(height uint) []byte {
	return []byte(fmt.Sprintf("H:%v", height))
}

func calcBlockPartKey(height uint, partIndex uint) []byte {
	return []byte(fmt.Sprintf("P:%v:%v", height, partIndex))
}

func calcBlockValidationKey(height uint) []byte {
	return []byte(fmt.Sprintf("V:%v", height))
}

func calcSeenValidationKey(height uint) []byte {
	return []byte(fmt.Sprintf("SV:%v", height))
}

//-----------------------------------------------------------------------------

var blockStoreKey = []byte("blockStore")

type BlockStoreStateJSON struct {
	Height uint
}

func (bsj BlockStoreStateJSON) Save(db dbm.DB) {
	bytes, err := json.Marshal(bsj)
	if err != nil {
		panic(Fmt("Could not marshal state bytes: %v", err))
	}
	db.Set(blockStoreKey, bytes)
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
		panic(Fmt("Could not unmarshal bytes: %X", bytes))
	}
	return bsj
}
