package lite

// Provider provides information for the lite client to sync validators.
// Examples: MemProvider, files.Provider, client.Provider, CacheProvider.
type Provider interface {

	// LatestFullCommit returns the latest commit with minHeight <= height <=
	// maxHeight.
	// If maxHeight is zero, returns the latest where minHeight <= height.
	LatestFullCommit(chainID string, minHeight, maxHeight int64) (FullCommit, error)
}

// A provider that can also persist new information.
// Examples: MemProvider, files.Provider, CacheProvider.
type PersistentProvider interface {
	Provider

	// SaveFullCommit saves a FullCommit (without verification).
	SaveFullCommit(fc FullCommit) error
}
