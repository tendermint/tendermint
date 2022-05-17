package core

import (
	"fmt"

	"github.com/dashevo/dashd-go/btcjson"
	rpc "github.com/dashevo/dashd-go/rpcclient"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/libs/log"
)

const ModuleName = "rpcclient"

// QuorumVerifier 
type QuorumVerifier interface {
	// QuorumVerify verifies quorum signature
	QuorumVerify(
		quorumType btcjson.LLMQType,
		requestID bytes.HexBytes,
		messageHash bytes.HexBytes,
		signature bytes.HexBytes,
		quorumHash bytes.HexBytes,
	) (bool, error)
}

type Client interface {
	QuorumVerifier

	// QuorumInfo returns quorum info
	QuorumInfo(quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash) (*btcjson.QuorumInfoResult, error)
	// MasternodeStatus returns masternode status
	MasternodeStatus() (*btcjson.MasternodeStatusResult, error)
	// GetNetworkInfo returns network info
	GetNetworkInfo() (*btcjson.GetNetworkInfoResult, error)
	// MasternodeListJSON returns masternode list json
	MasternodeListJSON(filter string) (map[string]btcjson.MasternodelistResultJSON, error)
	// QuorumSign signs message in a quorum session
	QuorumSign(
		quorumType btcjson.LLMQType,
		requestID bytes.HexBytes,
		messageHash bytes.HexBytes,
		quorumHash bytes.HexBytes,
	) (*btcjson.QuorumSignResult, error)
	QuorumVerify(
		quorumType btcjson.LLMQType,
		requestID bytes.HexBytes,
		messageHash bytes.HexBytes,
		signature bytes.HexBytes,
		quorumHash bytes.HexBytes,
	) (bool, error)
	// Close Closes connection to dashd
	Close() error
	// Ping Sends ping to dashd
	Ping() error
}

// RPCClient implements Client
// Handles connection to the underlying dashd instance
type RPCClient struct {
	endpoint *rpc.Client
	logger   log.Logger
}

// NewRPCClient returns an instance of Client.
// it will start the endpoint (if not already started)
func NewRPCClient(host string, username string, password string, logger log.Logger) (*RPCClient, error) {
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

	if logger == nil {
		return nil, fmt.Errorf("logger must be set")
	}

	dashCoreClient := RPCClient{
		endpoint: client,
		logger:   logger,
	}

	return &dashCoreClient, nil
}

// Close closes the underlying connection
func (rpcClient *RPCClient) Close() error {
	rpcClient.endpoint.Shutdown()
	return nil
}

// Ping sends a ping request to the remote signer
func (rpcClient *RPCClient) Ping() error {
	err := rpcClient.endpoint.Ping()
	if err != nil {
		return err
	}

	return nil
}

func (rpcClient *RPCClient) QuorumInfo(
	quorumType btcjson.LLMQType,
	quorumHash crypto.QuorumHash,
) (*btcjson.QuorumInfoResult, error) {
	return rpcClient.endpoint.QuorumInfo(quorumType, quorumHash.String(), false)
}

func (rpcClient *RPCClient) MasternodeStatus() (*btcjson.MasternodeStatusResult, error) {
	return rpcClient.endpoint.MasternodeStatus()
}

func (rpcClient *RPCClient) GetNetworkInfo() (*btcjson.GetNetworkInfoResult, error) {
	return rpcClient.endpoint.GetNetworkInfo()
}

func (rpcClient *RPCClient) MasternodeListJSON(filter string) (
	map[string]btcjson.MasternodelistResultJSON,
	error,
) {
	return rpcClient.endpoint.MasternodeListJSON(filter)
}

func (rpcClient *RPCClient) QuorumSign(
	quorumType btcjson.LLMQType,
	requestID bytes.HexBytes,
	messageHash bytes.HexBytes,
	quorumHash crypto.QuorumHash,
) (*btcjson.QuorumSignResult, error) {
	quorumSignResultWithBool, err := rpcClient.endpoint.QuorumSign(
		quorumType,
		requestID.String(),
		messageHash.String(),
		quorumHash.String(),
		false,
	)
	if quorumSignResultWithBool == nil {
		return nil, err
	}
	quorumSignResult := quorumSignResultWithBool.QuorumSignResult
	return &quorumSignResult, err
}

func (rpcClient *RPCClient) QuorumVerify(
	quorumType btcjson.LLMQType,
	requestID bytes.HexBytes,
	messageHash bytes.HexBytes,
	signature bytes.HexBytes,
	quorumHash crypto.QuorumHash,
) (bool, error) {
	rpcClient.logger.Debug("quorum verify", "sig", signature, "quorumhash", quorumHash)
	return rpcClient.endpoint.QuorumVerify(
		quorumType,
		requestID.String(),
		messageHash.String(),
		signature.String(),
		quorumHash.String(),
	)
}
