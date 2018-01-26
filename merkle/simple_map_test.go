package merkle

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type strHasher string

func (str strHasher) Hash() []byte {
	return SimpleHashFromBytes([]byte(str))
}

func TestSimpleMap(t *testing.T) {
	{
		db := NewSimpleMap()
		db.Set("key1", strHasher("value1"))
		assert.Equal(t, "19618304d1ad2635c4238bce87f72331b22a11a1", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := NewSimpleMap()
		db.Set("key1", strHasher("value2"))
		assert.Equal(t, "51cb96d3d41e1714def72eb4bacc211de9ddf284", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := NewSimpleMap()
		db.Set("key1", strHasher("value1"))
		db.Set("key2", strHasher("value2"))
		assert.Equal(t, "58a0a99d5019fdcad4bcf55942e833b2dfab9421", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := NewSimpleMap()
		db.Set("key2", strHasher("value2")) // NOTE: out of order
		db.Set("key1", strHasher("value1"))
		assert.Equal(t, "58a0a99d5019fdcad4bcf55942e833b2dfab9421", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := NewSimpleMap()
		db.Set("key1", strHasher("value1"))
		db.Set("key2", strHasher("value2"))
		db.Set("key3", strHasher("value3"))
		assert.Equal(t, "cb56db3c7993e977f4c2789559ae3e5e468a6e9b", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := NewSimpleMap()
		db.Set("key2", strHasher("value2")) // NOTE: out of order
		db.Set("key1", strHasher("value1"))
		db.Set("key3", strHasher("value3"))
		assert.Equal(t, "cb56db3c7993e977f4c2789559ae3e5e468a6e9b", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
}
