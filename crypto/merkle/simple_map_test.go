package merkle

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tendermint/tendermint/crypto/tmhash"
)

type strHasher string

func (str strHasher) Hash() []byte {
	return tmhash.Sum([]byte(str))
}

func TestSimpleMap(t *testing.T) {
	{
		db := newSimpleMap()
		db.Set("key1", strHasher("value1"))
		assert.Equal(t, "5e0c721faaa2e92bdb37e7d7297d6affb3bac87d", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := newSimpleMap()
		db.Set("key1", strHasher("value2"))
		assert.Equal(t, "968fdd7e6e94448b0ab5d38cd9b3618b860e2443", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := newSimpleMap()
		db.Set("key1", strHasher("value1"))
		db.Set("key2", strHasher("value2"))
		assert.Equal(t, "7dc11c8e1bc6fe0aaa0d691560c25c3f33940e14", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := newSimpleMap()
		db.Set("key2", strHasher("value2")) // NOTE: out of order
		db.Set("key1", strHasher("value1"))
		assert.Equal(t, "7dc11c8e1bc6fe0aaa0d691560c25c3f33940e14", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := newSimpleMap()
		db.Set("key1", strHasher("value1"))
		db.Set("key2", strHasher("value2"))
		db.Set("key3", strHasher("value3"))
		assert.Equal(t, "2584d8b484db08cd807b931e0b95f64a9105d41d", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
	{
		db := newSimpleMap()
		db.Set("key2", strHasher("value2")) // NOTE: out of order
		db.Set("key1", strHasher("value1"))
		db.Set("key3", strHasher("value3"))
		assert.Equal(t, "2584d8b484db08cd807b931e0b95f64a9105d41d", fmt.Sprintf("%x", db.Hash()), "Hash didn't match")
	}
}
