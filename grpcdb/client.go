package grpcdb

import (
	"google.golang.org/grpc"

	protodb "github.com/tendermint/tmlibs/proto"
)

func NewClient(serverAddr string, secure bool) (protodb.DBClient, error) {
	var opts []grpc.DialOption
	if !secure {
		opts = append(opts, grpc.WithInsecure())
	}
	cc, err := grpc.Dial(serverAddr, opts...)
	if err != nil {
		return nil, err
	}
	return protodb.NewDBClient(cc), nil
}
