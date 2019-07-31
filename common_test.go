package db

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"math/rand"
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

func benchmarkRandomReadsWrites(b *testing.B, db DB) {
	b.StopTimer()

	// create dummy data
	const numItems = int64(1000000)
	internal := map[int64]int64{}
	for i := 0; i < int(numItems); i++ {
		internal[int64(i)] = int64(0)
	}

	// fmt.Println("ok, starting")
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		// Write something
		{
			idx := int64(rand.Int()) % numItems // nolint:gosec testing file, so accepting weak random number generator
			internal[idx]++
			val := internal[idx]
			idxBytes := int642Bytes(int64(idx))
			valBytes := int642Bytes(int64(val))
			//fmt.Printf("Set %X -> %X\n", idxBytes, valBytes)
			db.Set(idxBytes, valBytes)
		}

		// Read something
		{
			idx := int64(rand.Int()) % numItems // nolint:gosec testing file, so accepting weak random number generator
			valExp := internal[idx]
			idxBytes := int642Bytes(int64(idx))
			valBytes := db.Get(idxBytes)
			//fmt.Printf("Get %X -> %X\n", idxBytes, valBytes)
			if valExp == 0 {
				if !bytes.Equal(valBytes, nil) {
					b.Errorf("Expected %v for %v, got %X", nil, idx, valBytes)
					break
				}
			} else {
				if len(valBytes) != 8 {
					b.Errorf("Expected length 8 for %v, got %X", idx, valBytes)
					break
				}
				valGot := bytes2Int64(valBytes)
				if valExp != valGot {
					b.Errorf("Expected %v for %v, got %v", valExp, idx, valGot)
					break
				}
			}
		}

	}
}

func int642Bytes(i int64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(i))
	return buf
}

func bytes2Int64(buf []byte) int64 {
	return int64(binary.BigEndian.Uint64(buf))
}
