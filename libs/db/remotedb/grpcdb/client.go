package grpcdb

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	protodb "github.com/tendermint/tendermint/libs/db/remotedb/proto"
)

// NewClient creates a gRPC client connected to the bound gRPC server at serverAddr.
// Use kind to set the level of security to either Secure or Insecure.
func NewClient(serverAddr, serverCert string) (protodb.DBClient, error) {
	creds, err := credentials.NewClientTLSFromFile(serverCert, "")
	if err != nil {
		return nil, err
	}
	cc, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, err
	}
	return protodb.NewDBClient(cc), nil
}
