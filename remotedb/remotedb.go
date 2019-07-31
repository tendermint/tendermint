package remotedb

import (
	"context"
	"fmt"

	db "github.com/tendermint/tm-db"
	"github.com/tendermint/tm-db/remotedb/grpcdb"
	protodb "github.com/tendermint/tm-db/remotedb/proto"
)

type RemoteDB struct {
	ctx context.Context
	dc  protodb.DBClient
}

func NewRemoteDB(serverAddr string, serverKey string) (*RemoteDB, error) {
	return newRemoteDB(grpcdb.NewClient(serverAddr, serverKey))
}

func newRemoteDB(gdc protodb.DBClient, err error) (*RemoteDB, error) {
	if err != nil {
		return nil, err
	}
	return &RemoteDB{dc: gdc, ctx: context.Background()}, nil
}

type Init struct {
	Dir  string
	Name string
	Type string
}

func (rd *RemoteDB) InitRemote(in *Init) error {
	_, err := rd.dc.Init(rd.ctx, &protodb.Init{Dir: in.Dir, Type: in.Type, Name: in.Name})
	return err
}

var _ db.DB = (*RemoteDB)(nil)

// Close is a noop currently
func (rd *RemoteDB) Close() {
}

func (rd *RemoteDB) Delete(key []byte) {
	if _, err := rd.dc.Delete(rd.ctx, &protodb.Entity{Key: key}); err != nil {
		panic(fmt.Sprintf("RemoteDB.Delete: %v", err))
	}
}

func (rd *RemoteDB) DeleteSync(key []byte) {
	if _, err := rd.dc.DeleteSync(rd.ctx, &protodb.Entity{Key: key}); err != nil {
		panic(fmt.Sprintf("RemoteDB.DeleteSync: %v", err))
	}
}

func (rd *RemoteDB) Set(key, value []byte) {
	if _, err := rd.dc.Set(rd.ctx, &protodb.Entity{Key: key, Value: value}); err != nil {
		panic(fmt.Sprintf("RemoteDB.Set: %v", err))
	}
}

func (rd *RemoteDB) SetSync(key, value []byte) {
	if _, err := rd.dc.SetSync(rd.ctx, &protodb.Entity{Key: key, Value: value}); err != nil {
		panic(fmt.Sprintf("RemoteDB.SetSync: %v", err))
	}
}

func (rd *RemoteDB) Get(key []byte) []byte {
	res, err := rd.dc.Get(rd.ctx, &protodb.Entity{Key: key})
	if err != nil {
		panic(fmt.Sprintf("RemoteDB.Get error: %v", err))
	}
	return res.Value
}

func (rd *RemoteDB) Has(key []byte) bool {
	res, err := rd.dc.Has(rd.ctx, &protodb.Entity{Key: key})
	if err != nil {
		panic(fmt.Sprintf("RemoteDB.Has error: %v", err))
	}
	return res.Exists
}

func (rd *RemoteDB) ReverseIterator(start, end []byte) db.Iterator {
	dic, err := rd.dc.ReverseIterator(rd.ctx, &protodb.Entity{Start: start, End: end})
	if err != nil {
		panic(fmt.Sprintf("RemoteDB.Iterator error: %v", err))
	}
	return makeReverseIterator(dic)
}

func (rd *RemoteDB) NewBatch() db.Batch {
	return &batch{
		db:  rd,
		ops: nil,
	}
}

// TODO: Implement Print when db.DB implements a method
// to print to a string and not db.Print to stdout.
func (rd *RemoteDB) Print() {
	panic("Unimplemented")
}

func (rd *RemoteDB) Stats() map[string]string {
	stats, err := rd.dc.Stats(rd.ctx, &protodb.Nothing{})
	if err != nil {
		panic(fmt.Sprintf("RemoteDB.Stats error: %v", err))
	}
	if stats == nil {
		return nil
	}
	return stats.Data
}

func (rd *RemoteDB) Iterator(start, end []byte) db.Iterator {
	dic, err := rd.dc.Iterator(rd.ctx, &protodb.Entity{Start: start, End: end})
	if err != nil {
		panic(fmt.Sprintf("RemoteDB.Iterator error: %v", err))
	}
	return makeIterator(dic)
}

func makeIterator(dic protodb.DB_IteratorClient) db.Iterator {
	return &iterator{dic: dic}
}

func makeReverseIterator(dric protodb.DB_ReverseIteratorClient) db.Iterator {
	return &reverseIterator{dric: dric}
}

type reverseIterator struct {
	dric protodb.DB_ReverseIteratorClient
	cur  *protodb.Iterator
}

var _ db.Iterator = (*iterator)(nil)

func (rItr *reverseIterator) Valid() bool {
	return rItr.cur != nil && rItr.cur.Valid
}

func (rItr *reverseIterator) Domain() (start, end []byte) {
	if rItr.cur == nil || rItr.cur.Domain == nil {
		return nil, nil
	}
	return rItr.cur.Domain.Start, rItr.cur.Domain.End
}

// Next advances the current reverseIterator
func (rItr *reverseIterator) Next() {
	var err error
	rItr.cur, err = rItr.dric.Recv()
	if err != nil {
		panic(fmt.Sprintf("RemoteDB.ReverseIterator.Next error: %v", err))
	}
}

func (rItr *reverseIterator) Key() []byte {
	if rItr.cur == nil {
		return nil
	}
	return rItr.cur.Key
}

func (rItr *reverseIterator) Value() []byte {
	if rItr.cur == nil {
		return nil
	}
	return rItr.cur.Value
}

func (rItr *reverseIterator) Close() {
}

// iterator implements the db.Iterator by retrieving
// streamed iterators from the remote backend as
// needed. It is NOT safe for concurrent usage,
// matching the behavior of other iterators.
type iterator struct {
	dic protodb.DB_IteratorClient
	cur *protodb.Iterator
}

var _ db.Iterator = (*iterator)(nil)

func (itr *iterator) Valid() bool {
	return itr.cur != nil && itr.cur.Valid
}

func (itr *iterator) Domain() (start, end []byte) {
	if itr.cur == nil || itr.cur.Domain == nil {
		return nil, nil
	}
	return itr.cur.Domain.Start, itr.cur.Domain.End
}

// Next advances the current iterator
func (itr *iterator) Next() {
	var err error
	itr.cur, err = itr.dic.Recv()
	if err != nil {
		panic(fmt.Sprintf("RemoteDB.Iterator.Next error: %v", err))
	}
}

func (itr *iterator) Key() []byte {
	if itr.cur == nil {
		return nil
	}
	return itr.cur.Key
}

func (itr *iterator) Value() []byte {
	if itr.cur == nil {
		return nil
	}
	return itr.cur.Value
}

func (itr *iterator) Close() {
	err := itr.dic.CloseSend()
	if err != nil {
		panic(fmt.Sprintf("Error closing iterator: %v", err))
	}
}

type batch struct {
	db  *RemoteDB
	ops []*protodb.Operation
}

var _ db.Batch = (*batch)(nil)

func (bat *batch) Set(key, value []byte) {
	op := &protodb.Operation{
		Entity: &protodb.Entity{Key: key, Value: value},
		Type:   protodb.Operation_SET,
	}
	bat.ops = append(bat.ops, op)
}

func (bat *batch) Delete(key []byte) {
	op := &protodb.Operation{
		Entity: &protodb.Entity{Key: key},
		Type:   protodb.Operation_DELETE,
	}
	bat.ops = append(bat.ops, op)
}

func (bat *batch) Write() {
	if _, err := bat.db.dc.BatchWrite(bat.db.ctx, &protodb.Batch{Ops: bat.ops}); err != nil {
		panic(fmt.Sprintf("RemoteDB.BatchWrite: %v", err))
	}
}

func (bat *batch) WriteSync() {
	if _, err := bat.db.dc.BatchWriteSync(bat.db.ctx, &protodb.Batch{Ops: bat.ops}); err != nil {
		panic(fmt.Sprintf("RemoteDB.BatchWriteSync: %v", err))
	}
}

func (bat *batch) Close() {
	bat.ops = nil
}
