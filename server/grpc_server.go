/*
Package server is used to start a new ABCI server.  

It defines the struct for gRPC server settings, and functions for: 

* Starting a new gRPC server
* Stopping a gRPC server

*/

package server

import (
	"net"
	"strings"

	"google.golang.org/grpc"

	"github.com/tendermint/abci/types"
	cmn "github.com/tendermint/tmlibs/common"
)

// var maxNumberConnections = 2

//GRPCServer is used to set the protocol and address for gRPC.  
type GRPCServer struct {
	cmn.BaseService

	proto    string
	addr     string
	listener net.Listener
	server   *grpc.Server

	app types.ABCIApplicationServer
}

//NewGRPCServer allows setting up a new gRPC ABCI server.
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

//Onstart registers a new gRPC service and tells that service to listen on the port that is set in NewGRPCServer.  
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

//OnStop is called when a gRPC server is stopped.  
func (s *GRPCServer) OnStop() {
	s.BaseService.OnStop()
	s.server.Stop()
}
