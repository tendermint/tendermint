package server

import (
	"context"
	"net"

	"google.golang.org/grpc"

	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	tmnet "github.com/tendermint/tendermint/libs/net"
	"github.com/tendermint/tendermint/libs/service"
)

type GRPCServer struct {
	service.BaseService
	logger log.Logger

	proto  string
	addr   string
	server *grpc.Server

	app types.ABCIApplicationServer
}

// NewGRPCServer returns a new gRPC ABCI server
func NewGRPCServer(logger log.Logger, protoAddr string, app types.ABCIApplicationServer) service.Service {
	proto, addr := tmnet.ProtocolAndAddress(protoAddr)
	s := &GRPCServer{
		logger: logger,
		proto:  proto,
		addr:   addr,
		app:    app,
	}
	s.BaseService = *service.NewBaseService(logger, "ABCIServer", s)
	return s
}

// OnStart starts the gRPC service.
func (s *GRPCServer) OnStart(ctx context.Context) error {
	ln, err := net.Listen(s.proto, s.addr)
	if err != nil {
		return err
	}

	s.server = grpc.NewServer()
	types.RegisterABCIApplicationServer(s.server, s.app)

	s.logger.Info("Listening", "proto", s.proto, "addr", s.addr)
	go func() {
		go func() {
			<-ctx.Done()
			s.server.GracefulStop()
		}()

		if err := s.server.Serve(ln); err != nil {
			s.logger.Error("error serving gRPC server", "err", err)
		}
	}()
	return nil
}

// OnStop stops the gRPC server.
func (s *GRPCServer) OnStop() { s.server.Stop() }
