package server

import (
	"net"
	"strings"

	"google.golang.org/grpc"

	"github.com/tendermint/abci/types"
	cmn "github.com/tendermint/tmlibs/common"
)

type GRPCServer struct {
	cmn.BaseService

	proto    string
	addr     string
	listener net.Listener
	server   *grpc.Server

	app types.ABCIApplicationServer
}

// NewGRPCServer returns a new gRPC ABCI server
func NewGRPCServer(protoAddr string, app types.ABCIApplicationServer) cmn.Service {
	parts := strings.SplitN(protoAddr, "://", 2)
	proto, addr := parts[0], parts[1]
	s := &GRPCServer{
		proto:    proto,
		addr:     addr,
		listener: nil,
		app:      app,
	}
	s.BaseService = *cmn.NewBaseService(nil, "ABCIServer", s)
	return s
}

// OnStart starts the gRPC service
func (s *GRPCServer) OnStart() error {
	s.BaseService.OnStart()
	ln, err := net.Listen(s.proto, s.addr)
	if err != nil {
		return err
	}
	s.listener = ln
	s.server = grpc.NewServer()
	types.RegisterABCIApplicationServer(s.server, s.app)
	go s.server.Serve(s.listener)
	return nil
}

// OnStop stops the gRPC server
func (s *GRPCServer) OnStop() {
	s.BaseService.OnStop()
	s.server.Stop()
}
