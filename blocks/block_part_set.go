package blocks

import (
	"bytes"
	"errors"
	"sync"

	"github.com/tendermint/tendermint/merkle"
)

// A collection of block parts.
// Doesn't do any validation.
type BlockPartSet struct {
	mtx      sync.Mutex
	height   uint32
	total    uint16 // total number of parts
	numParts uint16 // number of parts in this set
	parts    []*BlockPart

	_block *Block // cache
}

var (
	ErrInvalidBlockPartConflict = errors.New("Invalid block part conflict") // Signer signed conflicting parts
)

// parts may be nil if the parts aren't in hand.
func NewBlockPartSet(height uint32, parts []*BlockPart) *BlockPartSet {
	bps := &BlockPartSet{
		height:   height,
		parts:    parts,
		numParts: uint16(len(parts)),
	}
	if len(parts) > 0 {
		bps.total = parts[0].Total
	}
	return bps
}

func (bps *BlockPartSet) Height() uint32 {
	return bps.height
}

func (bps *BlockPartSet) BlockParts() []*BlockPart {
	bps.mtx.Lock()
	defer bps.mtx.Unlock()
	return bps.parts
}

func (bps *BlockPartSet) BitArray() []byte {
	bps.mtx.Lock()
	defer bps.mtx.Unlock()
	if bps.parts == nil {
		return nil
	}
	bitArray := make([]byte, (len(bps.parts)+7)/8)
	for i, part := range bps.parts {
		if part != nil {
			bitArray[i/8] |= 1 << uint(i%8)
		}
	}
	return bitArray
}

// If the part isn't valid, returns an error.
// err can be ErrInvalidBlockPartConflict
// NOTE: Caller must check the signature before adding.
func (bps *BlockPartSet) AddBlockPart(part *BlockPart) (added bool, err error) {
	bps.mtx.Lock()
	defer bps.mtx.Unlock()

	if bps.parts == nil {
		// First received part for this round.
		bps.parts = make([]*BlockPart, part.Total)
		bps.total = uint16(part.Total)
		bps.parts[int(part.Index)] = part
		bps.numParts++
		return true, nil
	} else {
		// Check part.Index and part.Total
		if uint16(part.Index) >= bps.total {
			return false, ErrInvalidBlockPartConflict
		}
		if uint16(part.Total) != bps.total {
			return false, ErrInvalidBlockPartConflict
		}
		// Check for existing parts.
		existing := bps.parts[part.Index]
		if existing != nil {
			if bytes.Equal(existing.Bytes, part.Bytes) {
				// Ignore duplicate
				return false, nil
			} else {
				return false, ErrInvalidBlockPartConflict
			}
		} else {
			bps.parts[int(part.Index)] = part
			bps.numParts++
			return true, nil
		}
	}

}

func (bps *BlockPartSet) IsComplete() bool {
	bps.mtx.Lock()
	defer bps.mtx.Unlock()
	return bps.total > 0 && bps.total == bps.numParts
}

func (bps *BlockPartSet) Block() *Block {
	if !bps.IsComplete() {
		return nil
	}
	bps.mtx.Lock()
	defer bps.mtx.Unlock()
	if bps._block == nil {
		block, err := BlockPartsToBlock(bps.parts)
		if err != nil {
			panic(err)
		}
		bps._block = block
	}
	return bps._block
}

func (bps *BlockPartSet) Hash() []byte {
	if !bps.IsComplete() {
		panic("Cannot get hash of an incomplete BlockPartSet")
	}
	hashes := [][]byte{}
	for _, part := range bps.parts {
		partHash := part.Hash()
		hashes = append(hashes, partHash)
	}
	return merkle.HashFromByteSlices(hashes)
}

// The proposal hash includes both the block hash
// as well as the BlockPartSet merkle hash.
func (bps *BlockPartSet) ProposalHash() []byte {
	bpsHash := bps.Hash()
	blockHash := bps.Block().Hash()
	return merkle.HashFromByteSlices([][]byte{bpsHash, blockHash})
}

//-----------------------------------------------------------------------------

func BlockPartsToBlock(parts []*BlockPart) (*Block, error) {
	blockBytes := []byte{}
	for _, part := range parts {
		blockBytes = append(blockBytes, part.Bytes...)
	}
	var n int64
	var err error
	block := ReadBlock(bytes.NewReader(blockBytes), &n, &err)
	return block, err
}
