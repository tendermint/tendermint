package http

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"strings"
	"time"

	"github.com/tendermint/tendermint/light/provider"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	"github.com/tendermint/tendermint/rpc/coretypes"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
	"github.com/tendermint/tendermint/types"
)

var defaultOptions = Options{
	MaxRetryAttempts:    5,
	Timeout:             5 * time.Second,
	NoBlockThreshold:    5,
	NoResponseThreshold: 5,
}

// http provider uses an RPC client to obtain the necessary information.
type http struct {
	chainID string
	client  rpcclient.RemoteClient

	// httt provider heuristics

	// The provider tracks the amount of times that the
	// client doesn't respond. If this exceeds the threshold
	// then the provider will return an unreliable provider error
	noResponseThreshold uint16
	noResponseCount     uint16

	// The provider tracks the amount of time the client
	// doesn't have a block. If this exceeds the threshold
	// then the provider will return an unreliable provider error
	noBlockThreshold uint16
	noBlockCount     uint16

	// In a single request, the provider attempts multiple times
	// with exponential backoff to reach the client. If this
	// exceeds the maxRetry attempts, this result in a ErrNoResponse
	maxRetryAttempts uint16
}

type Options struct {
	// 0 means no retries
	MaxRetryAttempts uint16
	// 0 means no timeout.
	Timeout time.Duration
	// The amount of requests that a client doesn't have the block
	// for before the provider deems the client unreliable
	NoBlockThreshold uint16
	// The amount of requests that a client doesn't respond to
	// before the provider deems the client unreliable
	NoResponseThreshold uint16
}

// New creates a HTTP provider, which is using the rpchttp.HTTP client under
// the hood. If no scheme is provided in the remote URL, http will be used by
// default. The 5s timeout is used for all requests.
func New(chainID, remote string) (provider.Provider, error) {
	return NewWithOptions(chainID, remote, defaultOptions)
}

// NewWithOptions is an extension to creating a new http provider that allows the addition
// of a specified timeout and maxRetryAttempts
func NewWithOptions(chainID, remote string, options Options) (provider.Provider, error) {
	// Ensure URL scheme is set (default HTTP) when not provided.
	if !strings.Contains(remote, "://") {
		remote = "http://" + remote
	}

	httpClient, err := rpchttp.NewWithTimeout(remote, options.Timeout)
	if err != nil {
		return nil, err
	}

	return NewWithClientAndOptions(chainID, httpClient, options), nil
}

func NewWithClient(chainID string, client rpcclient.RemoteClient) provider.Provider {
	return NewWithClientAndOptions(chainID, client, defaultOptions)
}

// NewWithClient allows you to provide a custom client.
func NewWithClientAndOptions(chainID string, client rpcclient.RemoteClient, options Options) provider.Provider {
	return &http{
		client:              client,
		chainID:             chainID,
		maxRetryAttempts:    options.MaxRetryAttempts,
		noResponseThreshold: options.NoResponseThreshold,
		noBlockThreshold:    options.NoBlockThreshold,
	}
}

// Identifies the provider with an IP in string format
func (p *http) ID() string {
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

	if height != 0 && sh.Height != height {
		return nil, provider.ErrBadLightBlock{
			Reason: fmt.Errorf("height %d responded doesn't match height %d requested", sh.Height, height),
		}
	}

	if sh.Header == nil {
		return nil, provider.ErrBadLightBlock{
			Reason: errors.New("returned header is nil unexpectedly"),
		}
	}

	vs, err := p.validatorSet(ctx, &sh.Height)
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
	// Since the malicious node could report a massive number of pages, making us
	// spend a considerable time iterating, we restrict the number of pages here.
	// => 10000 validators max
	const maxPages = 100

	var (
		perPage = 100
		vals    = []*types.Validator{}
		page    = 1
		total   = -1
	)

	for len(vals) != total && page <= maxPages {
		// create another for loop to control retries. If p.maxRetryAttempts
		// is negative we will keep repeating.
		attempt := uint16(0)
		for {
			res, err := p.client.Validators(ctx, height, &page, &perPage)
			if err == nil {
				if len(res.Validators) == 0 {
					return nil, provider.ErrBadLightBlock{
						Reason: fmt.Errorf("validator set is empty (height: %d, page: %d, per_page: %d)",
							height, page, perPage),
					}
				}
				if res.Total <= 0 {
					return nil, provider.ErrBadLightBlock{
						Reason: fmt.Errorf("total number of vals is <= 0: %d (height: %d, page: %d, per_page: %d)",
							res.Total, height, page, perPage),
					}
				}
			} else {
				switch e := err.(type) {

				case *url.Error:
					if e.Timeout() {
						// if we have exceeded retry attempts then return a no response error
						if attempt == p.maxRetryAttempts {
							return nil, p.noResponse()
						}
						attempt++
						// request timed out: we wait and try again with exponential backoff
						time.Sleep(backoffTimeout(attempt))
						continue
					}
					return nil, provider.ErrBadLightBlock{Reason: e}

				case *rpctypes.RPCError:
					// process the rpc error and return the corresponding error to the light client
					return nil, p.parseRPCError(e)

				default:
					// check if the error stems from the context
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						return nil, err
					}

					// If we don't know the error then by default we return an unreliable provider error and
					// terminate the connection with the peer.
					return nil, provider.ErrUnreliableProvider{Reason: e}
				}
			}
			// update the total and increment the page index so we can fetch the
			// next page of validators if need be
			total = res.Total
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
	// create a for loop to control retries. If p.maxRetryAttempts
	// is negative we will keep repeating.
	for attempt := uint16(0); attempt != p.maxRetryAttempts+1; attempt++ {
		commit, err := p.client.Commit(ctx, height)
		switch e := err.(type) {
		case nil: // success!!
			return &commit.SignedHeader, nil

		case *url.Error:
			// check if the request timed out
			if e.Timeout() {
				// we wait and try again with exponential backoff
				time.Sleep(backoffTimeout(attempt))
				continue
			}

			// check if the connection was refused or dropped
			if strings.Contains(e.Error(), "connection refused") {
				return nil, provider.ErrConnectionClosed
			}

			// else, as a catch all, we return the error as a bad light block response
			return nil, provider.ErrBadLightBlock{Reason: e}

		case *rpctypes.RPCError:
			// process the rpc error and return the corresponding error to the light client
			return nil, p.parseRPCError(e)

		default:
			// check if the error stems from the context
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}

			// If we don't know the error then by default we return an unreliable provider error and
			// terminate the connection with the peer.
			return nil, provider.ErrUnreliableProvider{Reason: e}
		}
	}
	return nil, p.noResponse()
}

func (p *http) noResponse() error {
	p.noResponseCount++
	if p.noResponseCount > p.noResponseThreshold {
		return provider.ErrUnreliableProvider{
			Reason: fmt.Errorf("failed to respond after %d attempts", p.noResponseCount),
		}
	}
	return provider.ErrNoResponse
}

func (p *http) noBlock(e error) error {
	p.noBlockCount++
	if p.noBlockCount > p.noBlockThreshold {
		return provider.ErrUnreliableProvider{
			Reason: fmt.Errorf("failed to provide a block after %d attempts", p.noBlockCount),
		}
	}
	return e
}

// parseRPCError process the error and return the corresponding error to the light clent
// NOTE: When an error is sent over the wire it gets "flattened" hence we are unable to use error
// checking functions like errors.Is() to unwrap the error.
func (p *http) parseRPCError(e *rpctypes.RPCError) error {
	switch {
	// 1) check if the error indicates that the peer doesn't have the block
	case strings.Contains(e.Data, coretypes.ErrHeightNotAvailable.Error()):
		return p.noBlock(provider.ErrLightBlockNotFound)

	// 2) check if the height requested is too high
	case strings.Contains(e.Data, coretypes.ErrHeightExceedsChainHead.Error()):
		return p.noBlock(provider.ErrHeightTooHigh)

	// 3) check if the provider closed the connection
	case strings.Contains(e.Data, "connection refused"):
		return provider.ErrConnectionClosed

	// 4) else return a generic error
	default:
		return provider.ErrBadLightBlock{Reason: e}
	}
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
