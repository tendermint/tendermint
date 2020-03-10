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
	return newBatch(rd)
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
