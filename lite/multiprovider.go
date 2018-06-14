package lite

import (
	lerr "github.com/tendermint/tendermint/lite/errors"
	"github.com/tendermint/tendermint/types"
)

// multiProvider allows you to place one or more caches in front of a source
// Provider.  It runs through them in order until a match is found.
type multiProvider struct {
	Providers []PersistentProvider
}

// NewMultiProvider returns a new provider which wraps multiple other providers.
func NewMultiProvider(providers ...PersistentProvider) multiProvider {
	return multiProvider{
		Providers: providers,
	}
}

// SaveFullCommit saves on all providers, and aborts on the first error.
func (mc multiProvider) SaveFullCommit(fc FullCommit) (err error) {
	for _, p := range mc.Providers {
		err = p.SaveFullCommit(fc)
		if err != nil {
			return
		}
	}
	return
}

// LatestFullCommit loads the latest from all providers and provides
// the latest FullCommit that satisfies the conditions.
// Returns the first error encountered.
func (mc multiProvider) LatestFullCommit(chainID string, minHeight, maxHeight int64) (fc FullCommit, err error) {
	for _, p := range mc.Providers {
		var fc_ FullCommit
		fc_, err = p.LatestFullCommit(chainID, minHeight, maxHeight)
		if lerr.IsErrCommitNotFound(err) {
			err = nil
			continue
		} else if err != nil {
			return
		}
		if fc == (FullCommit{}) {
			fc = fc_
		} else if fc_.Height() > fc.Height() {
			fc = fc_
		}
		if fc.Height() == maxHeight {
			return
		}
	}
	if fc == (FullCommit{}) {
		err = lerr.ErrCommitNotFound()
		return
	}
	return
}

// ValidatorSet returns validator set at height as provided by the first
// provider which has it, or an error otherwise.
func (mc multiProvider) ValidatorSet(chainID string, height int64) (valset *types.ValidatorSet, err error) {
	for _, p := range mc.Providers {
		valset, err = p.ValidatorSet(chainID, height)
		if err == nil {
			// TODO Log unexpected types of errors.
			return valset, nil
		}
	}
	return nil, lerr.ErrMissingValidators(chainID, height)
}
