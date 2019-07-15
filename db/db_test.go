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

func TestDBBatchWrite(t *testing.T) {
	testCases := []struct {
		modify func(batch Batch)
		calls  map[string]int
	}{
		0: {
			func(batch Batch) {
				batch.Set(bz("1"), bz("1"))
				batch.Set(bz("2"), bz("2"))
				batch.Delete(bz("3"))
				batch.Set(bz("4"), bz("4"))
				batch.Write()
			},
			map[string]int{
				"Set": 0, "SetSync": 0, "SetNoLock": 3, "SetNoLockSync": 0,
				"Delete": 0, "DeleteSync": 0, "DeleteNoLock": 1, "DeleteNoLockSync": 0,
			},
		},
		1: {
			func(batch Batch) {
				batch.Set(bz("1"), bz("1"))
				batch.Set(bz("2"), bz("2"))
				batch.Set(bz("4"), bz("4"))
				batch.Delete(bz("3"))
				batch.Write()
			},
			map[string]int{
				"Set": 0, "SetSync": 0, "SetNoLock": 3, "SetNoLockSync": 0,
				"Delete": 0, "DeleteSync": 0, "DeleteNoLock": 1, "DeleteNoLockSync": 0,
			},
		},
		2: {
			func(batch Batch) {
				batch.Set(bz("1"), bz("1"))
				batch.Set(bz("2"), bz("2"))
				batch.Delete(bz("3"))
				batch.Set(bz("4"), bz("4"))
				batch.WriteSync()
			},
			map[string]int{
				"Set": 0, "SetSync": 0, "SetNoLock": 2, "SetNoLockSync": 1,
				"Delete": 0, "DeleteSync": 0, "DeleteNoLock": 1, "DeleteNoLockSync": 0,
			},
		},
		3: {
			func(batch Batch) {
				batch.Set(bz("1"), bz("1"))
				batch.Set(bz("2"), bz("2"))
				batch.Set(bz("4"), bz("4"))
				batch.Delete(bz("3"))
				batch.WriteSync()
			},
			map[string]int{
				"Set": 0, "SetSync": 0, "SetNoLock": 3, "SetNoLockSync": 0,
				"Delete": 0, "DeleteSync": 0, "DeleteNoLock": 0, "DeleteNoLockSync": 1,
			},
		},
	}

	for i, tc := range testCases {
		mdb := newMockDB()
		batch := mdb.NewBatch()

		tc.modify(batch)

		for call, exp := range tc.calls {
			got := mdb.calls[call]
			assert.Equal(t, exp, got, "#%v - key: %s", i, call)
		}
	}
}
