package grpcdb

import (
	"context"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	db "github.com/tendermint/tm-db"
	protodb "github.com/tendermint/tm-db/remotedb/proto"
)

// ListenAndServe is a blocking function that sets up a gRPC based
// server at the address supplied, with the gRPC options passed in.
// Normally in usage, invoke it in a goroutine like you would for http.ListenAndServe.
func ListenAndServe(addr, cert, key string, opts ...grpc.ServerOption) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	srv, err := NewServer(cert, key, opts...)
	if err != nil {
		return err
	}
	return srv.Serve(ln)
}

func NewServer(cert, key string, opts ...grpc.ServerOption) (*grpc.Server, error) {
	creds, err := credentials.NewServerTLSFromFile(cert, key)
	if err != nil {
		return nil, err
	}
	opts = append(opts, grpc.Creds(creds))
	srv := grpc.NewServer(opts...)
	protodb.RegisterDBServer(srv, new(server))
	return srv, nil
}

type server struct {
	mu sync.Mutex
	db db.DB
}

var _ protodb.DBServer = (*server)(nil)

// Init initializes the server's database. Only one type of database
// can be initialized per server.
//
// Dir is the directory on the file system in which the DB will be stored(if backed by disk) (TODO: remove)
//
// Name is representative filesystem entry's basepath
//
// Type can be either one of:
//  * cleveldb (if built with gcc enabled)
//  * fsdb
//  * memdB
//  * goleveldb
// See https://godoc.org/github.com/tendermint/tendermint/libs/db#BackendType
func (s *server) Init(ctx context.Context, in *protodb.Init) (*protodb.Entity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.db = db.NewDB(in.Name, db.BackendType(in.Type), in.Dir)
	return &protodb.Entity{CreatedAt: time.Now().Unix()}, nil
}

func (s *server) Delete(ctx context.Context, in *protodb.Entity) (*protodb.Nothing, error) {
	err := s.db.Delete(in.Key)
	if err != nil {
		return nil, err
	}
	return nothing, nil
}

var nothing = new(protodb.Nothing)

func (s *server) DeleteSync(ctx context.Context, in *protodb.Entity) (*protodb.Nothing, error) {
	err := s.db.DeleteSync(in.Key)
	if err != nil {
		return nil, err
	}
	return nothing, nil
}

func (s *server) Get(ctx context.Context, in *protodb.Entity) (*protodb.Entity, error) {
	value, err := s.db.Get(in.Key)
	if err != nil {
		return nil, err
	}
	return &protodb.Entity{Value: value}, nil
}

func (s *server) GetStream(ds protodb.DB_GetStreamServer) error {
	// Receive routine
	responsesChan := make(chan *protodb.Entity)
	go func() {
		defer close(responsesChan)
		ctx := context.Background()
		for {
			in, err := ds.Recv()
			if err != nil {
				responsesChan <- &protodb.Entity{Err: err.Error()}
				return
			}
			out, err := s.Get(ctx, in)
			if err != nil {
				if out == nil {
					out = new(protodb.Entity)
					out.Key = in.Key
				}
				out.Err = err.Error()
				responsesChan <- out
				return
			}

			// Otherwise continue on
			responsesChan <- out
		}
	}()

	// Send routine, block until we return
	for out := range responsesChan {
		if err := ds.Send(out); err != nil {
			return err
		}
	}
	return nil
}

func (s *server) Has(ctx context.Context, in *protodb.Entity) (*protodb.Entity, error) {
	exists, err := s.db.Has(in.Key)
	if err != nil {
		return nil, err
	}
	return &protodb.Entity{Exists: exists}, nil
}

func (s *server) Set(ctx context.Context, in *protodb.Entity) (*protodb.Nothing, error) {
	err := s.db.Set(in.Key, in.Value)
	if err != nil {
		return nil, err
	}
	return nothing, nil
}

func (s *server) SetSync(ctx context.Context, in *protodb.Entity) (*protodb.Nothing, error) {
	err := s.db.SetSync(in.Key, in.Value)
	if err != nil {
		return nil, err
	}
	return nothing, nil
}

func (s *server) Iterator(query *protodb.Entity, dis protodb.DB_IteratorServer) error {
	it, err := s.db.Iterator(query.Start, query.End)
	if err != nil {
		return err
	}
	defer it.Close()
	return s.handleIterator(it, dis.Send)
}

func (s *server) handleIterator(it db.Iterator, sendFunc func(*protodb.Iterator) error) error {
	for it.Valid() {
		start, end := it.Domain()
		key := it.Key()
		value := it.Value()

		out := &protodb.Iterator{
			Domain: &protodb.Domain{Start: start, End: end},
			Valid:  it.Valid(),
			Key:    key,
			Value:  value,
		}
		if err := sendFunc(out); err != nil {
			return err
		}

		// Finally move the iterator forward,
		it.Next()

	}
	return nil
}

func (s *server) ReverseIterator(query *protodb.Entity, dis protodb.DB_ReverseIteratorServer) error {
	it, err := s.db.ReverseIterator(query.Start, query.End)
	if err != nil {
		return err
	}
	defer it.Close()
	return s.handleIterator(it, dis.Send)
}

func (s *server) Stats(context.Context, *protodb.Nothing) (*protodb.Stats, error) {
	stats := s.db.Stats()
	return &protodb.Stats{Data: stats, TimeAt: time.Now().Unix()}, nil
}

func (s *server) BatchWrite(c context.Context, b *protodb.Batch) (*protodb.Nothing, error) {
	return s.batchWrite(c, b, false)
}

func (s *server) BatchWriteSync(c context.Context, b *protodb.Batch) (*protodb.Nothing, error) {
	return s.batchWrite(c, b, true)
}

func (s *server) batchWrite(c context.Context, b *protodb.Batch, sync bool) (*protodb.Nothing, error) {
	bat := s.db.NewBatch()
	defer bat.Close()
	for _, op := range b.Ops {
		switch op.Type {
		case protodb.Operation_SET:
			bat.Set(op.Entity.Key, op.Entity.Value)
		case protodb.Operation_DELETE:
			bat.Delete(op.Entity.Key)
		}
	}
	if sync {
		err := bat.WriteSync()
		if err != nil {
			return nil, err
		}
	} else {
		err := bat.Write()
		if err != nil {
			return nil, err
		}
	}
	return nothing, nil
}
