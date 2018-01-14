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
		assert.Equal(t, "d7df3e1d47fe38b51f8d897a88828026807a86b6", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := NewSimpleMap()
		db.Set("key1", "value2")
		assert.Equal(t, "db415336c9be129ac38259b935a49d8e9c248c88", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := NewSimpleMap()
		db.Set("key1", "value1")
		db.Set("key2", "value2")
		assert.Equal(t, "fdb900a04c1de42bd3d924fc644e28a4bdce30ce", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := NewSimpleMap()
		db.Set("key2", "value2") // NOTE: out of order
		db.Set("key1", "value1")
		assert.Equal(t, "fdb900a04c1de42bd3d924fc644e28a4bdce30ce", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := NewSimpleMap()
		db.Set("key1", "value1")
		db.Set("key2", "value2")
		db.Set("key3", "value3")
		assert.Equal(t, "488cfdaea108ef8bd406f6163555752392ae1b4a", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := NewSimpleMap()
		db.Set("key2", "value2") // NOTE: out of order
		db.Set("key1", "value1")
		db.Set("key3", "value3")
		assert.Equal(t, "488cfdaea108ef8bd406f6163555752392ae1b4a", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
}
