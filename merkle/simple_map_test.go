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
		assert.Equal(t, "376bf717ebe3659a34f68edb833dfdcf4a2d3c10", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := NewSimpleMap()
		db.Set("key1", "value2")
		assert.Equal(t, "72fd3a7224674377952214cb10ef21753ec803eb", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := NewSimpleMap()
		db.Set("key1", "value1")
		db.Set("key2", "value2")
		assert.Equal(t, "23a160bd4eea5b2fcc0755d722f9112a15999abc", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := NewSimpleMap()
		db.Set("key2", "value2") // NOTE: out of order
		db.Set("key1", "value1")
		assert.Equal(t, "23a160bd4eea5b2fcc0755d722f9112a15999abc", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := NewSimpleMap()
		db.Set("key1", "value1")
		db.Set("key2", "value2")
		db.Set("key3", "value3")
		assert.Equal(t, "40df7416429148d03544cfafa86e1080615cd2bc", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := NewSimpleMap()
		db.Set("key2", "value2") // NOTE: out of order
		db.Set("key1", "value1")
		db.Set("key3", "value3")
		assert.Equal(t, "40df7416429148d03544cfafa86e1080615cd2bc", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
}
