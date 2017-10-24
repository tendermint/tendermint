/*
Package client defines a provider that uses a rpcclient
to get information, which is used to get new headers
and validators directly from a node.
*/
package client

import (
	"bytes"

	rpcclient "github.com/tendermint/tendermint/rpc/client"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"

	"github.com/tendermint/tendermint/certifiers"
	certerr "github.com/tendermint/tendermint/certifiers/errors"
)

type SignStatusClient interface {
	rpcclient.SignClient
	rpcclient.StatusClient
}

type provider struct {
	node       SignStatusClient
	lastHeight int
}

// NewProvider can wrap any rpcclient to expose it as
// a read-only provider.
func NewProvider(node SignStatusClient) certifiers.Provider {
	return &provider{node: node}
}

// NewProvider can connects to a tendermint json-rpc endpoint
// at the given url, and uses that as a read-only provider.
func NewHTTPProvider(remote string) certifiers.Provider {
	return &provider{
		node: rpcclient.NewHTTP(remote, "/websocket"),
	}
}

// StoreCommit is a noop, as clients can only read from the chain...
func (p *provider) StoreCommit(_ certifiers.FullCommit) error { return nil }

// GetHash gets the most recent validator and sees if it matches
//
// TODO: improve when the rpc interface supports more functionality
func (p *provider) GetByHash(hash []byte) (certifiers.FullCommit, error) {
	var fc certifiers.FullCommit
	vals, err := p.node.Validators(nil)
	// if we get no validators, or a different height, return an error
	if err != nil {
		return fc, err
	}
	p.updateHeight(vals.BlockHeight)
	vhash := types.NewValidatorSet(vals.Validators).Hash()
	if !bytes.Equal(hash, vhash) {
		return fc, certerr.ErrCommitNotFound()
	}
	return p.seedFromVals(vals)
}

// GetByHeight gets the validator set by height
func (p *provider) GetByHeight(h int) (fc certifiers.FullCommit, err error) {
	commit, err := p.node.Commit(&h)
	if err != nil {
		return fc, err
	}
	return p.seedFromCommit(commit)
}

func (p *provider) LatestCommit() (fc certifiers.FullCommit, err error) {
	commit, err := p.GetLatestCommit()
	if err != nil {
		return fc, err
	}
	return p.seedFromCommit(commit)
}

// GetLatestCommit should return the most recent commit there is,
// which handles queries for future heights as per the semantics
// of GetByHeight.
func (p *provider) GetLatestCommit() (*ctypes.ResultCommit, error) {
	status, err := p.node.Status()
	if err != nil {
		return nil, err
	}
	return p.node.Commit(&status.LatestBlockHeight)
}

func (p *provider) seedFromVals(vals *ctypes.ResultValidators) (certifiers.FullCommit, error) {
	// now get the commits and build a full commit
	commit, err := p.node.Commit(&vals.BlockHeight)
	if err != nil {
		return certifiers.FullCommit{}, err
	}
	fc := certifiers.NewFullCommit(
		certifiers.CommitFromResult(commit),
		types.NewValidatorSet(vals.Validators),
	)
	return fc, nil
}

func (p *provider) seedFromCommit(commit *ctypes.ResultCommit) (fc certifiers.FullCommit, err error) {
	fc.Commit = certifiers.CommitFromResult(commit)

	// now get the proper validators
	vals, err := p.node.Validators(&commit.Header.Height)
	if err != nil {
		return fc, err
	}

	// make sure they match the commit (as we cannot enforce height)
	vset := types.NewValidatorSet(vals.Validators)
	if !bytes.Equal(vset.Hash(), commit.Header.ValidatorsHash) {
		return fc, certerr.ErrValidatorsChanged()
	}

	p.updateHeight(commit.Header.Height)
	fc.Validators = vset
	return fc, nil
}

func (p *provider) updateHeight(h int) {
	if h > p.lastHeight {
		p.lastHeight = h
	}
}
