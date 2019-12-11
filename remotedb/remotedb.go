package remotedb

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

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
func (rd *RemoteDB) Close() error {
	return nil
}

func (rd *RemoteDB) Delete(key []byte) error {
	if _, err := rd.dc.Delete(rd.ctx, &protodb.Entity{Key: key}); err != nil {
		return errors.Errorf("remoteDB.Delete: %v", err)
	}
	return nil
}

func (rd *RemoteDB) DeleteSync(key []byte) error {
	if _, err := rd.dc.DeleteSync(rd.ctx, &protodb.Entity{Key: key}); err != nil {
		return errors.Errorf("remoteDB.DeleteSync: %v", err)
	}
	return nil
}

func (rd *RemoteDB) Set(key, value []byte) error {
	if _, err := rd.dc.Set(rd.ctx, &protodb.Entity{Key: key, Value: value}); err != nil {
		return errors.Errorf("remoteDB.Set: %v", err)
	}
	return nil
}

func (rd *RemoteDB) SetSync(key, value []byte) error {
	if _, err := rd.dc.SetSync(rd.ctx, &protodb.Entity{Key: key, Value: value}); err != nil {
		return errors.Errorf("remoteDB.SetSync: %v", err)
	}
	return nil
}

func (rd *RemoteDB) Get(key []byte) ([]byte, error) {
	res, err := rd.dc.Get(rd.ctx, &protodb.Entity{Key: key})
	if err != nil {
		return nil, errors.Errorf("remoteDB.Get error: %v", err)
	}
	return res.Value, nil
}

func (rd *RemoteDB) Has(key []byte) (bool, error) {
	res, err := rd.dc.Has(rd.ctx, &protodb.Entity{Key: key})
	if err != nil {
		return false, err
	}
	return res.Exists, nil
}

func (rd *RemoteDB) ReverseIterator(start, end []byte) (db.Iterator, error) {
	dic, err := rd.dc.ReverseIterator(rd.ctx, &protodb.Entity{Start: start, End: end})
	if err != nil {
		return nil, fmt.Errorf("RemoteDB.Iterator error: %w", err)
	}
	return makeReverseIterator(dic), nil
}

func (rd *RemoteDB) NewBatch() db.Batch {
	return &batch{
		db:  rd,
		ops: nil,
	}
}

// TODO: Implement Print when db.DB implements a method
// to print to a string and not db.Print to stdout.
func (rd *RemoteDB) Print() error {
	return errors.New("remoteDB.Print: unimplemented")
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

func (rd *RemoteDB) Iterator(start, end []byte) (db.Iterator, error) {
	dic, err := rd.dc.Iterator(rd.ctx, &protodb.Entity{Start: start, End: end})
	if err != nil {
		return nil, fmt.Errorf("RemoteDB.Iterator error: %w", err)
	}
	return makeIterator(dic), nil
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
func (rItr *reverseIterator) Next() error {
	var err error
	rItr.cur, err = rItr.dric.Recv()
	if err != nil {
		return errors.Errorf("RemoteDB.ReverseIterator.Next error: %v", err)
	}
	return nil
}

func (rItr *reverseIterator) Key() ([]byte, error) {
	if rItr.cur == nil {
		return nil, errors.New("key does not exist")
	}
	return rItr.cur.Key, nil
}

func (rItr *reverseIterator) Value() ([]byte, error) {
	if rItr.cur == nil {
		return nil, errors.New("key does not exist")
	}
	return rItr.cur.Value, nil
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
func (itr *iterator) Next() error {
	var err error
	itr.cur, err = itr.dic.Recv()
	if err != nil {
		return errors.Errorf("remoteDB.Iterator.Next error: %v", err)
	}
	return nil
}

func (itr *iterator) Key() ([]byte, error) {
	if itr.cur == nil {
		return nil, errors.New("key does not exist")
	}
	return itr.cur.Key, nil
}

func (itr *iterator) Value() ([]byte, error) {
	if itr.cur == nil {
		return nil, errors.New("current poisition is not valid")
	}
	return itr.cur.Value, nil
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

func (bat *batch) Write() error {
	if _, err := bat.db.dc.BatchWrite(bat.db.ctx, &protodb.Batch{Ops: bat.ops}); err != nil {
		return errors.Errorf("remoteDB.BatchWrite: %v", err)
	}
	return nil
}

func (bat *batch) WriteSync() error {
	if _, err := bat.db.dc.BatchWriteSync(bat.db.ctx, &protodb.Batch{Ops: bat.ops}); err != nil {
		return errors.Errorf("RemoteDB.BatchWriteSync: %v", err)
	}
	return nil
}

func (bat *batch) Close() {
	bat.ops = nil
}
