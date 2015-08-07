package types

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"

	acm "github.com/tendermint/tendermint/account"
	. "github.com/tendermint/tendermint/common"
	"github.com/tendermint/tendermint/merkle"
	"github.com/tendermint/tendermint/wire"
)

type Block struct {
	*Header        `json:"header"`
	*Data          `json:"data"`
	LastValidation *Validation `json:"last_validation"`
}

// Basic validation that doesn't involve state data.
func (b *Block) ValidateBasic(chainID string, lastBlockHeight int, lastBlockHash []byte,
	lastBlockParts PartSetHeader, lastBlockTime time.Time) error {
	if b.ChainID != chainID {
		return errors.New(Fmt("Wrong Block.Header.ChainID. Expected %v, got %v", chainID, b.ChainID))
	}
	if b.Height != lastBlockHeight+1 {
		return errors.New(Fmt("Wrong Block.Header.Height. Expected %v, got %v", lastBlockHeight+1, b.Height))
	}
	/*	TODO: Determine bounds for Time
		See blockchain/reactor "stopSyncingDurationMinutes"

		if !b.Time.After(lastBlockTime) {
			return errors.New("Invalid Block.Header.Time")
		}
	*/
	// TODO: validate Fees
	if b.NumTxs != len(b.Data.Txs) {
		return errors.New(Fmt("Wrong Block.Header.NumTxs. Expected %v, got %v", len(b.Data.Txs), b.NumTxs))
	}
	if !bytes.Equal(b.LastBlockHash, lastBlockHash) {
		return errors.New(Fmt("Wrong Block.Header.LastBlockHash.  Expected %X, got %X", lastBlockHash, b.LastBlockHash))
	}
	if !b.LastBlockParts.Equals(lastBlockParts) {
		return errors.New(Fmt("Wrong Block.Header.LastBlockParts. Expected %v, got %v", lastBlockParts, b.LastBlockParts))
	}
	if !bytes.Equal(b.LastValidationHash, b.LastValidation.Hash()) {
		return errors.New(Fmt("Wrong Block.Header.LastValidationHash.  Expected %X, got %X", b.LastValidationHash, b.LastValidation.Hash()))
	}
	if b.Header.Height != 1 {
		if err := b.LastValidation.ValidateBasic(); err != nil {
			return err
		}
	}
	if !bytes.Equal(b.DataHash, b.Data.Hash()) {
		return errors.New(Fmt("Wrong Block.Header.DataHash.  Expected %X, got %X", b.DataHash, b.Data.Hash()))
	}
	// NOTE: the StateHash is validated later.
	return nil
}

func (b *Block) FillHeader() {
	b.LastValidationHash = b.LastValidation.Hash()
	b.DataHash = b.Data.Hash()
}

// Computes and returns the block hash.
// If the block is incomplete (e.g. missing Header.StateHash)
// then the hash is nil, to prevent the usage of that hash.
func (b *Block) Hash() []byte {
	if b.Header == nil || b.Data == nil || b.LastValidation == nil {
		return nil
	}
	b.FillHeader()
	return b.Header.Hash()
}

func (b *Block) MakePartSet() *PartSet {
	return NewPartSetFromData(wire.BinaryBytes(b))
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
	ChainID            string        `json:"chain_id"`
	Height             int           `json:"height"`
	Time               time.Time     `json:"time"`
	Fees               int64         `json:"fees"`
	NumTxs             int           `json:"num_txs"`
	LastBlockHash      []byte        `json:"last_block_hash"`
	LastBlockParts     PartSetHeader `json:"last_block_parts"`
	LastValidationHash []byte        `json:"last_validation_hash"`
	DataHash           []byte        `json:"data_hash"`
	StateHash          []byte        `json:"state_hash"`
}

// NOTE: hash is nil if required fields are missing.
func (h *Header) Hash() []byte {
	if len(h.StateHash) == 0 {
		return nil
	}
	return merkle.SimpleHashFromMap(map[string]interface{}{
		"ChainID":        h.ChainID,
		"Height":         h.Height,
		"Time":           h.Time,
		"Fees":           h.Fees,
		"NumTxs":         h.NumTxs,
		"LastBlock":      h.LastBlockHash,
		"LastBlockParts": h.LastBlockParts,
		"LastValidation": h.LastValidationHash,
		"Data":           h.DataHash,
		"State":          h.StateHash,
	})
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
%s  NumTxs:         %v
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
	firstPrecommit *Vote
	hash           []byte
	bitArray       *BitArray
}

func (v *Validation) FirstPrecommit() *Vote {
	if len(v.Precommits) == 0 {
		return nil
	}
	if v.firstPrecommit != nil {
		return v.firstPrecommit
	}
	for _, precommit := range v.Precommits {
		if precommit != nil {
			v.firstPrecommit = precommit
			return precommit
		}
	}
	return nil
}

func (v *Validation) Height() int {
	if len(v.Precommits) == 0 {
		return 0
	}
	return v.FirstPrecommit().Height
}

func (v *Validation) Round() int {
	if len(v.Precommits) == 0 {
		return 0
	}
	return v.FirstPrecommit().Round
}

func (v *Validation) Type() byte {
	return VoteTypePrecommit
}

func (v *Validation) Size() int {
	if v == nil {
		return 0
	}
	return len(v.Precommits)
}

func (v *Validation) BitArray() *BitArray {
	if v.bitArray == nil {
		v.bitArray = NewBitArray(len(v.Precommits))
		for i, precommit := range v.Precommits {
			v.bitArray.SetIndex(i, precommit != nil)
		}
	}
	return v.bitArray
}

func (v *Validation) GetByIndex(index int) *Vote {
	return v.Precommits[index]
}

func (v *Validation) IsCommit() bool {
	if len(v.Precommits) == 0 {
		return false
	}
	return true
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
			bs[i] = acm.SignBytes(config.GetString("chain_id"), tx)
		}
		data.hash = merkle.SimpleHashFromBinaries(bs) // NOTE: leaves are TxIDs.
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
