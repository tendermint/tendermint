package light

import (
	"github.com/dashevo/dashd-go/btcjson"
	rpc "github.com/dashevo/dashd-go/rpcclient"
	"github.com/tendermint/tendermint/crypto"
)

// DashCoreVerifier is used to verify signatures of light blocks
type DashCoreVerifier struct {
	endpoint          *rpc.Client
	host              string
	cachedProTxHash   crypto.ProTxHash
	rpcUsername       string
	rpcPassword       string
	defaultQuorumType btcjson.LLMQType
}

// NewDashCoreSignerClient returns an instance of SignerClient.
// it will start the endpoint (if not already started)
func NewDashCoreVerifierClient(host string, rpcUsername string, rpcPassword string, defaultQuorumType btcjson.LLMQType) (*DashCoreVerifier, error) {
	// Connect to local dash core RPC server using HTTP POST mode.
	connCfg := &rpc.ConnConfig{
		Host:         host,
		User:         rpcUsername,
		Pass:         rpcPassword,
		HTTPPostMode: true, // Dash core only supports HTTP POST mode
		DisableTLS:   true, // Dash core does not provide TLS by default
	}
	// Notice the notification parameter is nil since notifications are
	// not supported in HTTP POST mode.
	client, err := rpc.New(connCfg, nil)
	if err != nil {
		return nil, err
	}

	return &DashCoreVerifier{endpoint: client, host: host, rpcUsername: rpcUsername, rpcPassword: rpcPassword, defaultQuorumType: defaultQuorumType}, nil
}

