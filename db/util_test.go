package db

import (
	"fmt"
	"testing"
)

func TestPrefixIteratorNoMatchNil(t *testing.T) {
	for backend, _ := range backends {
		t.Run(fmt.Sprintf("Prefix w/ backend %s", backend), func(t *testing.T) {
			db := newTempDB(t, backend)
			itr := IteratePrefix(db, []byte("2"))

			checkInvalid(t, itr)
		})
	}
}

func TestPrefixIteratorNoMatch1(t *testing.T) {
	for backend, _ := range backends {
		t.Run(fmt.Sprintf("Prefix w/ backend %s", backend), func(t *testing.T) {
			db := newTempDB(t, backend)
			itr := IteratePrefix(db, []byte("2"))
			db.SetSync(bz("1"), bz("value_1"))

			checkInvalid(t, itr)
		})
	}
}

func TestPrefixIteratorMatch2(t *testing.T) {
	for backend, _ := range backends {
		t.Run(fmt.Sprintf("Prefix w/ backend %s", backend), func(t *testing.T) {
			db := newTempDB(t, backend)
			db.SetSync(bz("2"), bz("value_2"))
			itr := IteratePrefix(db, []byte("2"))

			checkValid(t, itr, true)
			checkItem(t, itr, bz("2"), bz("value_2"))
			checkNext(t, itr, false)

			// Once invalid...
			checkInvalid(t, itr)
		})
	}
}

func TestPrefixIteratorMatch3(t *testing.T) {
	for backend, _ := range backends {
		t.Run(fmt.Sprintf("Prefix w/ backend %s", backend), func(t *testing.T) {
			db := newTempDB(t, backend)
			db.SetSync(bz("3"), bz("value_3"))
			itr := IteratePrefix(db, []byte("2"))

			// Once invalid...
			checkInvalid(t, itr)
		})
	}
}

// Search for a/1, fail by too much Next()
func TestPrefixIteratorMatches1N(t *testing.T) {
	for backend, _ := range backends {
		t.Run(fmt.Sprintf("Prefix w/ backend %s", backend), func(t *testing.T) {
			db := newTempDB(t, backend)
			db.SetSync(bz("a/1"), bz("value_1"))
			db.SetSync(bz("a/3"), bz("value_3"))
			itr := IteratePrefix(db, []byte("a/"))

			checkValid(t, itr, true)
			checkItem(t, itr, bz("a/1"), bz("value_1"))
			checkNext(t, itr, true)
			checkItem(t, itr, bz("a/3"), bz("value_3"))

			// Bad!
			checkNext(t, itr, false)

			// Once invalid...
			checkInvalid(t, itr)
		})
	}
}

// Search for a/2, fail by too much Next()
func TestPrefixIteratorMatches2N(t *testing.T) {
	for backend, _ := range backends {
		t.Run(fmt.Sprintf("Prefix w/ backend %s", backend), func(t *testing.T) {
			db := newTempDB(t, backend)
			db.SetSync(bz("a/1"), bz("value_1"))
			db.SetSync(bz("a/3"), bz("value_3"))
			itr := IteratePrefix(db, []byte("a/"))

			checkValid(t, itr, true)
			checkItem(t, itr, bz("a/1"), bz("value_1"))
			checkNext(t, itr, true)
			checkValid(t, itr, true)
			checkItem(t, itr, bz("a/3"), bz("value_3"))

			// Bad!
			checkNext(t, itr, false)

			// Once invalid...
			checkInvalid(t, itr)
		})
	}
}

// Search for a/3, fail by too much Next()
func TestPrefixIteratorMatches3N(t *testing.T) {
	for backend, _ := range backends {
		t.Run(fmt.Sprintf("Prefix w/ backend %s", backend), func(t *testing.T) {
			db := newTempDB(t, backend)
			db.SetSync(bz("a/1"), bz("value_1"))
			db.SetSync(bz("a/3"), bz("value_3"))
			itr := IteratePrefix(db, []byte("a/"))

			checkValid(t, itr, true)
			checkItem(t, itr, bz("a/1"), bz("value_1"))
			checkNext(t, itr, true)
			checkItem(t, itr, bz("a/3"), bz("value_3"))

			// Bad!
			checkNext(t, itr, false)

			// Once invalid...
			checkInvalid(t, itr)
		})
	}
}
