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
		assert.Equal(t, "3dafc06a52039d029be57c75c9d16356a4256ef4", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := NewSimpleMap()
		db.Set("key1", strHasher("value2"))
		assert.Equal(t, "03eb5cfdff646bc4e80fec844e72fd248a1c6b2c", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := NewSimpleMap()
		db.Set("key1", strHasher("value1"))
		db.Set("key2", strHasher("value2"))
		assert.Equal(t, "acc3971eab8513171cc90ce8b74f368c38f9657d", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := NewSimpleMap()
		db.Set("key2", strHasher("value2")) // NOTE: out of order
		db.Set("key1", strHasher("value1"))
		assert.Equal(t, "acc3971eab8513171cc90ce8b74f368c38f9657d", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := NewSimpleMap()
		db.Set("key1", strHasher("value1"))
		db.Set("key2", strHasher("value2"))
		db.Set("key3", strHasher("value3"))
		assert.Equal(t, "0cd117ad14e6cd22edcd9aa0d84d7063b54b862f", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := NewSimpleMap()
		db.Set("key2", strHasher("value2")) // NOTE: out of order
		db.Set("key1", strHasher("value1"))
		db.Set("key3", strHasher("value3"))
		assert.Equal(t, "0cd117ad14e6cd22edcd9aa0d84d7063b54b862f", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
}
