package db

import (
	"fmt"
	"os"
	"testing"
)

// Empty iterator for empty db.
func TestPrefixIteratorNoMatchNil(t *testing.T) {
	for backend := range backends {
		t.Run(fmt.Sprintf("Prefix w/ backend %s", backend), func(t *testing.T) {
			db, dir := newTempDB(t, backend)
			defer os.RemoveAll(dir)
			itr := IteratePrefix(db, []byte("2"))

			checkInvalid(t, itr)
		})
	}
}

// Empty iterator for db populated after iterator created.
func TestPrefixIteratorNoMatch1(t *testing.T) {
	for backend := range backends {
		if backend == BoltDBBackend {
			t.Log("bolt does not support concurrent writes while iterating")
			continue
		}

		t.Run(fmt.Sprintf("Prefix w/ backend %s", backend), func(t *testing.T) {
			db, dir := newTempDB(t, backend)
			defer os.RemoveAll(dir)
			itr := IteratePrefix(db, []byte("2"))
			db.SetSync(bz("1"), bz("value_1"))

			checkInvalid(t, itr)
		})
	}
}

// Empty iterator for prefix starting after db entry.
func TestPrefixIteratorNoMatch2(t *testing.T) {
	for backend := range backends {
		t.Run(fmt.Sprintf("Prefix w/ backend %s", backend), func(t *testing.T) {
			db, dir := newTempDB(t, backend)
			defer os.RemoveAll(dir)
			db.SetSync(bz("3"), bz("value_3"))
			itr := IteratePrefix(db, []byte("4"))

			checkInvalid(t, itr)
		})
	}
}

// Iterator with single val for db with single val, starting from that val.
func TestPrefixIteratorMatch1(t *testing.T) {
	for backend := range backends {
		t.Run(fmt.Sprintf("Prefix w/ backend %s", backend), func(t *testing.T) {
			db, dir := newTempDB(t, backend)
			defer os.RemoveAll(dir)
			db.SetSync(bz("2"), bz("value_2"))
			itr := IteratePrefix(db, bz("2"))

			checkValid(t, itr, true)
			checkItem(t, itr, bz("2"), bz("value_2"))
			checkNext(t, itr, false)

			// Once invalid...
			checkInvalid(t, itr)
		})
	}
}

// Iterator with prefix iterates over everything with same prefix.
func TestPrefixIteratorMatches1N(t *testing.T) {
	for backend := range backends {
		t.Run(fmt.Sprintf("Prefix w/ backend %s", backend), func(t *testing.T) {
			db, dir := newTempDB(t, backend)
			defer os.RemoveAll(dir)

			// prefixed
			db.SetSync(bz("a/1"), bz("value_1"))
			db.SetSync(bz("a/3"), bz("value_3"))

			// not
			db.SetSync(bz("b/3"), bz("value_3"))
			db.SetSync(bz("a-3"), bz("value_3"))
			db.SetSync(bz("a.3"), bz("value_3"))
			db.SetSync(bz("abcdefg"), bz("value_3"))
			itr := IteratePrefix(db, bz("a/"))

			checkValid(t, itr, true)
			checkItem(t, itr, bz("a/1"), bz("value_1"))
			checkNext(t, itr, true)
			checkItem(t, itr, bz("a/3"), bz("value_3"))

			// Bad!
			checkNext(t, itr, false)

			//Once invalid...
			checkInvalid(t, itr)
		})
	}
}
