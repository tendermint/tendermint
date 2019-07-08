/*
Package client defines a provider that uses an RPC client (or SignStatusClient
more generally) to get information like new headers and validators directly
from a Tendermint node.

Use either NewProvider or NewHTTPProvider to construct one.
*/
package client

import (
	"fmt"

	log "github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/lite"
	lerr "github.com/tendermint/tendermint/lite/errors"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

// SignStatusClient combines a SignClient and StatusClient.
type SignStatusClient interface {
	rpcclient.SignClient
	rpcclient.StatusClient
}

type provider struct {
	logger  log.Logger
	chainID string
	client  SignStatusClient
}

// NewProvider creates a lite.Provider using the given chain ID and
// SignStatusClient.
func NewProvider(chainID string, client SignStatusClient) lite.Provider {
	return &provider{
		logger:  log.NewNopLogger(),
		chainID: chainID,
		client:  client,
	}
}

// NewHTTPProvider creates a lite.Provider, which is using the rpcclient.HTTP
// client under the hood.
func NewHTTPProvider(chainID, remote string) lite.Provider {
	return NewProvider(chainID, rpcclient.NewHTTP(remote, "/websocket"))
}

// SetLogger implements lite.Provider.
func (p *provider) SetLogger(logger log.Logger) {
	logger = logger.With("module", "lite/client")
	p.logger = logger
}

// LatestFullCommit implements lite.Provider.
func (p *provider) LatestFullCommit(chainID string, minHeight, maxHeight int64) (fc lite.FullCommit, err error) {
	if chainID != p.chainID {
		return fc, fmt.Errorf("expected chainID %s, got %s", p.chainID, chainID)
	}

	if maxHeight != 0 && maxHeight < minHeight {
		return fc, fmt.Errorf("need maxHeight == 0 or minHeight <= maxHeight, got min %v and max %v",
			minHeight, maxHeight)
	}

	commit, err := p.fetchLatestCommit(minHeight, maxHeight)
	if err != nil {
		return fc, err
	}

	return p.fillFullCommit(commit.SignedHeader)
}

func (p *provider) fetchLatestCommit(minHeight int64, maxHeight int64) (*ctypes.ResultCommit, error) {
	status, err := p.client.Status()
	if err != nil {
		return nil, err
	}

	if status.SyncInfo.LatestBlockHeight < minHeight {
		return nil, fmt.Errorf("provider is at %d but require minHeight=%d",
			status.SyncInfo.LatestBlockHeight, minHeight)
	}

	if maxHeight == 0 {
		maxHeight = status.SyncInfo.LatestBlockHeight
	} else if status.SyncInfo.LatestBlockHeight < maxHeight {
		maxHeight = status.SyncInfo.LatestBlockHeight
	}

	return p.client.Commit(&maxHeight)
}

// This does no validation.
func (p *provider) fillFullCommit(signedHeader types.SignedHeader) (fc lite.FullCommit, err error) {
	// Get the validators.
	valset, err := p.getValidatorSet(signedHeader.ChainID, signedHeader.Height)
	if err != nil {
		return lite.FullCommit{}, err
	}

	// Get the next validators.
	nextValset, err := p.getValidatorSet(signedHeader.ChainID, signedHeader.Height+1)
	if err != nil {
		return lite.FullCommit{}, err
	}

	return lite.NewFullCommit(signedHeader, valset, nextValset), nil
}

// ValidatorSet implements lite.Provider.
func (p *provider) ValidatorSet(chainID string, height int64) (valset *types.ValidatorSet, err error) {
	return p.getValidatorSet(chainID, height)
}

func (p *provider) getValidatorSet(chainID string, height int64) (valset *types.ValidatorSet, err error) {
	if chainID != p.chainID {
		return nil, fmt.Errorf("expected chainID %s, got %s", p.chainID, chainID)
	}

	if height < 1 {
		return nil, fmt.Errorf("expected height >= 1, got height %d", height)
	}

	res, err := p.client.Validators(&height)
	if err != nil {
		// TODO pass through other types of errors.
		return nil, lerr.ErrUnknownValidators(chainID, height)
	}

	return types.NewValidatorSet(res.Validators), nil
}
