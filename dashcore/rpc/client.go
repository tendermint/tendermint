package dashcore

import (
	"fmt"
	"github.com/dashevo/dashd-go/btcjson"
	rpc "github.com/dashevo/dashd-go/rpcclient"
)

type RpcClient interface {
	// QuorumInfo returns quorum info
	QuorumInfo(quorumType btcjson.LLMQType, quorumHash string, includeSkShare bool) (*btcjson.QuorumInfoResult, error)
	// MasternodeStatus returns masternode status
	MasternodeStatus() (*btcjson.MasternodeStatusResult, error)
	// GetNetworkInfo returns network info
	GetNetworkInfo() (*btcjson.GetNetworkInfoResult, error)
	// MasternodeListJSON returns masternode list json
	MasternodeListJSON(filter string) (map[string]btcjson.MasternodelistResultJSON, error)
	// QuorumSign signs message in a quorum session
	QuorumSign(quorumType btcjson.LLMQType, requestID string, messageHash string, quorumHash string, submit bool) (*btcjson.QuorumSignResultWithBool, error)
	// Close Closes connection to dashd
	Close() error
	// Ping Sends ping to dashd
	Ping() error
}

// rpcClient implements RpcClient
// Handles connection to the underlying dashd instance
type rpcClient struct {
	endpoint *rpc.Client
}

// NewRpcClient returns an instance of RpcClient.
// it will start the endpoint (if not already started)
func NewRpcClient(host string, username string, password string) (RpcClient, error) {
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

	return &rpcClient{endpoint: client}, nil
}

// Close closes the underlying connection
func (rpcClient *rpcClient) Close() error {
	rpcClient.endpoint.Shutdown()
	return nil
}

// Ping sends a ping request to the remote signer
func (rpcClient *rpcClient) Ping() error {
	err := rpcClient.endpoint.Ping()
	if err != nil {
		return err
	}

	pb, err := rpcClient.endpoint.GetPeerInfo()
	if pb == nil {
		return err
	}

	return nil
}

func (rpcClient *rpcClient) QuorumInfo(quorumType btcjson.LLMQType, quorumHash string, includeSkShare bool) (*btcjson.QuorumInfoResult, error) {
	return rpcClient.endpoint.QuorumInfo(quorumType, quorumHash, includeSkShare)
}

func (rpcClient *rpcClient) MasternodeStatus() (*btcjson.MasternodeStatusResult, error) {
	return rpcClient.endpoint.MasternodeStatus()
}

func (rpcClient *rpcClient) GetNetworkInfo() (*btcjson.GetNetworkInfoResult, error) {
	return rpcClient.endpoint.GetNetworkInfo()
}

func (rpcClient *rpcClient) MasternodeListJSON(filter string) (map[string]btcjson.MasternodelistResultJSON, error) {
	return rpcClient.endpoint.MasternodeListJSON(filter)
}

func (rpcClient *rpcClient) QuorumSign(quorumType btcjson.LLMQType, requestID string, messageHash string, quorumHash string, submit bool) (*btcjson.QuorumSignResultWithBool, error) {
	return rpcClient.endpoint.QuorumSign(quorumType, requestID, messageHash, quorumHash, submit)
}
