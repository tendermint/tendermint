package blocks

import (
	"bytes"
	"encoding/binary"
	"encoding/json"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

var (
	blockStoreKey = []byte("blockStore")
)

//-----------------------------------------------------------------------------

type BlockStoreJSON struct {
	Height uint32
}

func (bsj BlockStoreJSON) Save(db *leveldb.DB) {
	bytes, err := json.Marshal(bsj)
	if err != nil {
		Panicf("Could not marshal state bytes: %v", err)
	}
	db.Put(blockStoreKey, bytes, nil)
}

func LoadBlockStoreJSON(db *leveldb.DB) BlockStoreJSON {
	bytes, err := db.Get(blockStoreKey, nil)
	if err != nil {
		Panicf("Could not load BlockStoreJSON from db: %v", err)
	}
	if bytes == nil {
		return BlockStoreJSON{
			Height: 0,
		}
	}
	bsj := BlockStoreJSON{}
	err = json.Unmarshal(bytes, &bsj)
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
	db     *leveldb.DB
}

func NewBlockStore(db *leveldb.DB) *BlockStore {
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
	blockBytes, err := bs.db.Get(calcBlockKey(height), nil)
	if err != nil {
		Panicf("Could not load block: %v", err)
	}
	if blockBytes == nil {
		return nil
	}
	var n int64
	return ReadBlock(bytes.NewReader(blockBytes), &n, &err)
}

// Writes are synchronous and atomic.
func (bs *BlockStore) SaveBlock(block *Block) error {
	height := block.Height
	if height != bs.height+1 {
		return Errorf("BlockStore can only save contiguous blocks. Wanted %v, got %v", bs.height+1, height)
	}
	// Save block
	blockBytes := BinaryBytes(block)
	err := bs.db.Put(calcBlockKey(height), blockBytes, &opt.WriteOptions{Sync: true})
	// Save new BlockStoreJSON descriptor
	BlockStoreJSON{Height: height}.Save(bs.db)
	return err
}

//-----------------------------------------------------------------------------

func calcBlockKey(height uint32) []byte {
	buf := [9]byte{'B'}
	binary.BigEndian.PutUint32(buf[1:9], height)
	return buf[:]
}
