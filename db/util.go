package db

import "bytes"

// A wrapper around itr that tries to keep the iterator
// within the bounds as defined by `prefix`
type prefixIterator struct {
	itr     Iterator
	prefix  []byte
	invalid bool
}

func (pi *prefixIterator) Seek(key []byte) {
	if !bytes.HasPrefix(key, pi.prefix) {
		pi.invalid = true
		return
	}
	pi.itr.Seek(key)
	pi.checkInvalid()
}

func (pi *prefixIterator) checkInvalid() {
	if !pi.itr.Valid() {
		pi.invalid = true
	}
}

func (pi *prefixIterator) Valid() bool {
	if pi.invalid {
		return false
	}
	key := pi.itr.Key()
	ok := bytes.HasPrefix(key, pi.prefix)
	if !ok {
		pi.invalid = true
		return false
	}
	return true
}

func (pi *prefixIterator) Next() {
	if pi.invalid {
		panic("prefixIterator Next() called when invalid")
	}
	pi.itr.Next()
	pi.checkInvalid()
}

func (pi *prefixIterator) Prev() {
	if pi.invalid {
		panic("prefixIterator Prev() called when invalid")
	}
	pi.itr.Prev()
	pi.checkInvalid()
}

func (pi *prefixIterator) Key() []byte {
	if pi.invalid {
		panic("prefixIterator Key() called when invalid")
	}
	return pi.itr.Key()
}

func (pi *prefixIterator) Value() []byte {
	if pi.invalid {
		panic("prefixIterator Value() called when invalid")
	}
	return pi.itr.Value()
}

func (pi *prefixIterator) Close()          { pi.itr.Close() }
func (pi *prefixIterator) GetError() error { return pi.itr.GetError() }

func IteratePrefix(db DB, prefix []byte) Iterator {
	itr := db.Iterator()
	pi := &prefixIterator{
		itr:    itr,
		prefix: prefix,
	}
	pi.Seek(prefix)
	return pi
}
