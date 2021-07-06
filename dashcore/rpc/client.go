package dashcore

import (
	"fmt"
	"github.com/dashevo/dashd-go/btcjson"
	rpc "github.com/dashevo/dashd-go/rpcclient"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/bytes"
)

type DashCoreClient interface {
	// QuorumInfo returns quorum info
	QuorumInfo(quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash) (*btcjson.QuorumInfoResult, error)
	// MasternodeStatus returns masternode status
	MasternodeStatus() (*btcjson.MasternodeStatusResult, error)
	// GetNetworkInfo returns network info
	GetNetworkInfo() (*btcjson.GetNetworkInfoResult, error)
	// MasternodeListJSON returns masternode list json
	MasternodeListJSON(filter string) (map[string]btcjson.MasternodelistResultJSON, error)
	// QuorumSign signs message in a quorum session
	QuorumSign(quorumType btcjson.LLMQType, requestID bytes.HexBytes, messageHash bytes.HexBytes, quorumHash bytes.HexBytes) (*btcjson.QuorumSignResult, error)
	// QuorumVerify verifies quorum signature
	QuorumVerify(quorumType btcjson.LLMQType, requestID bytes.HexBytes, messageHash bytes.HexBytes, signature bytes.HexBytes, quorumHash bytes.HexBytes) (bool, error)
	// Close Closes connection to dashd
	Close() error
	// Ping Sends ping to dashd
	Ping() error
}

// DashCoreRpcClient implements DashCoreClient
// Handles connection to the underlying dashd instance
type DashCoreRpcClient struct {
	endpoint *rpc.Client
}

// NewDashCoreRpcClient returns an instance of DashCoreClient.
// it will start the endpoint (if not already started)
func NewDashCoreRpcClient(host string, username string, password string) (*DashCoreRpcClient, error) {
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

	dashCoreClient := DashCoreRpcClient{endpoint: client}

	return &dashCoreClient, nil
}

// Close closes the underlying connection
func (rpcClient *DashCoreRpcClient) Close() error {
	rpcClient.endpoint.Shutdown()
	return nil
}

// Ping sends a ping request to the remote signer
func (rpcClient *DashCoreRpcClient) Ping() error {
	err := rpcClient.endpoint.Ping()
	if err != nil {
		return err
	}

	return nil
}

func (rpcClient *DashCoreRpcClient) QuorumInfo(quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash) (*btcjson.QuorumInfoResult, error) {
	return rpcClient.endpoint.QuorumInfo(quorumType, quorumHash.String(), false)
}

func (rpcClient *DashCoreRpcClient) MasternodeStatus() (*btcjson.MasternodeStatusResult, error) {
	return rpcClient.endpoint.MasternodeStatus()
}

func (rpcClient *DashCoreRpcClient) GetNetworkInfo() (*btcjson.GetNetworkInfoResult, error) {
	return rpcClient.endpoint.GetNetworkInfo()
}

func (rpcClient *DashCoreRpcClient) MasternodeListJSON(filter string) (map[string]btcjson.MasternodelistResultJSON, error) {
	return rpcClient.endpoint.MasternodeListJSON(filter)
}

func (rpcClient *DashCoreRpcClient) QuorumSign(quorumType btcjson.LLMQType, requestID bytes.HexBytes, messageHash bytes.HexBytes, quorumHash crypto.QuorumHash) (*btcjson.QuorumSignResult, error) {
	quorumSignResultWithBool, err := rpcClient.endpoint.QuorumSign(quorumType, requestID.String(), messageHash.String(), quorumHash.String(), false)
	if quorumSignResultWithBool == nil {
		return nil, err
	} else {
		quorumSignResult := quorumSignResultWithBool.QuorumSignResult
		return &quorumSignResult, err
	}

}

func (rpcClient *DashCoreRpcClient) QuorumVerify(quorumType btcjson.LLMQType, requestID bytes.HexBytes, messageHash bytes.HexBytes, signature bytes.HexBytes, quorumHash crypto.QuorumHash) (bool, error) {
	fmt.Printf("quorum verify sig %v quorumhash %s", signature, quorumHash)
	return rpcClient.endpoint.QuorumVerify(quorumType, requestID.String(), messageHash.String(), signature.String(), quorumHash.String())
}
