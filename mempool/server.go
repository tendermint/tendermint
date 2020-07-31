package mempool

import (
	"context"
	"net"

	"github.com/pkg/errors"
	tmnet "github.com/tendermint/tendermint/libs/net"
	mempoolproto "github.com/tendermint/tendermint/proto/tendermint/mempool"
	"google.golang.org/grpc"
)

type mempoolServer struct {
	proto    string
	addr     string
	listener net.Listener
	server   *grpc.Server
}

func (s *mempoolServer) GetNextTransaction(
	ctx context.Context,
	request *mempoolproto.GetNextTransactionRequest,
) (*mempoolproto.GetNextTransactionResponse, error) {
	// TODO
	return nil, errors.New("not implemented")
}

// Stop stops the gRPC server.
func (s *mempoolServer) Stop() {
	s.server.Stop()
}

func newMempoolServer(protoAddr string) (*mempoolServer, error) {
	proto, addr := tmnet.ProtocolAndAddress(protoAddr)
	ln, err := net.Listen(proto, addr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to start mempool gRPC server")
	}
	s := &mempoolServer{
		proto:    proto,
		addr:     addr,
		listener: ln,
		server:   grpc.NewServer(),
	}
	mempoolproto.RegisterMempoolServiceServer(s.server, s)
	go s.server.Serve(s.listener)
	return s, nil
}
