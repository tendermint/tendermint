package evidence

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbm "github.com/tendermint/tm-db"

	"github.com/tendermint/tendermint/types"
)

//-------------------------------------------

func TestStoreAddDuplicate(t *testing.T) {
	db := dbm.NewMemDB()
	store := NewStore(db)

	priority := int64(10)
	ev := types.NewMockEvidence(2, time.Now().UTC(), 1, []byte("val1"))

	added, err := store.AddNewEvidence(ev, priority)
	require.NoError(t, err)
	assert.True(t, added)

	// cant add twice
	added, err = store.AddNewEvidence(ev, priority)
	require.NoError(t, err)
	assert.False(t, added)
}

func TestStoreCommitDuplicate(t *testing.T) {
	db := dbm.NewMemDB()
	store := NewStore(db)

	priority := int64(10)
	ev := types.NewMockEvidence(2, time.Now().UTC(), 1, []byte("val1"))

	store.MarkEvidenceAsCommitted(ev)

	added, err := store.AddNewEvidence(ev, priority)
	require.NoError(t, err)
	assert.False(t, added)
}

func TestStoreMark(t *testing.T) {
	db := dbm.NewMemDB()
	store := NewStore(db)

	// before we do anything, priority/pending are empty
	priorityEv := store.PriorityEvidence()
	pendingEv := store.PendingEvidence(-1)
	assert.Equal(t, 0, len(priorityEv))
	assert.Equal(t, 0, len(pendingEv))

	priority := int64(10)
	ev := types.NewMockEvidence(2, time.Now().UTC(), 1, []byte("val1"))

	added, err := store.AddNewEvidence(ev, priority)
	require.NoError(t, err)
	assert.True(t, added)

	// get the evidence. verify. should be uncommitted
	ei := store.GetInfo(ev.Height(), ev.Hash())
	assert.Equal(t, ev, ei.Evidence)
	assert.Equal(t, priority, ei.Priority)
	assert.False(t, ei.Committed)

	// new evidence should be returns in priority/pending
	priorityEv = store.PriorityEvidence()
	pendingEv = store.PendingEvidence(-1)
	assert.Equal(t, 1, len(priorityEv))
	assert.Equal(t, 1, len(pendingEv))

	// priority is now empty
	store.MarkEvidenceAsBroadcasted(ev)
	priorityEv = store.PriorityEvidence()
	pendingEv = store.PendingEvidence(-1)
	assert.Equal(t, 0, len(priorityEv))
	assert.Equal(t, 1, len(pendingEv))

	// priority and pending are now empty
	store.MarkEvidenceAsCommitted(ev)
	priorityEv = store.PriorityEvidence()
	pendingEv = store.PendingEvidence(-1)
	assert.Equal(t, 0, len(priorityEv))
	assert.Equal(t, 0, len(pendingEv))

	// evidence should show committed
	newPriority := int64(0)
	ei = store.GetInfo(ev.Height(), ev.Hash())
	assert.Equal(t, ev, ei.Evidence)
	assert.Equal(t, newPriority, ei.Priority)
	assert.True(t, ei.Committed)
}

func TestStorePriority(t *testing.T) {
	db := dbm.NewMemDB()
	store := NewStore(db)

	// sorted by priority and then height
	cases := []struct {
		ev       types.MockEvidence
		priority int64
	}{
		{types.NewMockEvidence(2, time.Now().UTC(), 1, []byte("val1")), 17},
		{types.NewMockEvidence(5, time.Now().UTC(), 2, []byte("val2")), 15},
		{types.NewMockEvidence(10, time.Now().UTC(), 2, []byte("val2")), 13},
		{types.NewMockEvidence(100, time.Now().UTC(), 2, []byte("val2")), 11},
		{types.NewMockEvidence(90, time.Now().UTC(), 2, []byte("val2")), 11},
		{types.NewMockEvidence(80, time.Now().UTC(), 2, []byte("val2")), 11},
	}

	for _, c := range cases {
		added, err := store.AddNewEvidence(c.ev, c.priority)
		require.NoError(t, err)
		assert.True(t, added)
	}

	evList := store.PriorityEvidence()
	for i, ev := range evList {
		assert.Equal(t, ev, cases[i].ev)
	}
}
