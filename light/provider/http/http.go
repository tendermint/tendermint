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
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpctypes "github.com/tendermint/tendermint/rpc/jsonrpc/types"
	"github.com/tendermint/tendermint/types"
)

var defaultOptions = Options{
	MaxRetryAttempts: 5,
	Timeout:          3 * time.Second,
}

// http provider uses an RPC client to obtain the necessary information.
type http struct {
	chainID string
	client  rpcclient.RemoteClient

	maxRetryAttempts int
}

type Options struct {
	// -1 means no limit
	MaxRetryAttempts int
	// 0 means no timeout.
	Timeout time.Duration
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

	httpClient, err := rpchttp.NewWithTimeout(remote, "/websocket", options.Timeout)
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
		client:           client,
		chainID:          chainID,
		maxRetryAttempts: options.MaxRetryAttempts,
	}
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
		for attempt := 0; attempt != p.maxRetryAttempts+1; attempt++ {
			res, err := p.client.Validators(ctx, height, &page, &perPage)
			switch e := err.(type) {
			case nil: // success!! Now we validate the response
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

			case *url.Error:
				if e.Timeout() {
					// request timed out: we wait and try again with exponential backoff
					time.Sleep(backoffTimeout(uint16(attempt)))
					continue
				}
				return nil, provider.ErrBadLightBlock{Reason: e}

			case *rpctypes.RPCError:
				// check if the error indicates that the peer doesn't have the block
				if strings.Contains(e.Data, ctypes.ErrHeightNotAvailable.Error()) ||
					strings.Contains(e.Data, ctypes.ErrHeightExceedsChainHead.Error()) {
					return nil, provider.ErrLightBlockNotFound
				}
				return nil, provider.ErrBadLightBlock{Reason: e}

			default:
				// If we don't know the error then by default we return a bad light block error and
				// terminate the connection with the peer.
				return nil, provider.ErrBadLightBlock{Reason: e}
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
	for attempt := 0; attempt != p.maxRetryAttempts+1; attempt++ {
		commit, err := p.client.Commit(ctx, height)
		switch e := err.(type) {
		case nil: // success!!
			return &commit.SignedHeader, nil

		case *url.Error:
			if e.Timeout() {
				// we wait and try again with exponential backoff
				time.Sleep(backoffTimeout(uint16(attempt)))
				continue
			}
			return nil, provider.ErrBadLightBlock{Reason: e}

		case *rpctypes.RPCError:
			// Check if we got something other than internal error. This shouldn't happen unless the RPC module
			// or light client has been tampered with. If we do get this error, stop the connection with the
			// peer and return an error
			if e.Code != -32603 {
				return nil, provider.ErrBadLightBlock{Reason: errors.New(e.Data)}
			}

			// check if the error indicates that the peer doesn't have the block
			if strings.Contains(err.Error(), ctypes.ErrHeightNotAvailable.Error()) ||
				strings.Contains(err.Error(), ctypes.ErrHeightExceedsChainHead.Error()) {
				return nil, provider.ErrLightBlockNotFound
			}

		default:
			// If we don't know the error then by default we return a bad light block error and
			// terminate the connection with the peer.
			return nil, provider.ErrBadLightBlock{Reason: e}
		}
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
