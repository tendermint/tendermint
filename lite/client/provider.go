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
		return
	}
	if maxHeight == 0 {
		maxHeight = status.SyncInfo.LatestBlockHeight
	} else if status.SyncInfo.LatestBlockHeight < maxHeight {
		maxHeight = status.SyncInfo.LatestBlockHeight
	}
	return p.client.Commit(&maxHeight)
}

// Get the valset that corresponds to signedHeader and return.
// This checks the cryptographic signatures of fc.Commit against fc.Validators.
func (p *provider) FillFullCommit(signedHeader *types.SignedHeader) (fc lite.FullCommit, err error) {
	return p.fillFullCommit(signedHeader)
}

func (p *provider) fillFullCommit(signedHeader *types.SignedHeader) (fc lite.FullCommit, err error) {
	fc.SignedHeader = signedHeader

	// Get the validators.
	height := new(int)
	*height = signedHeader.Header.Height
	vals, err := p.client.Validators(height)
	if err != nil {
		return fc, err
	}

	// Check valset hash against signedHeader validators hash.
	vset := types.NewValidatorSet(vals.Validators)
	if !bytes.Equal(vset.Hash(), signedHeader.Header.ValidatorsHash) {
		return fc, errors.New("wrong validators received from client")
	}
	fc.Validators = vset

	// Get the next validators.
	nextHeight := new(int)
	*nextHeight = signedHeader.Header.Height + 1
	nextVals, err := p.client.Validators(nextHeight)
	if err != nil {
		return fc, err
	}

	// Check next valset hash against signedHeader next validators hash.
	vset := types.NewValidatorSet(nextVals.Validators)
	if !bytes.Equal(vset.Hash(), signedHeader.Header.NextValidatorsHash) {
		return fc, errors.New("wrong validators received from client")
	}
	fc.NextValidators = vset

	// Sanity check fc.
	// This checks the cryptographic signatures of fc.Commit against fc.Validators.
	if err := fc.ValidateBasic(signedHeader.Header.ChainID); err != nil {
		return nil, err
	}

	return fc, nil
}
