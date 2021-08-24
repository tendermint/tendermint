/*
Package server is used to start a new ABCI server.

It contains two server implementation:
 * gRPC server
 * socket server

*/
package server

import (
	"fmt"

	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/pkg/abci"
)

func NewServer(protoAddr, transport string, app abci.Application) (service.Service, error) {
	var s service.Service
	var err error
	switch transport {
	case "socket":
		s = NewSocketServer(protoAddr, app)
	case "grpc":
		s = NewGRPCServer(protoAddr, abci.NewGRPCApplication(app))
	default:
		err = fmt.Errorf("unknown server type %s", transport)
	}
	return s, err
}
