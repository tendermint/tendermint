package privval

import (
	"errors"
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

func (sc *RetrySignerClient) GetPubKey() crypto.PubKey {
	for i := 0; i < sc.retries || sc.retries == 0; i++ {
		pk := sc.next.GetPubKey()
		if pk != nil {
			return pk
		}
		time.Sleep(sc.timeout)
	}
	return nil
}

func (sc *RetrySignerClient) SignVote(chainID string, vote *types.Vote) error {
	for i := 0; i < sc.retries || sc.retries == 0; i++ {
		err := sc.next.SignVote(chainID, vote)
		if err == nil {
			return nil
		}
		// If remote signer errors, we don't retry.
		if _, ok := err.(*RemoteSignerError); ok {
			return err
		}
		time.Sleep(sc.timeout)
	}
	return errors.New("exhausted all attempts to sign vote")
}

func (sc *RetrySignerClient) SignProposal(chainID string, proposal *types.Proposal) error {
	for i := 0; i < sc.retries || sc.retries == 0; i++ {
		err := sc.next.SignProposal(chainID, proposal)
		if err == nil {
			return nil
		}
		// If remote signer errors, we don't retry.
		if _, ok := err.(*RemoteSignerError); ok {
			return err
		}
		time.Sleep(sc.timeout)
	}
	return errors.New("exhausted all attempts to sign proposal")
}
