package types

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"sync"

	"golang.org/x/crypto/ripemd160"

	"github.com/tendermint/go-wire"
	"github.com/tendermint/go-wire/data"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/merkle"
)

var (
	ErrPartSetUnexpectedIndex = errors.New("Error part set unexpected index")
	ErrPartSetInvalidProof    = errors.New("Error part set invalid proof")
)

type Part struct {
	Index int                `json:"index"`
	Bytes data.Bytes         `json:"bytes"`
	Proof merkle.SimpleProof `json:"proof"`

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
	return fmt.Sprintf(`Part{#%v
%s  Bytes: %X...
%s  Proof: %v
%s}`,
		part.Index,
		indent, cmn.Fingerprint(part.Bytes),
		indent, part.Proof.StringIndented(indent+"  "),
		indent)
}

//-------------------------------------

type PartSetHeader struct {
	Total int        `json:"total"`
	Hash  data.Bytes `json:"hash"`
}

func (psh PartSetHeader) String() string {
	return fmt.Sprintf("%v:%X", psh.Total, cmn.Fingerprint(psh.Hash))
}

func (psh PartSetHeader) IsZero() bool {
	return psh.Total == 0
}

func (psh PartSetHeader) Equals(other PartSetHeader) bool {
	return psh.Total == other.Total && bytes.Equal(psh.Hash, other.Hash)
}

func (psh PartSetHeader) WriteSignBytes(w io.Writer, n *int, err *error) {
	wire.WriteJSON(CanonicalPartSetHeader(psh), w, n, err)
}

//-------------------------------------

type PartSet struct {
	total int
	hash  []byte

	mtx           sync.Mutex
	parts         []*Part
	partsBitArray *cmn.BitArray
	count         int

	Abundance *PartCounts // how many peers have each part
}

// Returns an immutable, full PartSet from the data bytes.
// The data bytes are split into "partSize" chunks, and merkle tree computed.
func NewPartSetFromData(data []byte, partSize int) *PartSet {
	// divide data into 4kb parts.
	total := (len(data) + partSize - 1) / partSize
	parts := make([]*Part, total)
	parts_ := make([]merkle.Hashable, total)
	partsBitArray := cmn.NewBitArray(total)
	for i := 0; i < total; i++ {
		part := &Part{
			Index: i,
			Bytes: data[i*partSize : cmn.MinInt(len(data), (i+1)*partSize)],
		}
		parts[i] = part
		parts_[i] = part
		partsBitArray.SetIndex(i, true)
	}
	// Compute merkle proofs
	root, proofs := merkle.SimpleProofsFromHashables(parts_)
	for i := 0; i < total; i++ {
		parts[i].Proof = *proofs[i]
	}
	return &PartSet{
		total:         total,
		hash:          root,
		parts:         parts,
		partsBitArray: partsBitArray,
		count:         total,
		Abundance:     NewPartCounts(total),
	}
}

// Returns an empty PartSet ready to be populated.
func NewPartSetFromHeader(header PartSetHeader) *PartSet {
	return &PartSet{
		total:         header.Total,
		hash:          header.Hash,
		parts:         make([]*Part, header.Total),
		partsBitArray: cmn.NewBitArray(header.Total),
		count:         0,
		Abundance:     NewPartCounts(header.Total),
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

func (ps *PartSet) BitArray() *cmn.BitArray {
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

func (ps *PartSet) AddPart(part *Part, verify bool) (bool, error) {
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

	// Check hash proof
	if verify {
		if !part.Proof.Verify(part.Index, ps.total, part.Hash(), ps.Hash()) {
			return false, ErrPartSetInvalidProof
		}
	}

	// Add part
	ps.parts[part.Index] = part
	ps.partsBitArray.SetIndex(part.Index, true)
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
		cmn.PanicSanity("Cannot GetReader() on incomplete PartSet")
	}
	return NewPartSetReader(ps.parts)
}

type PartSetReader struct {
	i      int
	parts  []*Part
	reader *bytes.Reader
}

func NewPartSetReader(parts []*Part) *PartSetReader {
	return &PartSetReader{
		i:      0,
		parts:  parts,
		reader: bytes.NewReader(parts[0].Bytes),
	}
}

func (psr *PartSetReader) Read(p []byte) (n int, err error) {
	readerLen := psr.reader.Len()
	if readerLen >= len(p) {
		return psr.reader.Read(p)
	} else if readerLen > 0 {
		n1, err := psr.Read(p[:readerLen])
		if err != nil {
			return n1, err
		}
		n2, err := psr.Read(p[readerLen:])
		return n1 + n2, err
	}

	psr.i += 1
	if psr.i >= len(psr.parts) {
		return 0, io.EOF
	}
	psr.reader = bytes.NewReader(psr.parts[psr.i].Bytes)
	return psr.Read(p)
}

func (ps *PartSet) StringShort() string {
	if ps == nil {
		return "nil-PartSet"
	} else {
		ps.mtx.Lock()
		defer ps.mtx.Unlock()
		return fmt.Sprintf("(%v of %v)", ps.Count(), ps.Total())
	}
}

//--------------------------------------
// for gossiping parts rarest first

// TODO: access individual counts atomically instead
type PartCounts struct {
	mtx    sync.Mutex
	counts []uint32

	lowestIndices []int // allocated once, used to randomly choose one of the lowest indices each time
}

func NewPartCounts(size int) *PartCounts {
	return &PartCounts{
		counts:        make([]uint32, size),
		lowestIndices: make([]int, size),
	}
}

// check() checks that the peer is hearing about the part for the first time.
// the check/update must happen atomically on PartCounts since peer parts
// are updated concurrently
func (pc *PartCounts) CheckAndIncrementIndex(index int, check func() bool) {
	pc.mtx.Lock()
	defer pc.mtx.Unlock()
	if check() {
		pc.counts[index] += 1
	}
}

// panics if index >= len(pc.counts)
func (pc *PartCounts) IncrementIndex(index int) {
	pc.mtx.Lock()
	defer pc.mtx.Unlock()
	pc.counts[index] += 1
}

// Pick the first index from the bit array with the lowest count, increment the index,
// and pass it to the setFunc.
// Index must be incremented so that multiple concurrent calls don't pick the same value,
// and passed to setFunc so the abundance doesnt get accidentally incremented again
// Assumes possibleParts.Size() == len(pc.counts)
func (pc *PartCounts) PickRarest(possibleParts *cmn.BitArray, setFunc func(int)) int {
	pc.mtx.Lock()
	defer pc.mtx.Unlock()

	var lowest uint32 = 2 << 30 // lowest abundance
	var lowestCount int         // degeneracy of the lowest abundance
	for i, count := range pc.counts {
		if possibleParts.GetIndex(i) {
			if count < lowest {
				lowest = count
				pc.lowestIndices[0] = i
				lowestCount = 1
			} else if count == lowest {
				pc.lowestIndices[lowestCount] = i
				lowestCount += 1
			}
		}
	}
	var lowestIndex int
	if lowestCount > 1 {
		// pick a random one
		lowestIndex = pc.lowestIndices[rand.Intn(lowestCount)]
	} else {
		lowestIndex = pc.lowestIndices[0]
	}
	pc.counts[lowestIndex] += 1
	setFunc(lowestIndex)
	return lowestIndex
}
