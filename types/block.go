package types

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-merkle"
	"github.com/tendermint/go-wire"
)

const MaxBlockSize = 22020096 // 21MB TODO make it configurable

type Block struct {
	*Header    `json:"header"`
	*Data      `json:"data"`
	LastCommit *Commit `json:"last_commit"`
}

// TODO: version
func MakeBlock(height int, chainID string, txs []Tx, commit *Commit,
	prevBlockID BlockID, valHash, appHash []byte, partSize int) (*Block, *PartSet) {
	block := &Block{
		Header: &Header{
			ChainID:        chainID,
			Height:         height,
			Time:           time.Now(),
			NumTxs:         len(txs),
			LastBlockID:    prevBlockID,
			ValidatorsHash: valHash,
			AppHash:        appHash, // state merkle root of txs from the previous block.
		},
		LastCommit: commit,
		Data: &Data{
			Txs: txs,
		},
	}
	block.FillHeader()
	return block, block.MakePartSet(partSize)
}

// Basic validation that doesn't involve state data.
func (b *Block) ValidateBasic(chainID string, lastBlockHeight int, lastBlockID BlockID,
	lastBlockTime time.Time, appHash []byte) error {
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
	if b.NumTxs != len(b.Data.Txs) {
		return errors.New(Fmt("Wrong Block.Header.NumTxs. Expected %v, got %v", len(b.Data.Txs), b.NumTxs))
	}
	if !b.LastBlockID.Equals(lastBlockID) {
		return errors.New(Fmt("Wrong Block.Header.LastBlockID.  Expected %v, got %v", lastBlockID, b.LastBlockID))
	}
	if !bytes.Equal(b.LastCommitHash, b.LastCommit.Hash()) {
		return errors.New(Fmt("Wrong Block.Header.LastCommitHash.  Expected %X, got %X", b.LastCommitHash, b.LastCommit.Hash()))
	}
	if b.Header.Height != 1 {
		if err := b.LastCommit.ValidateBasic(); err != nil {
			return err
		}
	}
	if !bytes.Equal(b.DataHash, b.Data.Hash()) {
		return errors.New(Fmt("Wrong Block.Header.DataHash.  Expected %X, got %X", b.DataHash, b.Data.Hash()))
	}
	if !bytes.Equal(b.AppHash, appHash) {
		return errors.New(Fmt("Wrong Block.Header.AppHash.  Expected %X, got %X", appHash, b.AppHash))
	}
	// NOTE: the AppHash and ValidatorsHash are validated later.
	return nil
}

func (b *Block) FillHeader() {
	if b.LastCommitHash == nil {
		b.LastCommitHash = b.LastCommit.Hash()
	}
	if b.DataHash == nil {
		b.DataHash = b.Data.Hash()
	}
}

// Computes and returns the block hash.
// If the block is incomplete, block hash is nil for safety.
func (b *Block) Hash() []byte {
	// fmt.Println(">>", b.Data)
	if b.Header == nil || b.Data == nil || b.LastCommit == nil {
		return nil
	}
	b.FillHeader()
	return b.Header.Hash()
}

func (b *Block) MakePartSet(partSize int) *PartSet {
	return NewPartSetFromData(wire.BinaryBytes(b), partSize)
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
		indent, b.LastCommit.StringIndented(indent+"  "),
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
	ChainID        string    `json:"chain_id"`
	Height         int       `json:"height"`
	Time           time.Time `json:"time"`
	NumTxs         int       `json:"num_txs"` // XXX: Can we get rid of this?
	LastBlockID    BlockID   `json:"last_block_id"`
	LastCommitHash []byte    `json:"last_commit_hash"` // commit from validators from the last block
	DataHash       []byte    `json:"data_hash"`        // transactions
	ValidatorsHash []byte    `json:"validators_hash"`  // validators for the current block
	AppHash        []byte    `json:"app_hash"`         // state after txs from the previous block
}

// NOTE: hash is nil if required fields are missing.
func (h *Header) Hash() []byte {
	if len(h.ValidatorsHash) == 0 {
		return nil
	}
	return merkle.SimpleHashFromMap(map[string]interface{}{
		"ChainID":     h.ChainID,
		"Height":      h.Height,
		"Time":        h.Time,
		"NumTxs":      h.NumTxs,
		"LastBlockID": h.LastBlockID,
		"LastCommit":  h.LastCommitHash,
		"Data":        h.DataHash,
		"Validators":  h.ValidatorsHash,
		"App":         h.AppHash,
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
%s  NumTxs:         %v
%s  LastBlockID:    %v
%s  LastCommit:     %X
%s  Data:           %X
%s  Validators:     %X
%s  App:            %X
%s}#%X`,
		indent, h.ChainID,
		indent, h.Height,
		indent, h.Time,
		indent, h.NumTxs,
		indent, h.LastBlockID,
		indent, h.LastCommitHash,
		indent, h.DataHash,
		indent, h.ValidatorsHash,
		indent, h.AppHash,
		indent, h.Hash())
}

//-------------------------------------

// NOTE: Commit is empty for height 1, but never nil.
type Commit struct {
	// NOTE: The Precommits are in order of address to preserve the bonded ValidatorSet order.
	// Any peer with a block can gossip precommits by index with a peer without recalculating the
	// active ValidatorSet.
	BlockID    BlockID `json:"blockID"`
	Precommits []*Vote `json:"precommits"`

	// Volatile
	firstPrecommit *Vote
	hash           []byte
	bitArray       *BitArray
}

func (commit *Commit) FirstPrecommit() *Vote {
	if len(commit.Precommits) == 0 {
		return nil
	}
	if commit.firstPrecommit != nil {
		return commit.firstPrecommit
	}
	for _, precommit := range commit.Precommits {
		if precommit != nil {
			commit.firstPrecommit = precommit
			return precommit
		}
	}
	return nil
}

func (commit *Commit) Height() int {
	if len(commit.Precommits) == 0 {
		return 0
	}
	return commit.FirstPrecommit().Height
}

func (commit *Commit) Round() int {
	if len(commit.Precommits) == 0 {
		return 0
	}
	return commit.FirstPrecommit().Round
}

func (commit *Commit) Type() byte {
	return VoteTypePrecommit
}

func (commit *Commit) Size() int {
	if commit == nil {
		return 0
	}
	return len(commit.Precommits)
}

func (commit *Commit) BitArray() *BitArray {
	if commit.bitArray == nil {
		commit.bitArray = NewBitArray(len(commit.Precommits))
		for i, precommit := range commit.Precommits {
			commit.bitArray.SetIndex(i, precommit != nil)
		}
	}
	return commit.bitArray
}

func (commit *Commit) GetByIndex(index int) *Vote {
	return commit.Precommits[index]
}

func (commit *Commit) IsCommit() bool {
	if len(commit.Precommits) == 0 {
		return false
	}
	return true
}

func (commit *Commit) ValidateBasic() error {
	if commit.BlockID.IsZero() {
		return errors.New("Commit cannot be for nil block")
	}
	if len(commit.Precommits) == 0 {
		return errors.New("No precommits in commit")
	}
	height, round := commit.Height(), commit.Round()

	// validate the precommits
	for _, precommit := range commit.Precommits {
		// It's OK for precommits to be missing.
		if precommit == nil {
			continue
		}
		// Ensure that all votes are precommits
		if precommit.Type != VoteTypePrecommit {
			return fmt.Errorf("Invalid commit vote. Expected precommit, got %v",
				precommit.Type)
		}
		// Ensure that all heights are the same
		if precommit.Height != height {
			return fmt.Errorf("Invalid commit precommit height. Expected %v, got %v",
				height, precommit.Height)
		}
		// Ensure that all rounds are the same
		if precommit.Round != round {
			return fmt.Errorf("Invalid commit precommit round. Expected %v, got %v",
				round, precommit.Round)
		}
	}
	return nil
}

func (commit *Commit) Hash() []byte {
	if commit.hash == nil {
		bs := make([]interface{}, len(commit.Precommits))
		for i, precommit := range commit.Precommits {
			bs[i] = precommit
		}
		commit.hash = merkle.SimpleHashFromBinaries(bs)
	}
	return commit.hash
}

func (commit *Commit) StringIndented(indent string) string {
	if commit == nil {
		return "nil-Commit"
	}
	precommitStrings := make([]string, len(commit.Precommits))
	for i, precommit := range commit.Precommits {
		precommitStrings[i] = precommit.String()
	}
	return fmt.Sprintf(`Commit{
%s  BlockID:    %v
%s  Precommits: %v
%s}#%X`,
		indent, commit.BlockID,
		indent, strings.Join(precommitStrings, "\n"+indent+"  "),
		indent, commit.hash)
}

//-----------------------------------------------------------------------------

type Data struct {

	// Txs that will be applied by state @ block.Height+1.
	// NOTE: not all txs here are valid.  We're just agreeing on the order first.
	// This means that block.AppHash does not include these txs.
	Txs Txs `json:"txs"`

	// Volatile
	hash []byte
}

func (data *Data) Hash() []byte {
	if data.hash == nil {
		data.hash = data.Txs.Hash() // NOTE: leaves of merkle tree are TxIDs
	}
	return data.hash
}

func (data *Data) StringIndented(indent string) string {
	if data == nil {
		return "nil-Data"
	}
	txStrings := make([]string, MinInt(len(data.Txs), 21))
	for i, tx := range data.Txs {
		if i == 20 {
			txStrings[i] = fmt.Sprintf("... (%v total)", len(data.Txs))
			break
		}
		txStrings[i] = fmt.Sprintf("Tx:%v", tx)
	}
	return fmt.Sprintf(`Data{
%s  %v
%s}#%X`,
		indent, strings.Join(txStrings, "\n"+indent+"  "),
		indent, data.hash)
}

//--------------------------------------------------------------------------------

type BlockID struct {
	Hash        []byte        `json:"hash"`
	PartsHeader PartSetHeader `json:"parts"`
}

func (blockID BlockID) IsZero() bool {
	return len(blockID.Hash) == 0 && blockID.PartsHeader.IsZero()
}

func (blockID BlockID) Equals(other BlockID) bool {
	return bytes.Equal(blockID.Hash, other.Hash) &&
		blockID.PartsHeader.Equals(other.PartsHeader)
}

func (blockID BlockID) Key() string {
	return string(blockID.Hash) + string(wire.BinaryBytes(blockID.PartsHeader))
}

func (blockID BlockID) WriteSignBytes(w io.Writer, n *int, err *error) {
	if blockID.IsZero() {
		wire.WriteTo([]byte("null"), w, n, err)
	} else {
		wire.WriteJSON(CanonicalBlockID(blockID), w, n, err)
	}

}

func (blockID BlockID) String() string {
	return fmt.Sprintf(`%X:%v`, blockID.Hash, blockID.PartsHeader)
}
