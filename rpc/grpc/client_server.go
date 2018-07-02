package core_grpc

import (
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/net/netutil"
	"google.golang.org/grpc"

	cmn "github.com/tendermint/tendermint/libs/common"
)

// Config is an gRPC server configuration.
type Config struct {
	MaxOpenConnections int
}

// StartGRPCServer starts a new gRPC BroadcastAPIServer, listening on
// protoAddr, in a goroutine. Returns a listener and an error, if it fails to
// parse an address.
func StartGRPCServer(protoAddr string, config Config) (net.Listener, error) {
	parts := strings.SplitN(protoAddr, "://", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("Invalid listen address for grpc server (did you forget a tcp:// prefix?) : %s", protoAddr)
	}
	proto, addr := parts[0], parts[1]
	ln, err := net.Listen(proto, addr)
	if err != nil {
		return nil, err
	}
	if config.MaxOpenConnections > 0 {
		ln = netutil.LimitListener(ln, config.MaxOpenConnections)
	}

	grpcServer := grpc.NewServer()
	RegisterBroadcastAPIServer(grpcServer, &broadcastAPI{})
	go grpcServer.Serve(ln) // nolint: errcheck

	return ln, nil
}

// StartGRPCClient dials the gRPC server using protoAddr and returns a new
// BroadcastAPIClient.
func StartGRPCClient(protoAddr string) BroadcastAPIClient {
	conn, err := grpc.Dial(protoAddr, grpc.WithInsecure(), grpc.WithDialer(dialerFunc))
	if err != nil {
		panic(err)
	}
	return NewBroadcastAPIClient(conn)
}

func dialerFunc(addr string, timeout time.Duration) (net.Conn, error) {
	return cmn.Connect(addr)
}
