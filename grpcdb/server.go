package grpcdb

import (
"log"
	"context"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"

	"github.com/tendermint/tmlibs/db"
	protodb "github.com/tendermint/tmlibs/proto"
)

func BindRemoteDBServer(addr string, opts ...grpc.ServerOption) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	srv := grpc.NewServer(opts...)
	protodb.RegisterDBServer(srv, new(server))
	return srv.Serve(ln)
}

type server struct {
	mu sync.Mutex
	db db.DB
}

var _ protodb.DBServer = (*server)(nil)

func (s *server) Init(ctx context.Context, in *protodb.Init) (*protodb.Entity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

log.Printf("in: %+v\n", in)
	s.db = db.NewDB(in.Name, db.DBBackendType(in.Type), in.Dir)
	return &protodb.Entity{TimeAt: time.Now().Unix()}, nil
}

func (s *server) Delete(ctx context.Context, in *protodb.Entity) (*protodb.Nothing, error) {
	s.db.Delete(in.Key)
	return nothing, nil
}

var nothing = new(protodb.Nothing)
func (s *server) DeleteSync(ctx context.Context, in *protodb.Entity) (*protodb.Nothing, error) {
	s.db.DeleteSync(in.Key)
	return nothing, nil
}

func (s *server) Get(ctx context.Context, in *protodb.Entity) (*protodb.Entity, error) {
	value := s.db.Get(in.Key)
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
	exists := s.db.Has(in.Key)
	return &protodb.Entity{Exists: exists}, nil
}

func (s *server) Set(ctx context.Context, in *protodb.Entity) (*protodb.Nothing, error) {
	s.db.Set(in.Key, in.Value)
	return nothing, nil
}

func (s *server) SetSync(ctx context.Context, in *protodb.Entity) (*protodb.Nothing, error) {
	s.db.SetSync(in.Key, in.Value)
	return nothing, nil
}

func (s *server) Iterator(query *protodb.Entity, dis protodb.DB_IteratorServer) error {
	it := s.db.Iterator(query.Start, query.End)
	return s.handleIterator(it, dis.Send)
}

func (s *server) handleIterator(it db.Iterator, sendFunc func(*protodb.Iterator) error) error {
	for it.Valid() {
		start, end := it.Domain()
		out := &protodb.Iterator{
			Domain: &protodb.DDomain{Start: start, End: end},
			Valid:  it.Valid(),
			Key:    it.Key(),
			Value:  it.Value(),
		}
		if err := sendFunc(out); err != nil {
			return err
		}

		// Finally move the iterator forward
		it.Next()
	}
	return nil
}

func (s *server) ReverseIterator(query *protodb.Entity, dis protodb.DB_ReverseIteratorServer) error {
	it := s.db.ReverseIterator(query.Start, query.End)
	return s.handleIterator(it, dis.Send)
}

func (s *server) Stats(context.Context, *protodb.Nothing) (*protodb.Stats, error) {
	stats := s.db.Stats()
	return &protodb.Stats{Data: stats, TimeAt: time.Now().Unix()}, nil
}
