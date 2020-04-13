package http

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tendermint/tendermint/lite2/provider"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	"github.com/tendermint/tendermint/types"
)

// http provider uses an RPC client to obtain the necessary information.
type http struct {
	chainID string
	client  rpcclient.RemoteClient
}

// New creates a HTTP provider, which is using the rpchttp.HTTP client under
// the hood. If no scheme is provided in the remote URL, http will be used by
// default.
func New(chainID, remote string) (provider.Provider, error) {
	// Ensure URL scheme is set (default HTTP) when not provided.
	if !strings.Contains(remote, "://") {
		remote = "http://" + remote
	}

	httpClient, err := rpchttp.New(remote, "/websocket")
	if err != nil {
		return nil, err
	}

	return NewWithClient(chainID, httpClient), nil
}

// NewWithClient allows you to provide a custom client.
func NewWithClient(chainID string, client rpcclient.RemoteClient) provider.Provider {
	return &http{
		client:  client,
		chainID: chainID,
	}
}

// ChainID returns a chainID this provider was configured with.
func (p *http) ChainID() string {
	return p.chainID
}

func (p *http) String() string {
	return fmt.Sprintf("http{%s}", p.client.Remote())
}

// SignedHeader fetches a SignedHeader at the given height and checks the
// chainID matches.
func (p *http) SignedHeader(height int64) (*types.SignedHeader, error) {
	h, err := validateHeight(height)
	if err != nil {
		return nil, err
	}

	commit, err := p.client.Commit(h)
	if err != nil {
		// TODO: standartise errors on the RPC side
		if strings.Contains(err.Error(), "height must be less than or equal") {
			return nil, provider.ErrSignedHeaderNotFound
		}
		return nil, err
	}

	if commit.Header == nil {
		return nil, errors.New("header is nil")
	}

	// Verify we're still on the same chain.
	if p.chainID != commit.Header.ChainID {
		return nil, fmt.Errorf("expected chainID %s, got %s", p.chainID, commit.Header.ChainID)
	}

	return &commit.SignedHeader, nil
}

// ValidatorSet fetches a ValidatorSet at the given height. Multiple HTTP
// requests might be required if the validator set size is over 100.
func (p *http) ValidatorSet(height int64) (*types.ValidatorSet, error) {
	h, err := validateHeight(height)
	if err != nil {
		return nil, err
	}

	const maxPerPage = 100
	res, err := p.client.Validators(h, 0, maxPerPage)
	if err != nil {
		// TODO: standartise errors on the RPC side
		if strings.Contains(err.Error(), "height must be less than or equal") {
			return nil, provider.ErrValidatorSetNotFound
		}
		return nil, err
	}

	var (
		vals = res.Validators
		page = 1
	)

	// Check if there are more validators.
	for len(res.Validators) == maxPerPage {
		res, err = p.client.Validators(h, page, maxPerPage)
		if err != nil {
			return nil, err
		}
		if len(res.Validators) > 0 {
			vals = append(vals, res.Validators...)
		}
		page++
	}

	return types.NewValidatorSet(vals), nil
}

// ReportEvidence calls `/broadcast_evidence` endpoint.
func (p *http) ReportEvidence(ev types.Evidence) error {
	_, err := p.client.BroadcastEvidence(ev)
	return err
}

func validateHeight(height int64) (*int64, error) {
	if height < 0 {
		return nil, fmt.Errorf("expected height >= 0, got height %d", height)
	}

	h := &height
	if height == 0 {
		h = nil
	}
	return h, nil
}
