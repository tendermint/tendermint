package db

import (
	"fmt"
	"sync"
)

//----------------------------------------
// debugDB

type debugDB struct {
	label string
	db    DB
}

// For printing all operationgs to the console for debugging.
func NewDebugDB(label string, db DB) debugDB {
	return debugDB{
		label: label,
		db:    db,
	}
}

// Implements atomicSetDeleter.
func (ddb debugDB) Mutex() *sync.Mutex { return nil }

// Implements DB.
func (ddb debugDB) Get(key []byte) (value []byte) {
	defer fmt.Printf("%v.Get(%X) %X\n", ddb.label, key, value)
	value = ddb.db.Get(key)
	return
}

// Implements DB.
func (ddb debugDB) Has(key []byte) (has bool) {
	defer fmt.Printf("%v.Has(%X) %v\n", ddb.label, key, has)
	return ddb.db.Has(key)
}

// Implements DB.
func (ddb debugDB) Set(key []byte, value []byte) {
	fmt.Printf("%v.Set(%X, %X)\n", ddb.label, key, value)
	ddb.db.Set(key, value)
}

// Implements DB.
func (ddb debugDB) SetSync(key []byte, value []byte) {
	fmt.Printf("%v.SetSync(%X, %X)\n", ddb.label, key, value)
	ddb.db.SetSync(key, value)
}

// Implements atomicSetDeleter.
func (ddb debugDB) SetNoLock(key []byte, value []byte) {
	fmt.Printf("%v.SetNoLock(%X, %X)\n", ddb.label, key, value)
	ddb.db.Set(key, value)
}

// Implements atomicSetDeleter.
func (ddb debugDB) SetNoLockSync(key []byte, value []byte) {
	fmt.Printf("%v.SetNoLockSync(%X, %X)\n", ddb.label, key, value)
	ddb.db.SetSync(key, value)
}

// Implements DB.
func (ddb debugDB) Delete(key []byte) {
	fmt.Printf("%v.Delete(%X)\n", ddb.label, key)
	ddb.db.Delete(key)
}

// Implements DB.
func (ddb debugDB) DeleteSync(key []byte) {
	fmt.Printf("%v.DeleteSync(%X)\n", ddb.label, key)
	ddb.db.DeleteSync(key)
}

// Implements atomicSetDeleter.
func (ddb debugDB) DeleteNoLock(key []byte) {
	fmt.Printf("%v.DeleteNoLock(%X)\n", ddb.label, key)
	ddb.db.Delete(key)
}

// Implements atomicSetDeleter.
func (ddb debugDB) DeleteNoLockSync(key []byte) {
	fmt.Printf("%v.DeleteNoLockSync(%X)\n", ddb.label, key)
	ddb.db.DeleteSync(key)
}

// Implements DB.
func (ddb debugDB) Iterator(start, end []byte) Iterator {
	fmt.Printf("%v.Iterator(%X, %X)\n", ddb.label, start, end)
	return NewDebugIterator(ddb.label, ddb.db.Iterator(start, end))
}

// Implements DB.
func (ddb debugDB) ReverseIterator(start, end []byte) Iterator {
	fmt.Printf("%v.ReverseIterator(%X, %X)\n", ddb.label, start, end)
	return NewDebugIterator(ddb.label, ddb.db.ReverseIterator(start, end))
}

// Implements DB.
func (ddb debugDB) NewBatch() Batch {
	fmt.Printf("%v.NewBatch()\n", ddb.label)
	return NewDebugBatch(ddb.label, ddb.db.NewBatch())
}

// Implements DB.
func (ddb debugDB) Close() {
	fmt.Printf("%v.Close()\n", ddb.label)
	ddb.db.Close()
}

// Implements DB.
func (ddb debugDB) Print() {
	ddb.db.Print()
}

// Implements DB.
func (ddb debugDB) Stats() map[string]string {
	return ddb.db.Stats()
}

//----------------------------------------
// debugIterator

type debugIterator struct {
	label string
	itr   Iterator
}

// For printing all operationgs to the console for debugging.
func NewDebugIterator(label string, itr Iterator) debugIterator {
	return debugIterator{
		label: label,
		itr:   itr,
	}
}

// Implements Iterator.
func (ditr debugIterator) Domain() (start []byte, end []byte) {
	defer fmt.Printf("%v.itr.Domain() (%X,%X)\n", ditr.label, start, end)
	start, end = ditr.itr.Domain()
	return
}

// Implements Iterator.
func (ditr debugIterator) Valid() (ok bool) {
	defer fmt.Printf("%v.itr.Valid() %v\n", ditr.label, ok)
	ok = ditr.itr.Valid()
	return
}

// Implements Iterator.
func (ditr debugIterator) Next() {
	fmt.Printf("%v.itr.Next()\n", ditr.label)
	ditr.itr.Next()
}

// Implements Iterator.
func (ditr debugIterator) Key() (key []byte) {
	fmt.Printf("%v.itr.Key() %X\n", ditr.label, key)
	key = ditr.itr.Key()
	return
}

// Implements Iterator.
func (ditr debugIterator) Value() (value []byte) {
	fmt.Printf("%v.itr.Value() %X\n", ditr.label, value)
	value = ditr.itr.Value()
	return
}

// Implements Iterator.
func (ditr debugIterator) Close() {
	fmt.Printf("%v.itr.Close()\n", ditr.label)
	ditr.itr.Close()
}

//----------------------------------------
// debugBatch

type debugBatch struct {
	label string
	bch   Batch
}

// For printing all operationgs to the console for debugging.
func NewDebugBatch(label string, bch Batch) debugBatch {
	return debugBatch{
		label: label,
		bch:   bch,
	}
}

// Implements Batch.
func (dbch debugBatch) Set(key, value []byte) {
	fmt.Printf("%v.batch.Set(%X, %X)\n", dbch.label, key, value)
	dbch.bch.Set(key, value)
}

// Implements Batch.
func (dbch debugBatch) Delete(key []byte) {
	fmt.Printf("%v.batch.Delete(%X)\n", dbch.label, key)
	dbch.bch.Delete(key)
}

// Implements Batch.
func (dbch debugBatch) Write() {
	fmt.Printf("%v.batch.Write()\n", dbch.label)
	dbch.bch.Write()
}

// Implements Batch.
func (dbch debugBatch) WriteSync() {
	fmt.Printf("%v.batch.WriteSync()\n", dbch.label)
	dbch.bch.WriteSync()
}
