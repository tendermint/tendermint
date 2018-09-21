package evidence

import (
	"fmt"

	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/types"
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

Schema for indexing evidence (note you need both height and hash to find a piece of evidence):

"evidence-lookup"/<evidence-height>/<evidence-hash> -> EvidenceInfo
"evidence-outqueue"/<priority>/<evidence-height>/<evidence-hash> -> EvidenceInfo
"evidence-pending"/<evidence-height>/<evidence-hash> -> EvidenceInfo
*/

type EvidenceInfo struct {
	Committed bool
	Priority  int64
	Evidence  types.Evidence
}

const (
	baseKeyLookup   = "evidence-lookup"   // all evidence
	baseKeyOutqueue = "evidence-outqueue" // not-yet broadcast
	baseKeyPending  = "evidence-pending"  // broadcast but not committed
)

func keyLookup(evidence types.Evidence) []byte {
	return keyLookupFromHeightAndHash(evidence.Height(), evidence.Hash())
}

// big endian padded hex
func bE(h int64) string {
	return fmt.Sprintf("%0.16X", h)
}

func keyLookupFromHeightAndHash(height int64, hash []byte) []byte {
	return _key("%s/%s/%X", baseKeyLookup, bE(height), hash)
}

func keyOutqueue(evidence types.Evidence, priority int64) []byte {
	return _key("%s/%s/%s/%X", baseKeyOutqueue, bE(priority), bE(evidence.Height()), evidence.Hash())
}

func keyPending(evidence types.Evidence) []byte {
	return _key("%s/%s/%X", baseKeyPending, bE(evidence.Height()), evidence.Hash())
}

func _key(fmt_ string, o ...interface{}) []byte {
	return []byte(fmt.Sprintf(fmt_, o...))
}

// EvidenceStore is a store of all the evidence we've seen, including
// evidence that has been committed, evidence that has been verified but not broadcast,
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
	// reverse the order so highest priority is first
	l := store.listEvidence(baseKeyOutqueue, -1)
	l2 := make([]types.Evidence, len(l))
	for i := range l {
		l2[i] = l[len(l)-1-i]
	}
	return l2
}

// PendingEvidence returns known uncommitted evidence up to maxBytes.
// If maxBytes is -1, all evidence is returned.
func (store *EvidenceStore) PendingEvidence(maxBytes int64) (evidence []types.Evidence) {
	return store.listEvidence(baseKeyPending, maxBytes)
}

// listEvidence lists the evidence for the given prefix key up to maxBytes.
// It is wrapped by PriorityEvidence and PendingEvidence for convenience.
// If maxBytes is -1, there's no cap on the size of returned evidence.
func (store *EvidenceStore) listEvidence(prefixKey string, maxBytes int64) (evidence []types.Evidence) {
	var bytes int64
	iter := dbm.IteratePrefix(store.db, []byte(prefixKey))
	for ; iter.Valid(); iter.Next() {
		val := iter.Value()

		if maxBytes > 0 && bytes+int64(len(val)) > maxBytes {
			return evidence
		}
		bytes += int64(len(val))

		var ei EvidenceInfo
		err := cdc.UnmarshalBinaryBare(val, &ei)
		if err != nil {
			panic(err)
		}
		evidence = append(evidence, ei.Evidence)
	}
	return evidence
}

// GetEvidence fetches the evidence with the given height and hash.
func (store *EvidenceStore) GetEvidence(height int64, hash []byte) *EvidenceInfo {
	key := keyLookupFromHeightAndHash(height, hash)
	val := store.db.Get(key)

	if len(val) == 0 {
		return nil
	}
	var ei EvidenceInfo
	err := cdc.UnmarshalBinaryBare(val, &ei)
	if err != nil {
		panic(err)
	}
	return &ei
}

// AddNewEvidence adds the given evidence to the database.
// It returns false if the evidence is already stored.
func (store *EvidenceStore) AddNewEvidence(evidence types.Evidence, priority int64) bool {
	// check if we already have seen it
	ei_ := store.GetEvidence(evidence.Height(), evidence.Hash())
	if ei_ != nil && ei_.Evidence != nil {
		return false
	}

	ei := EvidenceInfo{
		Committed: false,
		Priority:  priority,
		Evidence:  evidence,
	}
	eiBytes := cdc.MustMarshalBinaryBare(ei)

	// add it to the store
	key := keyOutqueue(evidence, priority)
	store.db.Set(key, eiBytes)

	key = keyPending(evidence)
	store.db.Set(key, eiBytes)

	key = keyLookup(evidence)
	store.db.SetSync(key, eiBytes)

	return true
}

// MarkEvidenceAsBroadcasted removes evidence from Outqueue.
func (store *EvidenceStore) MarkEvidenceAsBroadcasted(evidence types.Evidence) {
	ei := store.getEvidenceInfo(evidence)
	key := keyOutqueue(evidence, ei.Priority)
	store.db.Delete(key)
}

// MarkEvidenceAsCommitted removes evidence from pending and outqueue and sets the state to committed.
func (store *EvidenceStore) MarkEvidenceAsCommitted(evidence types.Evidence) {
	// if its committed, its been broadcast
	store.MarkEvidenceAsBroadcasted(evidence)

	pendingKey := keyPending(evidence)
	store.db.Delete(pendingKey)

	ei := store.getEvidenceInfo(evidence)
	ei.Committed = true

	lookupKey := keyLookup(evidence)
	store.db.SetSync(lookupKey, cdc.MustMarshalBinaryBare(ei))
}

//---------------------------------------------------
// utils

func (store *EvidenceStore) getEvidenceInfo(evidence types.Evidence) EvidenceInfo {
	key := keyLookup(evidence)
	var ei EvidenceInfo
	b := store.db.Get(key)
	err := cdc.UnmarshalBinaryBare(b, &ei)
	if err != nil {
		panic(err)
	}
	return ei
}
