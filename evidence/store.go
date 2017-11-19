package evpool

import (
	"fmt"

	wire "github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tmlibs/db"
)

/*
Schema for indexing evidence:

"evidence-lookup"/<evidence-height>/<evidence-hash> -> evidence struct
"evidence-outqueue"/<index>/<evidence-height>/<evidence-hash> -> nil
"evidence-pending"/<evidence-height>/evidence-hash> -> nil

*/

var nullValue = []byte{0}

type evidenceInfo struct {
	Committed bool
	Priority  int
	Evidence  types.Evidence
}

const (
	baseKeyLookup   = "evidence-lookup"   // all evidence
	baseKeyOutqueue = "evidence-outqueue" // not-yet broadcast
	baseKeyPending  = "evidence-pending"  // broadcast but not committed
)

func keyLookup(evidence types.Evidence) []byte {
	return _key(baseKeyLookup, evidence)
}

func keyOutqueue(evidence types.Evidence) []byte {
	return _key(baseKeyOutqueue, evidence)
}

func keyPending(evidence types.Evidence) []byte {
	return _key(baseKeyPending, evidence)
}

func _key(key string, evidence types.Evidence) []byte {
	return []byte(fmt.Sprintf("%s/%d/%X", key, evidence.Height(), evidence.Hash()))
}

// EvidenceStore stores all the evidence we've seen, including
// evidence that has been committed, evidence that has been seen but not broadcast,
// and evidence that has been broadcast but not yet committed.
type EvidenceStore struct {
	db dbm.DB
}

func NewEvidenceStore(db dbm.DB) *EvidenceStore {
	return &EvidenceStore{
		db: db,
	}
}

// PriorityEvidence returns the evidence from the outqueue, sorted by highest priority.
func (store *EvidenceStore) PriorityEvidence() (evidence []types.Evidence) {
	iter := store.db.IteratorPrefix([]byte(baseKeyOutqueue))
	for iter.Next() {
		val := iter.Value()

		var ei evidenceInfo
		wire.ReadBinaryBytes(val, &ei)
		evidence = append(evidence, ei.Evidence)
	}
	// TODO: sort
	return evidence
}

func (store *EvidenceStore) PendingEvidence() (evidence []types.Evidence) {
	iter := store.db.IteratorPrefix([]byte(baseKeyPending))
	for iter.Next() {
		val := iter.Value()

		var ei evidenceInfo
		wire.ReadBinaryBytes(val, &ei)
		evidence = append(evidence, ei.Evidence)
	}
	return evidence
}

// AddNewEvidence adds the given evidence to the database.
func (store *EvidenceStore) AddNewEvidence(evidence types.Evidence, priority int) (bool, error) {
	// check if we already have seen it
	key := keyLookup(evidence)
	v := store.db.Get(key)
	if len(v) != 0 {
		return false, nil
	}

	ei := evidenceInfo{
		Committed: false,
		Priority:  priority,
		Evidence:  evidence,
	}
	eiBytes := wire.BinaryBytes(ei)

	// add it to the store
	key = keyLookup(evidence)
	store.db.Set(key, eiBytes)

	key = keyOutqueue(evidence)
	store.db.Set(key, eiBytes)

	key = keyPending(evidence)
	store.db.Set(key, eiBytes)

	return true, nil
}

// MarkEvidenceAsBroadcasted removes evidence from the outqueue.
func (store *EvidenceStore) MarkEvidenceAsBroadcasted(evidence types.Evidence) {
	key := keyOutqueue(evidence)
	store.db.Delete(key)
}

// MarkEvidenceAsPending removes evidence from pending and outqueue and sets the state to committed.
func (store *EvidenceStore) MarkEvidenceAsCommitted(evidence types.Evidence) {
	// if its committed, its been broadcast
	store.MarkEvidenceAsBroadcasted(evidence)

	key := keyPending(evidence)
	store.db.Delete(key)

	key = keyLookup(evidence)
	var ei evidenceInfo
	b := store.db.Get(key)
	wire.ReadBinaryBytes(b, &ei)
	ei.Committed = true
	store.db.Set(key, wire.BinaryBytes(ei))
}
