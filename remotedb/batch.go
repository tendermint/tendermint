package remotedb

import (
	"github.com/pkg/errors"

	db "github.com/tendermint/tm-db"
	protodb "github.com/tendermint/tm-db/remotedb/proto"
)

type batch struct {
	db  *RemoteDB
	ops []*protodb.Operation
}

var _ db.Batch = (*batch)(nil)

func newBatch(rdb *RemoteDB) *batch {
	return &batch{
		db:  rdb,
		ops: []*protodb.Operation{},
	}
}

func (b *batch) assertOpen() {
	if b.ops == nil {
		panic("batch has been written or closed")
	}
}

// Set implements Batch.
func (b *batch) Set(key, value []byte) {
	b.assertOpen()
	op := &protodb.Operation{
		Entity: &protodb.Entity{Key: key, Value: value},
		Type:   protodb.Operation_SET,
	}
	b.ops = append(b.ops, op)
}

// Delete implements Batch.
func (b *batch) Delete(key []byte) {
	b.assertOpen()
	op := &protodb.Operation{
		Entity: &protodb.Entity{Key: key},
		Type:   protodb.Operation_DELETE,
	}
	b.ops = append(b.ops, op)
}

// Write implements Batch.
func (b *batch) Write() error {
	b.assertOpen()
	_, err := b.db.dc.BatchWrite(b.db.ctx, &protodb.Batch{Ops: b.ops})
	if err != nil {
		return errors.Errorf("remoteDB.BatchWrite: %v", err)
	}
	// Make sure batch cannot be used afterwards. Callers should still call Close(), for errors.
	b.Close()
	return nil
}

// WriteSync implements Batch.
func (b *batch) WriteSync() error {
	b.assertOpen()
	_, err := b.db.dc.BatchWriteSync(b.db.ctx, &protodb.Batch{Ops: b.ops})
	if err != nil {
		return errors.Errorf("RemoteDB.BatchWriteSync: %v", err)
	}
	// Make sure batch cannot be used afterwards. Callers should still call Close(), for errors.
	b.Close()
	return nil
}

// Close implements Batch.
func (b *batch) Close() {
	b.ops = nil
}
