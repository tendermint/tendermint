package server

import (
	"net"
	"strings"

	"google.golang.org/grpc"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/tmsp/types"
)

// var maxNumberConnections = 2

type GRPCServer struct {
	QuitService

	proto    string
	addr     string
	listener net.Listener

	app types.TMSPApplicationServer
}

func NewGRPCServer(protoAddr string, app types.TMSPApplicationServer) (Service, error) {
	parts := strings.SplitN(protoAddr, "://", 2)
	proto, addr := parts[0], parts[1]
	s := &GRPCServer{
		proto:    proto,
		addr:     addr,
		listener: nil,
		app:      app,
	}
	s.QuitService = *NewQuitService(nil, "TMSPServer", s)
	_, err := s.Start() // Just start it
	return s, err
}

func (s *GRPCServer) OnStart() error {
	s.QuitService.OnStart()
	ln, err := net.Listen(s.proto, s.addr)
	if err != nil {
		return err
	}
	s.listener = ln
	grpcServer := grpc.NewServer()
	types.RegisterTMSPApplicationServer(grpcServer, s.app)
	go grpcServer.Serve(ln)
	return nil
}

func (s *GRPCServer) OnStop() {
	s.QuitService.OnStop()
	s.listener.Close()
}
