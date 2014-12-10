package blocks

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

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
	Index uint
	Trail [][]byte
	Bytes []byte

	// Cache
	hash []byte
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
	Total uint
	Hash  []byte
}

func (psh PartSetHeader) String() string {
	return fmt.Sprintf("PartSet{T:%v %X}", psh.Total, Fingerprint(psh.Hash))
}

func (psh PartSetHeader) IsZero() bool {
	return psh.Total == 0
}

func (psh PartSetHeader) Equals(other PartSetHeader) bool {
	return psh.Total == other.Total && bytes.Equal(psh.Hash, other.Hash)
}

//-------------------------------------

type PartSet struct {
	total uint
	hash  []byte

	mtx           sync.Mutex
	parts         []*Part
	partsBitArray BitArray
	count         uint
}

// Returns an immutable, full PartSet.
// TODO Name is confusing, Data/Header clash with Block.Data/Header
func NewPartSetFromData(data []byte) *PartSet {
	// divide data into 4kb parts.
	total := (len(data) + partSize - 1) / partSize
	parts := make([]*Part, total)
	parts_ := make([]merkle.Hashable, total)
	partsBitArray := NewBitArray(uint(total))
	for i := 0; i < total; i++ {
		part := &Part{
			Index: uint(i),
			Bytes: data[i*partSize : MinInt(len(data), (i+1)*partSize)],
		}
		parts[i] = part
		parts_[i] = part
		partsBitArray.SetIndex(uint(i), true)
	}
	// Compute merkle trails
	trails, rootTrail := merkle.HashTrailsFromHashables(parts_)
	for i := 0; i < total; i++ {
		parts[i].Trail = trails[i].Flatten()
	}
	return &PartSet{
		total:         uint(total),
		hash:          rootTrail.Hash,
		parts:         parts,
		partsBitArray: partsBitArray,
		count:         uint(total),
	}
}

// Returns an empty PartSet ready to be populated.
// TODO Name is confusing, Data/Header clash with Block.Data/Header
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

func (ps *PartSet) HasHeader(header PartSetHeader) bool {
	if ps == nil {
		return false
	} else {
		return ps.Header().Equals(header)
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

func (ps *PartSet) Count() uint {
	if ps == nil {
		return 0
	}
	return ps.count
}

func (ps *PartSet) Total() uint {
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
	if !merkle.VerifyHashTrail(uint(part.Index), uint(ps.total), part.Hash(), part.Trail, ps.hash) {
		return false, ErrPartSetInvalidTrail
	}

	// Add part
	ps.parts[part.Index] = part
	ps.partsBitArray.SetIndex(uint(part.Index), true)
	ps.count++
	return true, nil
}

func (ps *PartSet) GetPart(index uint) *Part {
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
