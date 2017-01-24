package server

import (
	"fmt"

	"github.com/tendermint/abci/types"
	cmn "github.com/tendermint/go-common"
)

func NewServer(protoAddr, transport string, app types.Application) (cmn.Service, error) {
	var s cmn.Service
	var err error
	switch transport {
	case "socket":
		s, err = NewSocketServer(protoAddr, app)
	case "grpc":
		s, err = NewGRPCServer(protoAddr, types.NewGRPCApplication(app))
	default:
		err = fmt.Errorf("Unknown server type %s", transport)
	}
	return s, err
}
