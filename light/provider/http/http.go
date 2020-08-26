package http

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/tendermint/tendermint/light/provider"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	"github.com/tendermint/tendermint/types"
)

// This is very brittle, see: https://github.com/tendermint/tendermint/issues/4740
var (
	regexpMissingHeight = regexp.MustCompile(`height \d+ (must be less than or equal to|is not available)`)
	maxRetryAttempts = 10
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

	for attempt = 0; attempt < maxRetryAttempts; i++ {
		commit, err := p.client.Commit(h)
		if err != nil {
			// TODO: standartise errors on the RPC side
			if regexpMissingHeight.MatchString(err.Error()) {
				return nil, provider.ErrSignedHeaderNotFound
			}
			time.Sleep(backoffTimeout(attempt))
			continue
		}
		if commit.Header == nil {
			return nil, provider.ErrBadSignedHeader{Reason: errors.New("header is nil")}
		}
		
		if err = commit.SignedHeader.ValidateBasic(p.ChainID); err != nil {
			return nil, provider.ErrBadSignedHeader{Reason: err}
		}
		
		return &commit.SignedHeader, nil
	}

	return nil, provider.ErrNoResponse{}
}

// ValidatorSet fetches a ValidatorSet at the given height. Multiple HTTP
// requests might be required if the validator set size is over 100.
func (p *http) ValidatorSet(height int64) (*types.ValidatorSet, error) {
	h, err := validateHeight(height)
	if err != nil {
		return nil, err
	}

	maxPerPage := 100
	res, err := p.client.Validators(h, nil, &maxPerPage)
	if err != nil {
		// TODO: standartise errors on the RPC side
		if regexpMissingHeight.MatchString(err.Error()) {
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
		res, err = p.client.Validators(h, &page, &maxPerPage)
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

// exponential backoff (with jitter)
// 0.5s -> 2s -> 4.5s -> 8s -> 12.5 with 1s variation
func backoffTimeout(attempt uint16) time.Duration {
	// nolint:gosec // G404: Use of weak random number generator
	return time.Duration(500*attempt*attempt)*time.Millisecond + time.Duration(rand.Intn(1000))*time.Millisecond
}
