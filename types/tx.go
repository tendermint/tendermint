package types

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

type TxRecordSet struct {
	txs        Txs
	added      Txs
	unmodified Txs
	removed    Txs
	unknown    Txs
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

func (txs Txs) Len() int      { return len(txs) }
func (txs Txs) Swap(i, j int) { txs[i], txs[j] = txs[j], txs[i] }
func (txs Txs) Less(i, j int) bool {
	return bytes.Compare(txs[i], txs[j]) == -1
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
		switch tr.GetAction() {
		case abci.TxRecord_UNKNOWN:
			txrSet.unknown = append(txrSet.unknown, txrSet.txs[i])
		case abci.TxRecord_UNMODIFIED:
			txrSet.unmodified = append(txrSet.unmodified, txrSet.txs[i])
		case abci.TxRecord_ADDED:
			txrSet.added = append(txrSet.added, txrSet.txs[i])
		case abci.TxRecord_REMOVED:
			txrSet.removed = append(txrSet.removed, txrSet.txs[i])
		}
	}
	return txrSet
}
func (t TxRecordSet) GetAddedTxs() []Tx {
	return nil
}

func (t TxRecordSet) GetRemovedTxs() []Tx {
	return nil
}

func (t TxRecordSet) GetUnknownTxs() []Tx {
	return nil
}
func (t TxRecordSet) GetUnmodifiedTxs() []Tx {
	return nil
}

func (t TxRecordSet) Validate(maxSizeBytes int64, otxs Txs) error {
	if len(t.unknown) > 0 {
		return fmt.Errorf("transaction incorrectly marked as unknown, transaction hash: %x", t.unknown[0].Hash())
	}
	var size int64
	cp := make([]Tx, len(t.txs))
	for i := range t.txs {
		size += int64(len(t.txs[i]))
		if size > maxSizeBytes {
			return fmt.Errorf("transaction data size %d exceeds maximum %d", size, maxSizeBytes)
		}
		cp[i] = t.txs[i]
	}
	sort.Sort(Txs(cp))

	// duplicate validation
	for i := 0; i < len(cp)-1; i++ {
		if bytes.Equal(cp[i], cp[i+1]) {
			return fmt.Errorf("TxRecords contains duplicate transaction, transaction hash: %x", cp[i].Hash())
		}
	}
	sort.Sort(otxs)
	sort.Sort(Txs(t.added))
	sort.Sort(Txs(t.removed))
	sort.Sort(Txs(t.unmodified))

	for i, j := 0, 0; i < len(t.added) && j < len(otxs); {
		switch bytes.Compare(t.added[i], otxs[j]) {
		case 0:
			return fmt.Errorf("existing transaction incorrectly marked as added, transaction hash: %x", otxs[j].Hash())
		case -1:
			i++
		case 1:
			j++
		}
	}

	for i, j := 0, 0; i < len(t.removed); {
		if j >= len(otxs) {
			// we reached the end of the original txs without finding a match for
			// all of the removed elements
			return fmt.Errorf("new transaction incorrectly marked as removed, transaction hash: %x", t.removed[i].Hash())
		}
		switch bytes.Compare(t.added[i], otxs[j]) {
		case 0:
			i++
			j++
		case -1:
			return fmt.Errorf("new transaction incorrectly marked as removed, transaction hash: %x", t.removed[i].Hash())
		case 1:
			j++
		}
	}
	for i, j := 0, 0; i < len(t.unmodified); {
		if j >= len(otxs) {
			// we reached the end of the original txs without finding a match for
			// all of the unmodified elements
			return fmt.Errorf("new transaction incorrectly marked as unmodified, transaction hash: %x", t.unmodified[i].Hash())
		}
		switch bytes.Compare(t.unmodified[i], otxs[j]) {
		case 0:
			i++
			j++
		case -1:
			return fmt.Errorf("new transaction incorrectly marked as unmodified, transaction hash: %x", t.unmodified[i].Hash())
		case 1:
			j++
		}
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
