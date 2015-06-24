package types

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
	"github.com/tendermint/tendermint/merkle"
)

type Block struct {
	*Header        `json:"header"`
	*Data          `json:"data"`
	LastValidation *Validation `json:"last_validation"`
}

// Basic validation that doesn't involve state data.
func (b *Block) ValidateBasic(chainID string, lastBlockHeight uint, lastBlockHash []byte,
	lastBlockParts PartSetHeader, lastBlockTime time.Time) error {
	if b.ChainID != chainID {
		return errors.New("Wrong Block.Header.ChainID")
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
	/*	TODO: Determine bounds
		See blockchain/reactor "stopSyncingDurationMinutes"

		if !b.Time.After(lastBlockTime) {
			return errors.New("Invalid Block.Header.Time")
		}
	*/
	if b.Header.Height != 1 {
		if err := b.LastValidation.ValidateBasic(); err != nil {
			return err
		}
	}
	// XXX more validation
	return nil
}

// Computes and returns the block hash.
// If the block is incomplete (e.g. missing Header.StateHash)
// then the hash is nil, to prevent the usage of that hash.
func (b *Block) Hash() []byte {
	if b.Header == nil || b.Data == nil || b.LastValidation == nil {
		return nil
	}
	hashHeader := b.Header.Hash()
	hashData := b.Data.Hash()
	hashLastValidation := b.LastValidation.Hash()

	// If hashHeader is nil, required fields are missing.
	if len(hashHeader) == 0 {
		return nil
	}

	// Merkle hash from subhashes.
	hashes := [][]byte{hashHeader, hashData, hashLastValidation}
	return merkle.SimpleHashFromHashes(hashes)
}

func (b *Block) MakePartSet() *PartSet {
	return NewPartSetFromData(binary.BinaryBytes(b))
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
	if b == nil {
		return "nil-Block"
	}
	return fmt.Sprintf(`Block{
%s  %v
%s  %v
%s  %v
%s}#%X`,
		indent, b.Header.StringIndented(indent+"  "),
		indent, b.Data.StringIndented(indent+"  "),
		indent, b.LastValidation.StringIndented(indent+"  "),
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
	ChainID        string        `json:"chain_id"`
	Height         uint          `json:"height"`
	Time           time.Time     `json:"time"`
	Fees           uint64        `json:"fees"`
	NumTxs         uint          `json:"num_txs"`
	LastBlockHash  []byte        `json:"last_block_hash"`
	LastBlockParts PartSetHeader `json:"last_block_parts"`
	StateHash      []byte        `json:"state_hash"`
}

// NOTE: hash is nil if required fields are missing.
func (h *Header) Hash() []byte {
	if len(h.StateHash) == 0 {
		return nil
	}

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
	if h == nil {
		return "nil-Header"
	}
	return fmt.Sprintf(`Header{
%s  ChainID:        %v
%s  Height:         %v
%s  Time:           %v
%s  Fees:           %v
%s  NumTxs:			%v
%s  LastBlockHash:  %X
%s  LastBlockParts: %v
%s  StateHash:      %X
%s}#%X`,
		indent, h.ChainID,
		indent, h.Height,
		indent, h.Time,
		indent, h.Fees,
		indent, h.NumTxs,
		indent, h.LastBlockHash,
		indent, h.LastBlockParts,
		indent, h.StateHash,
		indent, h.Hash())
}

//-------------------------------------

// NOTE: Validation is empty for height 1, but never nil.
type Validation struct {
	// NOTE: The Precommits are in order of address to preserve the bonded ValidatorSet order.
	// Any peer with a block can gossip precommits by index with a peer without recalculating the
	// active ValidatorSet.
	Precommits []*Vote `json:"precommits"`

	// Volatile
	hash     []byte
	bitArray *BitArray
}

func (v *Validation) Height() uint {
	if len(v.Precommits) == 0 {
		return 0
	}
	return v.Precommits[0].Height
}

func (v *Validation) Round() uint {
	if len(v.Precommits) == 0 {
		return 0
	}
	return v.Precommits[0].Round
}

func (v *Validation) ValidateBasic() error {
	if len(v.Precommits) == 0 {
		return errors.New("No precommits in validation")
	}
	height, round := v.Height(), v.Round()
	for _, precommit := range v.Precommits {
		// It's OK for precommits to be missing.
		if precommit == nil {
			continue
		}
		// Ensure that all votes are precommits
		if precommit.Type != VoteTypePrecommit {
			return fmt.Errorf("Invalid validation vote. Expected precommit, got %v",
				precommit.Type)
		}
		// Ensure that all heights are the same
		if precommit.Height != height {
			return fmt.Errorf("Invalid validation precommit height. Expected %v, got %v",
				height, precommit.Height)
		}
		// Ensure that all rounds are the same
		if precommit.Round != round {
			return fmt.Errorf("Invalid validation precommit round. Expected %v, got %v",
				round, precommit.Round)
		}
	}
	return nil
}

func (v *Validation) Hash() []byte {
	if v.hash == nil {
		bs := make([]interface{}, len(v.Precommits))
		for i, precommit := range v.Precommits {
			bs[i] = precommit
		}
		v.hash = merkle.SimpleHashFromBinaries(bs)
	}
	return v.hash
}

func (v *Validation) StringIndented(indent string) string {
	if v == nil {
		return "nil-Validation"
	}
	precommitStrings := make([]string, len(v.Precommits))
	for i, precommit := range v.Precommits {
		precommitStrings[i] = precommit.String()
	}
	return fmt.Sprintf(`Validation{
%s  Precommits: %v
%s}#%X`,
		indent, strings.Join(precommitStrings, "\n"+indent+"  "),
		indent, v.hash)
}

func (v *Validation) BitArray() *BitArray {
	if v.bitArray == nil {
		v.bitArray = NewBitArray(uint(len(v.Precommits)))
		for i, precommit := range v.Precommits {
			v.bitArray.SetIndex(uint(i), precommit != nil)
		}
	}
	return v.bitArray
}

//-----------------------------------------------------------------------------

type Data struct {
	Txs []Tx `json:"txs"`

	// Volatile
	hash []byte
}

func (data *Data) Hash() []byte {
	if data.hash == nil {
		bs := make([]interface{}, len(data.Txs))
		for i, tx := range data.Txs {
			bs[i] = account.SignBytes(config.GetString("chain_id"), tx)
		}
		data.hash = merkle.SimpleHashFromBinaries(bs)
	}
	return data.hash
}

func (data *Data) StringIndented(indent string) string {
	if data == nil {
		return "nil-Data"
	}
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
