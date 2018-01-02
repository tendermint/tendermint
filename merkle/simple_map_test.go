package merkle

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSimpleMap(t *testing.T) {
	{
		db := NewSimpleMap()
		db.Set("key1", "value1")
		assert.Equal(t, "3bb53f017d2f5b4f144692aa829a5c245ac2b123", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := NewSimpleMap()
		db.Set("key1", "value2")
		assert.Equal(t, "14a68db29e3f930ffaafeff5e07c17a439384f39", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := NewSimpleMap()
		db.Set("key1", "value1")
		db.Set("key2", "value2")
		assert.Equal(t, "275c6367f4be335f9c482b6ef72e49c84e3f8bda", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := NewSimpleMap()
		db.Set("key2", "value2") // NOTE: out of order
		db.Set("key1", "value1")
		assert.Equal(t, "275c6367f4be335f9c482b6ef72e49c84e3f8bda", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := NewSimpleMap()
		db.Set("key1", "value1")
		db.Set("key2", "value2")
		db.Set("key3", "value3")
		assert.Equal(t, "48d60701cb4c96916f68a958b3368205ebe3809b", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := NewSimpleMap()
		db.Set("key2", "value2") // NOTE: out of order
		db.Set("key1", "value1")
		db.Set("key3", "value3")
		assert.Equal(t, "48d60701cb4c96916f68a958b3368205ebe3809b", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
}
