package db

import "sync"

type atomicSetDeleter interface {
	Mutex() *sync.Mutex
	SetNoLock(key, value []byte)
	DeleteNoLock(key []byte)
}

type memBatch struct {
	db  atomicSetDeleter
	ops []operation
}

type opType int

const (
	opTypeSet    opType = 1
	opTypeDelete opType = 2
)

type operation struct {
	opType
	key   []byte
	value []byte
}

func (mBatch *memBatch) Set(key, value []byte) {
	mBatch.ops = append(mBatch.ops, operation{opTypeSet, key, value})
}

func (mBatch *memBatch) Delete(key []byte) {
	mBatch.ops = append(mBatch.ops, operation{opTypeDelete, key, nil})
}

func (mBatch *memBatch) Write() {
	mtx := mBatch.db.Mutex()
	mtx.Lock()
	defer mtx.Unlock()

	for _, op := range mBatch.ops {
		switch op.opType {
		case opTypeSet:
			mBatch.db.SetNoLock(op.key, op.value)
		case opTypeDelete:
			mBatch.db.DeleteNoLock(op.key)
		}
	}
}
