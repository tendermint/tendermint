package blocks

import (
	"bytes"
	"encoding/binary"
	"encoding/json"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	db_ "github.com/tendermint/tendermint/db"
)

var (
	blockStoreKey = []byte("blockStore")
)

//-----------------------------------------------------------------------------

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

func (bs *BlockStore) LoadBlock(height uint32) *Block {
	blockBytes := bs.db.Get(calcBlockKey(height))
	if blockBytes == nil {
		return nil
	}
	var n int64
	var err error
	block := ReadBlock(bytes.NewReader(blockBytes), &n, &err)
	if err != nil {
		Panicf("Error reading block: %v", err)
	}
	return block
}

// Writes are synchronous and atomic.
func (bs *BlockStore) SaveBlock(block *Block) {
	height := block.Height
	if height != bs.height+1 {
		Panicf("BlockStore can only save contiguous blocks. Wanted %v, got %v", bs.height+1, height)
	}
	// Save block
	blockBytes := BinaryBytes(block)
	bs.db.Set(calcBlockKey(height), blockBytes)
	// Save new BlockStoreJSON descriptor
	BlockStoreJSON{Height: height}.Save(bs.db)
}

//-----------------------------------------------------------------------------

func calcBlockKey(height uint32) []byte {
	buf := [9]byte{'B'}
	binary.BigEndian.PutUint32(buf[1:9], height)
	return buf[:]
}
