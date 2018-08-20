package lite

import (
	log "github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

// Provider provides information for the lite client to sync validators.
// Examples: MemProvider, files.Provider, client.Provider, CacheProvider.
type Provider interface {

	// LatestFullCommit returns the latest commit with minHeight <= height <=
	// maxHeight.
	// If maxHeight is zero, returns the latest where minHeight <= height.
	LatestFullCommit(chainID string, minHeight, maxHeight int64) (FullCommit, error)

	// Get the valset that corresponds to chainID and height and return.
	// Height must be >= 1.
	ValidatorSet(chainID string, height int64) (*types.ValidatorSet, error)

	// Set a logger.
	SetLogger(logger log.Logger)
}

// A provider that can also persist new information.
// Examples: MemProvider, files.Provider, CacheProvider.
type PersistentProvider interface {
	Provider

	// SaveFullCommit saves a FullCommit (without verification).
	SaveFullCommit(fc FullCommit) error
}
