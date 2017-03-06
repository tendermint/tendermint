package server

import (
	"net"
	"strings"

	"google.golang.org/grpc"

	"github.com/tendermint/abci/types"
	cmn "github.com/tendermint/go-common"
)

// var maxNumberConnections = 2

type GRPCServer struct {
	cmn.BaseService

	proto    string
	addr     string
	listener net.Listener
	server   *grpc.Server

	app types.ABCIApplicationServer
}

func NewGRPCServer(protoAddr string, app types.ABCIApplicationServer) (cmn.Service, error) {
	parts := strings.SplitN(protoAddr, "://", 2)
	proto, addr := parts[0], parts[1]
	s := &GRPCServer{
		proto:    proto,
		addr:     addr,
		listener: nil,
		app:      app,
	}
	s.BaseService = *cmn.NewBaseService(nil, "ABCIServer", s)
	_, err := s.Start() // Just start it
	return s, err
}

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

func (s *GRPCServer) OnStop() {
	s.BaseService.OnStop()
	s.server.Stop()
}
