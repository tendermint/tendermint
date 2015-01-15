package block

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	dbm "github.com/tendermint/tendermint/db"
)

/*
Simple low level store for blocks.

There are three types of information stored:
 - BlockMeta:   Meta information about each block
 - Block part:  Parts of each block, aggregated w/ PartSet
 - Validation:  The Validation part of each block, for gossiping commit votes

Currently the commit signatures are duplicated in the Block parts as
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

func (bs *BlockStore) LoadBlock(height uint) *Block {
	var n int64
	var err error
	r := bs.GetReader(calcBlockMetaKey(height))
	if r == nil {
		panic(Fmt("Block does not exist at height %v", height))
	}
	meta := binary.ReadBinary(&BlockMeta{}, r, &n, &err).(*BlockMeta)
	if err != nil {
		panic(Fmt("Error reading block meta: %v", err))
	}
	bytez := []byte{}
	for i := uint(0); i < meta.Parts.Total; i++ {
		part := bs.LoadBlockPart(height, i)
		bytez = append(bytez, part.Bytes...)
	}
	block := binary.ReadBinary(&Block{}, bytes.NewReader(bytez), &n, &err).(*Block)
	if err != nil {
		panic(Fmt("Error reading block: %v", err))
	}
	return block
}

func (bs *BlockStore) LoadBlockPart(height uint, index uint) *Part {
	var n int64
	var err error
	r := bs.GetReader(calcBlockPartKey(height, index))
	if r == nil {
		panic(Fmt("BlockPart does not exist for height %v index %v", height, index))
	}
	part := binary.ReadBinary(&Part{}, r, &n, &err).(*Part)
	if err != nil {
		panic(Fmt("Error reading block part: %v", err))
	}
	return part
}

func (bs *BlockStore) LoadBlockMeta(height uint) *BlockMeta {
	var n int64
	var err error
	r := bs.GetReader(calcBlockMetaKey(height))
	if r == nil {
		panic(Fmt("BlockMeta does not exist for height %v", height))
	}
	meta := binary.ReadBinary(&BlockMeta{}, r, &n, &err).(*BlockMeta)
	if err != nil {
		panic(Fmt("Error reading block meta: %v", err))
	}
	return meta
}

// NOTE: the Commit-vote heights are for the block at `height-1`
// Since these are included in the subsequent block, the height
// is off by 1.
func (bs *BlockStore) LoadBlockValidation(height uint) *Validation {
	var n int64
	var err error
	r := bs.GetReader(calcBlockValidationKey(height))
	if r == nil {
		panic(Fmt("BlockValidation does not exist for height %v", height))
	}
	validation := binary.ReadBinary(&Validation{}, r, &n, &err).(*Validation)
	if err != nil {
		panic(Fmt("Error reading validation: %v", err))
	}
	return validation
}

// NOTE: the Commit-vote heights are for the block at `height`
func (bs *BlockStore) LoadSeenValidation(height uint) *Validation {
	var n int64
	var err error
	r := bs.GetReader(calcSeenValidationKey(height))
	if r == nil {
		panic(Fmt("SeenValidation does not exist for height %v", height))
	}
	validation := binary.ReadBinary(&Validation{}, r, &n, &err).(*Validation)
	if err != nil {
		panic(Fmt("Error reading validation: %v", err))
	}
	return validation
}

// blockParts:     Must be parts of the block
// seenValidation: The +2/3 commits that were seen which finalized the height.
//                 If all the nodes restart after committing a block,
//                 we need this to reload the commits to catch-up nodes to the
//                 most recent height.  Otherwise they'd stall at H-1.
//				   Also good to have to debug consensus issues & punish wrong-signers
// 				   whose commits weren't included in the block.
func (bs *BlockStore) SaveBlock(block *Block, blockParts *PartSet, seenValidation *Validation) {
	height := block.Height
	if height != bs.height+1 {
		panic(Fmt("BlockStore can only save contiguous blocks. Wanted %v, got %v", bs.height+1, height))
	}
	if !blockParts.IsComplete() {
		panic(Fmt("BlockStore can only save complete block part sets"))
	}

	// Save block meta
	meta := makeBlockMeta(block, blockParts)
	metaBytes := binary.BinaryBytes(meta)
	bs.db.Set(calcBlockMetaKey(height), metaBytes)

	// Save block parts
	for i := uint(0); i < blockParts.Total(); i++ {
		bs.saveBlockPart(height, i, blockParts.GetPart(i))
	}

	// Save block validation (duplicate and separate from the Block)
	blockValidationBytes := binary.BinaryBytes(block.Validation)
	bs.db.Set(calcBlockValidationKey(height), blockValidationBytes)

	// Save seen validation (seen +2/3 commits)
	seenValidationBytes := binary.BinaryBytes(seenValidation)
	bs.db.Set(calcSeenValidationKey(height), seenValidationBytes)

	// Save new BlockStoreStateJSON descriptor
	BlockStoreStateJSON{Height: height}.Save(bs.db)

	// Done!
	bs.height = height
}

func (bs *BlockStore) saveBlockPart(height uint, index uint, part *Part) {
	if height != bs.height+1 {
		panic(Fmt("BlockStore can only save contiguous blocks. Wanted %v, got %v", bs.height+1, height))
	}
	partBytes := binary.BinaryBytes(part)
	bs.db.Set(calcBlockPartKey(height, index), partBytes)
}

//-----------------------------------------------------------------------------

type BlockMeta struct {
	Hash   []byte        // The block hash
	Header *Header       // The block's Header
	Parts  PartSetHeader // The PartSetHeader, for transfer
}

func makeBlockMeta(block *Block, blockParts *PartSet) *BlockMeta {
	return &BlockMeta{
		Hash:   block.Hash(),
		Header: block.Header,
		Parts:  blockParts.Header(),
	}
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
