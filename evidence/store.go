package evpool

import (
	"fmt"

	wire "github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tmlibs/db"
)

/*
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

func keyLookup(evidence types.Evidence) []byte {
	return []byte(fmt.Sprintf("evidence-lookup/%d/%X", evidence.Height(), evidence.Hash()))
}

func keyOutqueue(idx int, evidence types.Evidence) []byte {
	return []byte(fmt.Sprintf("evidence-outqueue/%d/%d/%X", idx, evidence.Height(), evidence.Hash()))
}

func keyPending(evidence types.Evidence) []byte {
	return []byte(fmt.Sprintf("evidence-pending/%d/%X", evidence.Height(), evidence.Hash()))
}

// EvidenceStore stores all the evidence we've seen, including
// evidence that has been committed, evidence that has been seen but not broadcast,
// and evidence that has been broadcast but not yet committed.
type EvidenceStore struct {
	chainID string
	db      dbm.DB
}

func NewEvidenceStore(chainID string, db dbm.DB) *EvidenceStore {
	return &EvidenceStore{
		chainID: chainID,
		db:      db,
	}
}

// AddNewEvidence adds the given evidence to the database.
func (store *EvidenceStore) AddNewEvidence(idx int, evidence types.Evidence) (bool, error) {
	// check if we already have seen it
	key := keyLookup(evidence)
	v := store.db.Get(key)
	if len(v) == 0 {
		return false, nil
	}

	// verify the evidence
	if err := evidence.Verify(store.chainID); err != nil {
		return false, err
	}

	// add it to the store
	ei := evidenceInfo{
		Committed: false,
		Priority:  idx,
		Evidence:  evidence,
	}
	store.db.Set(key, wire.BinaryBytes(ei))

	key = keyOutqueue(idx, evidence)
	store.db.Set(key, nullValue)

	key = keyPending(evidence)
	store.db.Set(key, nullValue)

	return true, nil
}

// MarkEvidenceAsBroadcasted removes evidence from the outqueue.
func (store *EvidenceStore) MarkEvidenceAsBroadcasted(idx int, evidence types.Evidence) {
	key := keyOutqueue(idx, evidence)
	store.db.Delete(key)
}

// MarkEvidenceAsPending removes evidence from pending and sets the state to committed.
func (store *EvidenceStore) MarkEvidenceAsCommitted(evidence types.Evidence) {
	key := keyPending(evidence)
	store.db.Delete(key)

	key = keyLookup(evidence)
	var ei evidenceInfo
	b := store.db.Get(key)
	wire.ReadBinaryBytes(b, &ei)
	ei.Committed = true
	store.db.Set(key, wire.BinaryBytes(ei))
}
