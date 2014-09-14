package blocks

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"time"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/merkle"
)

var (
	ErrBlockInvalidNetwork       = errors.New("Error block invalid network")
	ErrBlockInvalidBlockHeight   = errors.New("Error block invalid height")
	ErrBlockInvalidLastBlockHash = errors.New("Error block invalid last blockhash")
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

// Basic validation that doesn't involve state data.
func (b *Block) ValidateBasic(lastBlockHeight uint32, lastBlockHash []byte) error {
	if b.Header.Network != Config.Network {
		return ErrBlockInvalidNetwork
	}
	if b.Header.Height != lastBlockHeight {
		return ErrBlockInvalidBlockHeight
	}
	if !bytes.Equal(b.Header.LastBlockHash, lastBlockHash) {
		return ErrBlockInvalidLastBlockHash
	}
	// XXX more validation
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
		return merkle.HashFromHashes(hashes)
	}
}

// Convenience.
// A nil block never hashes to anything.
// Nothing hashes to a nil hash.
func (b *Block) HashesTo(hash []byte) bool {
	if len(hash) == 0 {
		return false
	}
	if b == nil {
		return false
	}
	return bytes.Equal(b.Hash(), hash)
}

// Makes an empty next block.
func (b *Block) MakeNextBlock() *Block {
	return &Block{
		Header: Header{
			Network: b.Header.Network,
			Height:  b.Header.Height + 1,
			//Fees:                uint64(0),
			Time:          time.Now(),
			LastBlockHash: b.Hash(),
			//ValidationStateHash: nil,
			//AccountStateHash:    nil,
		},
	}
}

//-----------------------------------------------------------------------------

type Header struct {
	Network             string
	Height              uint32
	Fees                uint64
	Time                time.Time
	LastBlockHash       []byte
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
		Network:             ReadString(r, n, err),
		Height:              ReadUInt32(r, n, err),
		Fees:                ReadUInt64(r, n, err),
		Time:                ReadTime(r, n, err),
		LastBlockHash:       ReadByteSlice(r, n, err),
		ValidationStateHash: ReadByteSlice(r, n, err),
		AccountStateHash:    ReadByteSlice(r, n, err),
	}
}

func (h *Header) WriteTo(w io.Writer) (n int64, err error) {
	WriteString(w, h.Network, &n, &err)
	WriteUInt32(w, h.Height, &n, &err)
	WriteUInt64(w, h.Fees, &n, &err)
	WriteTime(w, h.Time, &n, &err)
	WriteByteSlice(w, h.LastBlockHash, &n, &err)
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
		data.hash = merkle.HashFromBinaries(bs)
		return data.hash
	}
}
