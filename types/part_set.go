package types

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sync"

	"golang.org/x/crypto/ripemd160"

	"github.com/eris-ltd/tendermint/wire"
	. "github.com/eris-ltd/tendermint/common"
	"github.com/eris-ltd/tendermint/merkle"
)

const (
	partSize = 4096 // 4KB
)

var (
	ErrPartSetUnexpectedIndex = errors.New("Error part set unexpected index")
	ErrPartSetInvalidProof    = errors.New("Error part set invalid proof")
)

type Part struct {
	Proof merkle.SimpleProof `json:"proof"`
	Bytes []byte             `json:"bytes"`

	// Cache
	hash []byte
}

func (part *Part) Hash() []byte {
	if part.hash != nil {
		return part.hash
	} else {
		hasher := ripemd160.New()
		hasher.Write(part.Bytes) // doesn't err
		part.hash = hasher.Sum(nil)
		return part.hash
	}
}

func (part *Part) String() string {
	return part.StringIndented("")
}

func (part *Part) StringIndented(indent string) string {
	return fmt.Sprintf(`Part{
%s  Proof: %v
%s  Bytes: %X
%s}`,
		indent, part.Proof.StringIndented(indent+"  "),
		indent, part.Bytes,
		indent)
}

//-------------------------------------

type PartSetHeader struct {
	Total int    `json:"total"`
	Hash  []byte `json:"hash"`
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

func (psh PartSetHeader) WriteSignBytes(w io.Writer, n *int64, err *error) {
	wire.WriteTo([]byte(Fmt(`{"hash":"%X","total":%v}`, psh.Hash, psh.Total)), w, n, err)
}

//-------------------------------------

type PartSet struct {
	total int
	hash  []byte

	mtx           sync.Mutex
	parts         []*Part
	partsBitArray *BitArray
	count         int
}

// Returns an immutable, full PartSet from the data bytes.
// The data bytes are split into "partSize" chunks, and merkle tree computed.
func NewPartSetFromData(data []byte) *PartSet {
	// divide data into 4kb parts.
	total := (len(data) + partSize - 1) / partSize
	parts := make([]*Part, total)
	parts_ := make([]merkle.Hashable, total)
	partsBitArray := NewBitArray(total)
	for i := 0; i < total; i++ {
		part := &Part{
			Bytes: data[i*partSize : MinInt(len(data), (i+1)*partSize)],
		}
		parts[i] = part
		parts_[i] = part
		partsBitArray.SetIndex(i, true)
	}
	// Compute merkle proofs
	proofs := merkle.SimpleProofsFromHashables(parts_)
	for i := 0; i < total; i++ {
		parts[i].Proof = *proofs[i]
	}
	return &PartSet{
		total:         total,
		hash:          proofs[0].RootHash,
		parts:         parts,
		partsBitArray: partsBitArray,
		count:         total,
	}
}

// Returns an empty PartSet ready to be populated.
func NewPartSetFromHeader(header PartSetHeader) *PartSet {
	return &PartSet{
		total:         header.Total,
		hash:          header.Hash,
		parts:         make([]*Part, header.Total),
		partsBitArray: NewBitArray(header.Total),
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

func (ps *PartSet) BitArray() *BitArray {
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

func (ps *PartSet) Count() int {
	if ps == nil {
		return 0
	}
	return ps.count
}

func (ps *PartSet) Total() int {
	if ps == nil {
		return 0
	}
	return ps.total
}

func (ps *PartSet) AddPart(part *Part) (bool, error) {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()

	// Invalid part index
	if part.Proof.Index >= ps.total {
		return false, ErrPartSetUnexpectedIndex
	}

	// If part already exists, return false.
	if ps.parts[part.Proof.Index] != nil {
		return false, nil
	}

	// Check hash proof
	if !part.Proof.Verify(part.Hash(), ps.Hash()) {
		return false, ErrPartSetInvalidProof
	}

	// Add part
	ps.parts[part.Proof.Index] = part
	ps.partsBitArray.SetIndex(part.Proof.Index, true)
	ps.count++
	return true, nil
}

func (ps *PartSet) GetPart(index int) *Part {
	ps.mtx.Lock()
	defer ps.mtx.Unlock()
	return ps.parts[index]
}

func (ps *PartSet) IsComplete() bool {
	return ps.count == ps.total
}

func (ps *PartSet) GetReader() io.Reader {
	if !ps.IsComplete() {
		PanicSanity("Cannot GetReader() on incomplete PartSet")
	}
	buf := []byte{}
	for _, part := range ps.parts {
		buf = append(buf, part.Bytes...)
	}
	return bytes.NewReader(buf)
}

func (ps *PartSet) StringShort() string {
	if ps == nil {
		return "nil-PartSet"
	} else {
		return fmt.Sprintf("(%v of %v)", ps.Count(), ps.Total())
	}
}
