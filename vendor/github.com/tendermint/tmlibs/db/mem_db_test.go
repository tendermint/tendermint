package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemDbIterator(t *testing.T) {
	db := NewMemDB()
	keys := make([][]byte, 100)
	for i := 0; i < 100; i++ {
		keys[i] = []byte{byte(i)}
	}

	value := []byte{5}
	for _, k := range keys {
		db.Set(k, value)
	}

	iter := db.Iterator()
	i := 0
	for iter.Next() {
		assert.Equal(t, db.Get(iter.Key()), iter.Value(), "values dont match for key")
		i += 1
	}
	assert.Equal(t, i, len(db.db), "iterator didnt cover whole db")
}

func TestMemDBClose(t *testing.T) {
	db := NewMemDB()
	copyDB := func(orig map[string][]byte) map[string][]byte {
		copy := make(map[string][]byte)
		for k, v := range orig {
			copy[k] = v
		}
		return copy
	}
	k, v := []byte("foo"), []byte("bar")
	db.Set(k, v)
	require.Equal(t, db.Get(k), v, "expecting a successful get")
	copyBefore := copyDB(db.db)
	db.Close()
	require.Equal(t, db.Get(k), v, "Close is a noop, expecting a successful get")
	copyAfter := copyDB(db.db)
	require.Equal(t, copyBefore, copyAfter, "Close is a noop and shouldn't modify any internal data")
}
