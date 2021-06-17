package privval

import (
	"fmt"
	"github.com/dashevo/dashd-go/btcjson"
	"time"

	"github.com/tendermint/tendermint/crypto"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

// RetrySignerClient wraps SignerClient adding retry for each operation (except
// Ping) w/ a timeout.
type RetrySignerClient struct {
	next    *SignerClient
	retries int
	timeout time.Duration
}

// NewRetrySignerClient returns RetrySignerClient. If +retries+ is 0, the
// client will be retrying each operation indefinitely.
func NewRetrySignerClient(sc *SignerClient, retries int, timeout time.Duration) *RetrySignerClient {
	return &RetrySignerClient{sc, retries, timeout}
}

var _ types.PrivValidator = (*RetrySignerClient)(nil)

func (sc *RetrySignerClient) Close() error {
	return sc.next.Close()
}

func (sc *RetrySignerClient) IsConnected() bool {
	return sc.next.IsConnected()
}

func (sc *RetrySignerClient) WaitForConnection(maxWait time.Duration) error {
	return sc.next.WaitForConnection(maxWait)
}

//--------------------------------------------------------
// Implement PrivValidator

func (sc *RetrySignerClient) Ping() error {
	return sc.next.Ping()
}

func (sc *RetrySignerClient) ExtractIntoValidator(height int64, quorumHash crypto.QuorumHash) *types.Validator {
	pubKey, _ := sc.GetPubKey(quorumHash)
	proTxHash, _ := sc.GetProTxHash()
	if len(proTxHash) != crypto.DefaultHashSize {
		panic("proTxHash wrong length")
	}
	return &types.Validator{
		Address:     pubKey.Address(),
		PubKey:      pubKey,
		VotingPower: types.DefaultDashVotingPower,
		ProTxHash:   proTxHash,
	}
}

func (sc *RetrySignerClient) GetPubKey(quorumHash crypto.QuorumHash) (crypto.PubKey, error) {
	var (
		pk  crypto.PubKey
		err error
	)
	for i := 0; i < sc.retries || sc.retries == 0; i++ {
		pk, err = sc.next.GetPubKey(quorumHash)
		if err == nil {
			return pk, nil
		}
		// If remote signer errors, we don't retry.
		if _, ok := err.(*RemoteSignerError); ok {
			return nil, err
		}
		time.Sleep(sc.timeout)
	}
	return nil, fmt.Errorf("exhausted all attempts to get pubkey: %w", err)
}

func (sc *RetrySignerClient) GetProTxHash() (crypto.ProTxHash, error) {
	var (
		proTxHash crypto.ProTxHash
		err       error
	)
	for i := 0; i < sc.retries || sc.retries == 0; i++ {
		proTxHash, err = sc.next.GetProTxHash()
		if len(proTxHash) != crypto.ProTxHashSize {
			return nil, fmt.Errorf("retrySignerClient proTxHash is invalid size")
		}
		if err == nil {
			return proTxHash, nil
		}
		// If remote signer errors, we don't retry.
		if _, ok := err.(*RemoteSignerError); ok {
			return nil, err
		}
		time.Sleep(sc.timeout)
	}
	return nil, fmt.Errorf("exhausted all attempts to get protxhash: %w", err)
}

func (sc *RetrySignerClient) SignVote(chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, vote *tmproto.Vote) error {
	var err error
	for i := 0; i < sc.retries || sc.retries == 0; i++ {
		err = sc.next.SignVote(chainID, quorumType, quorumHash, vote)
		if err == nil {
			return nil
		}
		// If remote signer errors, we don't retry.
		if _, ok := err.(*RemoteSignerError); ok {
			return err
		}
		time.Sleep(sc.timeout)
	}
	return fmt.Errorf("exhausted all attempts to sign vote: %w", err)
}

func (sc *RetrySignerClient) SignProposal(chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, proposal *tmproto.Proposal) error {
	var err error
	for i := 0; i < sc.retries || sc.retries == 0; i++ {
		err = sc.next.SignProposal(chainID, quorumType, quorumHash, proposal)
		if err == nil {
			return nil
		}
		// If remote signer errors, we don't retry.
		if _, ok := err.(*RemoteSignerError); ok {
			return err
		}
		time.Sleep(sc.timeout)
	}
	return fmt.Errorf("exhausted all attempts to sign proposal: %w", err)
}

func (sc *RetrySignerClient) UpdatePrivateKey(privateKey crypto.PrivKey, height int64) error {
	// the private key is dealt with on the abci client
	return nil
}
