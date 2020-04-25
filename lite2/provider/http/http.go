package http

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/tendermint/tendermint/lite2/provider"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	"github.com/tendermint/tendermint/types"
)

// This is very brittle, see: https://github.com/tendermint/tendermint/issues/4740
var regexpMissingHeight = regexp.MustCompile(`height \d+ (must be less than or equal to|is not available)`)

// SignStatusClient combines a SignClient and StatusClient.
type SignStatusClient interface {
	rpcclient.SignClient
	rpcclient.StatusClient
	// Remote returns the remote network address in a string form.
	Remote() string
}

// http provider uses an RPC client (or SignStatusClient more generally) to
// obtain the necessary information.
type http struct {
	SignStatusClient // embed so interface can be converted to SignStatusClient for tests
	chainID          string
}

// New creates a HTTP provider, which is using the rpchttp.HTTP client under the
// hood. If no scheme is provided in the remote URL, http will be used by default.
func New(chainID, remote string) (provider.Provider, error) {
	// ensure URL scheme is set (default HTTP) when not provided
	if !strings.Contains(remote, "://") {
		remote = "http://" + remote
	}

	httpClient, err := rpchttp.New(remote, "/websocket")
	if err != nil {
		return nil, err
	}

	return NewWithClient(chainID, httpClient), nil
}

// NewWithClient allows you to provide custom SignStatusClient.
func NewWithClient(chainID string, client SignStatusClient) provider.Provider {
	return &http{
		SignStatusClient: client,
		chainID:          chainID,
	}
}

// ChainID returns a chainID this provider was configured with.
func (p *http) ChainID() string {
	return p.chainID
}

func (p *http) String() string {
	return fmt.Sprintf("http{%s}", p.Remote())
}

// SignedHeader fetches a SignedHeader at the given height and checks the
// chainID matches.
func (p *http) SignedHeader(height int64) (*types.SignedHeader, error) {
	h, err := validateHeight(height)
	if err != nil {
		return nil, err
	}

	commit, err := p.SignStatusClient.Commit(h)
	if err != nil {
		// TODO: standartise errors on the RPC side
		if regexpMissingHeight.MatchString(err.Error()) {
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
	res, err := p.SignStatusClient.Validators(h, 0, maxPerPage)
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
		res, err = p.SignStatusClient.Validators(h, page, maxPerPage)
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
