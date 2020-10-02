package http

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/tendermint/tendermint/light/provider"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	"github.com/tendermint/tendermint/types"
)

// This is very brittle, see: https://github.com/tendermint/tendermint/issues/4740
var (
	regexpMissingHeight = regexp.MustCompile(`height \d+ (must be less than or equal to|is not available)`)
	maxRetryAttempts    = 10
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

// LightBlock fetches a LightBlock at the given height and checks the
// chainID matches.
func (p *http) LightBlock(ctx context.Context, height int64) (*types.LightBlock, error) {
	h, err := validateHeight(height)
	if err != nil {
		return nil, provider.ErrBadLightBlock{Reason: err}
	}

	sh, err := p.signedHeader(ctx, h)
	if err != nil {
		return nil, err
	}

	vs, err := p.validatorSet(ctx, h)
	if err != nil {
		return nil, err
	}

	lb := &types.LightBlock{
		SignedHeader: sh,
		ValidatorSet: vs,
	}

	err = lb.ValidateBasic(p.chainID)
	if err != nil {
		return nil, provider.ErrBadLightBlock{Reason: err}
	}

	return lb, nil
}

// ReportEvidence calls `/broadcast_evidence` endpoint.
func (p *http) ReportEvidence(ctx context.Context, ev types.Evidence) error {
	_, err := p.client.BroadcastEvidence(ctx, ev)
	return err
}

func (p *http) validatorSet(ctx context.Context, height *int64) (*types.ValidatorSet, error) {
	var (
		maxPerPage = 100
		vals       = []*types.Validator{}
		page       = 1
	)

	for len(vals)%maxPerPage == 0 {
		for attempt := 1; attempt <= maxRetryAttempts; attempt++ {
			res, err := p.client.Validators(ctx, height, &page, &maxPerPage)
			if err != nil {
				// TODO: standardize errors on the RPC side
				if regexpMissingHeight.MatchString(err.Error()) {
					return nil, provider.ErrLightBlockNotFound
				}
				// if we have exceeded retry attempts then return no response error
				if attempt == maxRetryAttempts {
					return nil, provider.ErrNoResponse
				}
				// else we wait and try again with exponential backoff
				time.Sleep(backoffTimeout(uint16(attempt)))
				continue
			}
			if len(res.Validators) == 0 { // no more validators left
				valSet, err := types.ValidatorSetFromExistingValidators(vals)
				if err != nil {
					return nil, provider.ErrBadLightBlock{Reason: err}
				}
				return valSet, nil
			}
			vals = append(vals, res.Validators...)
			page++
			break
		}
	}
	valSet, err := types.ValidatorSetFromExistingValidators(vals)
	if err != nil {
		return nil, provider.ErrBadLightBlock{Reason: err}
	}
	return valSet, nil
}

func (p *http) signedHeader(ctx context.Context, height *int64) (*types.SignedHeader, error) {
	for attempt := 1; attempt <= maxRetryAttempts; attempt++ {
		commit, err := p.client.Commit(ctx, height)
		if err != nil {
			// TODO: standardize errors on the RPC side
			if regexpMissingHeight.MatchString(err.Error()) {
				return nil, provider.ErrLightBlockNotFound
			}
			// we wait and try again with exponential backoff
			time.Sleep(backoffTimeout(uint16(attempt)))
			continue
		}
		return &commit.SignedHeader, nil
	}
	return nil, provider.ErrNoResponse
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
