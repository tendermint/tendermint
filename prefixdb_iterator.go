package db

import "bytes"

// IteratePrefix is a convenience function for iterating over a key domain
// restricted by prefix.
func IteratePrefix(db DB, prefix []byte) (Iterator, error) {
	var start, end []byte
	if len(prefix) == 0 {
		start = nil
		end = nil
	} else {
		start = cp(prefix)
		end = cpIncr(prefix)
	}
	itr, err := db.Iterator(start, end)
	if err != nil {
		return nil, err
	}
	return itr, nil
}

/*
TODO: Make test, maybe rename.
// Like IteratePrefix but the iterator strips the prefix from the keys.
func IteratePrefixStripped(db DB, prefix []byte) Iterator {
	start, end := ...
	return newPrefixIterator(prefix, start, end, IteratePrefix(db, prefix))
}
*/

// Strips prefix while iterating from Iterator.
type prefixDBIterator struct {
	prefix []byte
	start  []byte
	end    []byte
	source Iterator
	valid  bool
}

var _ Iterator = (*prefixDBIterator)(nil)

func newPrefixIterator(prefix, start, end []byte, source Iterator) (*prefixDBIterator, error) {
	pitrInvalid := &prefixDBIterator{
		prefix: prefix,
		start:  start,
		end:    end,
		source: source,
		valid:  false,
	}

	if !source.Valid() {
		return pitrInvalid, nil
	}
	key := source.Key()

	if !bytes.HasPrefix(key, prefix) {
		return pitrInvalid, nil
	}
	return &prefixDBIterator{
		prefix: prefix,
		start:  start,
		end:    end,
		source: source,
		valid:  true,
	}, nil
}

// Domain implements Iterator.
func (itr *prefixDBIterator) Domain() (start []byte, end []byte) {
	return itr.start, itr.end
}

// Valid implements Iterator.
func (itr *prefixDBIterator) Valid() bool {
	return itr.valid && itr.source.Valid()
}

// Next implements Iterator.
func (itr *prefixDBIterator) Next() {
	if !itr.valid {
		panic("prefixIterator invalid; cannot call Next()")
	}
	itr.source.Next()

	if !itr.source.Valid() || !bytes.HasPrefix(itr.source.Key(), itr.prefix) {
		itr.valid = false
	}
}

// Next implements Iterator.
func (itr *prefixDBIterator) Key() (key []byte) {
	if !itr.valid {
		panic("prefixIterator invalid; cannot call Key()")
	}
	key = itr.source.Key()
	return stripPrefix(key, itr.prefix)
}

// Value implements Iterator.
func (itr *prefixDBIterator) Value() (value []byte) {
	if !itr.valid {
		panic("prefixIterator invalid; cannot call Value()")
	}
	value = itr.source.Value()
	return value
}

// Error implements Iterator.
func (itr *prefixDBIterator) Error() error {
	return itr.source.Error()
}

// Close implements Iterator.
func (itr *prefixDBIterator) Close() {
	itr.source.Close()
}

func stripPrefix(key []byte, prefix []byte) (stripped []byte) {
	if len(key) < len(prefix) {
		panic("should not happen")
	}
	if !bytes.Equal(key[:len(prefix)], prefix) {
		panic("should not happen")
	}
	return key[len(prefix):]
}
