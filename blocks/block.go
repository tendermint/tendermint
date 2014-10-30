package blocks

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"strings"
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
	if b.Header.Height != lastBlockHeight+1 {
		return ErrBlockInvalidBlockHeight
	}
	if !bytes.Equal(b.Header.LastBlockHash, lastBlockHash) {
		return ErrBlockInvalidLastBlockHash
	}
	// XXX We need to validate LastBlockParts too.
	// XXX more validation
	return nil
}

func (b *Block) Hash() []byte {
	if b.hash == nil {
		hashes := [][]byte{
			b.Header.Hash(),
			b.Validation.Hash(),
			b.Data.Hash(),
		}
		// Merkle hash from sub-hashes.
		b.hash = merkle.HashFromHashes(hashes)
	}
	return b.hash
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

func (b *Block) String() string {
	return b.StringWithIndent("")
}

func (b *Block) StringWithIndent(indent string) string {
	return fmt.Sprintf(`Block{
%s  %v
%s  %v
%s  %v
%s}#%X`,
		indent, b.Header.StringWithIndent(indent+"  "),
		indent, b.Validation.StringWithIndent(indent+"  "),
		indent, b.Data.StringWithIndent(indent+"  "),
		indent, b.hash)
}

func (b *Block) Description() string {
	if b == nil {
		return "nil-Block"
	} else {
		return fmt.Sprintf("Block#%X", b.Hash())
	}
}

//-----------------------------------------------------------------------------

type Header struct {
	Network        string
	Height         uint32
	Time           time.Time
	Fees           uint64
	LastBlockHash  []byte
	LastBlockParts PartSetHeader
	StateHash      []byte

	// Volatile
	hash []byte
}

func ReadHeader(r io.Reader, n *int64, err *error) (h Header) {
	if *err != nil {
		return Header{}
	}
	return Header{
		Network:        ReadString(r, n, err),
		Height:         ReadUInt32(r, n, err),
		Time:           ReadTime(r, n, err),
		Fees:           ReadUInt64(r, n, err),
		LastBlockHash:  ReadByteSlice(r, n, err),
		LastBlockParts: ReadPartSetHeader(r, n, err),
		StateHash:      ReadByteSlice(r, n, err),
	}
}

func (h *Header) WriteTo(w io.Writer) (n int64, err error) {
	WriteString(w, h.Network, &n, &err)
	WriteUInt32(w, h.Height, &n, &err)
	WriteTime(w, h.Time, &n, &err)
	WriteUInt64(w, h.Fees, &n, &err)
	WriteByteSlice(w, h.LastBlockHash, &n, &err)
	WriteBinary(w, h.LastBlockParts, &n, &err)
	WriteByteSlice(w, h.StateHash, &n, &err)
	return
}

func (h *Header) Hash() []byte {
	if h.hash == nil {
		hasher := sha256.New()
		_, err := h.WriteTo(hasher)
		if err != nil {
			panic(err)
		}
		h.hash = hasher.Sum(nil)
	}
	return h.hash
}

func (h *Header) StringWithIndent(indent string) string {
	return fmt.Sprintf(`Header{
%s  Network:        %v
%s  Height:         %v
%s  Time:           %v
%s  Fees:           %v
%s  LastBlockHash:  %X
%s  LastBlockParts: %v
%s  StateHash:      %X
%s}#%X`,
		indent, h.Network,
		indent, h.Height,
		indent, h.Time,
		indent, h.Fees,
		indent, h.LastBlockHash,
		indent, h.LastBlockParts,
		indent, h.StateHash,
		indent, h.hash)
}

//-----------------------------------------------------------------------------

type Validation struct {
	Commits []RoundSignature

	// Volatile
	hash []byte
}

func ReadValidation(r io.Reader, n *int64, err *error) Validation {
	return Validation{
		Commits: ReadRoundSignatures(r, n, err),
	}
}

func (v *Validation) WriteTo(w io.Writer) (n int64, err error) {
	WriteRoundSignatures(w, v.Commits, &n, &err)
	return
}

func (v *Validation) Hash() []byte {
	if v.hash == nil {
		bs := make([]Binary, len(v.Commits))
		for i, commit := range v.Commits {
			bs[i] = Binary(commit)
		}
		v.hash = merkle.HashFromBinaries(bs)
	}
	return v.hash
}

func (v *Validation) StringWithIndent(indent string) string {
	commitStrings := make([]string, len(v.Commits))
	for i, commit := range v.Commits {
		commitStrings[i] = commit.String()
	}
	return fmt.Sprintf(`Validation{
%s  %v
%s}#%X`,
		indent, strings.Join(commitStrings, "\n"+indent+"  "),
		indent, v.hash)
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
	if data.hash == nil {
		bs := make([]Binary, len(data.Txs))
		for i, tx := range data.Txs {
			bs[i] = Binary(tx)
		}
		data.hash = merkle.HashFromBinaries(bs)
	}
	return data.hash
}

func (data *Data) StringWithIndent(indent string) string {
	txStrings := make([]string, len(data.Txs))
	for i, tx := range data.Txs {
		txStrings[i] = tx.String()
	}
	return fmt.Sprintf(`Data{
%s  %v
%s}#%X`,
		indent, strings.Join(txStrings, "\n"+indent+"  "),
		indent, data.hash)
}
