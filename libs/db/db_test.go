package db

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDBIteratorSingleKey(t *testing.T) {
	for backend := range backends {
		t.Run(fmt.Sprintf("Backend %s", backend), func(t *testing.T) {
			db, dir := newTempDB(t, backend)
			defer os.RemoveAll(dir)

			db.SetSync(bz("1"), bz("value_1"))
			itr := db.Iterator(nil, nil)

			checkValid(t, itr, true)
			checkNext(t, itr, false)
			checkValid(t, itr, false)
			checkNextPanics(t, itr)

			// Once invalid...
			checkInvalid(t, itr)
		})
	}
}

func TestDBIteratorTwoKeys(t *testing.T) {
	for backend := range backends {
		t.Run(fmt.Sprintf("Backend %s", backend), func(t *testing.T) {
			db, dir := newTempDB(t, backend)
			defer os.RemoveAll(dir)

			db.SetSync(bz("1"), bz("value_1"))
			db.SetSync(bz("2"), bz("value_1"))

			{ // Fail by calling Next too much
				itr := db.Iterator(nil, nil)
				checkValid(t, itr, true)

				checkNext(t, itr, true)
				checkValid(t, itr, true)

				checkNext(t, itr, false)
				checkValid(t, itr, false)

				checkNextPanics(t, itr)

				// Once invalid...
				checkInvalid(t, itr)
			}
		})
	}
}

func TestDBIteratorMany(t *testing.T) {
	for backend := range backends {
		t.Run(fmt.Sprintf("Backend %s", backend), func(t *testing.T) {
			db, dir := newTempDB(t, backend)
			defer os.RemoveAll(dir)

			keys := make([][]byte, 100)
			for i := 0; i < 100; i++ {
				keys[i] = []byte{byte(i)}
			}

			value := []byte{5}
			for _, k := range keys {
				db.Set(k, value)
			}

			itr := db.Iterator(nil, nil)
			defer itr.Close()
			for ; itr.Valid(); itr.Next() {
				assert.Equal(t, db.Get(itr.Key()), itr.Value())
			}
		})
	}
}

func TestDBIteratorEmpty(t *testing.T) {
	for backend := range backends {
		t.Run(fmt.Sprintf("Backend %s", backend), func(t *testing.T) {
			db, dir := newTempDB(t, backend)
			defer os.RemoveAll(dir)

			itr := db.Iterator(nil, nil)

			checkInvalid(t, itr)
		})
	}
}

func TestDBIteratorEmptyBeginAfter(t *testing.T) {
	for backend := range backends {
		t.Run(fmt.Sprintf("Backend %s", backend), func(t *testing.T) {
			db, dir := newTempDB(t, backend)
			defer os.RemoveAll(dir)

			itr := db.Iterator(bz("1"), nil)

			checkInvalid(t, itr)
		})
	}
}

func TestDBIteratorNonemptyBeginAfter(t *testing.T) {
	for backend := range backends {
		t.Run(fmt.Sprintf("Backend %s", backend), func(t *testing.T) {
			db, dir := newTempDB(t, backend)
			defer os.RemoveAll(dir)

			db.SetSync(bz("1"), bz("value_1"))
			itr := db.Iterator(bz("2"), nil)

			checkInvalid(t, itr)
		})
	}
}

func TestDBBatchWrite1(t *testing.T) {
	mdb := newMockDB()
	ddb := NewDebugDB(t.Name(), mdb)
	batch := ddb.NewBatch()

	batch.Set(bz("1"), bz("1"))
	batch.Set(bz("2"), bz("2"))
	batch.Delete(bz("3"))
	batch.Set(bz("4"), bz("4"))
	batch.Write()

	assert.Equal(t, 0, mdb.calls["Set"])
	assert.Equal(t, 0, mdb.calls["SetSync"])
	assert.Equal(t, 3, mdb.calls["SetNoLock"])
	assert.Equal(t, 0, mdb.calls["SetNoLockSync"])
	assert.Equal(t, 0, mdb.calls["Delete"])
	assert.Equal(t, 0, mdb.calls["DeleteSync"])
	assert.Equal(t, 1, mdb.calls["DeleteNoLock"])
	assert.Equal(t, 0, mdb.calls["DeleteNoLockSync"])
}

func TestDBBatchWrite2(t *testing.T) {
	mdb := newMockDB()
	ddb := NewDebugDB(t.Name(), mdb)
	batch := ddb.NewBatch()

	batch.Set(bz("1"), bz("1"))
	batch.Set(bz("2"), bz("2"))
	batch.Set(bz("4"), bz("4"))
	batch.Delete(bz("3"))
	batch.Write()

	assert.Equal(t, 0, mdb.calls["Set"])
	assert.Equal(t, 0, mdb.calls["SetSync"])
	assert.Equal(t, 3, mdb.calls["SetNoLock"])
	assert.Equal(t, 0, mdb.calls["SetNoLockSync"])
	assert.Equal(t, 0, mdb.calls["Delete"])
	assert.Equal(t, 0, mdb.calls["DeleteSync"])
	assert.Equal(t, 1, mdb.calls["DeleteNoLock"])
	assert.Equal(t, 0, mdb.calls["DeleteNoLockSync"])
}

func TestDBBatchWriteSync1(t *testing.T) {
	mdb := newMockDB()
	ddb := NewDebugDB(t.Name(), mdb)
	batch := ddb.NewBatch()

	batch.Set(bz("1"), bz("1"))
	batch.Set(bz("2"), bz("2"))
	batch.Delete(bz("3"))
	batch.Set(bz("4"), bz("4"))
	batch.WriteSync()

	assert.Equal(t, 0, mdb.calls["Set"])
	assert.Equal(t, 0, mdb.calls["SetSync"])
	assert.Equal(t, 2, mdb.calls["SetNoLock"])
	assert.Equal(t, 1, mdb.calls["SetNoLockSync"])
	assert.Equal(t, 0, mdb.calls["Delete"])
	assert.Equal(t, 0, mdb.calls["DeleteSync"])
	assert.Equal(t, 1, mdb.calls["DeleteNoLock"])
	assert.Equal(t, 0, mdb.calls["DeleteNoLockSync"])
}

func TestDBBatchWriteSync2(t *testing.T) {
	mdb := newMockDB()
	ddb := NewDebugDB(t.Name(), mdb)
	batch := ddb.NewBatch()

	batch.Set(bz("1"), bz("1"))
	batch.Set(bz("2"), bz("2"))
	batch.Set(bz("4"), bz("4"))
	batch.Delete(bz("3"))
	batch.WriteSync()

	assert.Equal(t, 0, mdb.calls["Set"])
	assert.Equal(t, 0, mdb.calls["SetSync"])
	assert.Equal(t, 3, mdb.calls["SetNoLock"])
	assert.Equal(t, 0, mdb.calls["SetNoLockSync"])
	assert.Equal(t, 0, mdb.calls["Delete"])
	assert.Equal(t, 0, mdb.calls["DeleteSync"])
	assert.Equal(t, 0, mdb.calls["DeleteNoLock"])
	assert.Equal(t, 1, mdb.calls["DeleteNoLockSync"])
}
