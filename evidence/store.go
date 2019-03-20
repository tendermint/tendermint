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
	for i, j := 0, len(l)-1; i < j; i, j = i+1, j-1 {
		l[i], l[j] = l[j], l[i]
	}

	return l
}

// PendingEvidence returns up to maxNum known, uncommitted evidence.
// If maxNum is -1, all evidence is returned.
func (store *EvidenceStore) PendingEvidence(maxNum int64) (evidence []types.Evidence) {
	return store.listEvidence(baseKeyPending, maxNum)
}

// listEvidence lists up to maxNum pieces of evidence for the given prefix key.
// It is wrapped by PriorityEvidence and PendingEvidence for convenience.
// If maxNum is -1, there's no cap on the size of returned evidence.
func (store *EvidenceStore) listEvidence(prefixKey string, maxNum int64) (evidence []types.Evidence) {
	var count int64
	iter := dbm.IteratePrefix(store.db, []byte(prefixKey))
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		val := iter.Value()

		if count == maxNum {
			return evidence
		}
		count++

		var ei EvidenceInfo
		err := cdc.UnmarshalBinaryBare(val, &ei)
		if err != nil {
			panic(err)
		}
		evidence = append(evidence, ei.Evidence)
	}
	return evidence
}

// GetEvidenceInfo fetches the EvidenceInfo with the given height and hash.
// If not found, ei.Evidence is nil.
func (store *EvidenceStore) GetEvidenceInfo(height int64, hash []byte) EvidenceInfo {
	key := keyLookupFromHeightAndHash(height, hash)
	val := store.db.Get(key)

	if len(val) == 0 {
		return EvidenceInfo{}
	}
	var ei EvidenceInfo
	err := cdc.UnmarshalBinaryBare(val, &ei)
	if err != nil {
		panic(err)
	}
	return ei
}

// AddNewEvidence adds the given evidence to the database.
// It returns false if the evidence is already stored.
func (store *EvidenceStore) AddNewEvidence(evidence types.Evidence, priority int64) bool {
	// check if we already have seen it
	ei := store.getEvidenceInfo(evidence)
	if ei.Evidence != nil {
		return false
	}

	ei = EvidenceInfo{
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
	if ei.Evidence == nil {
		// nothing to do; we did not store the evidence yet (AddNewEvidence):
		return
	}
	// remove from the outqueue
	key := keyOutqueue(evidence, ei.Priority)
	store.db.Delete(key)
}

// MarkEvidenceAsCommitted removes evidence from pending and outqueue and sets the state to committed.
func (store *EvidenceStore) MarkEvidenceAsCommitted(evidence types.Evidence) {
	// if its committed, its been broadcast
	store.MarkEvidenceAsBroadcasted(evidence)

	pendingKey := keyPending(evidence)
	store.db.Delete(pendingKey)

	// committed EvidenceInfo doens't need priority
	ei := EvidenceInfo{
		Committed: true,
		Evidence:  evidence,
		Priority:  0,
	}

	lookupKey := keyLookup(evidence)
	store.db.SetSync(lookupKey, cdc.MustMarshalBinaryBare(ei))
}

//---------------------------------------------------
// utils

// getEvidenceInfo is convenience for calling GetEvidenceInfo if we have the full evidence.
func (store *EvidenceStore) getEvidenceInfo(evidence types.Evidence) EvidenceInfo {
	return store.GetEvidenceInfo(evidence.Height(), evidence.Hash())
}
