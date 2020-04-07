package evidence

import (
	"fmt"

	dbm "github.com/tendermint/tm-db"

	ep "github.com/tendermint/tendermint/proto/evidence"
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

func _key(format string, o ...interface{}) []byte {
	return []byte(fmt.Sprintf(format, o...))
}

type Info struct {
	Committed bool
	Priority  int64
	Evidence  types.Evidence
}

func (i Info) ToProto() (*ep.Info, error) {

	evi, err := types.EvidenceToProto(i.Evidence)
	if err != nil {
		return nil, err
	}

	ei := ep.Info{
		Committed: i.Committed,
		Priority:  i.Priority,
		Evidence:  *evi,
	}
	return &ei, nil
}

func (i *Info) FromProto(ip ep.Info) error {
	i.Committed = ip.Committed
	i.Priority = ip.Priority

	evi, err := types.EvidenceFromProto(ip.Evidence)
	if err != nil {
		return err
	}
	i.Evidence = evi

	return nil
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

// PriorityEvidence returns the evidence from the outqueue, sorted by highest priority.
func (store *Store) PriorityEvidence() (evidence []types.Evidence) {
	// reverse the order so highest priority is first
	l := store.listEvidence(baseKeyOutqueue, -1)
	for i, j := 0, len(l)-1; i < j; i, j = i+1, j-1 {
		l[i], l[j] = l[j], l[i]
	}

	return l
}

// PendingEvidence returns up to maxNum known, uncommitted evidence.
// If maxNum is -1, all evidence is returned.
func (store *Store) PendingEvidence(maxNum int64) (evidence []types.Evidence) {
	return store.listEvidence(baseKeyPending, maxNum)
}

// listEvidence lists up to maxNum pieces of evidence for the given prefix key.
// It is wrapped by PriorityEvidence and PendingEvidence for convenience.
// If maxNum is -1, there's no cap on the size of returned evidence.
func (store *Store) listEvidence(prefixKey string, maxNum int64) (evidence []types.Evidence) {
	var count int64
	iter, err := dbm.IteratePrefix(store.db, []byte(prefixKey))
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

		var ip ep.Info
		err := ip.Unmarshal(val)
		if err != nil {
			panic(err)
		}
		var ei Info
		err = ei.FromProto(ip)
		if err != nil {
			panic(err)
		}
		evidence = append(evidence, ei.Evidence)
	}
	return evidence
}

// GetInfo fetches the Info with the given height and hash.
// If not found, ei.Evidence is nil.
func (store *Store) GetInfo(height int64, hash []byte) Info {
	key := keyLookupFromHeightAndHash(height, hash)
	val, err := store.db.Get(key)
	if err != nil {
		panic(err)
	}
	if len(val) == 0 {
		return Info{}
	}
	var ip ep.Info
	err = ip.Unmarshal(val)
	if err != nil {
		panic(err)
	}

	var ei Info
	err = ei.FromProto(ip)
	if err != nil {
		panic(err)
	}

	return ei
}

// AddNewEvidence adds the given evidence to the database.
// It returns false if the evidence is already stored.
func (store *Store) AddNewEvidence(evidence types.Evidence, priority int64) bool {
	// check if we already have seen it
	ei := store.getInfo(evidence)
	if ei.Evidence != nil {
		return false
	}

	ei = Info{
		Committed: false,
		Priority:  priority,
		Evidence:  evidence,
	}
	ip, err := ei.ToProto()
	if err != nil {
		panic(err)
	}

	eiBytes, err := ip.Marshal()
	if err != nil {
		panic(err)
	}

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
func (store *Store) MarkEvidenceAsBroadcasted(evidence types.Evidence) {
	ei := store.getInfo(evidence)
	if ei.Evidence == nil {
		// nothing to do; we did not store the evidence yet (AddNewEvidence):
		return
	}
	// remove from the outqueue
	key := keyOutqueue(evidence, ei.Priority)
	store.db.Delete(key)
}

// MarkEvidenceAsCommitted removes evidence from pending and outqueue and sets the state to committed.
func (store *Store) MarkEvidenceAsCommitted(evidence types.Evidence) {
	// if its committed, its been broadcast
	store.MarkEvidenceAsBroadcasted(evidence)

	pendingKey := keyPending(evidence)
	store.db.Delete(pendingKey)

	// committed Info doens't need priority
	ei := Info{
		Committed: true,
		Evidence:  evidence,
		Priority:  0,
	}

	lookupKey := keyLookup(evidence)
	ip, err := ei.ToProto()
	if err != nil {
		panic(err)
	}
	eip, err := ip.Marshal()
	if err != nil {
		panic(err)
	}
	store.db.SetSync(lookupKey, eip)
}

//---------------------------------------------------
// utils

// getInfo is convenience for calling GetInfo if we have the full evidence.
func (store *Store) getInfo(evidence types.Evidence) Info {
	return store.GetInfo(evidence.Height(), evidence.Hash())
}
