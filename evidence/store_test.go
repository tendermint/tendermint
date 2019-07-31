package evidence

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tm-db"
)

//-------------------------------------------

func TestStoreAddDuplicate(t *testing.T) {
	assert := assert.New(t)

	db := dbm.NewMemDB()
	store := NewEvidenceStore(db)

	priority := int64(10)
	ev := types.NewMockGoodEvidence(2, 1, []byte("val1"))

	added := store.AddNewEvidence(ev, priority)
	assert.True(added)

	// cant add twice
	added = store.AddNewEvidence(ev, priority)
	assert.False(added)
}

func TestStoreCommitDuplicate(t *testing.T) {
	assert := assert.New(t)

	db := dbm.NewMemDB()
	store := NewEvidenceStore(db)

	priority := int64(10)
	ev := types.NewMockGoodEvidence(2, 1, []byte("val1"))

	store.MarkEvidenceAsCommitted(ev)

	added := store.AddNewEvidence(ev, priority)
	assert.False(added)
}

func TestStoreMark(t *testing.T) {
	assert := assert.New(t)

	db := dbm.NewMemDB()
	store := NewEvidenceStore(db)

	// before we do anything, priority/pending are empty
	priorityEv := store.PriorityEvidence()
	pendingEv := store.PendingEvidence(-1)
	assert.Equal(0, len(priorityEv))
	assert.Equal(0, len(pendingEv))

	priority := int64(10)
	ev := types.NewMockGoodEvidence(2, 1, []byte("val1"))

	added := store.AddNewEvidence(ev, priority)
	assert.True(added)

	// get the evidence. verify. should be uncommitted
	ei := store.GetEvidenceInfo(ev.Height(), ev.Hash())
	assert.Equal(ev, ei.Evidence)
	assert.Equal(priority, ei.Priority)
	assert.False(ei.Committed)

	// new evidence should be returns in priority/pending
	priorityEv = store.PriorityEvidence()
	pendingEv = store.PendingEvidence(-1)
	assert.Equal(1, len(priorityEv))
	assert.Equal(1, len(pendingEv))

	// priority is now empty
	store.MarkEvidenceAsBroadcasted(ev)
	priorityEv = store.PriorityEvidence()
	pendingEv = store.PendingEvidence(-1)
	assert.Equal(0, len(priorityEv))
	assert.Equal(1, len(pendingEv))

	// priority and pending are now empty
	store.MarkEvidenceAsCommitted(ev)
	priorityEv = store.PriorityEvidence()
	pendingEv = store.PendingEvidence(-1)
	assert.Equal(0, len(priorityEv))
	assert.Equal(0, len(pendingEv))

	// evidence should show committed
	newPriority := int64(0)
	ei = store.GetEvidenceInfo(ev.Height(), ev.Hash())
	assert.Equal(ev, ei.Evidence)
	assert.Equal(newPriority, ei.Priority)
	assert.True(ei.Committed)
}

func TestStorePriority(t *testing.T) {
	assert := assert.New(t)

	db := dbm.NewMemDB()
	store := NewEvidenceStore(db)

	// sorted by priority and then height
	cases := []struct {
		ev       types.MockGoodEvidence
		priority int64
	}{
		{types.NewMockGoodEvidence(2, 1, []byte("val1")), 17},
		{types.NewMockGoodEvidence(5, 2, []byte("val2")), 15},
		{types.NewMockGoodEvidence(10, 2, []byte("val2")), 13},
		{types.NewMockGoodEvidence(100, 2, []byte("val2")), 11},
		{types.NewMockGoodEvidence(90, 2, []byte("val2")), 11},
		{types.NewMockGoodEvidence(80, 2, []byte("val2")), 11},
	}

	for _, c := range cases {
		added := store.AddNewEvidence(c.ev, c.priority)
		assert.True(added)
	}

	evList := store.PriorityEvidence()
	for i, ev := range evList {
		assert.Equal(ev, cases[i].ev)
	}
}
