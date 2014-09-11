package blocks

import (
	"crypto/sha256"
	"io"
	"time"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/merkle"
)

const (
	defaultBlockPartSizeBytes = 4096
)

type Block struct {
	Header
	Validation
	Data

	// Volatile
	hash []byte
}

func ReadBlock(r io.Reader, n *int64, err *error) *Block {
	return &Block{
		Header:     ReadHeader(r, n, err),
		Validation: ReadValidation(r, n, err),
		Data:       ReadData(r, n, err),
	}
}

func (b *Block) WriteTo(w io.Writer) (n int64, err error) {
	WriteBinary(w, &b.Header, &n, &err)
	WriteBinary(w, &b.Validation, &n, &err)
	WriteBinary(w, &b.Data, &n, &err)
	return
}

func (b *Block) ValidateBasic() error {
	// TODO Basic validation that doesn't involve context.
	return nil
}

func (b *Block) Hash() []byte {
	if b.hash != nil {
		return b.hash
	} else {
		hashes := [][]byte{
			b.Header.Hash(),
			b.Validation.Hash(),
			b.Data.Hash(),
		}
		// Merkle hash from sub-hashes.
		return merkle.HashFromByteSlices(hashes)
	}
}

// The returns parts must be signed afterwards.
func (b *Block) ToBlockPartSet() *BlockPartSet {
	var parts []*BlockPart
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
	return NewBlockPartSet(b.Height, parts)
}

// Makes an empty next block.
func (b *Block) MakeNextBlock() *Block {
	return &Block{
		Header: Header{
			Name:   b.Header.Name,
			Height: b.Header.Height + 1,
			//Fees:                uint64(0),
			Time:     time.Now(),
			PrevHash: b.Hash(),
			//ValidationStateHash: nil,
			//AccountStateHash:    nil,
		},
	}
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

// Hash returns the hash of the block part data bytes.
func (bp *BlockPart) Hash() []byte {
	if bp.hash != nil {
		return bp.hash
	} else {
		hasher := sha256.New()
		hasher.Write(bp.Bytes)
		bp.hash = hasher.Sum(nil)
		return bp.hash
	}
}

//-----------------------------------------------------------------------------

type Header struct {
	Name                string
	Height              uint32
	Fees                uint64
	Time                time.Time
	PrevHash            []byte
	ValidationStateHash []byte
	AccountStateHash    []byte

	// Volatile
	hash []byte
}

func ReadHeader(r io.Reader, n *int64, err *error) (h Header) {
	if *err != nil {
		return Header{}
	}
	return Header{
		Name:                ReadString(r, n, err),
		Height:              ReadUInt32(r, n, err),
		Fees:                ReadUInt64(r, n, err),
		Time:                ReadTime(r, n, err),
		PrevHash:            ReadByteSlice(r, n, err),
		ValidationStateHash: ReadByteSlice(r, n, err),
		AccountStateHash:    ReadByteSlice(r, n, err),
	}
}

func (h *Header) WriteTo(w io.Writer) (n int64, err error) {
	WriteString(w, h.Name, &n, &err)
	WriteUInt32(w, h.Height, &n, &err)
	WriteUInt64(w, h.Fees, &n, &err)
	WriteTime(w, h.Time, &n, &err)
	WriteByteSlice(w, h.PrevHash, &n, &err)
	WriteByteSlice(w, h.ValidationStateHash, &n, &err)
	WriteByteSlice(w, h.AccountStateHash, &n, &err)
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

//-----------------------------------------------------------------------------

type Validation struct {
	Signatures []Signature

	// Volatile
	hash []byte
}

func ReadValidation(r io.Reader, n *int64, err *error) Validation {
	numSigs := ReadUInt32(r, n, err)
	sigs := make([]Signature, 0, numSigs)
	for i := uint32(0); i < numSigs; i++ {
		sigs = append(sigs, ReadSignature(r, n, err))
	}
	return Validation{
		Signatures: sigs,
	}
}

func (v *Validation) WriteTo(w io.Writer) (n int64, err error) {
	WriteUInt32(w, uint32(len(v.Signatures)), &n, &err)
	for _, sig := range v.Signatures {
		WriteBinary(w, sig, &n, &err)
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

//-----------------------------------------------------------------------------

type Data struct {
	Txs []Tx

	// Volatile
	hash []byte
}

func ReadData(r io.Reader, n *int64, err *error) Data {
	numTxs := ReadUInt32(r, n, err)
	txs := make([]Tx, 0, numTxs)
	for i := uint32(0); i < numTxs; i++ {
		txs = append(txs, ReadTx(r, n, err))
	}
	return Data{Txs: txs}
}

func (data *Data) WriteTo(w io.Writer) (n int64, err error) {
	WriteUInt32(w, uint32(len(data.Txs)), &n, &err)
	for _, tx := range data.Txs {
		WriteBinary(w, tx, &n, &err)
	}
	return
}

func (data *Data) Hash() []byte {
	if data.hash != nil {
		return data.hash
	} else {
		bs := make([]Binary, 0, len(data.Txs))
		for i, tx := range data.Txs {
			bs[i] = Binary(tx)
		}
		data.hash = merkle.HashFromBinarySlice(bs)
		return data.hash
	}
}
