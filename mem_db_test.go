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

	err = db.Set([]byte("foo"), []byte("bar"))
	assert.NoError(t, err)

	// single iteration
	itr, err = db.Iterator(nil, nil)
	assert.NoError(t, err)
	defer itr.Close()
	key := itr.Key()
	assert.True(t, itr.Valid())
	assert.Equal(t, []byte("foo"), key)

	value := itr.Value()
	assert.Equal(t, []byte("bar"), value)
	itr.Next()
	assert.False(t, itr.Valid())
}

func BenchmarkMemDBRangeScans1M(b *testing.B) {
	db := NewMemDB()
	defer db.Close()

	benchmarkRangeScans(b, db, int64(1e6))
}

func BenchmarkMemDBRangeScans10M(b *testing.B) {
	db := NewMemDB()
	defer db.Close()

	benchmarkRangeScans(b, db, int64(10e6))
}

func BenchmarkMemDBRandomReadsWrites(b *testing.B) {
	db := NewMemDB()
	defer db.Close()

	benchmarkRandomReadsWrites(b, db)
}
