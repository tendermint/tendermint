package db

import (
	"fmt"
	"io/ioutil"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func checkDomain(t *testing.T, itr Iterator, start, end []byte) {
	ds, de := itr.Domain()
	assert.Equal(t, start, ds, "checkDomain domain start incorrect")
	assert.Equal(t, end, de, "checkDomain domain end incorrect")
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
	assert.Panics(t, func() { itr.Value() }, "checkValuePanics expected panic but didn't")
}

func newTempDB(t *testing.T, backend DBBackendType) (db DB, dbDir string) {
	dirname, err := ioutil.TempDir("", "db_common_test")
	require.Nil(t, err)
	return NewDB("testdb", backend, dirname), dirname
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
	mdb.calls["Get"]++
	return nil
}

func (mdb *mockDB) Has([]byte) bool {
	mdb.calls["Has"]++
	return false
}

func (mdb *mockDB) Set([]byte, []byte) {
	mdb.calls["Set"]++
}

func (mdb *mockDB) SetSync([]byte, []byte) {
	mdb.calls["SetSync"]++
}

func (mdb *mockDB) SetNoLock([]byte, []byte) {
	mdb.calls["SetNoLock"]++
}

func (mdb *mockDB) SetNoLockSync([]byte, []byte) {
	mdb.calls["SetNoLockSync"]++
}

func (mdb *mockDB) Delete([]byte) {
	mdb.calls["Delete"]++
}

func (mdb *mockDB) DeleteSync([]byte) {
	mdb.calls["DeleteSync"]++
}

func (mdb *mockDB) DeleteNoLock([]byte) {
	mdb.calls["DeleteNoLock"]++
}

func (mdb *mockDB) DeleteNoLockSync([]byte) {
	mdb.calls["DeleteNoLockSync"]++
}

func (mdb *mockDB) Iterator(start, end []byte) Iterator {
	mdb.calls["Iterator"]++
	return &mockIterator{}
}

func (mdb *mockDB) ReverseIterator(start, end []byte) Iterator {
	mdb.calls["ReverseIterator"]++
	return &mockIterator{}
}

func (mdb *mockDB) Close() {
	mdb.calls["Close"]++
}

func (mdb *mockDB) NewBatch() Batch {
	mdb.calls["NewBatch"]++
	return &memBatch{db: mdb}
}

func (mdb *mockDB) Print() {
	mdb.calls["Print"]++
	fmt.Printf("mockDB{%v}", mdb.Stats())
}

func (mdb *mockDB) Stats() map[string]string {
	mdb.calls["Stats"]++

	res := make(map[string]string)
	for key, count := range mdb.calls {
		res[key] = fmt.Sprintf("%d", count)
	}
	return res
}

//----------------------------------------
// mockIterator

type mockIterator struct{}

func (mockIterator) Domain() (start []byte, end []byte) {
	return nil, nil
}

func (mockIterator) Valid() bool {
	return false
}

func (mockIterator) Next() {
}

func (mockIterator) Key() []byte {
	return nil
}

func (mockIterator) Value() []byte {
	return nil
}

func (mockIterator) Close() {
}
