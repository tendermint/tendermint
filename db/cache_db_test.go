package db

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func bz(s string) []byte { return []byte(s) }

func TestCacheDB(t *testing.T) {
	mem := NewMemDB()
	cdb := mem.CacheWrap().(*CacheDB)

	require.Empty(t, cdb.Get(bz("key1")), "Expected `key1` to be empty")

	mem.Set(bz("key1"), bz("value1"))
	cdb.Set(bz("key1"), bz("value1"))
	require.Equal(t, bz("value1"), cdb.Get(bz("key1")))

	cdb.Set(bz("key1"), bz("value2"))
	require.Equal(t, bz("value2"), cdb.Get(bz("key1")))
	require.Equal(t, bz("value1"), mem.Get(bz("key1")))

	cdb.Write()
	require.Equal(t, bz("value2"), mem.Get(bz("key1")))

	require.Panics(t, func() { cdb.Write() }, "Expected second cdb.Write() to fail")

	cdb = mem.CacheWrap().(*CacheDB)
	cdb.Delete(bz("key1"))
	require.Empty(t, cdb.Get(bz("key1")))
	require.Equal(t, mem.Get(bz("key1")), bz("value2"))

	cdb.Write()
	require.Empty(t, cdb.Get(bz("key1")), "Expected `key1` to be empty")
	require.Empty(t, mem.Get(bz("key1")), "Expected `key1` to be empty")
}

func TestCacheDBWriteLock(t *testing.T) {
	mem := NewMemDB()
	cdb := mem.CacheWrap().(*CacheDB)
	require.NotPanics(t, func() { cdb.Write() })
	require.Panics(t, func() { cdb.Write() })
	cdb = mem.CacheWrap().(*CacheDB)
	require.NotPanics(t, func() { cdb.Write() })
	require.Panics(t, func() { cdb.Write() })
}

func TestCacheDBWriteLockNested(t *testing.T) {
	mem := NewMemDB()
	cdb := mem.CacheWrap().(*CacheDB)
	cdb2 := cdb.CacheWrap().(*CacheDB)
	require.NotPanics(t, func() { cdb2.Write() })
	require.Panics(t, func() { cdb2.Write() })
	cdb2 = cdb.CacheWrap().(*CacheDB)
	require.NotPanics(t, func() { cdb2.Write() })
	require.Panics(t, func() { cdb2.Write() })
}

func TestCacheDBNested(t *testing.T) {
	mem := NewMemDB()
	cdb := mem.CacheWrap().(*CacheDB)
	cdb.Set(bz("key1"), bz("value1"))

	require.Empty(t, mem.Get(bz("key1")))
	require.Equal(t, bz("value1"), cdb.Get(bz("key1")))
	cdb2 := cdb.CacheWrap().(*CacheDB)
	require.Equal(t, bz("value1"), cdb2.Get(bz("key1")))

	cdb2.Set(bz("key1"), bz("VALUE2"))
	require.Equal(t, []byte(nil), mem.Get(bz("key1")))
	require.Equal(t, bz("value1"), cdb.Get(bz("key1")))
	require.Equal(t, bz("VALUE2"), cdb2.Get(bz("key1")))

	cdb2.Write()
	require.Equal(t, []byte(nil), mem.Get(bz("key1")))
	require.Equal(t, bz("VALUE2"), cdb.Get(bz("key1")))

	cdb.Write()
	require.Equal(t, bz("VALUE2"), mem.Get(bz("key1")))

}
