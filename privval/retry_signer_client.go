package privval

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dashevo/dashd-go/btcjson"

	"github.com/tendermint/tendermint/crypto"
	tmbytes "github.com/tendermint/tendermint/libs/bytes"
	"github.com/tendermint/tendermint/libs/log"
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

func (sc *RetrySignerClient) WaitForConnection(ctx context.Context, maxWait time.Duration) error {
	return sc.next.WaitForConnection(ctx, maxWait)
}

//--------------------------------------------------------
// Implement PrivValidator

func (sc *RetrySignerClient) Ping(ctx context.Context) error {
	return sc.next.Ping(ctx)
}

func (sc *RetrySignerClient) ExtractIntoValidator(ctx context.Context, quorumHash crypto.QuorumHash) *types.Validator {
	pubKey, _ := sc.GetPubKey(ctx, quorumHash)
	proTxHash, _ := sc.GetProTxHash(ctx)
	if len(proTxHash) != crypto.DefaultHashSize {
		panic("proTxHash wrong length")
	}
	return &types.Validator{
		PubKey:      pubKey,
		VotingPower: types.DefaultDashVotingPower,
		ProTxHash:   proTxHash,
	}
}

func (sc *RetrySignerClient) GetPubKey(ctx context.Context, quorumHash crypto.QuorumHash) (crypto.PubKey, error) {
	var (
		pk  crypto.PubKey
		err error
	)
	for i := 0; i < sc.retries || sc.retries == 0; i++ {
		pk, err = sc.next.GetPubKey(ctx, quorumHash)
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

func (sc *RetrySignerClient) GetProTxHash(ctx context.Context) (crypto.ProTxHash, error) {
	var (
		proTxHash crypto.ProTxHash
		err       error
	)
	for i := 0; i < sc.retries || sc.retries == 0; i++ {
		proTxHash, err = sc.next.GetProTxHash(ctx)
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

func (sc *RetrySignerClient) GetFirstQuorumHash(context context.Context) (crypto.QuorumHash, error) {
	return nil, errors.New("getFirstQuorumHash should not be called on a signer client")
}

func (sc *RetrySignerClient) GetThresholdPublicKey(ctx context.Context, quorumHash crypto.QuorumHash) (crypto.PubKey, error) {
	var (
		pk  crypto.PubKey
		err error
	)

	t := time.NewTimer(sc.timeout)
	for i := 0; i < sc.retries || sc.retries == 0; i++ {
		pk, err = sc.next.GetThresholdPublicKey(ctx, quorumHash)
		if err == nil {
			return pk, nil
		}
		// If remote signer errors, we don't retry.
		if _, ok := err.(*RemoteSignerError); ok {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-t.C:
			t.Reset(sc.timeout)
		}
	}
	return nil, fmt.Errorf("exhausted all attempts to get pubkey: %w", err)
}
func (sc *RetrySignerClient) GetHeight(ctx context.Context, quorumHash crypto.QuorumHash) (int64, error) {
	return 0, fmt.Errorf("getHeight should not be called on asigner client %s", quorumHash.String())
}

func (sc *RetrySignerClient) SignVote(
	ctx context.Context, chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash,
	vote *tmproto.Vote, stateID types.StateID, logger log.Logger) error {
	var err error
	for i := 0; i < sc.retries || sc.retries == 0; i++ {
		err = sc.next.SignVote(ctx, chainID, quorumType, quorumHash, vote, stateID, nil)
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

func (sc *RetrySignerClient) SignProposal(
	ctx context.Context, chainID string, quorumType btcjson.LLMQType, quorumHash crypto.QuorumHash, proposal *tmproto.Proposal,
) (tmbytes.HexBytes, error) {
	var signID []byte
	var err error
	for i := 0; i < sc.retries || sc.retries == 0; i++ {
		signID, err = sc.next.SignProposal(ctx, chainID, quorumType, quorumHash, proposal)
		if err == nil {
			return signID, nil
		}
		// If remote signer errors, we don't retry.
		if _, ok := err.(*RemoteSignerError); ok {
			return nil, err
		}
		time.Sleep(sc.timeout)
	}
	return signID, fmt.Errorf("exhausted all attempts to sign proposal: %w", err)
}

func (sc *RetrySignerClient) UpdatePrivateKey(
	ctx context.Context, privateKey crypto.PrivKey, quorumHash crypto.QuorumHash, thresholdPublicKey crypto.PubKey, height int64,
) {

}

func (sc *RetrySignerClient) GetPrivateKey(ctx context.Context, quorumHash crypto.QuorumHash) (crypto.PrivKey, error) {
	return nil, nil
}
