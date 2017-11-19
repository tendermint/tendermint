package evpool

import (
	"fmt"

	wire "github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tmlibs/db"
)

/*
Requirements:
	- Valid new evidence must be persisted immediately and never forgotten
	- Uncommitted evidence must be continuously broadcast
	- Uncommitted evidence has a partial order, the evidence's priority

Impl:
	- First commit atomically in outqueue, pending, lookup.
	- Once broadcast, remove from outqueue. No need to sync
	- Once committed, atomically remove from pending and update lookup.
		- TODO: If we crash after committed but before removing/updating,
			we'll be stuck broadcasting evidence we never know we committed.
			so either share the state db and atomically MarkCommitted
			with ApplyBlock, or check all outqueue/pending on Start to see if its committed

Schema for indexing evidence (note you need both height and hash to find a piece of evidence):

"evidence-lookup"/<evidence-height>/<evidence-hash> -> evidenceInfo
"evidence-outqueue"/<priority>/<evidence-height>/<evidence-hash> -> evidenceInfo
"evidence-pending"/<evidence-height>/<evidence-hash> -> evidenceInfo
*/

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
	return _key("%s/%d/%X", baseKeyLookup, evidence.Height(), evidence.Hash())
}

func keyOutqueue(evidence types.Evidence, priority int) []byte {
	return _key("%s/%d/%d/%X", baseKeyOutqueue, priority, evidence.Height(), evidence.Hash())
}

func keyPending(evidence types.Evidence) []byte {
	return _key("%s/%d/%X", baseKeyPending, evidence.Height(), evidence.Hash())
}

func _key(fmt_ string, o ...interface{}) []byte {
	return []byte(fmt.Sprintf(fmt_, o...))
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
	// TODO: revert order for highest first
	return evidence
}

// PendingEvidence returns all known uncommitted evidence.
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
	key = keyOutqueue(evidence, priority)
	store.db.Set(key, eiBytes)

	key = keyPending(evidence)
	store.db.Set(key, eiBytes)

	key = keyLookup(evidence)
	store.db.SetSync(key, eiBytes)

	return true, nil
}

// MarkEvidenceAsBroadcasted removes evidence from Outqueue.
func (store *EvidenceStore) MarkEvidenceAsBroadcasted(evidence types.Evidence) {
	ei := store.getEvidenceInfo(evidence)
	key := keyOutqueue(evidence, ei.Priority)
	store.db.Delete(key)
}

// MarkEvidenceAsPending removes evidence from pending and outqueue and sets the state to committed.
func (store *EvidenceStore) MarkEvidenceAsCommitted(evidence types.Evidence) {
	// if its committed, its been broadcast
	store.MarkEvidenceAsBroadcasted(evidence)

	key := keyPending(evidence)
	store.db.Delete(key)

	ei := store.getEvidenceInfo(evidence)
	ei.Committed = true

	// TODO: we should use the state db and db.Sync in state.Save instead.
	// Else, if we call this before state.Save, we may never mark committed evidence as committed.
	// Else, if we call this after state.Save, we may get stuck broadcasting evidence we never know we committed.
	store.db.SetSync(key, wire.BinaryBytes(ei))
}

func (store *EvidenceStore) getEvidenceInfo(evidence types.Evidence) evidenceInfo {
	key := keyLookup(evidence)
	var ei evidenceInfo
	b := store.db.Get(key)
	wire.ReadBinaryBytes(b, &ei)
	return ei
}
