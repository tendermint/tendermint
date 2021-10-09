package coregrpc

import (
	"context"
	"net"

	"google.golang.org/grpc"

	"github.com/tendermint/tendermint/internal/rpc/core"
	tmnet "github.com/tendermint/tendermint/libs/net"
)

// Config is an gRPC server configuration.
type Config struct {
	MaxOpenConnections int
}

type Closer func() error

var noopCloser Closer = func() error { return nil }

func mkCloser(fn func()) Closer { return func() error { fn(); return nil } }

// StartGRPCServer starts a new gRPC BroadcastAPIServer using the given
// net.Listener.
// NOTE: This function blocks - you may want to call it in a go-routine.
// Deprecated: gRPC  in the RPC layer of Tendermint will be removed in 0.36
func StartGRPCServer(env *core.Environment, ln net.Listener) (Closer, error) {
	grpcServer := grpc.NewServer()
	RegisterBroadcastAPIServer(grpcServer, &broadcastAPI{env: env})
	return mkCloser(grpcServer.Stop), grpcServer.Serve(ln)
}

// StartGRPCClient dials the gRPC server using protoAddr and returns a new
// BroadcastAPIClient.
func StartGRPCClient(protoAddr string) (BroadcastAPIClient, Closer) {
	conn, err := grpc.Dial(protoAddr, grpc.WithInsecure(), grpc.WithContextDialer(dialerFunc))
	if err != nil {
		panic(err)
	}
	return NewBroadcastAPIClient(conn), conn.Close
}

func dialerFunc(ctx context.Context, addr string) (net.Conn, error) {
	return tmnet.Connect(addr)
}
