package blocks

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

func (part *Part) WriteTo(w io.Writer) (n int64, err error) {
	WriteUInt16(w, part.Index, &n, &err)
	WriteByteSlices(w, part.Trail, &n, &err)
	WriteByteSlice(w, part.Bytes, &n, &err)
	return
}

func (part *Part) Hash() []byte {
	if part.hash != nil {
		return part.hash
	} else {
		hasher := sha256.New()
		_, err := hasher.Write(part.Bytes)
		if err != nil {
			panic(err)
		}
		part.hash = hasher.Sum(nil)
		return part.hash
	}
}

func (part *Part) String() string {
	return part.StringWithIndent("")
}

func (part *Part) StringWithIndent(indent string) string {
	trailStrings := make([]string, len(part.Trail))
	for i, hash := range part.Trail {
		trailStrings[i] = fmt.Sprintf("%X", hash)
	}
	return fmt.Sprintf(`Part{
%s  Index: %v
%s  Trail:
%s    %v
%s}`,
		indent, part.Index,
		indent,
		indent, strings.Join(trailStrings, "\n"+indent+"    "),
		indent)
}

//-------------------------------------

type PartSetHeader struct {
	Total uint16
	Hash  []byte
}

func ReadPartSetHeader(r io.Reader, n *int64, err *error) PartSetHeader {
	return PartSetHeader{
		Total: ReadUInt16(r, n, err),
		Hash:  ReadByteSlice(r, n, err),
	}
}

func (psh PartSetHeader) WriteTo(w io.Writer) (n int64, err error) {
	WriteUInt16(w, psh.Total, &n, &err)
	WriteByteSlice(w, psh.Hash, &n, &err)
	return
}

func (psh PartSetHeader) IsZero() bool {
	return psh.Total == 0
}

func (psh PartSetHeader) String() string {
	return fmt.Sprintf("PartSet{%X/%v}", psh.Hash, psh.Total)
}

func (psh PartSetHeader) Equals(other PartSetHeader) bool {
	return psh.Total == other.Total && bytes.Equal(psh.Hash, other.Hash)
}

//-------------------------------------

type PartSet struct {
	total uint16
	hash  []byte

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
		total:         uint16(total),
		hash:          hashTree[len(hashTree)/2],
		parts:         parts,
		partsBitArray: partsBitArray,
		count:         uint16(total),
	}
}

// Returns an empty PartSet ready to be populated.
func NewPartSetFromHeader(header PartSetHeader) *PartSet {
	return &PartSet{
		total:         header.Total,
		hash:          header.Hash,
		parts:         make([]*Part, header.Total),
		partsBitArray: NewBitArray(uint(header.Total)),
		count:         0,
	}
}

func (ps *PartSet) Header() PartSetHeader {
	if ps == nil {
		return PartSetHeader{}
	} else {
		return PartSetHeader{
			Total: ps.total,
			Hash:  ps.hash,
		}
	}
}

func (ps *PartSet) BitArray() BitArray {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return ps.partsBitArray.Copy()
}

func (ps *PartSet) Hash() []byte {
	if ps == nil {
		return nil
	}
	return ps.hash
}

func (ps *PartSet) HashesTo(hash []byte) bool {
	if ps == nil {
		return false
	}
	return bytes.Equal(ps.hash, hash)
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
	if !merkle.VerifyHashTrailForIndex(int(part.Index), part.Hash(), part.Trail, ps.hash) {
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
