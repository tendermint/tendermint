package block

import (
	"bytes"
	"encoding/json"
	"fmt"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	db_ "github.com/tendermint/tendermint/db"
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
	db     db_.DB
}

func NewBlockStore(db db_.DB) *BlockStore {
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

func (bs *BlockStore) GetReader(key []byte) Unreader {
	bytez := bs.db.Get(key)
	if bytez == nil {
		return nil
	}
	return bytes.NewReader(bytez)
}

func (bs *BlockStore) LoadBlock(height uint) *Block {
	var n int64
	var err error
	meta := ReadBinary(&BlockMeta{}, bs.GetReader(calcBlockMetaKey(height)), &n, &err).(*BlockMeta)
	if err != nil {
		Panicf("Error reading block meta: %v", err)
	}
	bytez := []byte{}
	for i := uint(0); i < meta.Parts.Total; i++ {
		part := bs.LoadBlockPart(height, i)
		bytez = append(bytez, part.Bytes...)
	}
	block := ReadBinary(&Block{}, bytes.NewReader(bytez), &n, &err).(*Block)
	if err != nil {
		Panicf("Error reading block: %v", err)
	}
	return block
}

func (bs *BlockStore) LoadBlockPart(height uint, index uint) *Part {
	var n int64
	var err error
	part := ReadBinary(&Part{}, bs.GetReader(calcBlockPartKey(height, index)), &n, &err).(*Part)
	if err != nil {
		Panicf("Error reading block part: %v", err)
	}
	return part
}

func (bs *BlockStore) LoadBlockMeta(height uint) *BlockMeta {
	var n int64
	var err error
	meta := ReadBinary(&BlockMeta{}, bs.GetReader(calcBlockMetaKey(height)), &n, &err).(*BlockMeta)
	if err != nil {
		Panicf("Error reading block meta: %v", err)
	}
	return meta
}

func (bs *BlockStore) LoadBlockValidation(height uint) *Validation {
	var n int64
	var err error
	validation := ReadBinary(&Validation{}, bs.GetReader(calcBlockValidationKey(height)), &n, &err).(*Validation)
	if err != nil {
		Panicf("Error reading validation: %v", err)
	}
	return validation
}

func (bs *BlockStore) SaveBlock(block *Block, blockParts *PartSet) {
	height := block.Height
	if height != bs.height+1 {
		Panicf("BlockStore can only save contiguous blocks. Wanted %v, got %v", bs.height+1, height)
	}
	if !blockParts.IsComplete() {
		Panicf("BlockStore can only save complete block part sets")
	}

	// Save block meta
	meta := makeBlockMeta(block, blockParts)
	metaBytes := BinaryBytes(meta)
	bs.db.Set(calcBlockMetaKey(height), metaBytes)

	// Save block parts
	for i := uint(0); i < blockParts.Total(); i++ {
		bs.saveBlockPart(height, i, blockParts.GetPart(i))
	}

	// Save block validation (duplicate and separate)
	validationBytes := BinaryBytes(block.Validation)
	bs.db.Set(calcBlockValidationKey(height), validationBytes)

	// Save new BlockStoreStateJSON descriptor
	BlockStoreStateJSON{Height: height}.Save(bs.db)

	// Done!
	bs.height = height
}

func (bs *BlockStore) saveBlockPart(height uint, index uint, part *Part) {
	if height != bs.height+1 {
		Panicf("BlockStore can only save contiguous blocks. Wanted %v, got %v", bs.height+1, height)
	}
	partBytes := BinaryBytes(part)
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

//-----------------------------------------------------------------------------

var blockStoreKey = []byte("blockStore")

type BlockStoreStateJSON struct {
	Height uint
}

func (bsj BlockStoreStateJSON) Save(db db_.DB) {
	bytes, err := json.Marshal(bsj)
	if err != nil {
		Panicf("Could not marshal state bytes: %v", err)
	}
	db.Set(blockStoreKey, bytes)
}

func LoadBlockStoreStateJSON(db db_.DB) BlockStoreStateJSON {
	bytes := db.Get(blockStoreKey)
	if bytes == nil {
		return BlockStoreStateJSON{
			Height: 0,
		}
	}
	bsj := BlockStoreStateJSON{}
	err := json.Unmarshal(bytes, &bsj)
	if err != nil {
		Panicf("Could not unmarshal bytes: %X", bytes)
	}
	return bsj
}
