package db

import (
	"bytes"
	"strings"
)

func IteratePrefix(db DB, prefix []byte) Iterator {
	var start, end []byte
	if len(prefix) == 0 {
		start = BeginningKey()
		end = EndingKey()
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
	return EndingKey()
}

func checkKeyCondition(key string, start, end []byte) bool {
	leftCondition := bytes.Equal(start, BeginningKey()) || strings.Compare(key, string(start)) >= 0
	rightCondition := bytes.Equal(end, EndingKey()) || strings.Compare(key, string(end)) < 0
	return leftCondition && rightCondition
}
