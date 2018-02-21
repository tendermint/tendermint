package db

import (
	"bytes"
)

func IteratePrefix(db DB, prefix []byte) Iterator {
	var start, end []byte
	if len(prefix) == 0 {
		start = nil
		end = nil
	} else {
		start = cp(prefix)
		end = cpIncr(prefix)
	}
	return db.Iterator(start, end)
}

//----------------------------------------

func cp(bz []byte) (ret []byte) {
	ret = make([]byte, len(bz))
	copy(ret, bz)
	return ret
}

// CONTRACT: len(bz) > 0
func cpIncr(bz []byte) (ret []byte) {
	ret = cp(bz)
	for i := len(bz) - 1; i >= 0; i-- {
		if ret[i] < byte(0xFF) {
			ret[i] += 1
			return
		} else {
			ret[i] = byte(0x00)
		}
	}
	return nil
}

// See DB interface documentation for more information.
func IsKeyInDomain(key, start, end []byte, isReverse bool) bool {
	if !isReverse {
		if bytes.Compare(key, start) < 0 {
			return false
		}
		if end != nil && bytes.Compare(end, key) <= 0 {
			return false
		}
		return true
	} else {
		if start != nil && bytes.Compare(start, key) < 0 {
			return false
		}
		if end != nil && bytes.Compare(key, end) <= 0 {
			return false
		}
		return true
	}
}
