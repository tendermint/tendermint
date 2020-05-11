package db

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Register a test backend for PrefixDB as well, with some unrelated junk data
func init() {
	// nolint: errcheck
	registerDBCreator("prefixdb", func(name, dir string) (DB, error) {
		mdb := NewMemDB()
		mdb.Set([]byte("a"), []byte{1})
		mdb.Set([]byte("b"), []byte{2})
		mdb.Set([]byte("t"), []byte{20})
		mdb.Set([]byte("test"), []byte{0})
		mdb.Set([]byte("u"), []byte{21})
		mdb.Set([]byte("z"), []byte{26})
		return NewPrefixDB(mdb, []byte("test/")), nil
	}, false)
}

func cleanupDBDir(dir, name string) {
	err := os.RemoveAll(filepath.Join(dir, name) + ".db")
	if err != nil {
		panic(err)
	}
}

func testBackendGetSetDelete(t *testing.T, backend BackendType) {
	// Default
	dirname, err := ioutil.TempDir("", fmt.Sprintf("test_backend_%s_", backend))
	require.Nil(t, err)
	db := NewDB("testdb", backend, dirname)
	defer cleanupDBDir(dirname, "testdb")

	// A nonexistent key should return nil, even if the key is empty
	item, err := db.Get([]byte(""))
	require.NoError(t, err)
	require.Nil(t, item)

	// A nonexistent key should return nil, even if the key is nil
	value, err := db.Get(nil)
	require.NoError(t, err)
	require.Nil(t, value)

	// A nonexistent key should return nil.
	key := []byte("abc")
	value, err = db.Get(key)
	require.NoError(t, err)
	require.Nil(t, value)

	// Set empty value.
	err = db.Set(key, []byte(""))
	require.NoError(t, err)

	value, err = db.Get(key)
	require.NoError(t, err)
	require.NotNil(t, value)
	require.Empty(t, value)

	// Set nil value.
	err = db.Set(key, nil)
	require.NoError(t, err)

	value, err = db.Get(key)
	require.NoError(t, err)
	require.NotNil(t, value)
	require.Empty(t, value)

	// Delete.
	err = db.Delete(key)
	require.NoError(t, err)

	value, err = db.Get(key)
	require.NoError(t, err)
	require.Nil(t, value)

	// Delete missing key.
	err = db.Delete([]byte{9})
	require.NoError(t, err)
}

func TestBackendsGetSetDelete(t *testing.T) {
	for dbType := range backends {
		testBackendGetSetDelete(t, dbType)
	}
}

func withDB(t *testing.T, creator dbCreator, fn func(DB)) {
	name := fmt.Sprintf("test_%x", randStr(12))
	dir := os.TempDir()
	db, err := creator(name, dir)
	require.Nil(t, err)
	defer cleanupDBDir(dir, name)
	fn(db)
	db.Close()
}

func TestBackendsNilKeys(t *testing.T) {

	// Test all backends.
	for dbType, creator := range backends {
		withDB(t, creator, func(db DB) {
			t.Run(fmt.Sprintf("Testing %s", dbType), func(t *testing.T) {
				expect := func(key, value []byte) {
					if len(key) == 0 { // nil or empty
						nilValue, err := db.Get(nil)
						assert.NoError(t, err)
						byteValue, err := db.Get([]byte(""))
						assert.NoError(t, err)
						assert.Equal(t, nilValue, byteValue)
						hasNil, err := db.Has(nil)
						assert.NoError(t, err)
						hasStr, err := db.Has([]byte(""))
						assert.NoError(t, err)
						assert.Equal(t, hasNil, hasStr)
					}
					value2, err := db.Get(key)
					assert.Equal(t, value2, value)
					assert.NoError(t, err)
					hasKey, err := db.Has(key)
					assert.NoError(t, err)
					assert.Equal(t, hasKey, value != nil)
				}

				// Not set
				expect(nil, nil)

				// Set nil value
				err := db.Set(nil, nil)
				require.NoError(t, err)
				expect(nil, []byte(""))

				// Set empty value
				err = db.Set(nil, []byte(""))
				require.NoError(t, err)
				expect(nil, []byte(""))

				// Set nil, Delete nil
				err = db.Set(nil, []byte("abc"))
				expect(nil, []byte("abc"))
				require.NoError(t, err)
				err = db.Delete(nil)
				require.NoError(t, err)
				expect(nil, nil)

				// Set nil, Delete empty
				err = db.Set(nil, []byte("abc"))
				expect(nil, []byte("abc"))
				require.NoError(t, err)
				err = db.Delete([]byte(""))
				require.NoError(t, err)
				expect(nil, nil)

				// Set empty, Delete nil
				err = db.Set([]byte(""), []byte("abc"))
				expect(nil, []byte("abc"))
				require.NoError(t, err)
				err = db.Delete(nil)
				require.NoError(t, err)
				expect(nil, nil)

				// Set empty, Delete empty
				err = db.Set([]byte(""), []byte("abc"))
				require.NoError(t, err)
				expect(nil, []byte("abc"))

				err = db.Delete([]byte(""))
				require.NoError(t, err)
				expect(nil, nil)

				// SetSync nil, DeleteSync nil
				err = db.SetSync(nil, []byte("abc"))
				require.NoError(t, err)
				expect(nil, []byte("abc"))
				err = db.DeleteSync(nil)
				require.NoError(t, err)
				expect(nil, nil)

				// SetSync nil, DeleteSync empty
				err = db.SetSync(nil, []byte("abc"))
				require.NoError(t, err)
				err = db.DeleteSync([]byte(""))
				require.NoError(t, err)

				// SetSync empty, DeleteSync nil
				err = db.SetSync([]byte(""), []byte("abc"))
				require.NoError(t, err)
				expect(nil, []byte("abc"))
				err = db.DeleteSync(nil)
				require.NoError(t, err)
				expect(nil, nil)

				// SetSync empty, DeleteSync empty
				err = db.SetSync([]byte(""), []byte("abc"))
				require.NoError(t, err)
				expect(nil, []byte("abc"))
				err = db.DeleteSync([]byte(""))
				require.NoError(t, err)
				expect(nil, nil)
			})
		})
	}
}

func TestGoLevelDBBackend(t *testing.T) {
	name := fmt.Sprintf("test_%x", randStr(12))
	db := NewDB(name, GoLevelDBBackend, "")
	defer cleanupDBDir("", name)

	_, ok := db.(*GoLevelDB)
	assert.True(t, ok)
}

func TestDBIterator(t *testing.T) {
	for dbType := range backends {
		t.Run(fmt.Sprintf("%v", dbType), func(t *testing.T) {
			testDBIterator(t, dbType)
			testDBIteratorBlankKey(t, dbType)
		})
	}
}

func testDBIterator(t *testing.T, backend BackendType) {
	name := fmt.Sprintf("test_%x", randStr(12))
	dir := os.TempDir()
	db := NewDB(name, backend, dir)
	defer cleanupDBDir(dir, name)

	for i := 0; i < 10; i++ {
		if i != 6 { // but skip 6.
			err := db.Set(int642Bytes(int64(i)), nil)
			require.NoError(t, err)
		}
	}

	itr, err := db.Iterator(nil, nil)
	require.NoError(t, err)
	verifyIterator(t, itr, []int64{0, 1, 2, 3, 4, 5, 7, 8, 9}, "forward iterator")

	ritr, err := db.ReverseIterator(nil, nil)
	require.NoError(t, err)
	verifyIterator(t, ritr, []int64{9, 8, 7, 5, 4, 3, 2, 1, 0}, "reverse iterator")

	itr, err = db.Iterator(nil, int642Bytes(0))
	require.NoError(t, err)
	verifyIterator(t, itr, []int64(nil), "forward iterator to 0")

	ritr, err = db.ReverseIterator(int642Bytes(10), nil)
	require.NoError(t, err)
	verifyIterator(t, ritr, []int64(nil), "reverse iterator from 10 (ex)")

	itr, err = db.Iterator(int642Bytes(0), nil)
	require.NoError(t, err)
	verifyIterator(t, itr, []int64{0, 1, 2, 3, 4, 5, 7, 8, 9}, "forward iterator from 0")

	itr, err = db.Iterator(int642Bytes(1), nil)
	require.NoError(t, err)
	verifyIterator(t, itr, []int64{1, 2, 3, 4, 5, 7, 8, 9}, "forward iterator from 1")

	ritr, err = db.ReverseIterator(nil, int642Bytes(10))
	require.NoError(t, err)
	verifyIterator(t, ritr,
		[]int64{9, 8, 7, 5, 4, 3, 2, 1, 0}, "reverse iterator from 10 (ex)")

	ritr, err = db.ReverseIterator(nil, int642Bytes(9))
	require.NoError(t, err)
	verifyIterator(t, ritr,
		[]int64{8, 7, 5, 4, 3, 2, 1, 0}, "reverse iterator from 9 (ex)")

	ritr, err = db.ReverseIterator(nil, int642Bytes(8))
	require.NoError(t, err)
	verifyIterator(t, ritr,
		[]int64{7, 5, 4, 3, 2, 1, 0}, "reverse iterator from 8 (ex)")

	itr, err = db.Iterator(int642Bytes(5), int642Bytes(6))
	require.NoError(t, err)
	verifyIterator(t, itr, []int64{5}, "forward iterator from 5 to 6")

	itr, err = db.Iterator(int642Bytes(5), int642Bytes(7))
	require.NoError(t, err)
	verifyIterator(t, itr, []int64{5}, "forward iterator from 5 to 7")

	itr, err = db.Iterator(int642Bytes(5), int642Bytes(8))
	require.NoError(t, err)
	verifyIterator(t, itr, []int64{5, 7}, "forward iterator from 5 to 8")

	itr, err = db.Iterator(int642Bytes(6), int642Bytes(7))
	require.NoError(t, err)
	verifyIterator(t, itr, []int64(nil), "forward iterator from 6 to 7")

	itr, err = db.Iterator(int642Bytes(6), int642Bytes(8))
	require.NoError(t, err)
	verifyIterator(t, itr, []int64{7}, "forward iterator from 6 to 8")

	itr, err = db.Iterator(int642Bytes(7), int642Bytes(8))
	require.NoError(t, err)
	verifyIterator(t, itr, []int64{7}, "forward iterator from 7 to 8")

	ritr, err = db.ReverseIterator(int642Bytes(4), int642Bytes(5))
	require.NoError(t, err)
	verifyIterator(t, ritr, []int64{4}, "reverse iterator from 5 (ex) to 4")

	ritr, err = db.ReverseIterator(int642Bytes(4), int642Bytes(6))
	require.NoError(t, err)
	verifyIterator(t, ritr,
		[]int64{5, 4}, "reverse iterator from 6 (ex) to 4")

	ritr, err = db.ReverseIterator(int642Bytes(4), int642Bytes(7))
	require.NoError(t, err)
	verifyIterator(t, ritr,
		[]int64{5, 4}, "reverse iterator from 7 (ex) to 4")

	ritr, err = db.ReverseIterator(int642Bytes(5), int642Bytes(6))
	require.NoError(t, err)
	verifyIterator(t, ritr, []int64{5}, "reverse iterator from 6 (ex) to 5")

	ritr, err = db.ReverseIterator(int642Bytes(5), int642Bytes(7))
	require.NoError(t, err)
	verifyIterator(t, ritr, []int64{5}, "reverse iterator from 7 (ex) to 5")

	ritr, err = db.ReverseIterator(int642Bytes(6), int642Bytes(7))
	require.NoError(t, err)
	verifyIterator(t, ritr,
		[]int64(nil), "reverse iterator from 7 (ex) to 6")

	ritr, err = db.ReverseIterator(int642Bytes(10), nil)
	require.NoError(t, err)
	verifyIterator(t, ritr, []int64(nil), "reverse iterator to 10")

	ritr, err = db.ReverseIterator(int642Bytes(6), nil)
	require.NoError(t, err)
	verifyIterator(t, ritr, []int64{9, 8, 7}, "reverse iterator to 6")

	ritr, err = db.ReverseIterator(int642Bytes(5), nil)
	require.NoError(t, err)
	verifyIterator(t, ritr, []int64{9, 8, 7, 5}, "reverse iterator to 5")

	// verifyIterator(t, db.Iterator(int642Bytes(0), int642Bytes(1)), []int64{0}, "forward iterator from 0 to 1")

	ritr, err = db.ReverseIterator(int642Bytes(8), int642Bytes(9))
	require.NoError(t, err)
	verifyIterator(t, ritr, []int64{8}, "reverse iterator from 9 (ex) to 8")

	// verifyIterator(t, db.Iterator(int642Bytes(2), int642Bytes(4)), []int64{2, 3}, "forward iterator from 2 to 4")
	// verifyIterator(t, db.Iterator(int642Bytes(4), int642Bytes(2)), []int64(nil), "forward iterator from 4 to 2")

	ritr, err = db.ReverseIterator(int642Bytes(2), int642Bytes(4))
	require.NoError(t, err)
	verifyIterator(t, ritr,
		[]int64{3, 2}, "reverse iterator from 4 (ex) to 2")

	ritr, err = db.ReverseIterator(int642Bytes(4), int642Bytes(2))
	require.NoError(t, err)
	verifyIterator(t, ritr,
		[]int64(nil), "reverse iterator from 2 (ex) to 4")
}

func testDBIteratorBlankKey(t *testing.T, backend BackendType) {
	name := fmt.Sprintf("test_%x", randStr(12))
	dir := os.TempDir()
	db := NewDB(name, backend, dir)
	defer cleanupDBDir(dir, name)

	err := db.Set([]byte(""), []byte{0})
	require.NoError(t, err)
	err = db.Set([]byte("a"), []byte{1})
	require.NoError(t, err)
	err = db.Set([]byte("b"), []byte{2})
	require.NoError(t, err)

	value, err := db.Get([]byte(""))
	require.NoError(t, err)
	assert.Equal(t, []byte{0}, value)

	i, err := db.Iterator(nil, nil)
	require.NoError(t, err)
	verifyIteratorStrings(t, i, []string{"", "a", "b"}, "forward")

	i, err = db.Iterator([]byte(""), nil)
	require.NoError(t, err)
	verifyIteratorStrings(t, i, []string{"", "a", "b"}, "forward from blank")

	i, err = db.Iterator([]byte("a"), nil)
	require.NoError(t, err)
	verifyIteratorStrings(t, i, []string{"a", "b"}, "forward from a")

	i, err = db.Iterator([]byte(""), []byte("b"))
	require.NoError(t, err)
	verifyIteratorStrings(t, i, []string{"", "a"}, "forward from blank to b")

	i, err = db.ReverseIterator(nil, nil)
	require.NoError(t, err)
	verifyIteratorStrings(t, i, []string{"b", "a", ""}, "reverse")

	i, err = db.ReverseIterator([]byte(""), nil)
	require.NoError(t, err)
	verifyIteratorStrings(t, i, []string{"b", "a", ""}, "reverse to blank")

	i, err = db.ReverseIterator([]byte(""), []byte("a"))
	require.NoError(t, err)
	verifyIteratorStrings(t, i, []string{""}, "reverse to blank from a")

	i, err = db.ReverseIterator([]byte("a"), nil)
	require.NoError(t, err)
	verifyIteratorStrings(t, i, []string{"b", "a"}, "reverse to a")
}

func verifyIterator(t *testing.T, itr Iterator, expected []int64, msg string) {
	var list []int64
	for itr.Valid() {
		key := itr.Key()
		list = append(list, bytes2Int64(key))
		itr.Next()
	}
	assert.Equal(t, expected, list, msg)
}

func verifyIteratorStrings(t *testing.T, itr Iterator, expected []string, msg string) {
	var list []string
	for itr.Valid() {
		key := itr.Key()
		list = append(list, string(key))
		itr.Next()
	}
	assert.Equal(t, expected, list, msg)
}

func TestDBBatch(t *testing.T) {
	for dbType := range backends {
		t.Run(fmt.Sprintf("%v", dbType), func(t *testing.T) {
			testDBBatch(t, dbType)
		})
	}
}

func testDBBatch(t *testing.T, backend BackendType) {
	name := fmt.Sprintf("test_%x", randStr(12))
	dir := os.TempDir()
	db := NewDB(name, backend, dir)
	defer cleanupDBDir(dir, name)

	// create a new batch, and some items - they should not be visible until we write
	batch := db.NewBatch()
	batch.Set([]byte("a"), []byte{1})
	batch.Set([]byte("b"), []byte{2})
	batch.Set([]byte("c"), []byte{3})
	assertKeyValues(t, db, map[string][]byte{})

	err := batch.Write()
	require.NoError(t, err)
	assertKeyValues(t, db, map[string][]byte{"a": {1}, "b": {2}, "c": {3}})

	// trying to modify or rewrite a written batch should panic, but closing it should work
	require.Panics(t, func() { batch.Set([]byte("a"), []byte{9}) })
	require.Panics(t, func() { batch.Delete([]byte("a")) })
	require.Panics(t, func() { batch.Write() })     // nolint: errcheck
	require.Panics(t, func() { batch.WriteSync() }) // nolint: errcheck
	batch.Close()

	// batches should write changes in order
	batch = db.NewBatch()
	batch.Delete([]byte("a"))
	batch.Set([]byte("a"), []byte{1})
	batch.Set([]byte("b"), []byte{1})
	batch.Set([]byte("b"), []byte{2})
	batch.Set([]byte("c"), []byte{3})
	batch.Delete([]byte("c"))
	err = batch.Write()
	require.NoError(t, err)
	batch.Close()
	assertKeyValues(t, db, map[string][]byte{"a": {1}, "b": {2}})

	// writing nil keys and values should be the same as empty keys and values
	// FIXME CLevelDB panics here: https://github.com/jmhodges/levigo/issues/55
	if backend != CLevelDBBackend {
		batch = db.NewBatch()
		batch.Set(nil, nil)
		err = batch.WriteSync()
		require.NoError(t, err)
		assertKeyValues(t, db, map[string][]byte{"": {}, "a": {1}, "b": {2}})

		batch = db.NewBatch()
		batch.Delete(nil)
		err = batch.Write()
		require.NoError(t, err)
		assertKeyValues(t, db, map[string][]byte{"a": {1}, "b": {2}})
	}

	// it should be possible to write an empty batch
	batch = db.NewBatch()
	err = batch.Write()
	require.NoError(t, err)
	assertKeyValues(t, db, map[string][]byte{"a": {1}, "b": {2}})

	// it should be possible to close an empty batch, and to re-close a closed batch
	batch = db.NewBatch()
	batch.Close()
	batch.Close()

	// all other operations on a closed batch should panic
	require.Panics(t, func() { batch.Set([]byte("a"), []byte{9}) })
	require.Panics(t, func() { batch.Delete([]byte("a")) })
	require.Panics(t, func() { batch.Write() })     // nolint: errcheck
	require.Panics(t, func() { batch.WriteSync() }) // nolint: errcheck
}

func assertKeyValues(t *testing.T, db DB, expect map[string][]byte) {
	iter, err := db.Iterator(nil, nil)
	require.NoError(t, err)
	defer iter.Close()

	actual := make(map[string][]byte)
	for ; iter.Valid(); iter.Next() {
		require.NoError(t, iter.Error())
		actual[string(iter.Key())] = iter.Value()
	}

	assert.Equal(t, expect, actual)
}
