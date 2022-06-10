/*
Package server is used to start a new ABCI server.

It contains two server implementation:
 * gRPC server
 * socket server

*/
package server

import (
	"fmt"

	"github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
)

func NewServer(logger log.Logger, protoAddr, transport string, app types.Application) (service.Service, error) {
	var s service.Service
	var err error
	switch transport {
	case "socket":
		s = NewSocketServer(logger, protoAddr, app)
	case "grpc":
		s = NewGRPCServer(logger, protoAddr, app)
	default:
		err = fmt.Errorf("unknown server type %s", transport)
	}
	return s, err
}
