package blocks

import (
	"crypto/sha256"
	"fmt"
	"io"
	"time"

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

func ReadBlock(r io.Reader, n *int64, err *error) *Block {
	return &Block{
		Header:     ReadHeader(r, n, err),
		Validation: ReadValidation(r, n, err),
		Txs:        ReadTxs(r, n, err),
	}
}

func (b *Block) WriteTo(w io.Writer) (n int64, err error) {
	WriteBinary(w, &b.Header, &n, &err)
	WriteBinary(w, &b.Validation, &n, &err)
	WriteBinary(w, &b.Txs, &n, &err)
	return
}

func (b *Block) ValidateBasic() error {
	// Basic validation that doesn't involve context.
	// XXX
	return nil
}

func (b *Block) URI() string {
	return CalcBlockURI(b.Height, b.Hash())
}

func (b *Block) Hash() []byte {
	if b.hash != nil {
		return b.hash
	} else {
		hashes := [][]byte{
			b.Header.Hash(),
			b.Validation.Hash(),
			b.Txs.Hash(),
		}
		// Merkle hash from sub-hashes.
		return merkle.HashFromByteSlices(hashes)
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
	Bytes  []byte
	Signature

	// Volatile
	hash []byte
}

func ReadBlockPart(r io.Reader, n *int64, err *error) *BlockPart {
	return &BlockPart{
		Height:    ReadUInt32(r, n, err),
		Round:     ReadUInt16(r, n, err),
		Index:     ReadUInt16(r, n, err),
		Total:     ReadUInt16(r, n, err),
		Bytes:     ReadByteSlice(r, n, err),
		Signature: ReadSignature(r, n, err),
	}
}

func (bp *BlockPart) WriteTo(w io.Writer) (n int64, err error) {
	WriteUInt32(w, bp.Height, &n, &err)
	WriteUInt16(w, bp.Round, &n, &err)
	WriteUInt16(w, bp.Index, &n, &err)
	WriteUInt16(w, bp.Total, &n, &err)
	WriteByteSlice(w, bp.Bytes, &n, &err)
	WriteBinary(w, bp.Signature, &n, &err)
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
	Name           string
	Height         uint32
	Fees           uint64
	Time           time.Time
	PrevHash       []byte
	ValidationHash []byte
	TxsHash        []byte

	// Volatile
	hash []byte
}

func ReadHeader(r io.Reader, n *int64, err *error) (h Header) {
	if *err != nil {
		return Header{}
	}
	return Header{
		Name:           ReadString(r, n, err),
		Height:         ReadUInt32(r, n, err),
		Fees:           ReadUInt64(r, n, err),
		Time:           ReadTime(r, n, err),
		PrevHash:       ReadByteSlice(r, n, err),
		ValidationHash: ReadByteSlice(r, n, err),
		TxsHash:        ReadByteSlice(r, n, err),
	}
}

func (h *Header) WriteTo(w io.Writer) (n int64, err error) {
	WriteString(w, h.Name, &n, &err)
	WriteUInt32(w, h.Height, &n, &err)
	WriteUInt64(w, h.Fees, &n, &err)
	WriteTime(w, h.Time, &n, &err)
	WriteByteSlice(w, h.PrevHash, &n, &err)
	WriteByteSlice(w, h.ValidationHash, &n, &err)
	WriteByteSlice(w, h.TxsHash, &n, &err)
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

func ReadValidation(r io.Reader, n *int64, err *error) Validation {
	numSigs := ReadUInt32(r, n, err)
	numAdjs := ReadUInt32(r, n, err)
	sigs := make([]Signature, 0, numSigs)
	for i := uint32(0); i < numSigs; i++ {
		sigs = append(sigs, ReadSignature(r, n, err))
	}
	adjs := make([]Adjustment, 0, numAdjs)
	for i := uint32(0); i < numAdjs; i++ {
		adjs = append(adjs, ReadAdjustment(r, n, err))
	}
	return Validation{
		Signatures:  sigs,
		Adjustments: adjs,
	}
}

func (v *Validation) WriteTo(w io.Writer) (n int64, err error) {
	WriteUInt32(w, uint32(len(v.Signatures)), &n, &err)
	WriteUInt32(w, uint32(len(v.Adjustments)), &n, &err)
	for _, sig := range v.Signatures {
		WriteBinary(w, sig, &n, &err)
	}
	for _, adj := range v.Adjustments {
		WriteBinary(w, adj, &n, &err)
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

func ReadTxs(r io.Reader, n *int64, err *error) Txs {
	numTxs := ReadUInt32(r, n, err)
	txs := make([]Tx, 0, numTxs)
	for i := uint32(0); i < numTxs; i++ {
		txs = append(txs, ReadTx(r, n, err))
	}
	return Txs{Txs: txs}
}

func (txs *Txs) WriteTo(w io.Writer) (n int64, err error) {
	WriteUInt32(w, uint32(len(txs.Txs)), &n, &err)
	for _, tx := range txs.Txs {
		WriteBinary(w, tx, &n, &err)
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
