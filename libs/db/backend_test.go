package db

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cmn "github.com/tendermint/tendermint/libs/common"
)

func cleanupDBDir(dir, name string) {
	err := os.RemoveAll(filepath.Join(dir, name) + ".db")
	if err != nil {
		panic(err)
	}
}

func testBackendGetSetDelete(t *testing.T, backend DBBackendType) {
	// Default
	dirname, err := ioutil.TempDir("", fmt.Sprintf("test_backend_%s_", backend))
	require.Nil(t, err)
	db := NewDB("testdb", backend, dirname)
	defer cleanupDBDir(dirname, "testdb")

	// A nonexistent key should return nil, even if the key is empty
	require.Nil(t, db.Get([]byte("")))

	// A nonexistent key should return nil, even if the key is nil
	require.Nil(t, db.Get(nil))

	// A nonexistent key should return nil.
	key := []byte("abc")
	require.Nil(t, db.Get(key))

	// Set empty value.
	db.Set(key, []byte(""))
	require.NotNil(t, db.Get(key))
	require.Empty(t, db.Get(key))

	// Set nil value.
	db.Set(key, nil)
	require.NotNil(t, db.Get(key))
	require.Empty(t, db.Get(key))

	// Delete.
	db.Delete(key)
	require.Nil(t, db.Get(key))
}

func TestBackendsGetSetDelete(t *testing.T) {
	for dbType := range backends {
		testBackendGetSetDelete(t, dbType)
	}
}

func withDB(t *testing.T, creator dbCreator, fn func(DB)) {
	name := fmt.Sprintf("test_%x", cmn.RandStr(12))
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

				// Nil keys are treated as the empty key for most operations.
				expect := func(key, value []byte) {
					if len(key) == 0 { // nil or empty
						assert.Equal(t, db.Get(nil), db.Get([]byte("")))
						assert.Equal(t, db.Has(nil), db.Has([]byte("")))
					}
					assert.Equal(t, db.Get(key), value)
					assert.Equal(t, db.Has(key), value != nil)
				}

				// Not set
				expect(nil, nil)

				// Set nil value
				db.Set(nil, nil)
				expect(nil, []byte(""))

				// Set empty value
				db.Set(nil, []byte(""))
				expect(nil, []byte(""))

				// Set nil, Delete nil
				db.Set(nil, []byte("abc"))
				expect(nil, []byte("abc"))
				db.Delete(nil)
				expect(nil, nil)

				// Set nil, Delete empty
				db.Set(nil, []byte("abc"))
				expect(nil, []byte("abc"))
				db.Delete([]byte(""))
				expect(nil, nil)

				// Set empty, Delete nil
				db.Set([]byte(""), []byte("abc"))
				expect(nil, []byte("abc"))
				db.Delete(nil)
				expect(nil, nil)

				// Set empty, Delete empty
				db.Set([]byte(""), []byte("abc"))
				expect(nil, []byte("abc"))
				db.Delete([]byte(""))
				expect(nil, nil)

				// SetSync nil, DeleteSync nil
				db.SetSync(nil, []byte("abc"))
				expect(nil, []byte("abc"))
				db.DeleteSync(nil)
				expect(nil, nil)

				// SetSync nil, DeleteSync empty
				db.SetSync(nil, []byte("abc"))
				expect(nil, []byte("abc"))
				db.DeleteSync([]byte(""))
				expect(nil, nil)

				// SetSync empty, DeleteSync nil
				db.SetSync([]byte(""), []byte("abc"))
				expect(nil, []byte("abc"))
				db.DeleteSync(nil)
				expect(nil, nil)

				// SetSync empty, DeleteSync empty
				db.SetSync([]byte(""), []byte("abc"))
				expect(nil, []byte("abc"))
				db.DeleteSync([]byte(""))
				expect(nil, nil)
			})
		})
	}
}

func TestGoLevelDBBackend(t *testing.T) {
	name := fmt.Sprintf("test_%x", cmn.RandStr(12))
	db := NewDB(name, GoLevelDBBackend, "")
	defer cleanupDBDir("", name)

	_, ok := db.(*GoLevelDB)
	assert.True(t, ok)
}

func TestDBIterator(t *testing.T) {
	for dbType := range backends {
		t.Run(fmt.Sprintf("%v", dbType), func(t *testing.T) {
			testDBIterator(t, dbType)
		})
	}
}

func testDBIterator(t *testing.T, backend DBBackendType) {
	name := fmt.Sprintf("test_%x", cmn.RandStr(12))
	dir := os.TempDir()
	db := NewDB(name, backend, dir)
	defer cleanupDBDir(dir, name)

	for i := 0; i < 10; i++ {
		if i != 6 { // but skip 6.
			db.Set(int642Bytes(int64(i)), nil)
		}
	}

	verifyIterator(t, db.Iterator(nil, nil), []int64{0, 1, 2, 3, 4, 5, 7, 8, 9}, "forward iterator")
	verifyIterator(t, db.ReverseIterator(nil, nil), []int64{9, 8, 7, 5, 4, 3, 2, 1, 0}, "reverse iterator")

	verifyIterator(t, db.Iterator(nil, int642Bytes(0)), []int64(nil), "forward iterator to 0")
	verifyIterator(t, db.ReverseIterator(int642Bytes(10), nil), []int64(nil), "reverse iterator from 10 (ex)")

	verifyIterator(t, db.Iterator(int642Bytes(0), nil), []int64{0, 1, 2, 3, 4, 5, 7, 8, 9}, "forward iterator from 0")
	verifyIterator(t, db.Iterator(int642Bytes(1), nil), []int64{1, 2, 3, 4, 5, 7, 8, 9}, "forward iterator from 1")
	verifyIterator(t, db.ReverseIterator(nil, int642Bytes(10)), []int64{9, 8, 7, 5, 4, 3, 2, 1, 0}, "reverse iterator from 10 (ex)")
	verifyIterator(t, db.ReverseIterator(nil, int642Bytes(9)), []int64{8, 7, 5, 4, 3, 2, 1, 0}, "reverse iterator from 9 (ex)")
	verifyIterator(t, db.ReverseIterator(nil, int642Bytes(8)), []int64{7, 5, 4, 3, 2, 1, 0}, "reverse iterator from 8 (ex)")

	verifyIterator(t, db.Iterator(int642Bytes(5), int642Bytes(6)), []int64{5}, "forward iterator from 5 to 6")
	verifyIterator(t, db.Iterator(int642Bytes(5), int642Bytes(7)), []int64{5}, "forward iterator from 5 to 7")
	verifyIterator(t, db.Iterator(int642Bytes(5), int642Bytes(8)), []int64{5, 7}, "forward iterator from 5 to 8")
	verifyIterator(t, db.Iterator(int642Bytes(6), int642Bytes(7)), []int64(nil), "forward iterator from 6 to 7")
	verifyIterator(t, db.Iterator(int642Bytes(6), int642Bytes(8)), []int64{7}, "forward iterator from 6 to 8")
	verifyIterator(t, db.Iterator(int642Bytes(7), int642Bytes(8)), []int64{7}, "forward iterator from 7 to 8")

	verifyIterator(t, db.ReverseIterator(int642Bytes(4), int642Bytes(5)), []int64{4}, "reverse iterator from 5 (ex) to 4")
	verifyIterator(t, db.ReverseIterator(int642Bytes(4), int642Bytes(6)), []int64{5, 4}, "reverse iterator from 6 (ex) to 4")
	verifyIterator(t, db.ReverseIterator(int642Bytes(4), int642Bytes(7)), []int64{5, 4}, "reverse iterator from 7 (ex) to 4")
	verifyIterator(t, db.ReverseIterator(int642Bytes(5), int642Bytes(6)), []int64{5}, "reverse iterator from 6 (ex) to 5")
	verifyIterator(t, db.ReverseIterator(int642Bytes(5), int642Bytes(7)), []int64{5}, "reverse iterator from 7 (ex) to 5")
	verifyIterator(t, db.ReverseIterator(int642Bytes(6), int642Bytes(7)), []int64(nil), "reverse iterator from 7 (ex) to 6")

	verifyIterator(t, db.Iterator(int642Bytes(0), int642Bytes(1)), []int64{0}, "forward iterator from 0 to 1")
	verifyIterator(t, db.ReverseIterator(int642Bytes(8), int642Bytes(9)), []int64{8}, "reverse iterator from 9 (ex) to 8")

	verifyIterator(t, db.Iterator(int642Bytes(2), int642Bytes(4)), []int64{2, 3}, "forward iterator from 2 to 4")
	verifyIterator(t, db.Iterator(int642Bytes(4), int642Bytes(2)), []int64(nil), "forward iterator from 4 to 2")
	verifyIterator(t, db.ReverseIterator(int642Bytes(2), int642Bytes(4)), []int64{3, 2}, "reverse iterator from 4 (ex) to 2")
	verifyIterator(t, db.ReverseIterator(int642Bytes(4), int642Bytes(2)), []int64(nil), "reverse iterator from 2 (ex) to 4")

}

func verifyIterator(t *testing.T, itr Iterator, expected []int64, msg string) {
	var list []int64
	for itr.Valid() {
		list = append(list, bytes2Int64(itr.Key()))
		itr.Next()
	}
	assert.Equal(t, expected, list, msg)
}
