package server

import (
	"net"

	"google.golang.org/grpc"

	tmnet "github.com/tendermint/tendermint/libs/net"
	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/pkg/abci"
)

type GRPCServer struct {
	service.BaseService

	proto    string
	addr     string
	listener net.Listener
	server   *grpc.Server

	app abci.ABCIApplicationServer
}

// NewGRPCServer returns a new gRPC ABCI server
func NewGRPCServer(protoAddr string, app abci.ABCIApplicationServer) service.Service {
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

// OnStart starts the gRPC service.
func (s *GRPCServer) OnStart() error {

	ln, err := net.Listen(s.proto, s.addr)
	if err != nil {
		return err
	}

	s.listener = ln
	s.server = grpc.NewServer()
	abci.RegisterABCIApplicationServer(s.server, s.app)

	s.Logger.Info("Listening", "proto", s.proto, "addr", s.addr)
	go func() {
		if err := s.server.Serve(s.listener); err != nil {
			s.Logger.Error("Error serving gRPC server", "err", err)
		}
	}()
	return nil
}

// OnStop stops the gRPC server.
func (s *GRPCServer) OnStop() {
	s.server.Stop()
}
