package core_grpc

import (
	"net"
	"strings"

	"google.golang.org/grpc"

	. "github.com/tendermint/go-common"
)

// var maxNumberConnections = 2

type GRPCServer struct {
	QuitService

	proto    string
	addr     string
	listener net.Listener
}

func NewGRPCServer(protoAddr string) (Service, error) {
	parts := strings.SplitN(protoAddr, "://", 2)
	proto, addr := parts[0], parts[1]
	s := &GRPCServer{
		proto:    proto,
		addr:     addr,
		listener: nil,
	}
	s.QuitService = *NewQuitService(nil, "TendermintRPCServer", s)
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
	RegisterBroadcastAPIServer(grpcServer, &broadcastAPI{})
	go grpcServer.Serve(ln)
	return nil
}

func (s *GRPCServer) OnStop() {
	s.QuitService.OnStop()
	s.listener.Close()
}
