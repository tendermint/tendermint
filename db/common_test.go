package db

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	cmn "github.com/tendermint/tmlibs/common"
)

func checkValid(t *testing.T, itr Iterator, expected bool) {
	valid := itr.Valid()
	assert.Equal(t, expected, valid)
}

func checkNext(t *testing.T, itr Iterator, expected bool) {
	itr.Next()
	valid := itr.Valid()
	assert.Equal(t, expected, valid)
}

func checkNextPanics(t *testing.T, itr Iterator) {
	assert.Panics(t, func() { itr.Next() }, "checkNextPanics expected panic but didn't")
}

func checkPrevPanics(t *testing.T, itr Iterator) {
	assert.Panics(t, func() { itr.Prev() }, "checkPrevPanics expected panic but didn't")
}

func checkPrev(t *testing.T, itr Iterator, expected bool) {
	itr.Prev()
	valid := itr.Valid()
	assert.Equal(t, expected, valid)
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
	checkPrevPanics(t, itr)
}

func checkKeyPanics(t *testing.T, itr Iterator) {
	assert.Panics(t, func() { itr.Key() }, "checkKeyPanics expected panic but didn't")
}

func checkValuePanics(t *testing.T, itr Iterator) {
	assert.Panics(t, func() { itr.Key() }, "checkValuePanics expected panic but didn't")
}

func newTempDB(t *testing.T, backend string) (db DB) {
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
			itr := db.Iterator()

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
				itr := db.Iterator()
				checkValid(t, itr, true)

				for i := 0; i < 10; i++ {
					checkNext(t, itr, true)
					checkValid(t, itr, true)

					checkPrev(t, itr, true)
					checkValid(t, itr, true)
				}

				checkNext(t, itr, true)
				checkValid(t, itr, true)

				checkNext(t, itr, false)
				checkValid(t, itr, false)

				checkNextPanics(t, itr)

				// Once invalid...
				checkInvalid(t, itr)
			}

			{ // Fail by calling Prev too much
				itr := db.Iterator()
				checkValid(t, itr, true)

				for i := 0; i < 10; i++ {
					checkNext(t, itr, true)
					checkValid(t, itr, true)

					checkPrev(t, itr, true)
					checkValid(t, itr, true)
				}

				checkPrev(t, itr, false)
				checkValid(t, itr, false)

				checkPrevPanics(t, itr)

				// Once invalid...
				checkInvalid(t, itr)
			}
		})
	}
}

func TestDBIteratorEmpty(t *testing.T) {
	for backend, _ := range backends {
		t.Run(fmt.Sprintf("Backend %s", backend), func(t *testing.T) {
			db := newTempDB(t, backend)
			itr := db.Iterator()

			checkInvalid(t, itr)
		})
	}
}

func TestDBIteratorEmptySeek(t *testing.T) {
	for backend, _ := range backends {
		t.Run(fmt.Sprintf("Backend %s", backend), func(t *testing.T) {
			db := newTempDB(t, backend)
			itr := db.Iterator()
			itr.Seek(bz("1"))

			checkInvalid(t, itr)
		})
	}
}

func TestDBIteratorBadSeek(t *testing.T) {
	for backend, _ := range backends {
		t.Run(fmt.Sprintf("Backend %s", backend), func(t *testing.T) {
			db := newTempDB(t, backend)
			db.SetSync(bz("1"), bz("value_1"))
			itr := db.Iterator()
			itr.Seek(bz("2"))

			checkInvalid(t, itr)
		})
	}
}
