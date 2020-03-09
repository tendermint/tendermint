// +build boltdb

package db

import (
	"bytes"

	"github.com/etcd-io/bbolt"
)

// boltDBIterator allows you to iterate on range of keys/values given some
// start / end keys (nil & nil will result in doing full scan).
type boltDBIterator struct {
	tx *bbolt.Tx

	itr   *bbolt.Cursor
	start []byte
	end   []byte

	currentKey   []byte
	currentValue []byte

	isInvalid bool
	isReverse bool
}

var _ Iterator = (*boltDBIterator)(nil)

// newBoltDBIterator creates a new boltDBIterator.
func newBoltDBIterator(tx *bbolt.Tx, start, end []byte, isReverse bool) *boltDBIterator {
	itr := tx.Bucket(bucket).Cursor()

	var ck, cv []byte
	if isReverse {
		if end == nil {
			ck, cv = itr.Last()
		} else {
			_, _ = itr.Seek(end) // after key
			ck, cv = itr.Prev()  // return to end key
		}
	} else {
		if start == nil {
			ck, cv = itr.First()
		} else {
			ck, cv = itr.Seek(start)
		}
	}

	return &boltDBIterator{
		tx:           tx,
		itr:          itr,
		start:        start,
		end:          end,
		currentKey:   ck,
		currentValue: cv,
		isReverse:    isReverse,
		isInvalid:    false,
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
	if len(itr.currentKey) == 0 {
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
	} else {
		itr.currentKey, itr.currentValue = itr.itr.Next()
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
