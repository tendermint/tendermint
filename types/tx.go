package types

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"

	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/merkle"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

// Tx is an arbitrary byte array.
// NOTE: Tx has no types at this level, so when wire encoded it's just length-prefixed.
// Might we want types here ?
type Tx []byte

// Key produces a fixed-length key for use in indexing.
func (tx Tx) Key() TxKey { return sha256.Sum256(tx) }

// Hash computes the TMHASH hash of the wire encoded transaction.
func (tx Tx) Hash() []byte { return crypto.Checksum(tx) }

// String returns the hex-encoded transaction as a string.
func (tx Tx) String() string { return fmt.Sprintf("Tx{%X}", []byte(tx)) }

// Txs is a slice of Tx.
type Txs []Tx

// Hash returns the Merkle root hash of the transaction hashes.
// i.e. the leaves of the tree are the hashes of the txs.
func (txs Txs) Hash() []byte {
	hl := txs.hashList()
	return merkle.HashFromByteSlices(hl)
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

func (txs Txs) Proof(i int) TxProof {
	hl := txs.hashList()
	root, proofs := merkle.ProofsFromByteSlices(hl)

	return TxProof{
		RootHash: root,
		Data:     txs[i],
		Proof:    *proofs[i],
	}
}

func (txs Txs) hashList() [][]byte {
	hl := make([][]byte, len(txs))
	for i := 0; i < len(txs); i++ {
		hl[i] = txs[i].Hash()
	}
	return hl
}

// Txs is a slice of transactions. Sorting a Txs value orders the transactions
// lexicographically.
func (txs Txs) Len() int      { return len(txs) }
func (txs Txs) Swap(i, j int) { txs[i], txs[j] = txs[j], txs[i] }
func (txs Txs) Less(i, j int) bool {
	return bytes.Compare(txs[i], txs[j]) == -1
}

// ToSliceOfBytes converts a Txs to slice of byte slices.
func (txs Txs) ToSliceOfBytes() [][]byte {
	txBzs := make([][]byte, len(txs))
	for i := 0; i < len(txs); i++ {
		txBzs[i] = txs[i]
	}
	return txBzs
}

// TxRecordSet contains indexes into an underlying set of transactions.
// These indexes are useful for validating and working with a list of TxRecords
// from the PrepareProposal response.
//
// Only one copy of the original data is referenced by all of the indexes but a
// transaction may appear in multiple indexes.
type TxRecordSet struct {
	// all holds the complete list of all transactions from the original list of
	// TxRecords.
	all Txs

	// included is an index of the transactions that will be included in the block
	// and is constructed from the list of both added and unmodified transactions.
	// included maintains the original order that the transactions were present
	// in the list of TxRecords.
	included Txs

	// added, unmodified, removed, and unknown are indexes for each of the actions
	// that may be supplied with a transaction.
	//
	// Because each transaction only has one action, it can be referenced by
	// at most 3 indexes in this data structure: the action-specific index, the
	// included index, and the all index.
	added      Txs
	unmodified Txs
	removed    Txs
	unknown    Txs
}

// NewTxRecordSet constructs a new set from the given transaction records.
// The contents of the input transactions are shared by the set, and must not
// be modified during the lifetime of the set.
func NewTxRecordSet(trs []*abci.TxRecord) TxRecordSet {
	txrSet := TxRecordSet{
		all: make([]Tx, len(trs)),
	}
	for i, tr := range trs {

		txrSet.all[i] = Tx(tr.Tx)

		// The following set of assignments do not allocate new []byte, they create
		// pointers to the already allocated slice.
		switch tr.GetAction() {
		case abci.TxRecord_UNKNOWN:
			txrSet.unknown = append(txrSet.unknown, txrSet.all[i])
		case abci.TxRecord_UNMODIFIED:
			txrSet.unmodified = append(txrSet.unmodified, txrSet.all[i])
			txrSet.included = append(txrSet.included, txrSet.all[i])
		case abci.TxRecord_ADDED:
			txrSet.added = append(txrSet.added, txrSet.all[i])
			txrSet.included = append(txrSet.included, txrSet.all[i])
		case abci.TxRecord_REMOVED:
			txrSet.removed = append(txrSet.removed, txrSet.all[i])
		}
	}
	return txrSet
}

// IncludedTxs returns the transactions marked for inclusion in a block. This
// list maintains the order that the transactions were included in the list of
// TxRecords that were used to construct the TxRecordSet.
func (t TxRecordSet) IncludedTxs() []Tx {
	return t.included
}

// RemovedTxs returns the transactions marked for removal by the application.
func (t TxRecordSet) RemovedTxs() []Tx {
	return t.removed
}

// Validate checks that the record set was correctly constructed from the original
// list of transactions.
func (t TxRecordSet) Validate(maxSizeBytes int64, otxs Txs) error {
	if len(t.unknown) > 0 {
		return fmt.Errorf("%d transactions marked unknown (first unknown hash: %x)", len(t.unknown), t.unknown[0].Hash())
	}

	// The following validation logic performs a set of sorts on the data in the TxRecordSet indexes.
	// It sorts the original transaction list, otxs, once.
	// It sorts the new transaction list twice: once when sorting 'all', the total list,
	// and once by sorting the set of the added, removed, and unmodified transactions indexes,
	// which, when combined, comprise the complete list of modified transactions.
	//
	// Each of the added, removed, and unmodified indices is then iterated and once
	// and each value index is checked against the sorted original list for containment.
	// Asymptotically, this yields a total runtime of O(N*log(N) + 2*M*log(M) + M*log(N)).
	// in the input size of the original list, N, and the input size of the new list, M, respectively.
	// Performance gains are likely possible, but this was preferred for readability and maintainability.

	// Sort a copy of the complete transaction slice so we can check for
	// duplication. The copy is so we do not change the original ordering.
	// Only the slices are copied, the transaction contents are shared.
	allCopy := sortedCopy(t.all)

	for i, cur := range allCopy {
		// allCopy is sorted, so any duplicated data will be adjacent.
		if i+1 < len(allCopy) && bytes.Equal(cur, allCopy[i+1]) {
			return fmt.Errorf("found duplicate transaction with hash: %x", cur.Hash())
		}
	}

	// create copies of each of the action-specific indexes so that order of the original
	// indexes can be preserved.
	addedCopy := sortedCopy(t.added)
	removedCopy := sortedCopy(t.removed)
	unmodifiedCopy := sortedCopy(t.unmodified)

	var size int64
	for _, cur := range append(unmodifiedCopy, addedCopy...) {
		size += int64(len(cur))
		if size > maxSizeBytes {
			return fmt.Errorf("transaction data size exceeds maximum %d", maxSizeBytes)
		}
	}

	// make a defensive copy of otxs so that the order of
	// the caller's data is not altered.
	otxsCopy := sortedCopy(otxs)

	if ix, ok := containsAll(otxsCopy, unmodifiedCopy); !ok {
		return fmt.Errorf("new transaction incorrectly marked as removed, transaction hash: %x", unmodifiedCopy[ix].Hash())
	}

	if ix, ok := containsAll(otxsCopy, removedCopy); !ok {
		return fmt.Errorf("new transaction incorrectly marked as removed, transaction hash: %x", removedCopy[ix].Hash())
	}
	if ix, ok := containsAny(otxsCopy, addedCopy); ok {
		return fmt.Errorf("existing transaction incorrectly marked as added, transaction hash: %x", addedCopy[ix].Hash())
	}
	return nil
}

func sortedCopy(txs Txs) Txs {
	cp := make(Txs, len(txs))
	copy(cp, txs)
	sort.Sort(cp)
	return cp
}

// containsAny checks that list a contains one of the transactions in list
// b. If a match is found, the index in b of the matching transaction is returned.
// Both lists must be sorted.
func containsAny(a, b []Tx) (int, bool) {
	for i, cur := range b {
		if _, ok := contains(a, cur); ok {
			return i, true
		}
	}
	return -1, false
}

// containsAll checks that super contains all of the transactions in the sub
// list. If not all values in sub are present in super, the index in sub of the
// first Tx absent from super is returned.
func containsAll(super, sub Txs) (int, bool) {
	for i, cur := range sub {
		if _, ok := contains(super, cur); !ok {
			return i, false
		}
	}
	return -1, true
}

// contains checks that the sorted list, set contains elem. If set does contain elem, then the
// index in set of elem is returned.
func contains(set []Tx, elem Tx) (int, bool) {
	n := sort.Search(len(set), func(i int) bool {
		return bytes.Compare(elem, set[i]) <= 0
	})
	if n == len(set) || !bytes.Equal(elem, set[n]) {
		return -1, false
	}
	return n, true
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
