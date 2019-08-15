package lite

import (
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

// Provider provides information for the lite client to sync validators.
type Provider interface {
	// LatestFullCommit returns the latest commit with minHeight <= height <=
	// maxHeight.
	// If maxHeight is zero, returns the latest where minHeight <= height.
	// If maxHeight is greater than the latest height, the latter one should be returned.
	LatestFullCommit(chainID string, minHeight, maxHeight int64) (FullCommit, error)

	// ValidatorSet returns the valset that corresponds to chainID and height.
	// height must be >= 1.
	ValidatorSet(chainID string, height int64) (*types.ValidatorSet, error)

	// SetLogger sets a logger.
	SetLogger(logger log.Logger)
}

// PersistentProvider is a provider that can also persist new information.
type PersistentProvider interface {
	Provider

	// SaveFullCommit saves a FullCommit (without verification).
	SaveFullCommit(fc FullCommit) error
}

// UpdatingProvider is a provider that can update itself w/ more recent commit
// data.
type UpdatingProvider interface {
	Provider

	// Update internal information by fetching information somehow.
	// UpdateToHeight will block until the request is complete, or returns an
	// error if the request cannot complete. Generally, one must call
	// UpdateToHeight(h) before LatestFullCommit(_,h,h) will return this height.
	//
	// NOTE: Behavior with concurrent requests is undefined. To make concurrent
	// calls safe, look at the struct `ConcurrentUpdatingProvider`.
	UpdateToHeight(chainID string, height int64) error
}
