package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMemDB_Iterator(t *testing.T) {
	db := NewMemDB()
	defer db.Close()

	// if db is empty, iterator is invalid
	itr := db.Iterator(nil, nil)
	defer itr.Close()
	assert.False(t, itr.Valid())

	db.Set([]byte("foo"), []byte("bar"))

	// single iteration
	itr = db.Iterator(nil, nil)
	defer itr.Close()
	assert.True(t, itr.Valid())
	assert.Equal(t, []byte("foo"), itr.Key())
	assert.Equal(t, []byte("bar"), itr.Value())
	itr.Next()
	assert.False(t, itr.Valid())
}
