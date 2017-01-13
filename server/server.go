package server

import (
	"fmt"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/abci/types"
)

func NewServer(protoAddr, transport string, app types.Application) (Service, error) {
	var s Service
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
