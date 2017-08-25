package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
