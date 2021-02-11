package db_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
	"github.com/tendermint/tm-db/internal/dbtest"
	"github.com/tendermint/tm-db/memdb"
)

func mockDBWithStuff(t *testing.T) tmdb.DB {
	db := memdb.NewDB()
	// Under "key" prefix
	require.NoError(t, db.Set([]byte("key"), []byte("value")))
	require.NoError(t, db.Set([]byte("key1"), []byte("value1")))
	require.NoError(t, db.Set([]byte("key2"), []byte("value2")))
	require.NoError(t, db.Set([]byte("key3"), []byte("value3")))
	require.NoError(t, db.Set([]byte("something"), []byte("else")))
	require.NoError(t, db.Set([]byte("k"), []byte("val")))
	require.NoError(t, db.Set([]byte("ke"), []byte("valu")))
	require.NoError(t, db.Set([]byte("kee"), []byte("valuu")))
	return db
}

func TestPrefixDBSimple(t *testing.T) {
	db := mockDBWithStuff(t)
	pdb := tmdb.NewPrefixDB(db, []byte("key"))

	dbtest.Value(t, pdb, []byte("key"), nil)
	dbtest.Value(t, pdb, []byte("key1"), nil)
	dbtest.Value(t, pdb, []byte("1"), []byte("value1"))
	dbtest.Value(t, pdb, []byte("key2"), nil)
	dbtest.Value(t, pdb, []byte("2"), []byte("value2"))
	dbtest.Value(t, pdb, []byte("key3"), nil)
	dbtest.Value(t, pdb, []byte("3"), []byte("value3"))
	dbtest.Value(t, pdb, []byte("something"), nil)
	dbtest.Value(t, pdb, []byte("k"), nil)
	dbtest.Value(t, pdb, []byte("ke"), nil)
	dbtest.Value(t, pdb, []byte("kee"), nil)
}

func TestPrefixDBIterator1(t *testing.T) {
	db := mockDBWithStuff(t)
	pdb := tmdb.NewPrefixDB(db, []byte("key"))

	itr, err := pdb.Iterator(nil, nil)
	require.NoError(t, err)
	dbtest.Domain(t, itr, nil, nil)
	dbtest.Item(t, itr, []byte("1"), []byte("value1"))
	dbtest.Next(t, itr, true)
	dbtest.Item(t, itr, []byte("2"), []byte("value2"))
	dbtest.Next(t, itr, true)
	dbtest.Item(t, itr, []byte("3"), []byte("value3"))
	dbtest.Next(t, itr, false)
	dbtest.Invalid(t, itr)
	itr.Close()
}

func TestPrefixDBReverseIterator1(t *testing.T) {
	db := mockDBWithStuff(t)
	pdb := tmdb.NewPrefixDB(db, []byte("key"))

	itr, err := pdb.ReverseIterator(nil, nil)
	require.NoError(t, err)
	dbtest.Domain(t, itr, nil, nil)
	dbtest.Item(t, itr, []byte("3"), []byte("value3"))
	dbtest.Next(t, itr, true)
	dbtest.Item(t, itr, []byte("2"), []byte("value2"))
	dbtest.Next(t, itr, true)
	dbtest.Item(t, itr, []byte("1"), []byte("value1"))
	dbtest.Next(t, itr, false)
	dbtest.Invalid(t, itr)
	itr.Close()
}

func TestPrefixDBReverseIterator5(t *testing.T) {
	db := mockDBWithStuff(t)
	pdb := tmdb.NewPrefixDB(db, []byte("key"))

	itr, err := pdb.ReverseIterator([]byte("1"), nil)
	require.NoError(t, err)
	dbtest.Domain(t, itr, []byte("1"), nil)
	dbtest.Item(t, itr, []byte("3"), []byte("value3"))
	dbtest.Next(t, itr, true)
	dbtest.Item(t, itr, []byte("2"), []byte("value2"))
	dbtest.Next(t, itr, true)
	dbtest.Item(t, itr, []byte("1"), []byte("value1"))
	dbtest.Next(t, itr, false)
	dbtest.Invalid(t, itr)
	itr.Close()
}

func TestPrefixDBReverseIterator6(t *testing.T) {
	db := mockDBWithStuff(t)
	pdb := tmdb.NewPrefixDB(db, []byte("key"))

	itr, err := pdb.ReverseIterator([]byte("2"), nil)
	require.NoError(t, err)
	dbtest.Domain(t, itr, []byte("2"), nil)
	dbtest.Item(t, itr, []byte("3"), []byte("value3"))
	dbtest.Next(t, itr, true)
	dbtest.Item(t, itr, []byte("2"), []byte("value2"))
	dbtest.Next(t, itr, false)
	dbtest.Invalid(t, itr)
	itr.Close()
}

func TestPrefixDBReverseIterator7(t *testing.T) {
	db := mockDBWithStuff(t)
	pdb := tmdb.NewPrefixDB(db, []byte("key"))

	itr, err := pdb.ReverseIterator(nil, []byte("2"))
	require.NoError(t, err)
	dbtest.Domain(t, itr, nil, []byte("2"))
	dbtest.Item(t, itr, []byte("1"), []byte("value1"))
	dbtest.Next(t, itr, false)
	dbtest.Invalid(t, itr)
	itr.Close()
}
