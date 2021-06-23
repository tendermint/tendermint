package dashcore

import (
	"github.com/dashevo/dashd-go/btcjson"
)

// rpcClientMock implements RpcClient for tests
type rpcClientMock struct{}

// NewRpcClientMock returns a mock instance of RpcClient.
func NewRpcClientMock() (RpcClient, error) {
	return &rpcClientMock{}, nil
}

// Close closes the underlying connection
func (rpcClientMock *rpcClientMock) Close() error {
	rpcClientMock.endpoint.Shutdown()
	return nil
}

// Ping sends a ping request to the remote signer
func (rpcClientMock *rpcClientMock) Ping() error {
	err := rpcClientMock.endpoint.Ping()
	if err != nil {
		return err
	}

	pb, err := rpcClientMock.endpoint.GetPeerInfo()
	if pb == nil {
		return err
	}

	return nil
}

func (rpcClientMock *rpcClientMock) QuorumInfo(quorumType btcjson.LLMQType, quorumHash string, includeSkShare bool) (*btcjson.QuorumInfoResult, error) {
	return rpcClientMock.endpoint.QuorumInfo(quorumType, quorumHash, includeSkShare)
}

func (rpcClientMock *rpcClientMock) MasternodeStatus() (*btcjson.MasternodeStatusResult, error) {
	return rpcClientMock.endpoint.MasternodeStatus()
}

func (rpcClientMock *rpcClientMock) GetNetworkInfo() (*btcjson.GetNetworkInfoResult, error) {
	return rpcClientMock.endpoint.GetNetworkInfo()
}

func (rpcClientMock *rpcClientMock) MasternodeListJSON(filter string) (map[string]btcjson.MasternodelistResultJSON, error) {
	return rpcClientMock.endpoint.MasternodeListJSON(filter)
}

func (rpcClientMock *rpcClientMock) QuorumSign(quorumType btcjson.LLMQType, requestID string, messageHash string, quorumHash string, submit bool) (*btcjson.QuorumSignResultWithBool, error) {
	return rpcClientMock.endpoint.QuorumSign(quorumType, requestID, messageHash, quorumHash, submit)
}
