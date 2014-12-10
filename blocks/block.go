package blocks

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	. "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	. "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/merkle"
)

type Block struct {
	*Header
	*Validation
	*Data

	// Volatile
	hash []byte
}

// Basic validation that doesn't involve state data.
func (b *Block) ValidateBasic(lastBlockHeight uint, lastBlockHash []byte,
	lastBlockParts PartSetHeader, lastBlockTime time.Time) error {
	if b.Network != Config.Network {
		return errors.New("Invalid block network")
	}
	if b.Height != lastBlockHeight+1 {
		return errors.New("Invalid block height")
	}
	if !bytes.Equal(b.LastBlockHash, lastBlockHash) {
		return errors.New("Invalid block hash")
	}
	if !b.LastBlockParts.Equals(lastBlockParts) {
		return errors.New("Invalid block parts header")
	}
	if !b.Time.After(lastBlockTime) {
		return errors.New("Invalid block time")
	}
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
	Height         uint
	Time           time.Time
	Fees           uint64
	LastBlockHash  []byte
	LastBlockParts PartSetHeader
	StateHash      []byte

	// Volatile
	hash []byte
}

func (h *Header) Hash() []byte {
	if h.hash == nil {
		hasher, n, err := sha256.New(), new(int64), new(error)
		WriteBinary(h, hasher, n, err)
		if *err != nil {
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

type Commit struct {
	// It's not strictly needed here, but consider adding address here for convenience
	Round     uint
	Signature SignatureEd25519
}

func (commit Commit) IsZero() bool {
	return commit.Round == 0 && commit.Signature.IsZero()
}

func (commit Commit) String() string {
	return fmt.Sprintf("Commit{R:%v %X}", commit.Round, Fingerprint(commit.Signature.Bytes))
}

//-------------------------------------

type Validation struct {
	Commits []Commit

	// Volatile
	hash     []byte
	bitArray BitArray
}

func (v *Validation) Hash() []byte {
	if v.hash == nil {
		bs := make([]interface{}, len(v.Commits))
		for i, commit := range v.Commits {
			bs[i] = commit
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

func (v *Validation) BitArray() BitArray {
	if v.bitArray.IsZero() {
		v.bitArray = NewBitArray(uint(len(v.Commits)))
		for i, commit := range v.Commits {
			v.bitArray.SetIndex(uint(i), !commit.IsZero())
		}
	}
	return v.bitArray
}

//-----------------------------------------------------------------------------

type Data struct {
	Txs []Tx

	// Volatile
	hash []byte
}

func (data *Data) Hash() []byte {
	if data.hash == nil {
		bs := make([]interface{}, len(data.Txs))
		for i, tx := range data.Txs {
			bs[i] = tx
		}
		data.hash = merkle.HashFromBinaries(bs)
	}
	return data.hash
}

func (data *Data) StringWithIndent(indent string) string {
	txStrings := make([]string, len(data.Txs))
	for i, tx := range data.Txs {
		txStrings[i] = fmt.Sprintf("Tx:%v", tx)
	}
	return fmt.Sprintf(`Data{
%s  %v
%s}#%X`,
		indent, strings.Join(txStrings, "\n"+indent+"  "),
		indent, data.hash)
}
