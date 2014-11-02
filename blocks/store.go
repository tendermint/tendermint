package blocks

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
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
	height uint32
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
func (bs *BlockStore) Height() uint32 {
	return bs.height
}

func (bs *BlockStore) GetReader(key []byte) io.Reader {
	bytez := bs.db.Get(key)
	if bytez == nil {
		return nil
	}
	return bytes.NewReader(bytez)
}

func (bs *BlockStore) LoadBlock(height uint32) *Block {
	var n int64
	var err error
	meta := ReadBlockMeta(bs.GetReader(calcBlockMetaKey(height)), &n, &err)
	if err != nil {
		Panicf("Error reading block meta: %v", err)
	}
	bytez := []byte{}
	for i := uint16(0); i < meta.Parts.Total; i++ {
		part := bs.LoadBlockPart(height, i)
		bytez = append(bytez, part.Bytes...)
	}
	block := ReadBlock(bytes.NewReader(bytez), &n, &err)
	if err != nil {
		Panicf("Error reading block: %v", err)
	}
	return block
}

func (bs *BlockStore) LoadBlockPart(height uint32, index uint16) *Part {
	var n int64
	var err error
	part := ReadPart(bs.GetReader(calcBlockPartKey(height, index)), &n, &err)
	if err != nil {
		Panicf("Error reading block part: %v", err)
	}
	return part
}

func (bs *BlockStore) LoadBlockMeta(height uint32) *BlockMeta {
	var n int64
	var err error
	meta := ReadBlockMeta(bs.GetReader(calcBlockMetaKey(height)), &n, &err)
	if err != nil {
		Panicf("Error reading block meta: %v", err)
	}
	return meta
}

func (bs *BlockStore) LoadBlockValidation(height uint32) *Validation {
	var n int64
	var err error
	validation := ReadValidation(bs.GetReader(calcBlockValidationKey(height)), &n, &err)
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
	for i := uint16(0); i < blockParts.Total(); i++ {
		bs.saveBlockPart(height, i, blockParts.GetPart(i))
	}
	// Save block validation (duplicate and separate)
	validationBytes := BinaryBytes(block.Validation)
	bs.db.Set(calcBlockValidationKey(height), validationBytes)
	// Save new BlockStoreJSON descriptor
	BlockStoreJSON{Height: height}.Save(bs.db)
}

func (bs *BlockStore) saveBlockPart(height uint32, index uint16, part *Part) {
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

func ReadBlockMeta(r io.Reader, n *int64, err *error) *BlockMeta {
	return &BlockMeta{
		Hash:   ReadByteSlice(r, n, err),
		Parts:  ReadPartSetHeader(r, n, err),
		Header: ReadHeader(r, n, err),
	}
}

func (bm *BlockMeta) WriteTo(w io.Writer) (n int64, err error) {
	WriteByteSlice(w, bm.Hash, &n, &err)
	WriteBinary(w, bm.Parts, &n, &err)
	WriteBinary(w, bm.Header, &n, &err)
	return
}

//-----------------------------------------------------------------------------

func calcBlockMetaKey(height uint32) []byte {
	buf := [5]byte{'H'}
	binary.BigEndian.PutUint32(buf[1:5], height)
	return buf[:]
}

func calcBlockPartKey(height uint32, partIndex uint16) []byte {
	buf := [7]byte{'P'}
	binary.BigEndian.PutUint32(buf[1:5], height)
	binary.BigEndian.PutUint16(buf[5:7], partIndex)
	return buf[:]
}

func calcBlockValidationKey(height uint32) []byte {
	buf := [5]byte{'V'}
	binary.BigEndian.PutUint32(buf[1:5], height)
	return buf[:]
}

//-----------------------------------------------------------------------------

var blockStoreKey = []byte("blockStore")

type BlockStoreJSON struct {
	Height uint32
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
