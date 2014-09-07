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

// LoadBlockPart loads a part of a block.
func (bs *BlockStore) LoadBlockPart(height uint32, index uint16) *BlockPart {
	partBytes, err := bs.db.Get(calcBlockPartKey(height, index), nil)
	if err != nil {
		Panicf("Could not load block part: %v", err)
	}
	if partBytes == nil {
		return nil
	}
	var n int64
	return ReadBlockPart(bytes.NewReader(partBytes), &n, &err)
}

// Convenience method for loading block parts and merging to a block.
func (bs *BlockStore) LoadBlock(height uint32) *Block {
	// Get the first part.
	part0 := bs.LoadBlockPart(height, 0)
	if part0 == nil {
		return nil
	}
	parts := []*BlockPart{part0}
	for i := uint16(1); i < part0.Total; i++ {
		part := bs.LoadBlockPart(height, i)
		if part == nil {
			Panicf("Failed to retrieve block part %v at height %v", i, height)
		}
		parts = append(parts, part)
	}
	block, err := BlockPartsToBlock(parts)
	if err != nil {
		panic(err)
	}
	return block
}

// NOTE: Assumes that parts as well as the block are valid. See StageBlockParts().
// Writes are synchronous and atomic.
func (bs *BlockStore) SaveBlockParts(height uint32, parts []*BlockPart) error {
	if height != bs.height+1 {
		return Errorf("BlockStore can only save contiguous blocks. Wanted %v, got %v", bs.height+1, height)
	}
	// Save parts
	batch := new(leveldb.Batch)
	for _, part := range parts {
		partBytes := BinaryBytes(part)
		batch.Put(calcBlockPartKey(uint32(part.Height), uint16(part.Index)), partBytes)
	}
	err := bs.db.Write(batch, &opt.WriteOptions{Sync: true})
	// Save new BlockStoreJSON descriptor
	BlockStoreJSON{Height: height}.Save(bs.db)
	return err
}

//-----------------------------------------------------------------------------

func calcBlockPartKey(height uint32, index uint16) []byte {
	buf := [11]byte{'B'}
	binary.BigEndian.PutUint32(buf[1:9], height)
	binary.BigEndian.PutUint16(buf[9:11], index)
	return buf[:]
}
