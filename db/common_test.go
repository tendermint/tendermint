package db

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cmn "github.com/tendermint/tmlibs/common"
)

//----------------------------------------
// Helper functions.

func checkValue(t *testing.T, db DB, key []byte, valueWanted []byte) {
	valueGot := db.Get(key)
	assert.Equal(t, valueWanted, valueGot)
}

func checkValid(t *testing.T, itr Iterator, expected bool) {
	valid := itr.Valid()
	require.Equal(t, expected, valid)
}

func checkNext(t *testing.T, itr Iterator, expected bool) {
	itr.Next()
	valid := itr.Valid()
	require.Equal(t, expected, valid)
}

func checkNextPanics(t *testing.T, itr Iterator) {
	assert.Panics(t, func() { itr.Next() }, "checkNextPanics expected panic but didn't")
}

func checkItem(t *testing.T, itr Iterator, key []byte, value []byte) {
	k, v := itr.Key(), itr.Value()
	assert.Exactly(t, key, k)
	assert.Exactly(t, value, v)
}

func checkInvalid(t *testing.T, itr Iterator) {
	checkValid(t, itr, false)
	checkKeyPanics(t, itr)
	checkValuePanics(t, itr)
	checkNextPanics(t, itr)
}

func checkKeyPanics(t *testing.T, itr Iterator) {
	assert.Panics(t, func() { itr.Key() }, "checkKeyPanics expected panic but didn't")
}

func checkValuePanics(t *testing.T, itr Iterator) {
	assert.Panics(t, func() { itr.Key() }, "checkValuePanics expected panic but didn't")
}

func newTempDB(t *testing.T, backend DBBackendType) (db DB) {
	dir, dirname := cmn.Tempdir("db_common_test")
	db = NewDB("testdb", backend, dirname)
	dir.Close()
	return db
}

//----------------------------------------
// mockDB

// NOTE: not actually goroutine safe.
// If you want something goroutine safe, maybe you just want a MemDB.
type mockDB struct {
	mtx   sync.Mutex
	calls map[string]int
}

func newMockDB() *mockDB {
	return &mockDB{
		calls: make(map[string]int),
	}
}

func (mdb *mockDB) Mutex() *sync.Mutex {
	return &(mdb.mtx)
}

func (mdb *mockDB) Get([]byte) []byte {
	mdb.calls["Get"] += 1
	return nil
}

func (mdb *mockDB) Has([]byte) bool {
	mdb.calls["Has"] += 1
	return false
}

func (mdb *mockDB) Set([]byte, []byte) {
	mdb.calls["Set"] += 1
}

func (mdb *mockDB) SetSync([]byte, []byte) {
	mdb.calls["SetSync"] += 1
}

func (mdb *mockDB) SetNoLock([]byte, []byte) {
	mdb.calls["SetNoLock"] += 1
}

func (mdb *mockDB) SetNoLockSync([]byte, []byte) {
	mdb.calls["SetNoLockSync"] += 1
}

func (mdb *mockDB) Delete([]byte) {
	mdb.calls["Delete"] += 1
}

func (mdb *mockDB) DeleteSync([]byte) {
	mdb.calls["DeleteSync"] += 1
}

func (mdb *mockDB) DeleteNoLock([]byte) {
	mdb.calls["DeleteNoLock"] += 1
}

func (mdb *mockDB) DeleteNoLockSync([]byte) {
	mdb.calls["DeleteNoLockSync"] += 1
}

func (mdb *mockDB) Iterator(start, end []byte) Iterator {
	mdb.calls["Iterator"] += 1
	return &mockIterator{}
}

func (mdb *mockDB) ReverseIterator(start, end []byte) Iterator {
	mdb.calls["ReverseIterator"] += 1
	return &mockIterator{}
}

func (mdb *mockDB) Close() {
	mdb.calls["Close"] += 1
}

func (mdb *mockDB) NewBatch() Batch {
	mdb.calls["NewBatch"] += 1
	return &memBatch{db: mdb}
}

func (mdb *mockDB) Print() {
	mdb.calls["Print"] += 1
	fmt.Printf("mockDB{%v}", mdb.Stats())
}

func (mdb *mockDB) Stats() map[string]string {
	mdb.calls["Stats"] += 1

	res := make(map[string]string)
	for key, count := range mdb.calls {
		res[key] = fmt.Sprintf("%d", count)
	}
	return res
}

//----------------------------------------
// mockIterator

type mockIterator struct{}

func (_ mockIterator) Domain() (start []byte, end []byte) {
	return nil, nil
}

func (_ mockIterator) Valid() bool {
	return false
}

func (_ mockIterator) Next() {
}

func (_ mockIterator) Key() []byte {
	return nil
}

func (_ mockIterator) Value() []byte {
	return nil
}

func (_ mockIterator) Close() {
}
