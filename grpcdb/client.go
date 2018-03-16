package grpcdb

import (
	"google.golang.org/grpc"

	protodb "github.com/tendermint/tmlibs/proto"
)

// Security defines how the client will talk to the gRPC server.
type Security uint

const (
	Insecure Security = iota
	Secure
)

// NewClient creates a gRPC client connected to the bound gRPC server at serverAddr.
// Use kind to set the level of security to either Secure or Insecure.
func NewClient(serverAddr string, kind Security) (protodb.DBClient, error) {
	var opts []grpc.DialOption
	if kind == Insecure {
		opts = append(opts, grpc.WithInsecure())
	}
	cc, err := grpc.Dial(serverAddr, opts...)
	if err != nil {
		return nil, err
	}
	return protodb.NewDBClient(cc), nil
}
