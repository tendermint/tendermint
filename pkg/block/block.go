package block

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/gogo/protobuf/proto"

	tmsync "github.com/tendermint/tendermint/internal/libs/sync"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmmath "github.com/tendermint/tendermint/libs/math"
	"github.com/tendermint/tendermint/pkg/evidence"
	"github.com/tendermint/tendermint/pkg/mempool"
	"github.com/tendermint/tendermint/pkg/metadata"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/version"
)

// Block defines the atomic unit of a Tendermint blockchain.
type Block struct {
	mtx tmsync.Mutex

	metadata.Header `json:"header"`
	Data            `json:"data"`
	Evidence        EvidenceData     `json:"evidence"`
	LastCommit      *metadata.Commit `json:"last_commit"`
}

// ValidateBasic performs basic validation that doesn't involve state data.
// It checks the internal consistency of the block.
// Further validation is done using state#ValidateBlock.
func (b *Block) ValidateBasic() error {
	if b == nil {
		return errors.New("nil block")
	}

	b.mtx.Lock()
	defer b.mtx.Unlock()

	if err := b.Header.ValidateBasic(); err != nil {
		return fmt.Errorf("invalid header: %w", err)
	}

	// Validate the last commit and its hash.
	if b.LastCommit == nil {
		return errors.New("nil LastCommit")
	}
	if err := b.LastCommit.ValidateBasic(); err != nil {
		return fmt.Errorf("wrong LastCommit: %v", err)
	}

	if w, g := b.LastCommit.Hash(), b.LastCommitHash; !bytes.Equal(w, g) {
		return fmt.Errorf("wrong Header.LastCommitHash. Expected %X, got %X", w, g)
	}

	// NOTE: b.Data.Txs may be nil, but b.Data.Hash() still works fine.
	if w, g := b.Data.Hash(), b.DataHash; !bytes.Equal(w, g) {
		return fmt.Errorf("wrong Header.DataHash. Expected %X, got %X", w, g)
	}

	// NOTE: b.Evidence.Evidence may be nil, but we're just looping.
	for i, ev := range b.Evidence.Evidence {
		if err := ev.ValidateBasic(); err != nil {
			return fmt.Errorf("invalid evidence (#%d): %v", i, err)
		}
	}

	if w, g := b.Evidence.Hash(), b.EvidenceHash; !bytes.Equal(w, g) {
		return fmt.Errorf("wrong Header.EvidenceHash. Expected %X, got %X", w, g)
	}

	return nil
}

// fillHeader fills in any remaining header fields that are a function of the block data
func (b *Block) fillHeader() {
	if b.LastCommitHash == nil {
		b.LastCommitHash = b.LastCommit.Hash()
	}
	if b.DataHash == nil {
		b.DataHash = b.Data.Hash()
	}
	if b.EvidenceHash == nil {
		b.EvidenceHash = b.Evidence.Hash()
	}
}

// Hash computes and returns the block hash.
// If the block is incomplete, block hash is nil for safety.
func (b *Block) Hash() tmbytes.HexBytes {
	if b == nil {
		return nil
	}
	b.mtx.Lock()
	defer b.mtx.Unlock()

	if b.LastCommit == nil {
		return nil
	}
	b.fillHeader()
	return b.Header.Hash()
}

// MakePartSet returns a PartSet containing parts of a serialized block.
// This is the form in which the block is gossipped to peers.
// CONTRACT: partSize is greater than zero.
func (b *Block) MakePartSet(partSize uint32) *metadata.PartSet {
	if b == nil {
		return nil
	}
	b.mtx.Lock()
	defer b.mtx.Unlock()

	pbb, err := b.ToProto()
	if err != nil {
		panic(err)
	}
	bz, err := proto.Marshal(pbb)
	if err != nil {
		panic(err)
	}
	return metadata.NewPartSetFromData(bz, partSize)
}

// HashesTo is a convenience function that checks if a block hashes to the given argument.
// Returns false if the block is nil or the hash is empty.
func (b *Block) HashesTo(hash []byte) bool {
	if len(hash) == 0 {
		return false
	}
	if b == nil {
		return false
	}
	return bytes.Equal(b.Hash(), hash)
}

// Size returns size of the block in bytes.
func (b *Block) Size() int {
	pbb, err := b.ToProto()
	if err != nil {
		return 0
	}

	return pbb.Size()
}

// String returns a string representation of the block
//
// See StringIndented.
func (b *Block) String() string {
	return b.StringIndented("")
}

// StringIndented returns an indented String.
//
// Header
// Data
// Evidence
// LastCommit
// Hash
func (b *Block) StringIndented(indent string) string {
	if b == nil {
		return "nil-Block"
	}
	return fmt.Sprintf(`Block{
%s  %v
%s  %v
%s  %v
%s  %v
%s}#%v`,
		indent, b.Header.StringIndented(indent+"  "),
		indent, b.Data.StringIndented(indent+"  "),
		indent, b.Evidence.StringIndented(indent+"  "),
		indent, b.LastCommit.StringIndented(indent+"  "),
		indent, b.Hash())
}

// StringShort returns a shortened string representation of the block.
func (b *Block) StringShort() string {
	if b == nil {
		return "nil-Block"
	}
	return fmt.Sprintf("Block#%X", b.Hash())
}

// ToProto converts Block to protobuf
func (b *Block) ToProto() (*tmproto.Block, error) {
	if b == nil {
		return nil, errors.New("nil Block")
	}

	pb := new(tmproto.Block)

	pb.Header = *b.Header.ToProto()
	pb.LastCommit = b.LastCommit.ToProto()
	pb.Data = b.Data.ToProto()

	protoEvidence, err := b.Evidence.ToProto()
	if err != nil {
		return nil, err
	}
	pb.Evidence = *protoEvidence

	return pb, nil
}

// FromProto sets a protobuf Block to the given pointer.
// It returns an error if the block is invalid.
func BlockFromProto(bp *tmproto.Block) (*Block, error) {
	if bp == nil {
		return nil, errors.New("nil block")
	}

	b := new(Block)
	h, err := metadata.HeaderFromProto(&bp.Header)
	if err != nil {
		return nil, err
	}
	b.Header = h
	data, err := DataFromProto(&bp.Data)
	if err != nil {
		return nil, err
	}
	b.Data = data
	if err := b.Evidence.FromProto(&bp.Evidence); err != nil {
		return nil, err
	}

	if bp.LastCommit != nil {
		lc, err := metadata.CommitFromProto(bp.LastCommit)
		if err != nil {
			return nil, err
		}
		b.LastCommit = lc
	}

	return b, b.ValidateBasic()
}

//-----------------------------------------------------------------------------

// MaxDataBytes returns the maximum size of block's data.
//
// XXX: Panics on negative result.
func MaxDataBytes(maxBytes, evidenceBytes int64, valsCount int) int64 {
	maxDataBytes := maxBytes -
		metadata.MaxOverheadForBlock -
		metadata.MaxHeaderBytes -
		metadata.MaxCommitBytes(valsCount) -
		evidenceBytes

	if maxDataBytes < 0 {
		panic(fmt.Sprintf(
			"Negative MaxDataBytes. Block.MaxBytes=%d is too small to accommodate header&lastCommit&evidence=%d",
			maxBytes,
			-(maxDataBytes - maxBytes),
		))
	}

	return maxDataBytes
}

// MaxDataBytesNoEvidence returns the maximum size of block's data when
// evidence count is unknown. MaxEvidencePerBlock will be used for the size
// of evidence.
//
// XXX: Panics on negative result.
func MaxDataBytesNoEvidence(maxBytes int64, valsCount int) int64 {
	maxDataBytes := maxBytes -
		metadata.MaxOverheadForBlock -
		metadata.MaxHeaderBytes -
		metadata.MaxCommitBytes(valsCount)

	if maxDataBytes < 0 {
		panic(fmt.Sprintf(
			"Negative MaxDataBytesUnknownEvidence. Block.MaxBytes=%d is too small to accommodate header&lastCommit&evidence=%d",
			maxBytes,
			-(maxDataBytes - maxBytes),
		))
	}

	return maxDataBytes
}

// MakeBlock returns a new block with an empty header, except what can be
// computed from itself.
// It populates the same set of fields validated by ValidateBasic.
func MakeBlock(height int64, txs []mempool.Tx, lastCommit *metadata.Commit, evidence []evidence.Evidence) *Block {
	block := &Block{
		Header: metadata.Header{
			Version: version.Consensus{Block: version.BlockProtocol, App: 0},
			Height:  height,
		},
		Data: Data{
			Txs: txs,
		},
		Evidence:   EvidenceData{Evidence: evidence},
		LastCommit: lastCommit,
	}
	block.fillHeader()
	return block
}

//-----------------------------------------------------------------------------

// Data contains the set of transactions included in the block
type Data struct {

	// Txs that will be applied by state @ block.Height+1.
	// NOTE: not all txs here are valid.  We're just agreeing on the order first.
	// This means that block.AppHash does not include these txs.
	Txs mempool.Txs `json:"txs"`

	// Volatile
	hash tmbytes.HexBytes
}

// Hash returns the hash of the data
func (data *Data) Hash() tmbytes.HexBytes {
	if data == nil {
		return (mempool.Txs{}).Hash()
	}
	if data.hash == nil {
		data.hash = data.Txs.Hash() // NOTE: leaves of merkle tree are TxIDs
	}
	return data.hash
}

// StringIndented returns an indented string representation of the transactions.
func (data *Data) StringIndented(indent string) string {
	if data == nil {
		return "nil-Data"
	}
	txStrings := make([]string, tmmath.MinInt(len(data.Txs), 21))
	for i, tx := range data.Txs {
		if i == 20 {
			txStrings[i] = fmt.Sprintf("... (%v total)", len(data.Txs))
			break
		}
		txStrings[i] = fmt.Sprintf("%X (%d bytes)", tx.Hash(), len(tx))
	}
	return fmt.Sprintf(`Data{
%s  %v
%s}#%v`,
		indent, strings.Join(txStrings, "\n"+indent+"  "),
		indent, data.hash)
}

// ToProto converts Data to protobuf
func (data *Data) ToProto() tmproto.Data {
	tp := new(tmproto.Data)

	if len(data.Txs) > 0 {
		txBzs := make([][]byte, len(data.Txs))
		for i := range data.Txs {
			txBzs[i] = data.Txs[i]
		}
		tp.Txs = txBzs
	}

	return *tp
}

// ClearCache removes the saved hash. This is predominantly used for testing.
func (data *Data) ClearCache() {
	data.hash = nil
}

// DataFromProto takes a protobuf representation of Data &
// returns the native type.
func DataFromProto(dp *tmproto.Data) (Data, error) {
	if dp == nil {
		return Data{}, errors.New("nil data")
	}
	data := new(Data)

	if len(dp.Txs) > 0 {
		txBzs := make(mempool.Txs, len(dp.Txs))
		for i := range dp.Txs {
			txBzs[i] = mempool.Tx(dp.Txs[i])
		}
		data.Txs = txBzs
	} else {
		data.Txs = mempool.Txs{}
	}

	return *data, nil
}

// ComputeProtoSizeForTxs wraps the transactions in tmproto.Data{} and calculates the size.
// https://developers.google.com/protocol-buffers/docs/encoding
func ComputeProtoSizeForTxs(txs []mempool.Tx) int64 {
	data := Data{Txs: txs}
	pdData := data.ToProto()
	return int64(pdData.Size())
}

//-----------------------------------------------------------------------------

// EvidenceData contains any evidence of malicious wrong-doing by validators
type EvidenceData struct {
	Evidence evidence.EvidenceList `json:"evidence"`

	// Volatile. Used as cache
	hash     tmbytes.HexBytes
	byteSize int64
}

// Hash returns the hash of the data.
func (data *EvidenceData) Hash() tmbytes.HexBytes {
	if data.hash == nil {
		data.hash = data.Evidence.Hash()
	}
	return data.hash
}

// ByteSize returns the total byte size of all the evidence
func (data *EvidenceData) ByteSize() int64 {
	if data.byteSize == 0 && len(data.Evidence) != 0 {
		pb, err := data.ToProto()
		if err != nil {
			panic(err)
		}
		data.byteSize = int64(pb.Size())
	}
	return data.byteSize
}

// StringIndented returns a string representation of the evidence.
func (data *EvidenceData) StringIndented(indent string) string {
	if data == nil {
		return "nil-Evidence"
	}
	evStrings := make([]string, tmmath.MinInt(len(data.Evidence), 21))
	for i, ev := range data.Evidence {
		if i == 20 {
			evStrings[i] = fmt.Sprintf("... (%v total)", len(data.Evidence))
			break
		}
		evStrings[i] = fmt.Sprintf("Evidence:%v", ev)
	}
	return fmt.Sprintf(`EvidenceData{
%s  %v
%s}#%v`,
		indent, strings.Join(evStrings, "\n"+indent+"  "),
		indent, data.hash)
}

// ToProto converts EvidenceData to protobuf
func (data *EvidenceData) ToProto() (*tmproto.EvidenceList, error) {
	if data == nil {
		return nil, errors.New("nil evidence data")
	}

	evi := new(tmproto.EvidenceList)
	eviBzs := make([]tmproto.Evidence, len(data.Evidence))
	for i := range data.Evidence {
		protoEvi, err := evidence.EvidenceToProto(data.Evidence[i])
		if err != nil {
			return nil, err
		}
		eviBzs[i] = *protoEvi
	}
	evi.Evidence = eviBzs

	return evi, nil
}

// FromProto sets a protobuf EvidenceData to the given pointer.
func (data *EvidenceData) FromProto(eviData *tmproto.EvidenceList) error {
	if eviData == nil {
		return errors.New("nil evidenceData")
	}

	eviBzs := make(evidence.EvidenceList, len(eviData.Evidence))
	for i := range eviData.Evidence {
		evi, err := evidence.EvidenceFromProto(&eviData.Evidence[i])
		if err != nil {
			return err
		}
		eviBzs[i] = evi
	}
	data.Evidence = eviBzs
	data.byteSize = int64(eviData.Size())

	return nil
}

//--------------------------------------------------------------------------------
