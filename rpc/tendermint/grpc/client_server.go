package core_grpc

import (
	"fmt"
	"net"
	"strings"
	"time"

	"google.golang.org/grpc"

	. "github.com/tendermint/go-common"
)

// Start the grpcServer in a go routine
func StartGRPCServer(protoAddr string) (net.Listener, error) {
	parts := strings.SplitN(protoAddr, "://", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("Invalid listen address for grpc server (did you forget a tcp:// prefix?) : %s", protoAddr)
	}
	proto, addr := parts[0], parts[1]
	ln, err := net.Listen(proto, addr)
	if err != nil {
		return nil, err
	}

	grpcServer := grpc.NewServer()
	RegisterBroadcastAPIServer(grpcServer, &broadcastAPI{})
	go grpcServer.Serve(ln)

	return ln, nil
}

// Start the client by dialing the server
func StartGRPCClient(protoAddr string) BroadcastAPIClient {
	conn, err := grpc.Dial(protoAddr, grpc.WithInsecure(), grpc.WithDialer(dialerFunc))
	if err != nil {
		panic(err)
	}
	return NewBroadcastAPIClient(conn)
}

func dialerFunc(addr string, timeout time.Duration) (net.Conn, error) {
	return Connect(addr)
}
