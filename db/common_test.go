package db

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cmn "github.com/tendermint/tmlibs/common"
)

func checkValid(t *testing.T, itr Iterator, expected bool) {
	valid := itr.Valid()
	require.Equal(t, expected, valid)
}

func checkNext(t *testing.T, itr Iterator, expected bool) {
	itr.Next()
	valid := itr.Valid()
	require.Equal(t, expected, valid)
}

func checkNextPanics(t *testing.T, itr Iterator) {
	assert.Panics(t, func() { itr.Next() }, "checkNextPanics expected panic but didn't")
}

func checkItem(t *testing.T, itr Iterator, key []byte, value []byte) {
	k, v := itr.Key(), itr.Value()
	assert.Exactly(t, key, k)
	assert.Exactly(t, value, v)
}

func checkInvalid(t *testing.T, itr Iterator) {
	checkValid(t, itr, false)
	checkKeyPanics(t, itr)
	checkValuePanics(t, itr)
	checkNextPanics(t, itr)
}

func checkKeyPanics(t *testing.T, itr Iterator) {
	assert.Panics(t, func() { itr.Key() }, "checkKeyPanics expected panic but didn't")
}

func checkValuePanics(t *testing.T, itr Iterator) {
	assert.Panics(t, func() { itr.Key() }, "checkValuePanics expected panic but didn't")
}

func newTempDB(t *testing.T, backend DBBackendType) (db DB) {
	dir, dirname := cmn.Tempdir("test_go_iterator")
	db = NewDB("testdb", backend, dirname)
	dir.Close()
	return db
}

func TestDBIteratorSingleKey(t *testing.T) {
	for backend, _ := range backends {
		t.Run(fmt.Sprintf("Backend %s", backend), func(t *testing.T) {
			db := newTempDB(t, backend)
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
	for backend, _ := range backends {
		t.Run(fmt.Sprintf("Backend %s", backend), func(t *testing.T) {
			db := newTempDB(t, backend)
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
	for backend, _ := range backends {
		t.Run(fmt.Sprintf("Backend %s", backend), func(t *testing.T) {
			db := newTempDB(t, backend)

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
	for backend, _ := range backends {
		t.Run(fmt.Sprintf("Backend %s", backend), func(t *testing.T) {
			db := newTempDB(t, backend)
			itr := db.Iterator(nil, nil)

			checkInvalid(t, itr)
		})
	}
}

func TestDBIteratorEmptyBeginAfter(t *testing.T) {
	for backend, _ := range backends {
		t.Run(fmt.Sprintf("Backend %s", backend), func(t *testing.T) {
			db := newTempDB(t, backend)
			itr := db.Iterator(bz("1"), nil)

			checkInvalid(t, itr)
		})
	}
}

func TestDBIteratorNonemptyBeginAfter(t *testing.T) {
	for backend, _ := range backends {
		t.Run(fmt.Sprintf("Backend %s", backend), func(t *testing.T) {
			db := newTempDB(t, backend)
			db.SetSync(bz("1"), bz("value_1"))
			itr := db.Iterator(bz("2"), nil)

			checkInvalid(t, itr)
		})
	}
}
