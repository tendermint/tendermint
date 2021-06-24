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
	return nil
}

// Ping sends a ping request to the remote signer
func (rpcClientMock *rpcClientMock) Ping() error {
	return nil
}

func (rpcClientMock *rpcClientMock) QuorumInfo(quorumType btcjson.LLMQType, quorumHash string, includeSkShare bool) (*btcjson.QuorumInfoResult, error) {
	return &btcjson.QuorumInfoResult{
		Height:          0,
		Type:            "",
		QuorumHash:      "",
		MinedBlock:      "",
		Members:         nil,
		QuorumPublicKey: "",
		SecretKeyShare:  "",
	}, nil
}

func (rpcClientMock *rpcClientMock) MasternodeStatus() (*btcjson.MasternodeStatusResult, error) {
	return &btcjson.MasternodeStatusResult{
		Outpoint:        "",
		Service:         "",
		ProTxHash:       "",
		CollateralHash:  "",
		CollateralIndex: 0,
		DMNState:        btcjson.DMNState{},
		State:           "",
		Status:          "",
	}, nil
}

func (rpcClientMock *rpcClientMock) GetNetworkInfo() (*btcjson.GetNetworkInfoResult, error) {
	return &btcjson.GetNetworkInfoResult{
		Version:         0,
		SubVersion:      "",
		ProtocolVersion: 0,
		LocalServices:   "",
		LocalRelay:      false,
		TimeOffset:      0,
		Connections:     0,
		NetworkActive:   false,
		Networks:        nil,
		RelayFee:        0,
		IncrementalFee:  0,
		LocalAddresses:  nil,
		Warnings:        "",
	}, nil
}

func (rpcClientMock *rpcClientMock) MasternodeListJSON(filter string) (map[string]btcjson.MasternodelistResultJSON, error) {
	m := make(map[string]btcjson.MasternodelistResultJSON)
	m[""] = btcjson.MasternodelistResultJSON{
		Address:           "",
		Collateraladdress: "",
		Lastpaidblock:     0,
		Lastpaidtime:      0,
		Owneraddress:      "",
		Payee:             "",
		ProTxHash:         "",
		Pubkeyoperator:    "",
		Status:            "",
		Votingaddress:     "",
	}

	return m, nil
}

func (rpcClientMock *rpcClientMock) QuorumSign(quorumType btcjson.LLMQType, requestID string, messageHash string, quorumHash string, submit bool) (*btcjson.QuorumSignResultWithBool, error) {
	return &btcjson.QuorumSignResultWithBool{
		QuorumSignResult: btcjson.QuorumSignResult{},
		Result:           false,
	}, nil
}
