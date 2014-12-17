package block

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	db_ "github.com/tendermint/tendermint/db"
)

//-----------------------------------------------------------------------------

/*
Simple low level store for blocks, which is actually stored as separte parts (wire format).
*/
type BlockStore struct {
	height uint
	db     db_.DB
}

func NewBlockStore(db db_.DB) *BlockStore {
	bsjson := LoadBlockStoreJSON(db)
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
	meta := &BlockMeta{
		Hash:   block.Hash(),
		Parts:  blockParts.Header(),
		Header: block.Header,
	}
	// Save block meta
	metaBytes := BinaryBytes(meta)
	bs.db.Set(calcBlockMetaKey(height), metaBytes)
	// Save block parts
	for i := uint(0); i < blockParts.Total(); i++ {
		bs.saveBlockPart(height, i, blockParts.GetPart(i))
	}
	// Save block validation (duplicate and separate)
	validationBytes := BinaryBytes(block.Validation)
	bs.db.Set(calcBlockValidationKey(height), validationBytes)
	// Save new BlockStoreJSON descriptor
	BlockStoreJSON{Height: height}.Save(bs.db)
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
	Hash   []byte        // The BlockHash
	Parts  PartSetHeader // The PartSetHeader, for transfer
	Header *Header       // The block's Header
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

type BlockStoreJSON struct {
	Height uint
}

func (bsj BlockStoreJSON) Save(db db_.DB) {
	bytes, err := json.Marshal(bsj)
	if err != nil {
		Panicf("Could not marshal state bytes: %v", err)
	}
	db.Set(blockStoreKey, bytes)
}

func LoadBlockStoreJSON(db db_.DB) BlockStoreJSON {
	bytes := db.Get(blockStoreKey)
	if bytes == nil {
		return BlockStoreJSON{
			Height: 0,
		}
	}
	bsj := BlockStoreJSON{}
	err := json.Unmarshal(bytes, &bsj)
	if err != nil {
		Panicf("Could not unmarshal bytes: %X", bytes)
	}
	return bsj
}
