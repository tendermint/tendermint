package blocks

import (
	"crypto/sha256"
	"fmt"
	"io"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/merkle"
)

const (
	defaultBlockPartSizeBytes = 4096
)

func CalcBlockURI(height uint32, hash []byte) string {
	return fmt.Sprintf("%v://block/%v#%X",
		config.Config.Network,
		height,
		hash,
	)
}

type Block struct {
	Header
	Validation
	Txs

	// Volatile
	hash []byte
}

func ReadBlock(r io.Reader) *Block {
	return &Block{
		Header:     ReadHeader(r),
		Validation: ReadValidation(r),
		Txs:        ReadTxs(r),
	}
}

func (b *Block) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(&b.Header, w, n, err)
	n, err = WriteTo(&b.Validation, w, n, err)
	n, err = WriteTo(&b.Txs, w, n, err)
	return
}

func (b *Block) ValidateBasic() error {
	// Basic validation that doesn't involve context.
	// XXX
	return nil
}

func (b *Block) URI() string {
	return CalcBlockURI(uint32(b.Height), b.Hash())
}

func (b *Block) Hash() []byte {
	if b.hash != nil {
		return b.hash
	} else {
		hashes := []Binary{
			ByteSlice(b.Header.Hash()),
			ByteSlice(b.Validation.Hash()),
			ByteSlice(b.Txs.Hash()),
		}
		// Merkle hash from sub-hashes.
		return merkle.HashFromBinarySlice(hashes)
	}
}

// The returns parts must be signed afterwards.
func (b *Block) ToBlockParts() (parts []*BlockPart) {
	blockBytes := BinaryBytes(b)
	total := (len(blockBytes) + defaultBlockPartSizeBytes - 1) / defaultBlockPartSizeBytes
	for i := 0; i < total; i++ {
		start := defaultBlockPartSizeBytes * i
		end := MinInt(start+defaultBlockPartSizeBytes, len(blockBytes))
		partBytes := make([]byte, end-start)
		copy(partBytes, blockBytes[start:end]) // Do not ref the original byteslice.
		part := &BlockPart{
			Height:    b.Height,
			Index:     uint16(i),
			Total:     uint16(total),
			Bytes:     partBytes,
			Signature: Signature{}, // No signature.
		}
		parts = append(parts, part)
	}
	return parts
}

//-----------------------------------------------------------------------------

/*
BlockPart represents a chunk of the bytes of a block.
Each block is divided into fixed length chunks (e.g. 4Kb)
for faster propagation across the gossip network.
*/
type BlockPart struct {
	Height uint32
	Round  uint16 // Add Round? Well I need to know...
	Index  uint16
	Total  uint16
	Bytes  ByteSlice
	Signature

	// Volatile
	hash []byte
}

func ReadBlockPart(r io.Reader) *BlockPart {
	return &BlockPart{
		Height:    Readuint32(r),
		Round:     Readuint16(r),
		Index:     Readuint16(r),
		Total:     Readuint16(r),
		Bytes:     ReadByteSlice(r),
		Signature: ReadSignature(r),
	}
}

func (bp *BlockPart) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(UInt32(bp.Height), w, n, err)
	n, err = WriteTo(UInt16(bp.Round), w, n, err)
	n, err = WriteTo(UInt16(bp.Index), w, n, err)
	n, err = WriteTo(UInt16(bp.Total), w, n, err)
	n, err = WriteTo(bp.Bytes, w, n, err)
	n, err = WriteTo(bp.Signature, w, n, err)
	return
}

func (bp *BlockPart) URI() string {
	return fmt.Sprintf("%v://block/%v/%v[%v/%v]#%X\n",
		config.Config.Network,
		bp.Height,
		bp.Round,
		bp.Index,
		bp.Total,
		bp.BlockPartHash(),
	)
}

func (bp *BlockPart) BlockPartHash() []byte {
	if bp.hash != nil {
		return bp.hash
	} else {
		hasher := sha256.New()
		hasher.Write(bp.Bytes)
		bp.hash = hasher.Sum(nil)
		return bp.hash
	}
}

// Signs the URI, which includes all data and metadata.
// XXX implement or change
func (bp *BlockPart) Sign(acc *PrivAccount) {
	// TODO: populate Signature
}

// XXX maybe change.
func (bp *BlockPart) ValidateWithSigner(signer *Account) error {
	// TODO: Sanity check height, index, total, bytes, etc.
	if !signer.Verify([]byte(bp.URI()), bp.Signature.Bytes) {
		return ErrInvalidBlockPartSignature
	}
	return nil
}

//-----------------------------------------------------------------------------

/* Header is part of a Block */
type Header struct {
	Name           String
	Height         uint32
	Fees           uint64
	Time           Time
	PrevHash       ByteSlice
	ValidationHash ByteSlice
	TxsHash        ByteSlice

	// Volatile
	hash []byte
}

func ReadHeader(r io.Reader) Header {
	return Header{
		Name:           ReadString(r),
		Height:         Readuint32(r),
		Fees:           Readuint64(r),
		Time:           ReadTime(r),
		PrevHash:       ReadByteSlice(r),
		ValidationHash: ReadByteSlice(r),
		TxsHash:        ReadByteSlice(r),
	}
}

func (h *Header) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(h.Name, w, n, err)
	n, err = WriteTo(UInt32(h.Height), w, n, err)
	n, err = WriteTo(UInt64(h.Fees), w, n, err)
	n, err = WriteTo(h.Time, w, n, err)
	n, err = WriteTo(h.PrevHash, w, n, err)
	n, err = WriteTo(h.ValidationHash, w, n, err)
	n, err = WriteTo(h.TxsHash, w, n, err)
	return
}

func (h *Header) Hash() []byte {
	if h.hash != nil {
		return h.hash
	} else {
		hasher := sha256.New()
		_, err := h.WriteTo(hasher)
		if err != nil {
			panic(err)
		}
		h.hash = hasher.Sum(nil)
		return h.hash
	}
}

/* Validation is part of a block */
type Validation struct {
	Signatures  []Signature
	Adjustments []Adjustment

	// Volatile
	hash []byte
}

func ReadValidation(r io.Reader) Validation {
	numSigs := Readuint32(r)
	numAdjs := Readuint32(r)
	sigs := make([]Signature, 0, numSigs)
	for i := uint32(0); i < numSigs; i++ {
		sigs = append(sigs, ReadSignature(r))
	}
	adjs := make([]Adjustment, 0, numAdjs)
	for i := uint32(0); i < numAdjs; i++ {
		adjs = append(adjs, ReadAdjustment(r))
	}
	return Validation{
		Signatures:  sigs,
		Adjustments: adjs,
	}
}

func (v *Validation) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(UInt32(len(v.Signatures)), w, n, err)
	n, err = WriteTo(UInt32(len(v.Adjustments)), w, n, err)
	for _, sig := range v.Signatures {
		n, err = WriteTo(sig, w, n, err)
	}
	for _, adj := range v.Adjustments {
		n, err = WriteTo(adj, w, n, err)
	}
	return
}

func (v *Validation) Hash() []byte {
	if v.hash != nil {
		return v.hash
	} else {
		hasher := sha256.New()
		_, err := v.WriteTo(hasher)
		if err != nil {
			panic(err)
		}
		v.hash = hasher.Sum(nil)
		return v.hash
	}
}

/* Txs is part of a block */
type Txs struct {
	Txs []Tx

	// Volatile
	hash []byte
}

func ReadTxs(r io.Reader) Txs {
	numTxs := Readuint32(r)
	txs := make([]Tx, 0, numTxs)
	for i := uint32(0); i < numTxs; i++ {
		txs = append(txs, ReadTx(r))
	}
	return Txs{Txs: txs}
}

func (txs *Txs) WriteTo(w io.Writer) (n int64, err error) {
	n, err = WriteTo(UInt32(len(txs.Txs)), w, n, err)
	for _, tx := range txs.Txs {
		n, err = WriteTo(tx, w, n, err)
	}
	return
}

func (txs *Txs) Hash() []byte {
	if txs.hash != nil {
		return txs.hash
	} else {
		bs := make([]Binary, 0, len(txs.Txs))
		for i, tx := range txs.Txs {
			bs[i] = Binary(tx)
		}
		txs.hash = merkle.HashFromBinarySlice(bs)
		return txs.hash
	}
}
