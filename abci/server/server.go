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
	cmn "github.com/tendermint/tendermint/libs/common"
)

func NewServer(protoAddr, transport string, app types.Application) (cmn.Service, error) {
	var s cmn.Service
	var err error
	switch transport {
	case "socket":
		s = NewSocketServer(protoAddr, app)
	case "grpc":
		s = NewGRPCServer(protoAddr, types.NewGRPCApplication(app))
	default:
		err = fmt.Errorf("Unknown server type %s", transport)
	}
	return s, err
}
