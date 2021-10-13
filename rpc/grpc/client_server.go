package coregrpc

import (
	"context"
	io "io"
	"net"

	"google.golang.org/grpc"

	"github.com/tendermint/tendermint/internal/rpc/core"
	tmnet "github.com/tendermint/tendermint/libs/net"
)

// Config is an gRPC server configuration.
type Config struct {
	MaxOpenConnections int
}

type closer func() error

func (c closer) Close() error { return c() } // to satisfy io.Closer

type emptyCloser func()

func (ec emptyCloser) Close() error { c(); return nil } // to satisfy io.Closer

// StartGRPCServer starts a new gRPC BroadcastAPIServer using the given
// net.Listener.
// NOTE: This function blocks - you may want to call it in a go-routine.
// Deprecated: gRPC  in the RPC layer of Tendermint will be removed in 0.36
func StartGRPCServer(env *core.Environment, ln net.Listener) (io.Closer, error) {
	grpcServer := grpc.NewServer()
	RegisterBroadcastAPIServer(grpcServer, &broadcastAPI{env: env})
	return emptyCloser(grpcServer.Stop), grpcServer.Serve(ln)
}

// StartGRPCClient dials the gRPC server using protoAddr and returns a new
// BroadcastAPIClient.
func StartGRPCClient(protoAddr string) (BroadcastAPIClient, io.Closer) {
	conn, err := grpc.Dial(protoAddr, grpc.WithInsecure(), grpc.WithContextDialer(dialerFunc))
	if err != nil {
		panic(err)
	}
	return NewBroadcastAPIClient(conn), conn
}

func dialerFunc(ctx context.Context, addr string) (net.Conn, error) {
	return tmnet.Connect(addr)
}
