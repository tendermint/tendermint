package consensus

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/merkle"
)

const (
	partSize = 4096 // 4KB
)

var (
	ErrPartSetUnexpectedIndex = errors.New("Error part set unexpected index")
	ErrPartSetInvalidTrail    = errors.New("Error part set invalid trail")
)

type Part struct {
	Index uint16
	Trail [][]byte
	Bytes []byte

	// Cache
	hash []byte
}

func ReadPart(r io.Reader, n *int64, err *error) *Part {
	return &Part{
		Index: ReadUInt16(r, n, err),
		Trail: ReadByteSlices(r, n, err),
		Bytes: ReadByteSlice(r, n, err),
	}
}

func (b *Part) WriteTo(w io.Writer) (n int64, err error) {
	WriteUInt16(w, b.Index, &n, &err)
	WriteByteSlices(w, b.Trail, &n, &err)
	WriteByteSlice(w, b.Bytes, &n, &err)
	return
}

func (pt *Part) Hash() []byte {
	if pt.hash != nil {
		return pt.hash
	} else {
		hasher := sha256.New()
		_, err := hasher.Write(pt.Bytes)
		if err != nil {
			panic(err)
		}
		pt.hash = hasher.Sum(nil)
		return pt.hash
	}
}

func (pt *Part) String() string {
	return pt.StringWithIndent("")
}

func (pt *Part) StringWithIndent(indent string) string {
	trailStrings := make([]string, len(pt.Trail))
	for i, hash := range pt.Trail {
		trailStrings[i] = fmt.Sprintf("%X", hash)
	}
	return fmt.Sprintf(`Part{
%s  Index: %v
%s  Trail:
%s    %v
%s}`,
		indent, pt.Index,
		indent,
		indent, strings.Join(trailStrings, "\n"+indent+"    "),
		indent)
}

//-------------------------------------

type PartSet struct {
	rootHash []byte
	total    uint16

	mtx           sync.Mutex
	parts         []*Part
	partsBitArray BitArray
	count         uint16
}

// Returns an immutable, full PartSet.
func NewPartSetFromData(data []byte) *PartSet {
	// divide data into 4kb parts.
	total := (len(data) + partSize - 1) / partSize
	parts := make([]*Part, total)
	parts_ := make([]merkle.Hashable, total)
	partsBitArray := NewBitArray(uint(total))
	for i := 0; i < total; i++ {
		part := &Part{
			Index: uint16(i),
			Bytes: data[i*partSize : MinInt(len(data), (i+1)*partSize)],
		}
		parts[i] = part
		parts_[i] = part
		partsBitArray.SetIndex(uint(i), true)
	}
	// Compute merkle trails
	hashTree := merkle.HashTreeFromHashables(parts_)
	for i := 0; i < total; i++ {
		parts[i].Trail = merkle.HashTrailForIndex(hashTree, i)
	}
	return &PartSet{
		parts:         parts,
		partsBitArray: partsBitArray,
		rootHash:      hashTree[len(hashTree)/2],
		total:         uint16(total),
		count:         uint16(total),
	}
}

// Returns an empty PartSet ready to be populated.
func NewPartSetFromMetadata(total uint16, rootHash []byte) *PartSet {
	return &PartSet{
		parts:         make([]*Part, total),
		partsBitArray: NewBitArray(uint(total)),
		rootHash:      rootHash,
		total:         total,
		count:         0,
	}
}

func (ps *PartSet) BitArray() BitArray {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return ps.partsBitArray.Copy()
}

func (ps *PartSet) RootHash() []byte {
	if ps == nil {
		return nil
	}
	return ps.rootHash
}

func (ps *PartSet) Count() uint16 {
	if ps == nil {
		return 0
	}
	return ps.count
}

func (ps *PartSet) Total() uint16 {
	if ps == nil {
		return 0
	}
	return ps.total
}

func (ps *PartSet) AddPart(part *Part) (bool, error) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	// Invalid part index
	if part.Index >= ps.total {
		return false, ErrPartSetUnexpectedIndex
	}

	// If part already exists, return false.
	if ps.parts[part.Index] != nil {
		return false, nil
	}

	// Check hash trail
	if !merkle.VerifyHashTrailForIndex(int(part.Index), part.Hash(), part.Trail, ps.rootHash) {
		return false, ErrPartSetInvalidTrail
	}

	// Add part
	ps.parts[part.Index] = part
	ps.partsBitArray.SetIndex(uint(part.Index), true)
	ps.count++
	return true, nil
}

func (ps *PartSet) GetPart(index uint16) *Part {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return ps.parts[index]
}

func (ps *PartSet) IsComplete() bool {
	return ps.count == ps.total
}

func (ps *PartSet) GetReader() io.Reader {
	if !ps.IsComplete() {
		panic("Cannot GetReader() on incomplete PartSet")
	}
	buf := []byte{}
	for _, part := range ps.parts {
		buf = append(buf, part.Bytes...)
	}
	return bytes.NewReader(buf)
}

func (ps *PartSet) Description() string {
	if ps == nil {
		return "nil-PartSet"
	} else {
		return fmt.Sprintf("(%v of %v)", ps.Count(), ps.Total())
	}
}
