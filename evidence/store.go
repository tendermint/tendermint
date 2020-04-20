package evidence

import (
	dbm "github.com/tendermint/tm-db"

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

"evidence-lookup"/<evidence-height>/<evidence-hash> -> Info
"evidence-outqueue"/<priority>/<evidence-height>/<evidence-hash> -> Info
"evidence-pending"/<evidence-height>/<evidence-hash> -> Info
*/

type Info struct {
	Committed bool
	Priority  int64
	Evidence  types.Evidence
}

// Store is a store of all the evidence we've seen, including
// evidence that has been committed, evidence that has been verified but not broadcast,
// and evidence that has been broadcast but not yet committed.
type Store struct {
	db dbm.DB
}

func NewStore(db dbm.DB) *Store {
	return &Store{
		db: db,
	}
}

// PendingEvidence returns up to maxNum known, uncommitted evidence.
// If maxNum is -1, all evidence is returned.
func (store *Store) PendingEvidence(maxNum int64) (evidence []types.Evidence) {
	return store.listEvidence(baseKeyPending, maxNum)
}

// listEvidence lists up to maxNum pieces of evidence for the given prefix key.
// It is wrapped by PriorityEvidence and PendingEvidence for convenience.
// If maxNum is -1, there's no cap on the size of returned evidence.
func (store *Store) listEvidence(prefixKey byte, maxNum int64) (evidence []types.Evidence) {
	var count int64
	iter, err := dbm.IteratePrefix(store.db, []byte{prefixKey})
	if err != nil {
		panic(err)
	}
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		val := iter.Value()

		if count == maxNum {
			return evidence
		}
		count++

		var ei Info
		err := cdc.UnmarshalBinaryBare(val, &ei)
		if err != nil {
			panic(err)
		}
		evidence = append(evidence, ei.Evidence)
	}
	return evidence
}

// Has checks if the evidence is already stored
func (store *Store) Has(evidence types.Evidence) (bool, error) {
	key := keyPending(evidence)
	ok, err := store.db.Has(key)
	if err != nil {
		return true, err
	}
	if ok {
		return true, nil
	}
	key = keyCommitted(evidence)
	ok, err = store.db.Has(key)
	if err != nil {
		return true, err
	}
	return ok, nil
}

// AddNewEvidence adds the given evidence to the database.
// It returns false if the evidence is already stored.
func (store *Store) addEvidence(evidence types.Evidence, priority int64) error {

	ei := Info{
		Committed: false,
		Priority:  priority,
		Evidence:  evidence,
	}
	eiBytes := cdc.MustMarshalBinaryBare(ei)

	// add it to the store
	var err error
	key := keyPending(evidence)
	if err = store.db.Set(key, eiBytes); err != nil {
		return err
	}

	return nil
}

// MarkEvidenceAsCommitted removes evidence from pending and outqueue and sets the state to committed.
func (store *Store) MarkEvidenceAsCommitted(evidence types.Evidence) {

	pendingKey := keyPending(evidence)
	store.db.Delete(pendingKey)

	// committed Info doens't need priority
	ei := Info{
		Committed: true,
		Evidence:  evidence,
		Priority:  0,
	}

	committedKey := keyCommitted(evidence)
	store.db.Set(committedKey, cdc.MustMarshalBinaryBare(ei))
}
