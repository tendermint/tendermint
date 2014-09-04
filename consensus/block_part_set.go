package consensus

import (
	"bytes"
	"errors"
	"sync"

	. "github.com/tendermint/tendermint/blocks"
	. "github.com/tendermint/tendermint/state"
)

// Helper for keeping track of block parts.
type BlockPartSet struct {
	mtx      sync.Mutex
	signer   *Account
	height   uint32
	round    uint16 // Not used
	total    uint16
	numParts uint16
	parts    []*BlockPart

	_block *Block // cache
}

var (
	ErrInvalidBlockPartSignature = errors.New("Invalid block part signature") // Peer gave us a fake part
	ErrInvalidBlockPartConflict  = errors.New("Invalid block part conflict")  // Signer signed conflicting parts
)

// Signer may be nil if signer is unknown beforehand.
func NewBlockPartSet(height uint32, round uint16, signer *Account) *BlockPartSet {
	return &BlockPartSet{
		signer: signer,
		height: height,
		round:  round,
	}
}

// In the case where the signer wasn't known prior to NewBlockPartSet(),
// user should call SetSigner() prior to AddBlockPart().
func (bps *BlockPartSet) SetSigner(signer *Account) {
	bps.mtx.Lock()
	defer bps.mtx.Unlock()
	if bps.signer != nil {
		panic("BlockPartSet signer already set.")
	}
	bps.signer = signer
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
// err can be ErrInvalidBlockPart[Conflict|Signature]
func (bps *BlockPartSet) AddBlockPart(part *BlockPart) (added bool, err error) {
	bps.mtx.Lock()
	defer bps.mtx.Unlock()

	// If part is invalid, return an error.
	/* XXX
	err = part.ValidateWithSigner(bps.signer)
	if err != nil {
		return false, err
	}
	*/

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
		blockBytes := []byte{}
		for _, part := range bps.parts {
			blockBytes = append(blockBytes, part.Bytes...)
		}
		var n int64
		var err error
		block := ReadBlock(bytes.NewReader(blockBytes), &n, &err)
		bps._block = block
	}
	return bps._block
}
