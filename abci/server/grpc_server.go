package server

import (
	"net"

	"google.golang.org/grpc"

	"github.com/tendermint/tendermint/abci/types"
	tmnet "github.com/tendermint/tendermint/libs/net"
	"github.com/tendermint/tendermint/libs/service"
)

type GRPCServer struct {
	service.BaseService

	proto    string
	addr     string
	listener net.Listener
	server   *grpc.Server

	app types.ABCIApplicationServer
}

// NewGRPCServer returns a new gRPC ABCI server
func NewGRPCServer(protoAddr string, app types.ABCIApplicationServer) service.Service {
	proto, addr := tmnet.ProtocolAndAddress(protoAddr)
	s := &GRPCServer{
		proto:    proto,
		addr:     addr,
		listener: nil,
		app:      app,
	}
	s.BaseService = *service.NewBaseService(nil, "ABCIServer", s)
	return s
}

// OnStart starts the gRPC service
func (s *GRPCServer) OnStart() error {
	if err := s.BaseService.OnStart(); err != nil {
		return err
	}
	ln, err := net.Listen(s.proto, s.addr)
	if err != nil {
		return err
	}
	s.Logger.Info("Listening", "proto", s.proto, "addr", s.addr)
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
