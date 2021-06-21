package dashcore

import (
	"fmt"
	rpc "github.com/dashevo/dashd-go/rpcclient"
)

// RpcClient implements PrivValidator.
// Handles remote validator connections that provide signing services
type RpcClient struct {
	Endpoint *rpc.Client
	host     string
	username string
	password string
}

// New returns an instance of SignerClient.
// it will start the endpoint (if not already started)
func NewRpcClient(host string, username string, password string) (*RpcClient, error) {
	if host == "" {
		return nil, fmt.Errorf("unable to establish connection to the Dash Core node")
	}

	// Connect to local dash core RPC server using HTTP POST mode.
	connCfg := &rpc.ConnConfig{
		Host:         host,
		User:         username,
		Pass:         password,
		HTTPPostMode: true, // Dash core only supports HTTP POST mode
		DisableTLS:   true, // Dash core does not provide TLS by default
	}
	// Notice the notification parameter is nil since notifications are
	// not supported in HTTP POST mode.
	client, err := rpc.New(connCfg, nil)
	if err != nil {
		return nil, err
	}

	return &RpcClient{Endpoint: client, host: host, username: username, password: password}, nil
}

// Close closes the underlying connection
func (rpcClient *RpcClient) Close() error {
	rpcClient.Endpoint.Shutdown()
	return nil
}

// Ping sends a ping request to the remote signer
func (rpcClient *RpcClient) Ping() error {
	err := rpcClient.Endpoint.Ping()
	if err != nil {
		return err
	}

	pb, err := rpcClient.Endpoint.GetPeerInfo()
	if pb == nil {
		return err
	}

	return nil
}
