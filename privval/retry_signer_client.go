package privval

import (
	"fmt"
	"time"

	"github.com/tendermint/tendermint/crypto"
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

func (sc *RetrySignerClient) GetPubKey() (crypto.PubKey, error) {
	var (
		pk  crypto.PubKey
		err error
	)
	for i := 0; i < sc.retries || sc.retries == 0; i++ {
		pk, err = sc.next.GetPubKey()
		if err == nil {
			return pk, nil
		}
		time.Sleep(sc.timeout)
	}
	return nil, fmt.Errorf("exhausted all attempts to get pubkey: %w", err)
}

func (sc *RetrySignerClient) SignVote(chainID string, vote *types.Vote) error {
	var err error
	for i := 0; i < sc.retries || sc.retries == 0; i++ {
		err = sc.next.SignVote(chainID, vote)
		if err == nil {
			return nil
		}
		time.Sleep(sc.timeout)
	}
	return fmt.Errorf("exhausted all attempts to sign vote: %w", err)
}

func (sc *RetrySignerClient) SignProposal(chainID string, proposal *types.Proposal) error {
	var err error
	for i := 0; i < sc.retries || sc.retries == 0; i++ {
		err = sc.next.SignProposal(chainID, proposal)
		if err == nil {
			return nil
		}
		time.Sleep(sc.timeout)
	}
	return fmt.Errorf("exhausted all attempts to sign proposal: %w", err)
}
