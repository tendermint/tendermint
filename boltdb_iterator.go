// +build boltdb

package db

import (
	"bytes"

	"go.etcd.io/bbolt"
)

// boltDBIterator allows you to iterate on range of keys/values given some
// start / end keys (nil & nil will result in doing full scan).
type boltDBIterator struct {
	tx *bbolt.Tx

	itr           *bbolt.Cursor
	start         []byte
	end           []byte
	emptyKeyValue []byte // Tracks the value of the empty key, if it exists

	currentKey   []byte
	currentValue []byte

	isInvalid bool
	isReverse bool
}

var _ Iterator = (*boltDBIterator)(nil)

// newBoltDBIterator creates a new boltDBIterator.
func newBoltDBIterator(tx *bbolt.Tx, start, end []byte, isReverse bool) *boltDBIterator {
	// We can check for empty key at the start, because we use a read/write transaction that blocks
	// the entire database for writes while the iterator exists. If we change to a read-only txn
	// that supports concurrency we'll need to rewrite this logic.
	emptyKeyValue := tx.Bucket(bucket).Get(boltDBEmptyKey)
	itr := tx.Bucket(bucket).Cursor()

	var ck, cv []byte
	if isReverse {
		switch {
		case end == nil:
			ck, cv = itr.Last()
		case len(end) == 0:
			// If end is the blank key, then we don't return any keys by definition
			ck = nil
			cv = nil
		default:
			_, _ = itr.Seek(end) // after key
			ck, cv = itr.Prev()  // return to end key
		}
		// If we're currently positioned at the placeholder for the empty key, skip it (handle later)
		if emptyKeyValue != nil && bytes.Equal(ck, boltDBEmptyKey) {
			ck, cv = itr.Prev()
		}
		// If we didn't find any initial key, but there's a placeholder for the empty key at the
		// end that we've skipped, then the initial key should be the empty one (the final one).
		if emptyKeyValue != nil && ck == nil && (end == nil || len(end) > 0) {
			ck = []byte{}
			cv = emptyKeyValue
			emptyKeyValue = nil // ensure call to Next() skips this
		}
	} else {
		switch {
		case (start == nil || len(start) == 0) && emptyKeyValue != nil:
			ck = []byte{}
			cv = emptyKeyValue
		case (start == nil || len(start) == 0) && emptyKeyValue == nil:
			ck, cv = itr.First()
		default:
			ck, cv = itr.Seek(start)
		}
	}

	return &boltDBIterator{
		tx:            tx,
		itr:           itr,
		start:         start,
		end:           end,
		emptyKeyValue: emptyKeyValue,
		currentKey:    ck,
		currentValue:  cv,
		isReverse:     isReverse,
		isInvalid:     false,
	}
}

// Domain implements Iterator.
func (itr *boltDBIterator) Domain() ([]byte, []byte) {
	return itr.start, itr.end
}

// Valid implements Iterator.
func (itr *boltDBIterator) Valid() bool {
	if itr.isInvalid {
		return false
	}

	// iterated to the end of the cursor
	if itr.currentKey == nil {
		itr.isInvalid = true
		return false
	}

	if itr.isReverse {
		if itr.start != nil && bytes.Compare(itr.currentKey, itr.start) < 0 {
			itr.isInvalid = true
			return false
		}
	} else {
		if itr.end != nil && bytes.Compare(itr.end, itr.currentKey) <= 0 {
			itr.isInvalid = true
			return false
		}
	}

	// Valid
	return true
}

// Next implements Iterator.
func (itr *boltDBIterator) Next() {
	itr.assertIsValid()
	if itr.isReverse {
		itr.currentKey, itr.currentValue = itr.itr.Prev()
		if itr.emptyKeyValue != nil && itr.currentKey == nil {
			// If we reached the end, but there exists an empty key whose placeholder we skipped,
			// we should set up the empty key and its value as the final pair.
			itr.currentKey = []byte{}
			itr.currentValue = itr.emptyKeyValue
			itr.emptyKeyValue = nil // This ensures the next call to Next() terminates
		}
	} else {
		if len(itr.currentKey) == 0 {
			// If the first key was the empty key, then we need to move to the first non-empty key
			itr.currentKey, itr.currentValue = itr.itr.First()
		} else {
			itr.currentKey, itr.currentValue = itr.itr.Next()
		}
	}
	// If we encounter the placeholder for the empty key, skip it
	if itr.emptyKeyValue != nil && bytes.Equal(itr.currentKey, boltDBEmptyKey) {
		itr.Next()
	}
}

// Key implements Iterator.
func (itr *boltDBIterator) Key() []byte {
	itr.assertIsValid()
	return append([]byte{}, itr.currentKey...)
}

// Value implements Iterator.
func (itr *boltDBIterator) Value() []byte {
	itr.assertIsValid()
	var value []byte
	if itr.currentValue != nil {
		value = append([]byte{}, itr.currentValue...)
	}
	return value
}

// Error implements Iterator.
func (itr *boltDBIterator) Error() error {
	return nil
}

// Close implements Iterator.
func (itr *boltDBIterator) Close() {
	err := itr.tx.Rollback()
	if err != nil {
		panic(err)
	}
}

func (itr *boltDBIterator) assertIsValid() {
	if !itr.Valid() {
		panic("boltdb-iterator is invalid")
	}
}
