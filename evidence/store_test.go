package evidence

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	wire "github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/types"
	dbm "github.com/tendermint/tmlibs/db"
)

//-------------------------------------------

func TestStoreAddDuplicate(t *testing.T) {
	assert := assert.New(t)

	db := dbm.NewMemDB()
	store := NewEvidenceStore(db)

	priority := 10
	ev := newMockGoodEvidence(2, 1, []byte("val1"))

	added := store.AddNewEvidence(ev, priority)
	assert.True(added)

	// cant add twice
	added = store.AddNewEvidence(ev, priority)
	assert.False(added)
}

func TestStoreMark(t *testing.T) {
	assert := assert.New(t)

	db := dbm.NewMemDB()
	store := NewEvidenceStore(db)

	// before we do anything, priority/pending are empty
	priorityEv := store.PriorityEvidence()
	pendingEv := store.PendingEvidence()
	assert.Equal(0, len(priorityEv))
	assert.Equal(0, len(pendingEv))

	priority := 10
	ev := newMockGoodEvidence(2, 1, []byte("val1"))

	added := store.AddNewEvidence(ev, priority)
	assert.True(added)

	// get the evidence. verify. should be uncommitted
	ei := store.GetEvidence(ev.Height(), ev.Hash())
	assert.Equal(ev, ei.Evidence)
	assert.Equal(priority, ei.Priority)
	assert.False(ei.Committed)

	// new evidence should be returns in priority/pending
	priorityEv = store.PriorityEvidence()
	pendingEv = store.PendingEvidence()
	assert.Equal(1, len(priorityEv))
	assert.Equal(1, len(pendingEv))

	// priority is now empty
	store.MarkEvidenceAsBroadcasted(ev)
	priorityEv = store.PriorityEvidence()
	pendingEv = store.PendingEvidence()
	assert.Equal(0, len(priorityEv))
	assert.Equal(1, len(pendingEv))

	// priority and pending are now empty
	store.MarkEvidenceAsCommitted(ev)
	priorityEv = store.PriorityEvidence()
	pendingEv = store.PendingEvidence()
	assert.Equal(0, len(priorityEv))
	assert.Equal(0, len(pendingEv))

	// evidence should show committed
	ei = store.GetEvidence(ev.Height(), ev.Hash())
	assert.Equal(ev, ei.Evidence)
	assert.Equal(priority, ei.Priority)
	assert.True(ei.Committed)
}

func TestStorePriority(t *testing.T) {
	assert := assert.New(t)

	db := dbm.NewMemDB()
	store := NewEvidenceStore(db)

	// sorted by priority and then height
	cases := []struct {
		ev       MockGoodEvidence
		priority int
	}{
		{newMockGoodEvidence(2, 1, []byte("val1")), 17},
		{newMockGoodEvidence(5, 2, []byte("val2")), 15},
		{newMockGoodEvidence(10, 2, []byte("val2")), 13},
		{newMockGoodEvidence(100, 2, []byte("val2")), 11},
		{newMockGoodEvidence(90, 2, []byte("val2")), 11},
		{newMockGoodEvidence(80, 2, []byte("val2")), 11},
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

//-------------------------------------------
const (
	evidenceTypeMock = byte(0x01)
)

var _ = wire.RegisterInterface(
	struct{ types.Evidence }{},
	wire.ConcreteType{MockGoodEvidence{}, evidenceTypeMock},
)

type MockGoodEvidence struct {
	Height_  int
	Address_ []byte
	Index_   int
}

func newMockGoodEvidence(height, index int, address []byte) MockGoodEvidence {
	return MockGoodEvidence{height, address, index}
}

func (e MockGoodEvidence) Height() int     { return e.Height_ }
func (e MockGoodEvidence) Address() []byte { return e.Address_ }
func (e MockGoodEvidence) Index() int      { return e.Index_ }
func (e MockGoodEvidence) Hash() []byte {
	return []byte(fmt.Sprintf("%d-%d", e.Height_, e.Index_))
}
func (e MockGoodEvidence) Verify(chainID string) error { return nil }
func (e MockGoodEvidence) Equal(ev types.Evidence) bool {
	e2 := ev.(MockGoodEvidence)
	return e.Height_ == e2.Height_ &&
		bytes.Equal(e.Address_, e2.Address_) &&
		e.Index_ == e2.Index_
}
func (e MockGoodEvidence) String() string {
	return fmt.Sprintf("GoodEvidence: %d/%s/%d", e.Height_, e.Address_, e.Index_)
}

type MockBadEvidence struct {
	MockGoodEvidence
}

func (e MockBadEvidence) Verify(chainID string) error { return fmt.Errorf("MockBadEvidence") }
func (e MockBadEvidence) Equal(ev types.Evidence) bool {
	e2 := ev.(MockBadEvidence)
	return e.Height_ == e2.Height_ &&
		bytes.Equal(e.Address_, e2.Address_) &&
		e.Index_ == e2.Index_
}
func (e MockBadEvidence) String() string {
	return fmt.Sprintf("BadEvidence: %d/%s/%d", e.Height_, e.Address_, e.Index_)
}
