package types

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

type TxRecordSet struct {
	txs           []Tx
	unknownIdx    []*Tx
	unmodifiedIdx []*Tx
	addedIdx      []*Tx
	removedIdx    []*Tx
}

// Tx is an arbitrary byte array.
// NOTE: Tx has no types at this level, so when wire encoded it's just length-prefixed.
// Might we want types here ?
type Tx []byte

// Key produces a fixed-length key for use in indexing.
func (tx Tx) Key() TxKey { return sha256.Sum256(tx) }

// Hash computes the TMHASH hash of the wire encoded transaction.
func (tx Tx) Hash() []byte { return tmhash.Sum(tx) }

// String returns the hex-encoded transaction as a string.
func (tx Tx) String() string { return fmt.Sprintf("Tx{%X}", []byte(tx)) }

// Txs is a slice of Tx.
type Txs []Tx

// Hash returns the Merkle root hash of the transaction hashes.
// i.e. the leaves of the tree are the hashes of the txs.
func (txs Txs) Hash() []byte {
	// These allocations will be removed once Txs is switched to [][]byte,
	// ref #2603. This is because golang does not allow type casting slices without unsafe
	txBzs := make([][]byte, len(txs))
	for i := 0; i < len(txs); i++ {
		txBzs[i] = txs[i].Hash()
	}
	return merkle.HashFromByteSlices(txBzs)
}

// Index returns the index of this transaction in the list, or -1 if not found
func (txs Txs) Index(tx Tx) int {
	for i := range txs {
		if bytes.Equal(txs[i], tx) {
			return i
		}
	}
	return -1
}

// IndexByHash returns the index of this transaction hash in the list, or -1 if not found
func (txs Txs) IndexByHash(hash []byte) int {
	for i := range txs {
		if bytes.Equal(txs[i].Hash(), hash) {
			return i
		}
	}
	return -1
}

// ToSliceOfBytes converts a Txs to slice of byte slices.
//
// NOTE: This method should become obsolete once Txs is switched to [][]byte.
// ref: #2603
// TODO This function is to disappear when TxRecord is introduced
func (txs Txs) ToSliceOfBytes() [][]byte {
	txBzs := make([][]byte, len(txs))
	for i := 0; i < len(txs); i++ {
		txBzs[i] = txs[i]
	}
	return txBzs
}

func (txs Txs) ToSet() map[string]struct{} {
	m := make(map[string]struct{}, len(txs))
	for _, tx := range txs {
		m[string(tx.Hash())] = struct{}{}
	}
	return m
}

// ToTxs converts a raw slice of byte slices into a Txs type.
// TODO This function is to disappear when TxRecord is introduced
func ToTxs(txs [][]byte) Txs {
	txBzs := make(Txs, len(txs))
	for i := 0; i < len(txs); i++ {
		txBzs[i] = txs[i]
	}
	return txBzs
}

// TxRecordsToTxs converts from the abci Tx type to the the Txs type.
func TxRecordsToTxs(trs []*abci.TxRecord) Txs {
	txs := make([]Tx, len(trs))
	for i, tr := range trs {
		txs[i] = Tx(tr.Tx)
	}
	return txs
}

func NewTxRecordSet(trs []*abci.TxRecord) TxRecordSet {
	txrSet := TxRecordSet{}
	txrSet.txs = make([]Tx, len(trs))
	for i, tr := range trs {
		txrSet.txs[i] = Tx(tr.Tx)
		if tr.Action == abci.TxRecord_UNKNOWN {
			txrSet.unknownIdx = append(txrSet.unknownIdx, &txrSet.txs[i])
		}
		if tr.Action == abci.TxRecord_UNMODIFIED {
			txrSet.unmodifiedIdx = append(txrSet.unmodifiedIdx, &txrSet.txs[i])
		}
		if tr.Action == abci.TxRecord_ADDED {
			txrSet.addedIdx = append(txrSet.addedIdx, &txrSet.txs[i])
		}
		if tr.Action == abci.TxRecord_REMOVED {
			txrSet.removedIdx = append(txrSet.removedIdx, &txrSet.txs[i])
		}
	}
	return txrSet
}

func (t TxRecordSet) GetUnmodifiedTxs() []*Tx {
	return t.unmodifiedIdx
}

func (t TxRecordSet) GetAddedTxs() []*Tx {
	return t.addedIdx
}

func (t TxRecordSet) GetRemovedTxs() []*Tx {
	return t.removedIdx
}

func (t TxRecordSet) GetUnknownTxs() []*Tx {
	return t.unknownIdx
}

func (t TxRecordSet) Validate(maxSizeBytes int64, otxs Txs) error {
	otxSet := otxs.ToSet()
	ntxSet := map[string]struct{}{}
	var size int64
	for _, tx := range t.GetAddedTxs() {
		size += int64(len(*tx))
		if size > maxSizeBytes {
			return fmt.Errorf("transaction data size %d exceeds maximum %d", size, maxSizeBytes)
		}
		hash := tx.Hash()
		if _, ok := otxSet[string(hash)]; ok {
			return fmt.Errorf("unmodified transaction incorrectly marked as %s, transaction hash: %x", abci.TxRecord_ADDED, hash)
		}
		if _, ok := ntxSet[string(hash)]; ok {
			return fmt.Errorf("TxRecords contains duplicate transaction, transaction hash: %x", hash)
		}
		ntxSet[string(hash)] = struct{}{}
	}
	for _, tx := range t.GetUnmodifiedTxs() {
		size += int64(len(*tx))
		if size > maxSizeBytes {
			return fmt.Errorf("transaction data size %d exceeds maximum %d", size, maxSizeBytes)
		}
		hash := tx.Hash()
		if _, ok := otxSet[string(hash)]; !ok {
			return fmt.Errorf("new transaction incorrectly marked as %s, transaction hash: %x", abci.TxRecord_UNMODIFIED, hash)
		}
	}
	for _, tx := range t.GetRemovedTxs() {
		hash := tx.Hash()
		if _, ok := otxSet[string(hash)]; !ok {
			return fmt.Errorf("new transaction incorrectly marked as %s, transaction hash: %x", abci.TxRecord_REMOVED, hash)
		}
	}
	if len(t.GetUnknownTxs()) > 0 {
		utx := t.GetUnknownTxs()[0]
		return fmt.Errorf("transaction incorrectly marked as %s, transaction hash: %x", utx, utx.Hash())
	}
	return nil
}

func (t TxRecordSet) GetTxs() []Tx {
	return t.txs
}

// TxsToTxRecords converts from a list of Txs to a list of TxRecords. All of the
// resulting TxRecords are returned with the status TxRecord_UNMODIFIED.
func TxsToTxRecords(txs []Tx) []*abci.TxRecord {
	trs := make([]*abci.TxRecord, len(txs))
	for i, tx := range txs {
		trs[i] = &abci.TxRecord{
			Action: abci.TxRecord_UNMODIFIED,
			Tx:     tx,
		}
	}
	return trs
}

// TxProof represents a Merkle proof of the presence of a transaction in the Merkle tree.
type TxProof struct {
	RootHash tmbytes.HexBytes `json:"root_hash"`
	Data     Tx               `json:"data"`
	Proof    merkle.Proof     `json:"proof"`
}

// Leaf returns the hash(tx), which is the leaf in the merkle tree which this proof refers to.
func (tp TxProof) Leaf() []byte {
	return tp.Data.Hash()
}

// Validate verifies the proof. It returns nil if the RootHash matches the dataHash argument,
// and if the proof is internally consistent. Otherwise, it returns a sensible error.
func (tp TxProof) Validate(dataHash []byte) error {
	if !bytes.Equal(dataHash, tp.RootHash) {
		return errors.New("proof matches different data hash")
	}
	if tp.Proof.Index < 0 {
		return errors.New("proof index cannot be negative")
	}
	if tp.Proof.Total <= 0 {
		return errors.New("proof total must be positive")
	}
	valid := tp.Proof.Verify(tp.RootHash, tp.Leaf())
	if valid != nil {
		return errors.New("proof is not internally consistent")
	}
	return nil
}

func (tp TxProof) ToProto() tmproto.TxProof {

	pbProof := tp.Proof.ToProto()

	pbtp := tmproto.TxProof{
		RootHash: tp.RootHash,
		Data:     tp.Data,
		Proof:    pbProof,
	}

	return pbtp
}
func TxProofFromProto(pb tmproto.TxProof) (TxProof, error) {

	pbProof, err := merkle.ProofFromProto(pb.Proof)
	if err != nil {
		return TxProof{}, err
	}

	pbtp := TxProof{
		RootHash: pb.RootHash,
		Data:     pb.Data,
		Proof:    *pbProof,
	}

	return pbtp, nil
}

// ComputeProtoSizeForTxs wraps the transactions in tmproto.Data{} and calculates the size.
// https://developers.google.com/protocol-buffers/docs/encoding
func ComputeProtoSizeForTxs(txs []Tx) int64 {
	data := Data{Txs: txs}
	pdData := data.ToProto()
	return int64(pdData.Size())
}
