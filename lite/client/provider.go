/*
Package client defines a provider that uses a rpcclient
to get information, which is used to get new headers
and validators directly from a Tendermint client.
*/
package client

import (
	"bytes"
	"errors"
	"fmt"

	rpcclient "github.com/tendermint/tendermint/rpc/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"

	"github.com/tendermint/tendermint/lite"
	lerr "github.com/tendermint/tendermint/lite/errors"
)

// SignStatusClient combines a SignClient and StatusClient.
type SignStatusClient interface {
	rpcclient.SignClient
	rpcclient.StatusClient
}

type provider struct {
	chainID string
	client  SignStatusClient
}

// NewProvider implements Provider (but not PersistentProvider).
func NewProvider(chainID string, client SignStatusClient) lite.Provider {
	return &provider{chainID: chainID, client: client}
}

// NewHTTPProvider can connect to a tendermint json-rpc endpoint
// at the given url, and uses that as a read-only provider.
func NewHTTPProvider(remote string) lite.Provider {
	return &provider{
		client: rpcclient.NewHTTP(remote, "/websocket"),
	}
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
		err = fmt.Errorf("need maxHeight == 0 or minHeight <= maxHeight, got %v and %v",
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
		err = fmt.Errorf("expected height >= 1, got %v", height)
		return
	}
	heightPtr := new(int64)
	*heightPtr = height
	res, err := p.client.Validators(heightPtr)
	if err != nil {
		// TODO pass through other types of errors.
		return nil, lerr.ErrMissingValidators(chainID, height)
	}
	valset = types.NewValidatorSet(res.Validators)
	valset.TotalVotingPower() // to test deep equality.
	return
}

// This checks the cryptographic signatures of fc.Commit against fc.Validators.
func (p *provider) fillFullCommit(signedHeader types.SignedHeader) (fc lite.FullCommit, err error) {
	fc.SignedHeader = signedHeader

	// Get the validators.
	valset, err := p.getValidatorSet(signedHeader.ChainID, signedHeader.Height)
	if err != nil {
		return lite.FullCommit{}, err
	}

	// Check valset hash against signedHeader validators hash.
	if !bytes.Equal(valset.Hash(), signedHeader.ValidatorsHash) {
		return lite.FullCommit{}, errors.New("wrong validators received from client")
	}
	fc.Validators = valset

	// Get the next validators.
	nvalset, err := p.getValidatorSet(signedHeader.ChainID, signedHeader.Height+1)
	if err != nil {
		return lite.FullCommit{}, err
	}

	// Check next valset hash against signedHeader next validators hash.
	if !bytes.Equal(nvalset.Hash(), signedHeader.NextValidatorsHash) {
		return lite.FullCommit{}, errors.New("wrong validators received from client")
	}
	fc.NextValidators = nvalset

	// Sanity check fc.
	// This checks the cryptographic signatures of fc.Commit against fc.Validators.
	if err := fc.ValidateBasic(signedHeader.ChainID); err != nil {
		return lite.FullCommit{}, err
	}

	return fc, nil
}
