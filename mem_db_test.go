package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMemDB_Iterator(t *testing.T) {
	db := NewMemDB()
	defer db.Close()

	// if db is empty, iterator is invalid
	itr, err := db.Iterator(nil, nil)
	assert.NoError(t, err)
	defer itr.Close()
	assert.False(t, itr.Valid())

	db.Set([]byte("foo"), []byte("bar"))

	// single iteration
	itr, err = db.Iterator(nil, nil)
	assert.NoError(t, err)
	defer itr.Close()
	key, err := itr.Key()
	assert.NoError(t, err)
	assert.True(t, itr.Valid())
	assert.Equal(t, []byte("foo"), key)
	value, err := itr.Value()
	assert.NoError(t, err)
	assert.Equal(t, []byte("bar"), value)
	itr.Next()
	assert.False(t, itr.Valid())
}
