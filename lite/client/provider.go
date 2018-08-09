/*
Package client defines a provider that uses a rpcclient
to get information, which is used to get new headers
and validators directly from a Tendermint client.
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

// NewProvider implements Provider (but not PersistentProvider).
func NewProvider(chainID string, client SignStatusClient) lite.Provider {
	return &provider{
		logger:  log.NewNopLogger(),
		chainID: chainID,
		client:  client,
	}
}

// NewHTTPProvider can connect to a tendermint json-rpc endpoint
// at the given url, and uses that as a read-only provider.
func NewHTTPProvider(chainID, remote string) lite.Provider {
	return NewProvider(chainID, rpcclient.NewHTTP(remote, "/websocket"))
}

// Implements Provider.
func (p *provider) SetLogger(logger log.Logger) {
	logger = logger.With("module", "lite/client")
	p.logger = logger
}

// StatusClient returns the internal client as a StatusClient
func (p *provider) StatusClient() rpcclient.StatusClient {
	return p.client
}

// LatestFullCommit implements Provider.
func (p *provider) LatestFullCommit(chainID string, minHeight, maxHeight int64) (fc lite.FullCommit, err error) {
	if chainID != p.chainID {
		err = fmt.Errorf("expected chainID %s, got %s", p.chainID, chainID)
		return
	}
	if maxHeight != 0 && maxHeight < minHeight {
		err = fmt.Errorf("need maxHeight == 0 or minHeight <= maxHeight, got min %v and max %v",
			minHeight, maxHeight)
		return
	}
	commit, err := p.fetchLatestCommit(minHeight, maxHeight)
	if err != nil {
		return
	}
	fc, err = p.fillFullCommit(commit.SignedHeader)
	return
}

// fetchLatestCommit fetches the latest commit from the client.
func (p *provider) fetchLatestCommit(minHeight int64, maxHeight int64) (*ctypes.ResultCommit, error) {
	status, err := p.client.Status()
	if err != nil {
		return nil, err
	}
	if status.SyncInfo.LatestBlockHeight < minHeight {
		err = fmt.Errorf("provider is at %v but require minHeight=%v",
			status.SyncInfo.LatestBlockHeight, minHeight)
		return nil, err
	}
	if maxHeight == 0 {
		maxHeight = status.SyncInfo.LatestBlockHeight
	} else if status.SyncInfo.LatestBlockHeight < maxHeight {
		maxHeight = status.SyncInfo.LatestBlockHeight
	}
	return p.client.Commit(&maxHeight)
}

// Implements Provider.
func (p *provider) ValidatorSet(chainID string, height int64) (valset *types.ValidatorSet, err error) {
	return p.getValidatorSet(chainID, height)
}

func (p *provider) getValidatorSet(chainID string, height int64) (valset *types.ValidatorSet, err error) {
	if chainID != p.chainID {
		err = fmt.Errorf("expected chainID %s, got %s", p.chainID, chainID)
		return
	}
	if height < 1 {
		err = fmt.Errorf("expected height >= 1, got height %v", height)
		return
	}
	res, err := p.client.Validators(&height)
	if err != nil {
		// TODO pass through other types of errors.
		return nil, lerr.ErrUnknownValidators(chainID, height)
	}
	valset = types.NewValidatorSet(res.Validators)
	return
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
