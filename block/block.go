package block

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/merkle"
)

type Block struct {
	*Header
	*Validation
	*Data
}

// Basic validation that doesn't involve state data.
func (b *Block) ValidateBasic(lastBlockHeight uint, lastBlockHash []byte,
	lastBlockParts PartSetHeader, lastBlockTime time.Time) error {
	if b.Network != config.App().GetString("Network") {
		return errors.New("Wrong Block.Header.Network")
	}
	if b.Height != lastBlockHeight+1 {
		return errors.New("Wrong Block.Header.Height")
	}
	if b.NumTxs != uint(len(b.Data.Txs)) {
		return errors.New("Wrong Block.Header.NumTxs")
	}
	if !bytes.Equal(b.LastBlockHash, lastBlockHash) {
		return errors.New("Wrong Block.Header.LastBlockHash")
	}
	if !b.LastBlockParts.Equals(lastBlockParts) {
		return errors.New("Wrong Block.Header.LastBlockParts")
	}
	/*	TODO: Determine bounds.
		if !b.Time.After(lastBlockTime) {
			return errors.New("Invalid Block.Header.Time")
		}
	*/
	if b.Header.Height != 1 {
		if err := b.Validation.ValidateBasic(); err != nil {
			return err
		}
	}
	// XXX more validation
	return nil
}

func (b *Block) Hash() []byte {
	hashes := [][]byte{
		b.Header.Hash(),
		b.Validation.Hash(),
		b.Data.Hash(),
	}
	// Merkle hash from sub-hashes.
	return merkle.HashFromHashes(hashes)
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
	return b.StringIndented("")
}

func (b *Block) StringIndented(indent string) string {
	return fmt.Sprintf(`Block{
%s  %v
%s  %v
%s  %v
%s}#%X`,
		indent, b.Header.StringIndented(indent+"  "),
		indent, b.Validation.StringIndented(indent+"  "),
		indent, b.Data.StringIndented(indent+"  "),
		indent, b.Hash())
}

func (b *Block) StringShort() string {
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
	GasPrice       uint64
	NumTxs         uint
	LastBlockHash  []byte
	LastBlockParts PartSetHeader
	StateHash      []byte
}

// possible for actual gas price to be less than 1
var GasPriceDivisor = 1000000

func (h *Header) Hash() []byte {
	buf := new(bytes.Buffer)
	hasher, n, err := sha256.New(), new(int64), new(error)
	binary.WriteBinary(h, buf, n, err)
	if *err != nil {
		panic(err)
	}
	hasher.Write(buf.Bytes())
	hash := hasher.Sum(nil)
	return hash
}

func (h *Header) StringIndented(indent string) string {
	return fmt.Sprintf(`Header{
%s  Network:        %v
%s  Height:         %v
%s  Time:           %v
%s  Fees:           %v
%s  NumTxs:			%v
%s  LastBlockHash:  %X
%s  LastBlockParts: %v
%s  StateHash:      %X
%s}#%X`,
		indent, h.Network,
		indent, h.Height,
		indent, h.Time,
		indent, h.Fees,
		indent, h.NumTxs,
		indent, h.LastBlockHash,
		indent, h.LastBlockParts,
		indent, h.StateHash,
		indent, h.Hash())
}

//-----------------------------------------------------------------------------

type Commit struct {
	Address   []byte
	Round     uint
	Signature account.SignatureEd25519
}

func (commit Commit) IsZero() bool {
	return commit.Round == 0 && commit.Signature.IsZero()
}

func (commit Commit) String() string {
	return fmt.Sprintf("Commit{A:%X R:%v %X}", commit.Address, commit.Round, Fingerprint(commit.Signature))
}

//-------------------------------------

// NOTE: The Commits are in order of address to preserve the bonded ValidatorSet order.
// Any peer with a block can gossip commits by index with a peer without recalculating the
// active ValidatorSet.
type Validation struct {
	Commits []Commit // Commits (or nil) of all active validators in address order.

	// Volatile
	hash     []byte
	bitArray BitArray
}

func (v *Validation) ValidateBasic() error {
	if len(v.Commits) == 0 {
		return errors.New("No commits in validation")
	}
	lastAddress := []byte{}
	for i := 0; i < len(v.Commits); i++ {
		commit := v.Commits[i]
		if commit.IsZero() {
			if len(commit.Address) > 0 {
				return errors.New("Zero commits should not have an address")
			}
		} else {
			if len(commit.Address) == 0 {
				return errors.New("Nonzero commits should have an address")
			}
			if len(lastAddress) > 0 && bytes.Compare(lastAddress, commit.Address) != -1 {
				return errors.New("Invalid commit order")
			}
			lastAddress = commit.Address
		}
	}
	return nil
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

func (v *Validation) StringIndented(indent string) string {
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

func (data *Data) StringIndented(indent string) string {
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
